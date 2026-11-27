package postgres

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"

	"github.com/jhonsferg/corelog/internal/platform/apperror"
	"github.com/jhonsferg/corelog/internal/platform/database/testdb"
	projectport "github.com/jhonsferg/corelog/internal/project/application/port"
	projectusecase "github.com/jhonsferg/corelog/internal/project/application/usecase"
	"github.com/jhonsferg/corelog/internal/project/domain"
)

// defaultOrganizationID is the organization that migration 000024 creates.
const defaultOrganizationID = "00000000-0000-4000-8000-000000000002"

// ownerOrgs reports every user as owner of every organization, with the default policy.
type ownerOrgs struct{}

func (ownerOrgs) MemberRole(context.Context, uuid.UUID, uuid.UUID) (string, bool, error) {
	return domain.OrgRoleOwner, true, nil
}

func (ownerOrgs) ProjectCreationPolicy(context.Context, uuid.UUID) (string, string, error) {
	return domain.CreationPolicyMembers, "open", nil
}

func insertUser(t *testing.T, db *sqlx.DB) uuid.UUID {
	t.Helper()
	id := uuid.New()
	if _, err := db.Exec(`INSERT INTO users (id, name, display_name, email, password_hash, role) VALUES ($1, 'Lead', 'Lead', $2, 'x', 'user')`,
		id, id.String()+"@example.test"); err != nil {
		t.Fatal(err)
	}
	return id
}

func TestProjectCreationProvisionsDefaultBoardPostgres(t *testing.T) {
	db := testdb.New(t)
	ctx := context.Background()
	leadID := insertUser(t, db)
	repository := NewProjectRepository(db)
	service := projectusecase.NewProjectService(repository, NewMemberRepository(db), ownerOrgs{})

	created, err := service.Create(ctx, leadID, uuid.MustParse(defaultOrganizationID), "NEW", "New Scrum", "", domain.ProjectTypeScrum, "")
	if err != nil {
		t.Fatal(err)
	}
	assertDefaultBoard(t, db, created.ID, "scrum")

	var workflowCount, transitionCount int
	if err := db.Get(&workflowCount, `SELECT count(*) FROM workflows WHERE project_id=$1 AND is_default`, created.ID); err != nil {
		t.Fatal(err)
	}
	if err := db.Get(&transitionCount, `SELECT count(*) FROM workflow_transitions wt JOIN workflows w ON w.id=wt.workflow_id WHERE w.project_id=$1 AND w.is_default`, created.ID); err != nil {
		t.Fatal(err)
	}
	if workflowCount != 1 || transitionCount != 6 {
		t.Fatalf("default workflow was not provisioned: workflows=%d transitions=%d", workflowCount, transitionCount)
	}

	_, err = service.Create(ctx, leadID, uuid.MustParse(defaultOrganizationID), "NEW", "Duplicate", "", domain.ProjectTypeKanban, "")
	if apperror.CodeOf(err) != "project.key_taken" {
		t.Fatalf("duplicate key in the same organization: %v, want project.key_taken", err)
	}

	otherOrg := uuid.New()
	if _, err := db.Exec(`INSERT INTO organizations (id, slug, name) VALUES ($1, $2, 'Other')`, otherOrg, "other-"+otherOrg.String()[:8]); err != nil {
		t.Fatal(err)
	}
	if _, err := service.Create(ctx, leadID, otherOrg, "NEW", "Same key elsewhere", "", domain.ProjectTypeKanban, ""); err != nil {
		t.Fatalf("same key in another organization must be allowed: %v", err)
	}

	if _, err := db.Exec(`INSERT INTO boards(id,project_id,name,type,is_default) VALUES($1,$2,'Second default','kanban',true)`, uuid.New(), created.ID); err == nil {
		t.Fatal("expected the database to reject a second default board for the project")
	}

	rollbackProject, err := domain.NewProject(uuid.MustParse(defaultOrganizationID), "ROLLBACK", "Rollback", "", domain.ProjectTypeKanban, domain.VisibilityOpen, leadID)
	if err != nil {
		t.Fatal(err)
	}
	err = repository.CreateWithDefaults(ctx, rollbackProject, projectport.Member{
		ProjectID: rollbackProject.ID, UserID: uuid.New(), Role: "lead",
	})
	if !errors.Is(err, domain.ErrUserNotFound) {
		t.Fatalf("lead that does not exist: got %v, want project.user_not_found", err)
	}
	var projectCount, boardCount, remainingWorkflows int
	if err := db.Get(&projectCount, `SELECT count(*) FROM projects WHERE key=$1`, "ROLLBACK"); err != nil {
		t.Fatal(err)
	}
	if err := db.Get(&boardCount, `SELECT count(*) FROM boards b JOIN projects p ON p.id=b.project_id WHERE p.key=$1`, "ROLLBACK"); err != nil {
		t.Fatal(err)
	}
	if err := db.Get(&remainingWorkflows, `SELECT count(*) FROM workflows w JOIN projects p ON p.id=w.project_id WHERE p.key=$1`, "ROLLBACK"); err != nil {
		t.Fatal(err)
	}
	if projectCount != 0 || boardCount != 0 || remainingWorkflows != 0 {
		t.Fatalf("failed project creation left project=%d boards=%d workflows=%d", projectCount, boardCount, remainingWorkflows)
	}

	// Deleting is a soft delete: the project disappears from reads and its rows stay for the purge job.
	if err := repository.Delete(ctx, uuid.MustParse(defaultOrganizationID), created.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := repository.FindByID(ctx, created.ID); err == nil {
		t.Fatal("soft deleted project is still readable")
	}
	var remainingColumns int
	if err := db.Get(&remainingColumns, `SELECT count(*) FROM board_columns c JOIN boards b ON b.id=c.board_id WHERE b.project_id=$1`, created.ID); err != nil {
		t.Fatal(err)
	}
	if remainingColumns == 0 {
		t.Fatal("soft delete must keep the project rows until the purge job runs")
	}
}

func assertDefaultBoard(t *testing.T, db *sqlx.DB, projectID uuid.UUID, wantType string) {
	t.Helper()
	var boardID uuid.UUID
	var boardCount int
	if err := db.Get(&boardID, `SELECT id FROM boards WHERE project_id=$1 AND is_default AND type=$2`, projectID, wantType); err != nil {
		t.Fatal(err)
	}
	if err := db.Get(&boardCount, `SELECT count(*) FROM boards WHERE project_id=$1`, projectID); err != nil || boardCount != 1 {
		t.Fatalf("project %s board count=%d err=%v, want 1", projectID, boardCount, err)
	}
	var statuses []string
	if err := db.Select(&statuses, `SELECT status FROM board_columns WHERE board_id=$1 ORDER BY position`, boardID); err != nil {
		t.Fatal(err)
	}
	want := []string{"open", "in_progress", "resolved", "closed"}
	if len(statuses) != len(want) {
		t.Fatalf("board statuses=%v, want %v", statuses, want)
	}
	for i := range want {
		if statuses[i] != want[i] {
			t.Fatalf("board statuses=%v, want %v", statuses, want)
		}
	}
}
