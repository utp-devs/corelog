package postgres

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/google/uuid"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/jhonsferg/corelog/internal/ticket/application/port"
	"github.com/jhonsferg/corelog/internal/ticket/domain"
	"github.com/jmoiron/sqlx"
)

// TestTicketRepositoryCountMatchesFindAllPostgres verifica que Count y FindAll usan la misma
// clausula WHERE: el total cuenta todas las coincidencias y la pagina devuelve solo su parte.
func TestTicketRepositoryCountMatchesFindAllPostgres(t *testing.T) {
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

	schema := "list_test_" + strings.ReplaceAll(uuid.NewString(), "-", "")
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

	repo := NewTicketRepository(db)
	projectID := uuid.New()
	requesterID := uuid.New()
	for i := 0; i < 3; i++ {
		ticket, err := domain.NewTicket("Issue", "Description", domain.PriorityLow, requesterID)
		if err != nil {
			t.Fatal(err)
		}
		ticket.ProjectID = projectID
		if err := repo.Save(context.Background(), ticket); err != nil {
			t.Fatal(err)
		}
	}

	ctx := context.Background()
	page := port.ListFilter{ProjectIDs: []uuid.UUID{projectID}, Page: 1, PageSize: 2}
	items, err := repo.FindAll(ctx, page)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 2 {
		t.Errorf("FindAll page 1 returned %d items, want 2", len(items))
	}
	total, err := repo.Count(ctx, page)
	if err != nil {
		t.Fatal(err)
	}
	if total != 3 {
		t.Errorf("Count = %d, want 3 (all matches, not only the page)", total)
	}

	none := port.ListFilter{ProjectIDs: []uuid.UUID{projectID}, Status: domain.StatusResolved, Page: 1, PageSize: 20}
	zero, err := repo.Count(ctx, none)
	if err != nil {
		t.Fatal(err)
	}
	if zero != 0 {
		t.Errorf("Count with no matches = %d, want 0", zero)
	}
}
