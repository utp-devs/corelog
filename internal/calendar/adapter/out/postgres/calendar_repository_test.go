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

// TestCalendarRepositoryEventsPostgres comprueba que se devuelven inicio y fin de sprint y fechas limite
// dentro del rango, y que los hitos de otro proyecto no aparecen.
func TestCalendarRepositoryEventsPostgres(t *testing.T) {
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

	schema := "calendar_repo_test_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	if _, err := db.Exec(`CREATE SCHEMA ` + schema); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = db.Exec(`DROP SCHEMA ` + schema + ` CASCADE`) })
	if _, err := db.Exec(`SET search_path TO ` + schema); err != nil {
		t.Fatal(err)
	}
	for _, statement := range []string{
		`CREATE TABLE sprints (id UUID PRIMARY KEY, project_id UUID NOT NULL, name TEXT NOT NULL, start_date DATE, end_date DATE)`,
		`CREATE TABLE issues (id UUID PRIMARY KEY, project_id UUID NOT NULL, title TEXT NOT NULL, due_date TIMESTAMPTZ)`,
	} {
		if _, err := db.Exec(statement); err != nil {
			t.Fatal(err)
		}
	}

	project, other := uuid.New(), uuid.New()
	sprint, issue, foreignIssue := uuid.New(), uuid.New(), uuid.New()
	if _, err := db.Exec(`INSERT INTO sprints VALUES ($1, $2, 'Sprint 1', '2026-11-02', '2026-11-08')`, sprint, project); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO sprints VALUES ($1, $2, 'Lejano', '2027-03-01', '2027-03-10')`, uuid.New(), project); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO issues VALUES ($1, $2, 'Entregar', '2026-11-05 15:30:00+00')`, issue, project); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO issues VALUES ($1, $2, 'Ajena', '2026-11-05 10:00:00+00')`, foreignIssue, other); err != nil {
		t.Fatal(err)
	}

	from := time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)
	to := time.Date(2026, 11, 30, 0, 0, 0, 0, time.UTC)
	events, err := NewCalendarRepository(db).Events(context.Background(), project, from, to)
	if err != nil {
		t.Fatalf("Events: %v", err)
	}
	kinds := map[string]int{}
	for _, event := range events {
		kinds[event.Kind]++
		if event.SourceID == foreignIssue {
			t.Fatal("events of another project must not appear")
		}
	}
	if kinds["sprint_start"] != 1 || kinds["sprint_end"] != 1 || kinds["issue_due"] != 1 || len(events) != 3 {
		t.Fatalf("events by kind = %v (total %d), want one start, one end and one due date", kinds, len(events))
	}
	for _, event := range events {
		if event.Kind == "issue_due" && (event.Date.Day() != 5 || event.Link != "/issues/"+issue.String()) {
			t.Fatalf("issue due event = %+v", event)
		}
	}
}
