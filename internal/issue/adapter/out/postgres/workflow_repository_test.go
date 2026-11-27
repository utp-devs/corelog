package postgres

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/google/uuid"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/jhonsferg/corelog/internal/issue/application/port"
	"github.com/jhonsferg/corelog/internal/issue/domain"
	"github.com/jmoiron/sqlx"
)

// TestWorkflowRepositoryDefaultWorkflowPostgres comprueba que se lee el flujo por defecto con sus
// transiciones y roles, y que un proyecto sin flujo por defecto devuelve nil sin error.
func TestWorkflowRepositoryDefaultWorkflowPostgres(t *testing.T) {
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

	schema := "workflow_test_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	if _, err := db.Exec(`CREATE SCHEMA ` + schema); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = db.Exec(`DROP SCHEMA ` + schema + ` CASCADE`) })
	if _, err := db.Exec(`SET search_path TO ` + schema); err != nil {
		t.Fatal(err)
	}
	for _, statement := range []string{
		`CREATE TABLE workflows (id UUID PRIMARY KEY, project_id UUID NOT NULL, name VARCHAR(80) NOT NULL, is_default BOOLEAN NOT NULL DEFAULT false, created_at TIMESTAMPTZ NOT NULL DEFAULT now())`,
		`CREATE TABLE workflow_transitions (workflow_id UUID NOT NULL, from_status VARCHAR(20) NOT NULL, to_status VARCHAR(20) NOT NULL, allowed_roles TEXT NOT NULL, PRIMARY KEY (workflow_id, from_status, to_status))`,
	} {
		if _, err := db.Exec(statement); err != nil {
			t.Fatal(err)
		}
	}

	project, other, workflowID := uuid.New(), uuid.New(), uuid.New()
	if _, err := db.Exec(`INSERT INTO workflows (id, project_id, name, is_default) VALUES ($1, $2, 'Flujo', true)`, workflowID, project); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO workflow_transitions (workflow_id, from_status, to_status, allowed_roles) VALUES
		($1, 'open', 'in_progress', 'admin,lead,contributor,viewer'),
		($1, 'in_progress', 'resolved', 'admin,lead')`, workflowID); err != nil {
		t.Fatal(err)
	}

	repo := NewWorkflowRepository(db)
	ctx := context.Background()

	workflow, err := repo.DefaultWorkflow(ctx, project)
	if err != nil || workflow == nil {
		t.Fatalf("DefaultWorkflow = %v, %v; want the workflow", workflow, err)
	}
	if workflow.Name != "Flujo" || len(workflow.Transitions) != 2 {
		t.Fatalf("unexpected workflow %+v", workflow)
	}
	// Las transiciones vienen ordenadas por estado de origen: in_progress antes que open.
	first := workflow.Transitions[0]
	if first.From != domain.StatusInProgress || first.To != domain.StatusResolved || len(first.Roles) != 2 || first.Roles[1] != "lead" {
		t.Fatalf("first transition = %+v", first)
	}
	second := workflow.Transitions[1]
	if second.From != domain.StatusOpen || second.To != domain.StatusInProgress || len(second.Roles) != 4 {
		t.Fatalf("second transition = %+v", second)
	}

	missing, err := repo.DefaultWorkflow(ctx, other)
	if err != nil || missing != nil {
		t.Fatalf("project without workflow = %v, %v; want nil and no error", missing, err)
	}
}

// TestWorkflowRepositoryReplaceTransitionsPostgres comprueba que ReplaceTransitions sustituye el
// conjunto completo de transiciones y guarda los roles como lista separada por comas.
func TestWorkflowRepositoryReplaceTransitionsPostgres(t *testing.T) {
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

	schema := "workflow_replace_test_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	if _, err := db.Exec(`CREATE SCHEMA ` + schema); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = db.Exec(`DROP SCHEMA ` + schema + ` CASCADE`) })
	if _, err := db.Exec(`SET search_path TO ` + schema); err != nil {
		t.Fatal(err)
	}
	for _, statement := range []string{
		`CREATE TABLE workflows (id UUID PRIMARY KEY, project_id UUID NOT NULL, name VARCHAR(80) NOT NULL, is_default BOOLEAN NOT NULL DEFAULT false, created_at TIMESTAMPTZ NOT NULL DEFAULT now())`,
		`CREATE TABLE workflow_transitions (workflow_id UUID NOT NULL, from_status VARCHAR(20) NOT NULL, to_status VARCHAR(20) NOT NULL, allowed_roles TEXT NOT NULL, PRIMARY KEY (workflow_id, from_status, to_status))`,
	} {
		if _, err := db.Exec(statement); err != nil {
			t.Fatal(err)
		}
	}

	project, workflowID := uuid.New(), uuid.New()
	if _, err := db.Exec(`INSERT INTO workflows (id, project_id, name, is_default) VALUES ($1, $2, 'Flujo', true)`, workflowID, project); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO workflow_transitions VALUES ($1, 'open', 'closed', 'admin,lead')`, workflowID); err != nil {
		t.Fatal(err)
	}

	repo := NewWorkflowRepository(db)
	ctx := context.Background()
	replacement := []port.WorkflowTransition{
		{From: domain.StatusInProgress, To: domain.StatusResolved, Roles: []string{"lead", "contributor"}},
	}
	if err := repo.ReplaceTransitions(ctx, workflowID, replacement); err != nil {
		t.Fatalf("ReplaceTransitions returned error: %v", err)
	}

	var stored []string
	if err := db.Select(&stored, `SELECT from_status || '>' || to_status || ':' || allowed_roles FROM workflow_transitions WHERE workflow_id = $1 ORDER BY 1`, workflowID); err != nil {
		t.Fatal(err)
	}
	if len(stored) != 1 || stored[0] != "in_progress>resolved:lead,contributor" {
		t.Fatalf("stored transitions = %v, want only the replacement", stored)
	}
}
