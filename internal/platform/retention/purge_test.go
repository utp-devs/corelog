package retention

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"

	"github.com/jhonsferg/corelog/internal/platform/clock"
	"github.com/jhonsferg/corelog/internal/platform/database/testdb"
)

const orgID = "00000000-0000-4000-8000-000000000002"

var now = time.Date(2026, 11, 27, 10, 0, 0, 0, time.UTC)

type fixture struct {
	t    *testing.T
	db   *sqlx.DB
	user uuid.UUID
}

func (f *fixture) exec(query string, args ...any) {
	f.t.Helper()
	if _, err := f.db.Exec(query, args...); err != nil {
		f.t.Fatalf("%v\n%s", err, query)
	}
}

func (f *fixture) project(key string, deletedDaysAgo int) uuid.UUID {
	id := uuid.New()
	f.exec(`INSERT INTO projects (id, organization_id, key, name, type, template, lead_id, visibility) VALUES ($1, $2, $3, $3, 'kanban', 'scrum', $4, 'open')`, id, orgID, key, f.user)
	f.exec(`SELECT corelog_ensure_workflow($1)`, id)
	if deletedDaysAgo > 0 {
		f.exec(`UPDATE projects SET deleted_at = $2 WHERE id = $1`, id, now.AddDate(0, 0, -deletedDaysAgo))
	}
	return id
}

func (f *fixture) issue(project uuid.UUID, number, deletedDaysAgo int) uuid.UUID {
	id := uuid.New()
	f.exec(`INSERT INTO issues (id, project_id, summary, description, status, priority, reporter_id, key_number, issue_type_id, priority_id, status_id)
		SELECT $1, $2, 's', '', 'open', 'medium', $3, $4,
		       (SELECT it.id FROM issue_types it WHERE it.organization_id = $5 AND it.is_default),
		       (SELECT pr.id FROM priorities pr WHERE pr.organization_id = $5 AND pr.is_default),
		       (SELECT (wf.definition -> 'statuses' -> 0 ->> 'id')::uuid FROM workflows wf WHERE wf.project_id = $2)`, id, project, f.user, number, orgID)
	if deletedDaysAgo > 0 {
		f.exec(`UPDATE issues SET deleted_at = $2 WHERE id = $1`, id, now.AddDate(0, 0, -deletedDaysAgo))
	}
	return id
}

func (f *fixture) count(table string) int {
	var n int
	if err := f.db.Get(&n, `SELECT count(*) FROM `+table); err != nil {
		f.t.Fatal(err)
	}
	return n
}

func newFixture(t *testing.T) *fixture {
	t.Helper()
	f := &fixture{t: t, db: testdb.New(t), user: uuid.New()}
	f.exec(`INSERT INTO users (id, name, display_name, email, password_hash, role) VALUES ($1, 'u', 'u', 'u@example.test', 'x', 'user')`, f.user)
	return f
}

func TestPurgeDryRunOnlyCounts(t *testing.T) {
	f := newFixture(t)
	live := f.project("LIV", 0)
	f.issue(live, 1, 40)
	f.issue(live, 2, 5)
	f.issue(live, 3, 0)
	gone := f.project("OLD", 45)
	res, err := NewPurger(f.db, clock.NewFixed(now), 30*24*time.Hour, true).Run(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if res.IssuesFound != 1 || res.IssuesPurged != 0 || res.ProjectsFound != 1 || res.ProjectsPurged != 0 {
		t.Fatalf("dry run: %+v", res)
	}
	if f.count("issues") != 3 || f.count("projects") < 2 {
		t.Fatal("a dry run deleted rows")
	}
	_ = gone
}

func TestPurgeRemovesOldSoftDeletedRowsAndKeepsTheRest(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	live := f.project("LIV", 0)
	f.issue(live, 1, 40) // purged
	f.issue(live, 2, 5)  // too recent
	f.issue(live, 3, 0)  // live
	empty := f.project("OLD", 45)
	withIssues := f.project("WIT", 45)
	f.issue(withIssues, 1, 0) // live issue keeps its deleted project
	recent := f.project("REC", 3)

	purger := NewPurger(f.db, clock.NewFixed(now), 30*24*time.Hour, false)
	res, err := purger.Run(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if res.IssuesPurged != 1 || res.ProjectsPurged != 1 || res.Blocked != 0 {
		t.Fatalf("result %+v", res)
	}
	exists := func(table string, id uuid.UUID) bool {
		var n int
		_ = f.db.Get(&n, `SELECT count(*) FROM `+table+` WHERE id = $1`, id)
		return n == 1
	}
	if exists("projects", empty) {
		t.Fatal("the old empty project survived")
	}
	for name, id := range map[string]uuid.UUID{"live": live, "with issues": withIssues, "recent": recent} {
		if !exists("projects", id) {
			t.Fatalf("project %s was purged", name)
		}
	}
	if f.count("issues") != 3 {
		t.Fatalf("%d issues left, want 3", f.count("issues"))
	}
	// Running again finds nothing to do.
	if again, err := purger.Run(ctx); err != nil || again.IssuesPurged+again.ProjectsPurged != 0 {
		t.Fatalf("second run %+v %v", again, err)
	}
}

func TestPurgeLeavesRowsThatOthersStillReference(t *testing.T) {
	f := newFixture(t)
	project := f.project("LIV", 0)
	issue := f.issue(project, 1, 40)
	f.exec(`CREATE TABLE issue_pins (issue_id UUID NOT NULL REFERENCES issues (id))`)
	f.exec(`INSERT INTO issue_pins (issue_id) VALUES ($1)`, issue)
	res, err := NewPurger(f.db, clock.NewFixed(now), 0, false).Run(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if res.IssuesFound != 1 || res.IssuesPurged != 0 || res.Blocked != 1 {
		t.Fatalf("result %+v", res)
	}
	if f.count("issues") != 1 {
		t.Fatal("a referenced issue was deleted")
	}
}
