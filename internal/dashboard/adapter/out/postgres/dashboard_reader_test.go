package postgres

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/jmoiron/sqlx"
)

// TestDashboardReaderSummaryPostgres comprueba conteos, vencidas y sprints activos del usuario, y que
// un sprint activo de un proyecto ajeno no aparece.
func TestDashboardReaderSummaryPostgres(t *testing.T) {
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

	schema := "dashboard_test_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	if _, err := db.Exec(`CREATE SCHEMA ` + schema); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = db.Exec(`DROP SCHEMA ` + schema + ` CASCADE`) })
	if _, err := db.Exec(`SET search_path TO ` + schema); err != nil {
		t.Fatal(err)
	}
	for _, statement := range []string{
		`CREATE TABLE projects (id UUID PRIMARY KEY, key TEXT NOT NULL, name TEXT NOT NULL, deleted_at TIMESTAMPTZ)`,
		`CREATE TABLE project_members (project_id UUID NOT NULL, user_id UUID NOT NULL, PRIMARY KEY (project_id, user_id))`,
		`CREATE TABLE issues (id UUID PRIMARY KEY, project_id UUID NOT NULL REFERENCES projects (id), summary TEXT NOT NULL,
			status TEXT NOT NULL, priority TEXT NOT NULL DEFAULT 'medium', assignee_id UUID, due_date DATE,
			updated_at TIMESTAMPTZ NOT NULL DEFAULT now(), deleted_at TIMESTAMPTZ)`,
		`CREATE TABLE sprints (id UUID PRIMARY KEY, project_id UUID NOT NULL REFERENCES projects (id), name TEXT NOT NULL,
			status TEXT NOT NULL, end_date DATE)`,
	} {
		if _, err := db.Exec(statement); err != nil {
			t.Fatal(err)
		}
	}

	me, other := uuid.New(), uuid.New()
	mine, foreign := uuid.New(), uuid.New()
	if _, err := db.Exec(`INSERT INTO projects (id, key, name) VALUES ($1, 'CORE', 'Core'), ($2, 'OPS', 'Ops')`, mine, foreign); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO project_members VALUES ($1, $2)`, mine, me); err != nil {
		t.Fatal(err)
	}
	// The reader runs at noon UTC on 22 November. due_date is a calendar date, so overdue means a due date
	// before 22 November; an issue due today is not overdue yet.
	now := time.Date(2026, 11, 22, 12, 0, 0, 0, time.UTC)
	past := now.AddDate(0, 0, -2).Format("2006-01-02")
	today := now.Format("2006-01-02")
	future := now.AddDate(0, 0, 5).Format("2006-01-02")
	for _, row := range []struct {
		project uuid.UUID
		status  string
		due     *string
		owner   uuid.UUID
	}{
		{mine, "open", &past, me},
		{mine, "in_progress", &future, me},
		{mine, "closed", &past, me},
		{foreign, "open", nil, me},
		{mine, "open", nil, other},
		{mine, "open", &today, me},
	} {
		if _, err := db.Exec(`INSERT INTO issues (id, project_id, summary, status, assignee_id, due_date) VALUES ($1, $2, 'x', $3, $4, $5)`,
			uuid.New(), row.project, row.status, row.owner, row.due); err != nil {
			t.Fatal(err)
		}
	}
	// A soft-deleted issue assigned to me, overdue and open, must not count anywhere.
	if _, err := db.Exec(`INSERT INTO issues (id, project_id, summary, status, assignee_id, due_date, deleted_at) VALUES ($1, $2, 'removed', 'open', $3, $4, now())`,
		uuid.New(), mine, me, past); err != nil {
		t.Fatal(err)
	}
	end := future
	if _, err := db.Exec(`INSERT INTO sprints VALUES ($1, $2, 'Sprint activo', 'active', $3)`, uuid.New(), mine, end); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO sprints VALUES ($1, $2, 'Ajeno', 'active', $3)`, uuid.New(), foreign, end); err != nil {
		t.Fatal(err)
	}

	// A soft-deleted project where I am a member, with an overdue open issue and an active sprint, must not count.
	gone := uuid.New()
	if _, err := db.Exec(`INSERT INTO projects (id, key, name, deleted_at) VALUES ($1, 'GONE', 'Gone', now())`, gone); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO project_members VALUES ($1, $2)`, gone, me); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO issues (id, project_id, summary, status, assignee_id, due_date) VALUES ($1, $2, 'deleted', 'open', $3, $4)`,
		uuid.New(), gone, me, past); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO sprints VALUES ($1, $2, 'Sprint borrado', 'active', $3)`, uuid.New(), gone, end); err != nil {
		t.Fatal(err)
	}

	summary, err := NewDashboardReader(db).Summary(context.Background(), me, now)
	if err != nil {
		t.Fatalf("Summary: %v", err)
	}
	// Open issues: past, future, due today. The closed one and the soft-deleted one are excluded.
	if summary.AssignedOpen != 4 || summary.AssignedByStatus["closed"] != 1 {
		t.Fatalf("assigned open=%d by status=%v; want 4 open and 1 closed", summary.AssignedOpen, summary.AssignedByStatus)
	}
	if summary.Overdue != 1 {
		t.Fatalf("overdue = %d, want 1 (due before today and not closed)", summary.Overdue)
	}
	if summary.ProjectCount != 1 {
		t.Fatalf("project count = %d, want 1", summary.ProjectCount)
	}
	if len(summary.ActiveSprints) != 1 || summary.ActiveSprints[0].Name != "Sprint activo" {
		t.Fatalf("active sprints = %+v, want only the sprint of my project", summary.ActiveSprints)
	}
	if len(summary.RecentIssues) != 4 {
		t.Fatalf("recent issues = %d, want the 4 open issues assigned to me (closed and deleted ones excluded)", len(summary.RecentIssues))
	}
}
