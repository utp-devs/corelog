package postgres

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/google/uuid"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/jhonsferg/corelog/internal/ticket/domain"
	"github.com/jmoiron/sqlx"
)

// TestTicketRepositoryUnassignClearsAssigneePostgres comprueba que Update escribe NULL en assignee_id al desasignar.
func TestTicketRepositoryUnassignClearsAssigneePostgres(t *testing.T) {
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

	schema := "unassign_test_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	if _, err := db.Exec(`CREATE SCHEMA ` + schema); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = db.Exec(`DROP SCHEMA ` + schema + ` CASCADE`) })
	if _, err := db.Exec(`SET search_path TO ` + schema); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`CREATE TABLE issues (
		id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
		project_id UUID NOT NULL,
		title VARCHAR(200) NOT NULL,
		description TEXT NOT NULL,
		status VARCHAR(20) NOT NULL,
		priority VARCHAR(20) NOT NULL,
		issue_type VARCHAR(20) NOT NULL DEFAULT 'task',
		parent_id UUID,
		due_date TIMESTAMPTZ,
		story_points NUMERIC(4,1),
		rank VARCHAR(64) NOT NULL DEFAULT 'm',
		closed_at TIMESTAMPTZ,
		requester_id UUID NOT NULL,
		assignee_id UUID,
		created_at TIMESTAMPTZ NOT NULL,
		updated_at TIMESTAMPTZ NOT NULL
	)`); err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()
	repo := NewTicketRepository(db)
	ticket, err := domain.NewTicket("Unassign", "Description", domain.PriorityLow, uuid.New())
	if err != nil {
		t.Fatal(err)
	}
	ticket.ProjectID = uuid.New()
	assignee := uuid.New()
	ticket.AssigneeID = &assignee
	if err := repo.Save(ctx, ticket); err != nil {
		t.Fatal(err)
	}

	ticket.ClearAssignee()
	if err := repo.Update(ctx, ticket); err != nil {
		t.Fatalf("update: %v", err)
	}

	var isNull bool
	if err := db.QueryRowContext(ctx, `SELECT assignee_id IS NULL FROM issues WHERE id = $1`, ticket.ID).Scan(&isNull); err != nil {
		t.Fatal(err)
	}
	if !isNull {
		t.Fatal("assignee_id is still set after unassign")
	}
	stored, err := repo.FindByID(ctx, ticket.ID)
	if err != nil {
		t.Fatal(err)
	}
	if stored.AssigneeID != nil {
		t.Errorf("FindByID assignee = %v, want nil", stored.AssigneeID)
	}
}
