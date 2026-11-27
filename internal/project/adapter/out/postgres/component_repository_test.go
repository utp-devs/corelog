package postgres

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"

	"github.com/jhonsferg/corelog/internal/platform/apperror"
	"github.com/jhonsferg/corelog/internal/platform/database/testdb"
	"github.com/jhonsferg/corelog/internal/project/application/port"
	"github.com/jhonsferg/corelog/internal/project/domain"
)

// TestComponentRepositoryLeadCountsAndMove covers the component columns added for leads and assignee policies,
// the issue counts of the list, the rename conflict and moving the issues of a deleted component.
func TestComponentRepositoryLeadCountsAndMove(t *testing.T) {
	db := testdb.New(t)
	ctx := context.Background()
	projects := NewProjectRepository(db)
	catalog := NewCatalogRepository(db)
	leadID := insertUser(t, db)
	orgID := uuid.MustParse(defaultOrganizationID)

	project := newDomainProject(t, orgID, leadID, "CMP", domain.TemplateKanban, domain.VisibilityOpen)
	if err := projects.CreateWithDefaults(ctx, project, port.Member{UserID: leadID, Role: "lead"}); err != nil {
		t.Fatal(err)
	}

	payments, err := domain.NewComponent(project.ID, "Pagos", "cobros")
	if err != nil {
		t.Fatal(err)
	}
	lead := leadID
	if err := payments.Update(domain.ComponentPatch{LeadIDSet: true, LeadID: &lead, Policy: strPtr(domain.AssigneeComponentLead)}, testNow()); err != nil {
		t.Fatal(err)
	}
	if err := catalog.CreateComponent(ctx, payments); err != nil {
		t.Fatal(err)
	}
	billing, err := domain.NewComponent(project.ID, "Facturacion", "")
	if err != nil {
		t.Fatal(err)
	}
	if err := catalog.CreateComponent(ctx, billing); err != nil {
		t.Fatal(err)
	}

	// Duplicate names are a conflict with a stable code.
	dup, _ := domain.NewComponent(project.ID, "Pagos", "")
	if err := catalog.CreateComponent(ctx, dup); apperror.CodeOf(err) != "component.name_taken" {
		t.Fatalf("duplicate component: %v", err)
	}

	issue := insertProjectIssue(t, db, project.ID, leadID, orgID)
	if _, err := db.Exec(`INSERT INTO issue_components (issue_id, component_id) VALUES ($1, $2)`, issue, payments.ID); err != nil {
		t.Fatal(err)
	}

	list, err := catalog.ListComponents(ctx, project.ID)
	if err != nil {
		t.Fatal(err)
	}
	found := map[string]domain.Component{}
	for _, c := range list {
		found[c.Name] = c
	}
	if got := found["Pagos"]; got.LeadID == nil || *got.LeadID != leadID || got.LeadName == "" || got.DefaultAssigneePolicy != domain.AssigneeComponentLead || got.IssueCount != 1 {
		t.Fatalf("Pagos = %+v", got)
	}
	if got := found["Facturacion"]; got.LeadID != nil || got.DefaultAssigneePolicy != domain.AssigneeNone || got.IssueCount != 0 {
		t.Fatalf("Facturacion = %+v", got)
	}

	// Moving issues links them to the target and keeps the ones already there unique.
	if err := catalog.MoveComponentIssues(ctx, payments.ID, billing.ID); err != nil {
		t.Fatal(err)
	}
	moved, err := catalog.FindComponent(ctx, project.ID, billing.ID)
	if err != nil || moved.IssueCount != 1 {
		t.Fatalf("after move: %+v %v", moved, err)
	}

	// Updates persist the lead and the policy; a cleared lead is stored as NULL.
	billing.LeadID = &lead
	billing.DefaultAssigneePolicy = domain.AssigneeProjectLead
	if err := catalog.UpdateComponent(ctx, billing); err != nil {
		t.Fatal(err)
	}
	again, err := catalog.FindComponent(ctx, project.ID, billing.ID)
	if err != nil || again.LeadID == nil || again.DefaultAssigneePolicy != domain.AssigneeProjectLead {
		t.Fatalf("update not stored: %+v %v", again, err)
	}
	billing.LeadID = nil
	if err := catalog.UpdateComponent(ctx, billing); err != nil {
		t.Fatal(err)
	}
	if again, _ = catalog.FindComponent(ctx, project.ID, billing.ID); again.LeadID != nil {
		t.Fatal("cleared lead still stored")
	}

	if err := catalog.DeleteComponent(ctx, project.ID, payments.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := catalog.FindComponent(ctx, project.ID, payments.ID); apperror.CodeOf(err) != "component.not_found" {
		t.Fatalf("deleted component still found: %v", err)
	}
}

func strPtr(s string) *string { return &s }

func testNow() time.Time { return time.Date(2026, 11, 27, 9, 0, 0, 0, time.UTC) }

// insertProjectIssue inserts one live issue of the project with the default catalogs of the organization.
func insertProjectIssue(t *testing.T, db *sqlx.DB, project, reporter, org uuid.UUID) uuid.UUID {
	t.Helper()
	id := uuid.New()
	if _, err := db.Exec(`INSERT INTO issues (id, project_id, summary, description, status, priority, reporter_id, issue_type, key_number, rank, issue_type_id, priority_id)
		VALUES ($1, $2, 'Componente', '', 'open', 'medium', $3, 'task', 1, 'ra',
			(SELECT id FROM issue_types WHERE organization_id = $4 AND legacy_key = 'task'),
			(SELECT id FROM priorities WHERE organization_id = $4 AND legacy_key = 'medium'))`,
		id, project, reporter, org); err != nil {
		t.Fatal(err)
	}
	return id
}
