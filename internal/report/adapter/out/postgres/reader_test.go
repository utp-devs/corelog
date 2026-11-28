package postgres

import (
	"context"
	"database/sql"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"

	"github.com/jhonsferg/corelog/internal/platform/database/testdb"
	"github.com/jhonsferg/corelog/internal/report/application/port"
	"github.com/jhonsferg/corelog/internal/report/domain"
)

const defaultOrg = "00000000-0000-4000-8000-000000000002"

// countingDB counts the statements the reader sends.
type countingDB struct {
	*sqlx.DB
	n *atomic.Int32
}

func (c countingDB) QueryxContext(ctx context.Context, q string, args ...any) (*sqlx.Rows, error) {
	c.n.Add(1)
	return c.DB.QueryxContext(ctx, q, args...)
}

func (c countingDB) QueryRowxContext(ctx context.Context, q string, args ...any) *sqlx.Row {
	c.n.Add(1)
	return c.DB.QueryRowxContext(ctx, q, args...)
}

func (c countingDB) QueryContext(ctx context.Context, q string, args ...any) (*sql.Rows, error) {
	c.n.Add(1)
	return c.DB.QueryContext(ctx, q, args...)
}

type fixture struct {
	t       *testing.T
	db      *sqlx.DB
	n       *atomic.Int32
	reader  *Reader
	project uuid.UUID
	other   uuid.UUID
	user    uuid.UUID
	sprint  uuid.UUID
	version uuid.UUID
}

func (f *fixture) exec(q string, args ...any) {
	f.t.Helper()
	if _, err := f.db.Exec(q, args...); err != nil {
		f.t.Fatalf("%v\n%s", err, q)
	}
}

func newFixture(t *testing.T) *fixture {
	t.Helper()
	db := testdb.New(t)
	n := &atomic.Int32{}
	f := &fixture{t: t, db: db, n: n, reader: NewReader(countingDB{DB: db, n: n}),
		project: uuid.New(), other: uuid.New(), user: uuid.New(), sprint: uuid.New(), version: uuid.New()}
	f.exec(`INSERT INTO users (id, name, display_name, email, password_hash, role) VALUES ($1, 'ada', 'Ada L', 'ada@example.test', 'x', 'user')`, f.user)
	f.exec(`UPDATE organizations SET settings = jsonb_set(settings, '{timezone}', '"America/Bogota"') WHERE id = $1`, defaultOrg)
	for i, id := range []uuid.UUID{f.project, f.other} {
		f.exec(`INSERT INTO projects (id, organization_id, key, name, type, template, lead_id, visibility) VALUES ($1, $2, $3, 'P', 'kanban', 'scrum', $4, 'private')`,
			id, defaultOrg, []string{"RPT", "OTH"}[i], f.user)
		f.exec(`SELECT corelog_ensure_workflow($1)`, id)
	}
	f.exec(`INSERT INTO sprints (id, project_id, name, status, start_date, end_date, started_at) VALUES ($1, $2, 'Sprint 1', 'active', '2024-01-01', '2024-01-14', '2024-01-01T09:00:00Z')`, f.sprint, f.project)
	f.exec(`INSERT INTO versions (id, project_id, name) VALUES ($1, $2, '1.0')`, f.version, f.project)
	return f
}

// issue inserts an issue of the project in the sprint, with a status of the category, one history row, one
// scope event, one worklog and the version link.
func (f *fixture) issue(project uuid.UUID, category string) uuid.UUID {
	f.t.Helper()
	id := uuid.New()
	f.exec(`INSERT INTO issues (id, project_id, summary, description, status, priority, reporter_id, key_number, issue_type_id, priority_id, status_id,
			sprint_id, story_points, original_estimate_seconds, assignee_id, created_at)
		SELECT $1, $2, 'Incidencia', '', 'open', 'medium', $3,
		       (SELECT COALESCE(max(key_number), 0) + 1 FROM issues WHERE project_id = $2),
		       (SELECT it.id FROM issue_types it WHERE it.organization_id = $4 AND it.is_default),
		       (SELECT pr.id FROM priorities pr WHERE pr.organization_id = $4 AND pr.is_default),
		       (SELECT s.id FROM statuses s WHERE s.organization_id = $4 AND s.category = $5 ORDER BY s.position LIMIT 1),
		       $6, 3, 3600, $3, '2024-01-02T10:00:00Z'`, id, project, f.user, defaultOrg, category, f.sprint)
	f.exec(`INSERT INTO issue_status_history (issue_id, to_status_id, changed_at)
		SELECT $1, status_id, '2024-01-03T10:00:00Z' FROM issues WHERE id = $1`, id)
	f.exec(`INSERT INTO sprint_issue_events (sprint_id, issue_id, event, points_at_time, happened_at) VALUES ($1, $2, 'added', 3, '2024-01-01T09:00:00Z')`, f.sprint, id)
	f.exec(`INSERT INTO worklogs (issue_id, author_id, started_at, time_spent_seconds) VALUES ($1, $2, '2024-01-04T10:00:00Z', 1800)`, id, f.user)
	f.exec(`INSERT INTO issue_versions (issue_id, version_id, kind) VALUES ($1, $2, 'fix')`, id, f.version)
	return id
}

// runAll sends every reader query once and returns the statements used.
func (f *fixture) runAll(ctx context.Context) int32 {
	f.t.Helper()
	f.n.Store(0)
	before := ts("2030-01-01T00:00:00Z")
	if _, err := f.reader.Project(ctx, f.project); err != nil {
		f.t.Fatal(err)
	}
	if _, err := f.reader.Sprint(ctx, f.project, nil); err != nil {
		f.t.Fatal(err)
	}
	if _, err := f.reader.RecentSprints(ctx, f.project, 5); err != nil {
		f.t.Fatal(err)
	}
	if _, err := f.reader.ScopeEvents(ctx, f.sprint); err != nil {
		f.t.Fatal(err)
	}
	if _, err := f.reader.Version(ctx, f.project, f.version); err != nil {
		f.t.Fatal(err)
	}
	if _, err := f.reader.Issues(ctx, port.IssueQuery{ProjectID: f.project, VersionID: &f.version, CreatedBefore: &before}); err != nil {
		f.t.Fatal(err)
	}
	if _, err := f.reader.StatusHistory(ctx, port.HistoryQuery{ProjectID: f.project, Before: before}); err != nil {
		f.t.Fatal(err)
	}
	if _, err := f.reader.Worklogs(ctx, port.WorklogQuery{ProjectID: f.project, Lo: ts("2024-01-01T00:00:00Z"), Hi: before}); err != nil {
		f.t.Fatal(err)
	}
	return f.n.Load()
}

func ts(s string) time.Time {
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		panic(err)
	}
	return t
}

func TestReaderStatementsDoNotGrowWithData(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	f.issue(f.project, "todo")
	small := f.runAll(ctx)
	for i := 0; i < 40; i++ {
		f.issue(f.project, []string{"todo", "in_progress", "done"}[i%3])
	}
	large := f.runAll(ctx)
	if small != large {
		t.Fatalf("statements grew with the data: %d with 1 issue, %d with 41", small, large)
	}
	if large != 8 {
		t.Fatalf("statements = %d, want 8 (one per reader call)", large)
	}
}

func TestReaderContentAndScoping(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	todo, done := f.issue(f.project, "todo"), f.issue(f.project, "done")
	foreign := f.issue(f.other, "todo")

	info, err := f.reader.Project(ctx, f.project)
	if err != nil || info.Timezone != "America/Bogota" || info.Key != "RPT" {
		t.Fatalf("project = %+v %v", info, err)
	}
	if _, err := f.reader.Project(ctx, uuid.New()); err == nil {
		t.Fatal("unknown project must fail")
	}

	issues, err := f.reader.Issues(ctx, port.IssueQuery{ProjectID: f.project})
	if err != nil || len(issues) != 2 {
		t.Fatalf("issues = %v %v", issues, err)
	}
	byID := map[uuid.UUID]domain.IssueRow{}
	for _, i := range issues {
		byID[i.ID] = i
	}
	if _, leaked := byID[foreign]; leaked {
		t.Fatal("an issue of another project leaked")
	}
	if byID[todo].Category != "todo" || byID[todo].ResolvedAt != nil || byID[done].Category != "done" || byID[done].ResolvedAt == nil {
		t.Fatalf("categories = %+v", byID)
	}
	if byID[todo].Key != "RPT-1" || byID[todo].AssigneeName != "Ada L" || *byID[todo].Points != 3 || *byID[todo].OriginalEst != 3600 || byID[todo].TypeName == "" {
		t.Fatalf("row = %+v", byID[todo])
	}

	open, _ := f.reader.Issues(ctx, port.IssueQuery{ProjectID: f.project, OpenOnly: true})
	if len(open) != 1 || open[0].ID != todo {
		t.Fatalf("open = %v", open)
	}
	only := []uuid.UUID{done, foreign}
	sel, _ := f.reader.Issues(ctx, port.IssueQuery{ProjectID: f.project, OnlyIDs: &only})
	if len(sel) != 1 || sel[0].ID != done {
		t.Fatalf("only ids = %v", sel)
	}
	none := []uuid.UUID{}
	if sel, _ = f.reader.Issues(ctx, port.IssueQuery{ProjectID: f.project, OnlyIDs: &none}); len(sel) != 0 {
		t.Fatalf("empty id list must match nothing: %v", sel)
	}
	early := ts("2024-01-02T00:00:00Z")
	if sel, _ = f.reader.Issues(ctx, port.IssueQuery{ProjectID: f.project, CreatedBefore: &early}); len(sel) != 0 {
		t.Fatalf("created before: %v", sel)
	}
	late := ts("2024-02-01T00:00:00Z")
	if sel, _ = f.reader.Issues(ctx, port.IssueQuery{ProjectID: f.project, TouchedSince: &late}); len(sel) != 1 {
		t.Fatalf("touched since (only the resolved one changed after the date): %v", sel)
	}
	otherSprint := uuid.New()
	if sel, _ = f.reader.Issues(ctx, port.IssueQuery{ProjectID: f.project, SprintID: &otherSprint}); len(sel) != 0 {
		t.Fatalf("sprint filter: %v", sel)
	}
	f.exec(`UPDATE issues SET deleted_at = now() WHERE id = $1`, todo)
	if sel, _ = f.reader.Issues(ctx, port.IssueQuery{ProjectID: f.project}); len(sel) != 1 {
		t.Fatalf("soft deleted issue must be hidden: %v", sel)
	}

	history, err := f.reader.StatusHistory(ctx, port.HistoryQuery{ProjectID: f.project, Before: ts("2030-01-01T00:00:00Z")})
	if err != nil || len(history) != 1 || history[0].IssueID != done || history[0].Category != "done" {
		t.Fatalf("history = %v %v", history, err)
	}
	if h, _ := f.reader.StatusHistory(ctx, port.HistoryQuery{ProjectID: f.project, Before: ts("2024-01-03T10:00:00Z")}); len(h) != 0 {
		t.Fatalf("history before the change: %v", h)
	}

	logs, err := f.reader.Worklogs(ctx, port.WorklogQuery{ProjectID: f.project, Lo: ts("2024-01-04T00:00:00Z"), Hi: ts("2024-01-05T00:00:00Z")})
	if err != nil || len(logs) != 1 || logs[0].Seconds != 1800 || logs[0].AuthorName != "Ada L" || logs[0].IssueKey != "RPT-2" {
		t.Fatalf("worklogs = %v %v", logs, err)
	}
	if logs, _ = f.reader.Worklogs(ctx, port.WorklogQuery{ProjectID: f.project, Lo: ts("2024-01-05T00:00:00Z"), Hi: ts("2024-01-06T00:00:00Z")}); len(logs) != 0 {
		t.Fatalf("worklogs outside the window: %v", logs)
	}

	events, err := f.reader.ScopeEvents(ctx, f.sprint)
	if err != nil || len(events) != 3 || events[0].Kind != "added" {
		t.Fatalf("events = %v %v", events, err)
	}
}

func TestReaderSprintsAndVersions(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	s, err := f.reader.Sprint(ctx, f.project, nil)
	if err != nil || s.ID != f.sprint || s.Status != "active" || s.StartDate == nil || s.EndDate == nil {
		t.Fatalf("default sprint = %+v %v", s, err)
	}
	if _, err := f.reader.Sprint(ctx, f.other, &f.sprint); !errors.Is(err, domain.ErrSprintNotFound) {
		t.Fatalf("sprint of another project: %v", err)
	}
	closed := uuid.New()
	f.exec(`INSERT INTO sprints (id, project_id, name, status, start_date, end_date, started_at, completed_at, committed_points, completed_points)
		VALUES ($1, $2, 'Sprint 0', 'closed', '2023-12-01', '2023-12-14', '2023-12-01T09:00:00Z', '2023-12-14T17:00:00Z', 10, 8)`, closed, f.project)
	recent, err := f.reader.RecentSprints(ctx, f.project, 5)
	if err != nil || len(recent) != 1 || recent[0].ID != closed || *recent[0].Completed != 8 {
		t.Fatalf("recent = %+v %v", recent, err)
	}
	if v, err := f.reader.Version(ctx, f.project, f.version); err != nil || v.Name != "1.0" || v.Status != "unreleased" {
		t.Fatalf("version = %+v %v", v, err)
	}
	if _, err := f.reader.Version(ctx, f.other, f.version); !errors.Is(err, domain.ErrVersionNotFound) {
		t.Fatalf("version of another project: %v", err)
	}
	empty, err := NewReader(f.db).Sprint(ctx, f.other, nil)
	if !errors.Is(err, domain.ErrSprintNotFound) {
		t.Fatalf("project without sprints: %+v %v", empty, err)
	}
}
