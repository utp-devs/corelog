package postgres

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/google/uuid"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/jhonsferg/corelog/internal/issue/domain"
	"github.com/jmoiron/sqlx"
)

// TestBacklogRepositoryOrderAndSaveRanksPostgres comprueba que el backlog sale ordenado por rank y que
// SaveRanks cambia el orden solo dentro del proyecto indicado.
func TestBacklogRepositoryOrderAndSaveRanksPostgres(t *testing.T) {
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

	schema := "backlog_test_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	if _, err := db.Exec(`CREATE SCHEMA ` + schema); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = db.Exec(`DROP SCHEMA ` + schema + ` CASCADE`) })
	if _, err := db.Exec(`SET search_path TO ` + schema); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`CREATE TABLE projects (id UUID PRIMARY KEY, key VARCHAR(10) NOT NULL DEFAULT 'TST', next_issue_number INT NOT NULL DEFAULT 1, deleted_at TIMESTAMPTZ)`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`CREATE TABLE issues (
		id UUID PRIMARY KEY, project_id UUID NOT NULL, key_number INT NOT NULL DEFAULT 0, summary TEXT NOT NULL, description TEXT NOT NULL DEFAULT '',
		status TEXT NOT NULL, priority TEXT NOT NULL, issue_type TEXT NOT NULL, parent_id UUID,
		due_date TIMESTAMPTZ, story_points DOUBLE PRECISION, rank VARCHAR(64) NOT NULL DEFAULT 'm',
		closed_at TIMESTAMPTZ, reporter_id UUID NOT NULL, assignee_id UUID,
		created_at TIMESTAMPTZ NOT NULL DEFAULT now(), updated_at TIMESTAMPTZ NOT NULL DEFAULT now())`); err != nil {
		t.Fatal(err)
	}

	project, other := uuid.New(), uuid.New()
	if _, err := db.Exec(`INSERT INTO projects (id) VALUES ($1)`, project); err != nil {
		t.Fatal(err)
	}
	a, b, c, foreign := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	for _, row := range []struct {
		id      uuid.UUID
		project uuid.UUID
		rank    string
	}{
		{a, project, "000002"}, {b, project, "000001"}, {c, project, "m"}, {foreign, other, "000000"},
	} {
		if _, err := db.Exec(`INSERT INTO issues (id, project_id, summary, status, priority, issue_type, rank, reporter_id)
			VALUES ($1, $2, 'x', 'open', 'medium', 'task', $3, $4)`, row.id, row.project, row.rank, uuid.New()); err != nil {
			t.Fatal(err)
		}
	}

	repo := NewIssueRepository(db)
	ctx := context.Background()

	items, err := repo.ListBacklog(ctx, project)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 3 || items[0].ID != b || items[1].ID != a || items[2].ID != c {
		t.Fatalf("backlog order = %v, want b, a, c (rank ascending, 'm' last)", ids(items))
	}

	if err := repo.SaveRanks(ctx, project, map[uuid.UUID]string{a: "000001", b: "000002", c: "000003", foreign: "999999"}); err != nil {
		t.Fatalf("SaveRanks returned error: %v", err)
	}
	items, err = repo.ListBacklog(ctx, project)
	if err != nil {
		t.Fatal(err)
	}
	if items[0].ID != a || items[1].ID != b || items[2].ID != c {
		t.Fatalf("backlog after SaveRanks = %v, want a, b, c", ids(items))
	}

	var foreignRank string
	if err := db.Get(&foreignRank, `SELECT rank FROM issues WHERE id = $1`, foreign); err != nil {
		t.Fatal(err)
	}
	if foreignRank != "000000" {
		t.Fatalf("SaveRanks changed an issue of another project: rank %s", foreignRank)
	}
}

func ids(items []*domain.Issue) []uuid.UUID {
	out := make([]uuid.UUID, 0, len(items))
	for _, item := range items {
		out = append(out, item.ID)
	}
	return out
}
