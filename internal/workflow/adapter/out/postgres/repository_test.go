package postgres_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"

	"github.com/jhonsferg/corelog/internal/platform/apperror"
	"github.com/jhonsferg/corelog/internal/platform/database"
	"github.com/jhonsferg/corelog/internal/platform/database/testdb"
	"github.com/jhonsferg/corelog/internal/workflow/adapter/out/postgres"
	"github.com/jhonsferg/corelog/internal/workflow/application/port"
	"github.com/jhonsferg/corelog/internal/workflow/domain"
)

const defaultOrg = "00000000-0000-4000-8000-000000000002"

func insertProject(t *testing.T, db *sqlx.DB, template string) uuid.UUID {
	t.Helper()
	leadID := uuid.New()
	if _, err := db.Exec(`INSERT INTO users (id, name, display_name, email, password_hash, role) VALUES ($1, 'Lead', 'Lead', $2, 'x', 'user')`, leadID, leadID.String()+"@example.test"); err != nil {
		t.Fatal(err)
	}
	id := uuid.New()
	if _, err := db.Exec(`INSERT INTO projects (id, organization_id, key, name, type, template, lead_id, visibility)
		VALUES ($1, $2, $3, 'P', 'kanban', $4, $5, 'open')`, id, defaultOrg, "K"+id.String()[:6], template, leadID); err != nil {
		t.Fatal(err)
	}
	return id
}

func TestFindWorkflowProvisionsTheTemplateOnDemand(t *testing.T) {
	db := testdb.New(t)
	repo := postgres.NewRepository(db)
	ctx := context.Background()

	for _, template := range domain.TemplateNames() {
		project := insertProject(t, db, template)
		record, err := repo.FindWorkflow(ctx, project)
		if err != nil {
			t.Fatalf("%s: %v", template, err)
		}
		want, _ := domain.TemplateWorkflow(template)
		if record.Version != 1 || len(record.Workflow.Statuses) != len(want.Statuses) || len(record.Workflow.Transitions) != len(want.Transitions) {
			t.Fatalf("%s: workflow = %d statuses, %d transitions, version %d", template, len(record.Workflow.Statuses), len(record.Workflow.Transitions), record.Version)
		}
		if issues := record.Workflow.Validate(); len(issues) != 0 {
			t.Fatalf("%s: provisioned workflow is invalid: %+v", template, issues)
		}
		initial, ok := record.Workflow.Status(record.Workflow.Initial)
		if !ok || initial.Category != domain.CategoryTodo {
			t.Fatalf("%s: initial status = %+v", template, initial)
		}
	}
	if _, err := repo.FindWorkflow(ctx, uuid.New()); apperror.CodeOf(err) != "workflow.not_found" {
		t.Fatalf("unknown project: %v, want workflow.not_found", err)
	}
}

func TestProvisionerIsIdempotentAndSharesOrganizationStatuses(t *testing.T) {
	db := testdb.New(t)
	repo := postgres.NewRepository(db)
	ctx := context.Background()
	first := insertProject(t, db, domain.TemplateScrum)
	second := insertProject(t, db, domain.TemplateKanban)

	err := database.NewTxManager(db).InTx(ctx, func(ctx context.Context) error {
		tx, _ := database.TxFromContext(ctx)
		for _, project := range []uuid.UUID{first, second, first} {
			if err := postgres.NewProvisioner().ProvisionWorkflow(ctx, tx, project, ""); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	statuses, err := repo.ListStatuses(ctx, uuid.MustParse(defaultOrg))
	if err != nil {
		t.Fatal(err)
	}
	names := map[string]int{}
	for _, s := range statuses {
		names[s.Name]++
	}
	if names["Por hacer"] != 1 || names["Hecho"] != 1 {
		t.Fatalf("statuses = %v, want each template status once per organization", names)
	}
	a, _ := repo.FindWorkflow(ctx, first)
	b, _ := repo.FindWorkflow(ctx, second)
	if a.Workflow.Initial != b.Workflow.Initial {
		t.Fatal("projects of one organization must share the status records of the same name")
	}
}

func TestSaveWorkflowChecksTheVersion(t *testing.T) {
	db := testdb.New(t)
	repo := postgres.NewRepository(db)
	ctx := context.Background()
	project := insertProject(t, db, domain.TemplateTaskTracking)
	record, err := repo.FindWorkflow(ctx, project)
	if err != nil {
		t.Fatal(err)
	}
	w := record.Workflow
	w.Transitions[0].Name = "Empezar"
	w.Transitions[0].Validators = []domain.Validator{domain.CommentRequired{}}
	at := time.Date(2026, 11, 27, 12, 0, 0, 0, time.UTC)
	version, err := repo.SaveWorkflow(ctx, project, w, 1, at)
	if err != nil || version != 2 {
		t.Fatalf("save = %d, %v", version, err)
	}
	reloaded, _ := repo.FindWorkflow(ctx, project)
	if reloaded.Version != 2 || reloaded.Workflow.Transitions[0].Name != "Empezar" || len(reloaded.Workflow.Transitions[0].Validators) != 1 || !reloaded.UpdatedAt.Equal(at) {
		t.Fatalf("reloaded = %+v", reloaded)
	}
	if _, err := repo.SaveWorkflow(ctx, project, w, 1, at); apperror.CodeOf(err) != "workflow.version_conflict" {
		t.Fatalf("stale save: %v", err)
	}
	if _, err := repo.SaveWorkflow(ctx, uuid.New(), w, 1, at); apperror.CodeOf(err) != "workflow.not_found" {
		t.Fatalf("save for a project without workflow: %v", err)
	}
}

func TestStatusCatalogRepository(t *testing.T) {
	db := testdb.New(t)
	repo := postgres.NewRepository(db)
	ctx := context.Background()
	org := uuid.MustParse(defaultOrg)

	created, err := repo.CreateStatus(ctx, port.OrgStatus{OrganizationID: org, Name: "En pruebas", Category: domain.CategoryInProgress})
	if err != nil || created.ID == uuid.Nil {
		t.Fatalf("create = %+v, %v", created, err)
	}
	if _, err := repo.CreateStatus(ctx, port.OrgStatus{OrganizationID: org, Name: "EN PRUEBAS", Category: domain.CategoryTodo}); apperror.CodeOf(err) != "workflow.status_name_taken" {
		t.Fatalf("duplicate: %v", err)
	}
	other, err := repo.CreateStatus(ctx, port.OrgStatus{OrganizationID: org, Name: "Bloqueado", Category: domain.CategoryTodo})
	if err != nil || other.Position <= created.Position {
		t.Fatalf("second status = %+v, %v; want a later position", other, err)
	}

	other.Name = "EN PRUEBAS"
	if err := repo.UpdateStatus(ctx, other); apperror.CodeOf(err) != "workflow.status_name_taken" {
		t.Fatalf("rename onto an existing name: %v", err)
	}
	created.Name, created.Category = "QA", domain.CategoryDone
	if err := repo.UpdateStatus(ctx, created); err != nil {
		t.Fatal(err)
	}
	if err := repo.UpdateStatus(ctx, port.OrgStatus{ID: uuid.New(), OrganizationID: org, Name: "Z", Category: domain.CategoryTodo}); apperror.CodeOf(err) != "workflow.status_not_found" {
		t.Fatalf("update of an unknown status: %v", err)
	}
	list, _ := repo.ListStatuses(ctx, org)
	found := false
	for _, s := range list {
		found = found || (s.ID == created.ID && s.Name == "QA" && s.Category == domain.CategoryDone)
	}
	if !found {
		t.Fatalf("renamed status missing from %+v", list)
	}
	if foreign, _ := repo.ListStatuses(ctx, uuid.New()); len(foreign) != 0 {
		t.Fatal("statuses leaked across organizations")
	}

	if err := repo.DeleteStatus(ctx, uuid.New(), created.ID); apperror.CodeOf(err) != "workflow.status_not_found" {
		t.Fatalf("delete from another organization: %v", err)
	}
	if err := repo.DeleteStatus(ctx, org, created.ID); err != nil {
		t.Fatal(err)
	}
	if err := repo.DeleteStatus(ctx, org, created.ID); apperror.CodeOf(err) != "workflow.status_not_found" {
		t.Fatalf("second delete: %v", err)
	}
}

func TestDeleteStatusInUseByAWorkflowOrAnIssue(t *testing.T) {
	db := testdb.New(t)
	repo := postgres.NewRepository(db)
	ctx := context.Background()
	project := insertProject(t, db, domain.TemplateScrum)
	record, err := repo.FindWorkflow(ctx, project)
	if err != nil {
		t.Fatal(err)
	}
	org := uuid.MustParse(defaultOrg)
	inWorkflow := uuid.MustParse(record.Workflow.Initial)
	if err := repo.DeleteStatus(ctx, org, inWorkflow); apperror.CodeOf(err) != "workflow.status_in_use" || apperror.KindOf(err) != apperror.KindConflict {
		t.Fatalf("status used by a workflow: %v, want workflow.status_in_use", err)
	}

	// A status that no workflow lists but an issue still holds cannot go either.
	orphan, err := repo.CreateStatus(ctx, port.OrgStatus{OrganizationID: org, Name: "Huérfano", Category: domain.CategoryTodo})
	if err != nil {
		t.Fatal(err)
	}
	var reporter uuid.UUID
	if err := db.Get(&reporter, `SELECT lead_id FROM projects WHERE id = $1`, project); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO issues (project_id, summary, description, status, priority, issue_type, reporter_id, key_number, issue_type_id, priority_id, status_id)
		SELECT $1, 'S', 'D', 'open', 'medium', 'task', $2, 1,
		       (SELECT id FROM issue_types WHERE organization_id = $3 AND is_default),
		       (SELECT id FROM priorities WHERE organization_id = $3 AND is_default), $4`, project, reporter, defaultOrg, orphan.ID); err != nil {
		t.Fatal(err)
	}
	if err := repo.DeleteStatus(ctx, org, orphan.ID); apperror.CodeOf(err) != "workflow.status_in_use" {
		t.Fatalf("status held by an issue: %v, want workflow.status_in_use", err)
	}
}

func TestColumnStatusIDsFollowTheBoardBinding(t *testing.T) {
	db := testdb.New(t)
	repo := postgres.NewRepository(db)
	ctx := context.Background()
	project := insertProject(t, db, domain.TemplateScrum)
	boardID := uuid.New()
	if _, err := db.Exec(`INSERT INTO boards (id, project_id, name, type, is_default) VALUES ($1, $2, 'B', 'kanban', true)`, boardID, project); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO board_columns (board_id, name, status, position) VALUES ($1, 'Open', 'open', 0), ($1, 'In progress', 'in_progress', 1)`, boardID); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.FindWorkflow(ctx, project); err != nil {
		t.Fatal(err)
	}
	var progressColumn uuid.UUID
	if err := db.Get(&progressColumn, `SELECT id FROM board_columns WHERE board_id = $1 AND status = 'in_progress'`, boardID); err != nil {
		t.Fatal(err)
	}
	ids, err := repo.ColumnStatusIDs(ctx, progressColumn)
	if err != nil || len(ids) != 2 {
		t.Fatalf("in-progress column statuses = %v (%v), want the two in-progress statuses of the template", ids, err)
	}
	if none, err := repo.ColumnStatusIDs(ctx, uuid.New()); err != nil || len(none) != 0 {
		t.Fatalf("unknown column = %v, %v", none, err)
	}
}
