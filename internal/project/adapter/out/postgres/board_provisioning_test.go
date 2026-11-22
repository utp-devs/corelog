package postgres

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/uuid"
	_ "github.com/jackc/pgx/v5/stdlib"
	projectport "github.com/jhonsferg/corelog/internal/project/application/port"
	projectusecase "github.com/jhonsferg/corelog/internal/project/application/usecase"
	"github.com/jhonsferg/corelog/internal/project/domain"
	"github.com/jmoiron/sqlx"
)

func TestProjectCreationProvisionsDefaultBoardPostgres(t *testing.T) {
	dsn := os.Getenv("CORELOG_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("set CORELOG_TEST_DATABASE_URL to run PostgreSQL integration tests")
	}
	db, err := sqlx.Connect("pgx", dsn)
	if err != nil {
		t.Fatal(err)
	}
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = db.Close() })
	schema := "board_test_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	if _, err := db.Exec(`CREATE SCHEMA ` + schema); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = db.Exec(`DROP SCHEMA ` + schema + ` CASCADE`) })
	if _, err := db.Exec(`SET search_path TO ` + schema); err != nil {
		t.Fatal(err)
	}
	for _, statement := range []string{
		`CREATE TABLE users(id UUID PRIMARY KEY)`,
		`CREATE TABLE projects(id UUID PRIMARY KEY, key VARCHAR(20) NOT NULL UNIQUE, name VARCHAR(120) NOT NULL, description TEXT NOT NULL DEFAULT '', type VARCHAR(10) NOT NULL CHECK(type IN ('kanban','scrum')), lead_id UUID NOT NULL REFERENCES users(id), visibility VARCHAR(10) NOT NULL, archived_at TIMESTAMPTZ, created_at TIMESTAMPTZ NOT NULL, updated_at TIMESTAMPTZ NOT NULL)`,
		`CREATE TABLE project_members(project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE, user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE, role VARCHAR(20) NOT NULL, created_at TIMESTAMPTZ NOT NULL DEFAULT now(), PRIMARY KEY(project_id,user_id))`,
	} {
		if _, err := db.Exec(statement); err != nil {
			t.Fatal(err)
		}
	}
	leadID := uuid.New()
	if _, err := db.Exec(`INSERT INTO users(id) VALUES($1)`, leadID); err != nil {
		t.Fatal(err)
	}
	existingID := uuid.New()
	if _, err := db.Exec(`INSERT INTO projects(id,key,name,type,lead_id,visibility,created_at,updated_at) VALUES($1,'OLD','Existing','kanban',$2,'team',now(),now())`, existingID, leadID); err != nil {
		t.Fatal(err)
	}
	workflowUpPath := filepath.Join("..", "..", "..", "..", "..", "migrations", "postgres", "0010_workflows.up.sql")
	workflowUp, err := os.ReadFile(workflowUpPath)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(string(workflowUp)); err != nil {
		t.Fatal(err)
	}
	workflowDownPath := filepath.Join("..", "..", "..", "..", "..", "migrations", "postgres", "0010_workflows.down.sql")
	workflowDown, err := os.ReadFile(workflowDownPath)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = db.Exec(string(workflowDown)) })
	var seededWorkflows int
	if err := db.Get(&seededWorkflows, `SELECT count(*) FROM workflows`); err != nil {
		t.Fatal(err)
	}
	if seededWorkflows != 0 {
		t.Fatalf("workflow migration seeded %d workflows without the default project", seededWorkflows)
	}
	upPath := filepath.Join("..", "..", "..", "..", "..", "migrations", "postgres", "000014_boards.up.sql")
	up, err := os.ReadFile(upPath)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(string(up)); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`ALTER TABLE projects ALTER COLUMN id SET DEFAULT gen_random_uuid()`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`ALTER TABLE boards ALTER COLUMN id SET DEFAULT gen_random_uuid()`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`ALTER TABLE board_columns ALTER COLUMN id SET DEFAULT gen_random_uuid()`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`ALTER TABLE workflows ALTER COLUMN id SET DEFAULT gen_random_uuid()`); err != nil {
		t.Fatal(err)
	}
	downPath := filepath.Join("..", "..", "..", "..", "..", "migrations", "postgres", "000014_boards.down.sql")
	down, err := os.ReadFile(downPath)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = db.Exec(string(down)) })
	repository := NewProjectRepository(db)
	assertDefaultBoard(t, db, existingID, "kanban")
	service := projectusecase.NewProjectService(repository, nil)
	created, err := service.Create(context.Background(), "NEW", "New Scrum", "", domain.ProjectTypeScrum, leadID, domain.VisibilityTeam)
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
	if _, err := service.Create(context.Background(), "NEW", "Duplicate", "", domain.ProjectTypeKanban, leadID, domain.VisibilityTeam); err != domain.ErrProjectKeyTaken {
		t.Fatalf("duplicate project error=%v, want %v", err, domain.ErrProjectKeyTaken)
	}
	if _, err := db.Exec(`INSERT INTO boards(id,project_id,name,type,is_default) VALUES($1,$2,'Second default','kanban',true)`, uuid.New(), existingID); err == nil {
		t.Fatal("expected the database to reject a second default board for the project")
	}

	rollbackProject, err := domain.NewProject("ROLLBACK", "Rollback", "", domain.ProjectTypeKanban, leadID, domain.VisibilityTeam)
	if err != nil {
		t.Fatal(err)
	}
	err = repository.CreateWithDefaults(context.Background(), rollbackProject, projectport.Member{
		ProjectID: rollbackProject.ID, UserID: uuid.New(), Role: "lead",
	})
	if err == nil {
		t.Fatal("expected project creation to fail when its lead membership cannot be saved")
	}
	var projectCount, boardCount, remainingWorkflows int
	if err := db.Get(&projectCount, `SELECT count(*) FROM projects WHERE id=$1`, rollbackProject.ID); err != nil {
		t.Fatal(err)
	}
	if err := db.Get(&boardCount, `SELECT count(*) FROM boards WHERE project_id=$1`, rollbackProject.ID); err != nil {
		t.Fatal(err)
	}
	if err := db.Get(&remainingWorkflows, `SELECT count(*) FROM workflows WHERE project_id=$1`, rollbackProject.ID); err != nil {
		t.Fatal(err)
	}
	if projectCount != 0 || boardCount != 0 || remainingWorkflows != 0 {
		t.Fatalf("failed project creation left project=%d boards=%d workflows=%d", projectCount, boardCount, remainingWorkflows)
	}
	if err := repository.Delete(context.Background(), existingID); err != nil {
		t.Fatal(err)
	}
	var remainingColumns int
	if err := db.Get(&remainingColumns, `SELECT count(*) FROM board_columns c JOIN boards b ON b.id=c.board_id WHERE b.project_id=$1`, existingID); err != nil {
		t.Fatal(err)
	}
	if remainingColumns != 0 {
		t.Fatalf("deleting project left %d board columns", remainingColumns)
	}
	if _, err := db.Exec(string(down)); err != nil {
		t.Fatalf("down migration: %v", err)
	}
	var boardsTableExists bool
	if err := db.Get(&boardsTableExists, `SELECT to_regclass('boards') IS NOT NULL`); err != nil {
		t.Fatal(err)
	}
	var projectsCount int
	if err := db.Get(&projectsCount, `SELECT count(*) FROM projects`); err != nil {
		t.Fatal(err)
	}
	if boardsTableExists || projectsCount != 1 {
		t.Fatalf("down migration boards table exists=%v, project count=%d", boardsTableExists, projectsCount)
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
