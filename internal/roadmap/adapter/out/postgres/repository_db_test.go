package postgres

import (
	"context"
	"database/sql"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"

	"github.com/jhonsferg/corelog/internal/platform/apperror"
	"github.com/jhonsferg/corelog/internal/platform/database/testdb"
	"github.com/jhonsferg/corelog/internal/roadmap/domain"
)

const defaultOrg = "00000000-0000-4000-8000-000000000002"

type countingQuerier struct {
	*sqlx.DB
	n *atomic.Int32
}

func (c countingQuerier) SelectContext(ctx context.Context, dest any, q string, a ...any) error {
	c.n.Add(1)
	return c.DB.SelectContext(ctx, dest, q, a...)
}

func (c countingQuerier) BeginTxx(ctx context.Context, o *sql.TxOptions) (*sqlx.Tx, error) {
	return c.DB.BeginTxx(ctx, o)
}

type fixture struct {
	t       *testing.T
	db      *sqlx.DB
	repo    *Repository
	counter *atomic.Int32
	user    uuid.UUID
	project uuid.UUID
	other   uuid.UUID
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
	counter := &atomic.Int32{}
	repo := NewRepository(db)
	repo.db = countingQuerier{DB: db, n: counter}
	f := &fixture{t: t, db: db, repo: repo, counter: counter, user: uuid.New(), project: uuid.New(), other: uuid.New()}
	f.exec(`INSERT INTO users (id, name, display_name, email, password_hash, role) VALUES ($1, 'ana', 'Ana', 'ana@example.test', 'x', 'user')`, f.user)
	f.exec(`INSERT INTO projects (id, organization_id, key, name, type, template, lead_id, visibility) VALUES ($1, $3, 'RMP', 'Roadmap', 'kanban', 'scrum', $4, 'open'), ($2, $3, 'OTR', 'Otro', 'kanban', 'scrum', $4, 'open')`, f.project, f.other, defaultOrg, f.user)
	return f
}

type spec struct {
	project  uuid.UUID
	kind     string // epic, story, task
	summary  string
	status   string
	parent   *uuid.UUID
	start    any
	due      any
	points   any
	assignee any
}

func (f *fixture) issue(s spec) uuid.UUID {
	f.t.Helper()
	id := uuid.New()
	f.exec(`INSERT INTO issues (id, project_id, summary, description, status, priority, reporter_id, assignee_id, key_number, issue_type_id, priority_id, parent_id, start_date, due_date, story_points)
		SELECT $1, $2, $3, '', $4, 'medium', $5, $6, (SELECT COALESCE(max(key_number), 0) + 1 FROM issues WHERE project_id = $2),
		       (SELECT it.id FROM issue_types it WHERE it.organization_id = p.organization_id AND it.legacy_key = $7),
		       (SELECT pr.id FROM priorities pr WHERE pr.organization_id = p.organization_id AND pr.is_default), $8, $9, $10, $11
		FROM projects p WHERE p.id = $2`, id, s.project, s.summary, s.status, f.user, s.assignee, s.kind, s.parent, s.start, s.due, s.points)
	return id
}

func TestEpicsDerivationAndStatementCountPostgres(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	p := f.project
	first := f.issue(spec{project: p, kind: "epic", summary: "Plataforma", status: "open", start: "2026-01-05", due: "2026-02-20", assignee: f.user})
	second := f.issue(spec{project: p, kind: "epic", summary: "Pagos", status: "open"})
	f.issue(spec{project: p, kind: "epic", summary: "Vacía", status: "closed"})
	f.issue(spec{project: f.other, kind: "epic", summary: "De otro proyecto", status: "open"})
	f.issue(spec{project: p, kind: "task", summary: "Una tarea no es épica", status: "open"})
	pt := func(v float64) *float64 { return &v }
	f.issue(spec{project: p, kind: "story", summary: "h1", status: "closed", parent: &first, start: "2026-01-10", due: "2026-01-20", points: pt(3)})
	f.issue(spec{project: p, kind: "story", summary: "h2", status: "in_progress", parent: &first, due: "2026-03-30", points: pt(2)})
	f.issue(spec{project: p, kind: "story", summary: "h3", status: "open", parent: &first})
	f.issue(spec{project: p, kind: "story", summary: "p1", status: "closed", parent: &second, start: "2026-04-01", due: "2026-04-10"})
	f.issue(spec{project: p, kind: "story", summary: "p2", status: "open", parent: &second, due: "2026-05-02"})
	f.exec(`UPDATE issues SET deleted_at = now() WHERE summary = 'h3'`)

	measure := func() (int32, []domain.EpicRow) {
		f.counter.Store(0)
		rows, err := f.repo.Epics(ctx, p)
		if err != nil {
			t.Fatal(err)
		}
		return f.counter.Load(), rows
	}
	small, rows := measure()
	if len(rows) != 3 || small != 1 {
		t.Fatalf("epics = %d, statements = %d", len(rows), small)
	}
	byKey := map[string]domain.EpicRow{}
	for _, r := range rows {
		byKey[r.Title] = r
	}
	plat := byKey["Plataforma"]
	if plat.Key != "RMP-1" || plat.Assignee != "Ana" || plat.Start.Format("2006-01-02") != "2026-01-05" || plat.Due.Format("2006-01-02") != "2026-02-20" {
		t.Fatalf("own fields: %+v", plat)
	}
	if plat.Total != 2 || plat.Done != 1 || plat.InProgress != 1 || plat.Todo != 0 || plat.PointsTotal != 5 || plat.PointsDone != 3 {
		t.Fatalf("progress (deleted child excluded): %+v", plat)
	}
	if plat.ChildStart.Format("2006-01-02") != "2026-01-10" || plat.ChildDue.Format("2006-01-02") != "2026-03-30" {
		t.Fatalf("child dates: %v %v", plat.ChildStart, plat.ChildDue)
	}
	pagos := byKey["Pagos"]
	if pagos.Start != nil || pagos.Due != nil || pagos.ChildStart.Format("2006-01-02") != "2026-04-01" || pagos.ChildDue.Format("2006-01-02") != "2026-05-02" || pagos.Total != 2 || pagos.Done != 1 || pagos.Todo != 1 {
		t.Fatalf("derived epic: %+v", pagos)
	}
	// The due-only child contributes its due date as the start when it has no start date.
	f.issue(spec{project: p, kind: "story", summary: "solo fin", status: "open", parent: &second, due: "2026-03-15"})
	if _, rows := measure(); rows[1].ChildStart.Format("2006-01-02") != "2026-03-15" && rows[0].ChildStart.Format("2006-01-02") != "2026-03-15" {
		t.Fatalf("due-only child start: %+v", rows)
	}
	if byKey["Vacía"].StatusCategory != "done" || byKey["Vacía"].Total != 0 || byKey["Vacía"].ChildStart != nil {
		t.Fatalf("empty epic: %+v", byKey["Vacía"])
	}

	// The number of statements does not grow with the epics.
	for i := 0; i < 30; i++ {
		e := f.issue(spec{project: p, kind: "epic", summary: "Carga", status: "open"})
		f.issue(spec{project: p, kind: "story", summary: "hija", status: "open", parent: &e, due: "2026-06-01"})
	}
	if large, rows := measure(); large != small || len(rows) != 33 {
		t.Fatalf("statements: %d with 3 epics, %d with 33", small, large)
	}
}

func TestRankOrdersEpicsAndRejectsForeignIDsPostgres(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	a := f.issue(spec{project: f.project, kind: "epic", summary: "A", status: "open"})
	b := f.issue(spec{project: f.project, kind: "epic", summary: "B", status: "open"})
	c := f.issue(spec{project: f.project, kind: "epic", summary: "C", status: "open"})
	foreign := f.issue(spec{project: f.other, kind: "epic", summary: "X", status: "open"})
	task := f.issue(spec{project: f.project, kind: "task", summary: "T", status: "open"})
	order := func() string {
		rows, err := f.repo.Epics(ctx, f.project)
		if err != nil {
			t.Fatal(err)
		}
		out := ""
		for _, r := range rows {
			out += r.Title
		}
		return out
	}
	if got := order(); got != "ABC" {
		t.Fatalf("default order = %q", got)
	}
	if err := f.repo.SaveRank(ctx, f.project, []uuid.UUID{c, a}); err != nil {
		t.Fatal(err)
	}
	// Ranked epics first, in the given order; the rest keep their default order after them.
	if got := order(); got != "CAB" {
		t.Fatalf("after rank = %q", got)
	}
	if err := f.repo.SaveRank(ctx, f.project, []uuid.UUID{b, c, a}); err != nil {
		t.Fatal(err)
	}
	if got := order(); got != "BCA" {
		t.Fatalf("second rank = %q", got)
	}
	for name, bad := range map[string][]uuid.UUID{"foreign epic": {a, foreign}, "not an epic": {a, task}, "unknown id": {uuid.New()}} {
		if err := f.repo.SaveRank(ctx, f.project, bad); apperror.CodeOf(err) != domain.CodeInvalidRank {
			t.Errorf("%s: err = %v", name, err)
		}
	}
	if got := order(); got != "BCA" {
		t.Fatalf("a rejected rank changed the order: %q", got)
	}
	if err := f.repo.SaveRank(ctx, f.project, nil); err != nil {
		t.Fatal(err)
	}
	if got := order(); got != "ABC" {
		t.Fatalf("cleared rank = %q", got)
	}
	var n int
	if err := f.db.Get(&n, `SELECT count(*) FROM roadmap_epic_ranks`); err != nil || n != 0 {
		t.Fatalf("ranks left: %d %v", n, err)
	}
}

func TestVersionsSprintsAndDependenciesPostgres(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	p := f.project
	e1 := f.issue(spec{project: p, kind: "epic", summary: "E1", status: "open"})
	e2 := f.issue(spec{project: p, kind: "epic", summary: "E2", status: "open"})
	e3 := f.issue(spec{project: p, kind: "epic", summary: "E3", status: "open"})
	task := f.issue(spec{project: p, kind: "task", summary: "tarea", status: "open"})
	foreign := f.issue(spec{project: f.other, kind: "epic", summary: "ajena", status: "open"})
	link := func(src, dst uuid.UUID, kind string) {
		f.exec(`INSERT INTO issue_links (id, source_issue_id, target_issue_id, kind, created_by) VALUES ($1, $2, $3, $4, $5)`, uuid.New(), src, dst, kind, f.user)
	}
	link(e1, e2, "blocks")
	link(e2, e3, "blocks")
	link(e3, e1, "relates")
	link(e1, task, "blocks")
	link(e1, foreign, "blocks")
	deps, err := f.repo.Dependencies(ctx, p)
	if err != nil || len(deps) != 2 || deps[0].From != e1 || deps[0].To != e2 || deps[1].From != e2 || deps[1].To != e3 || deps[0].Kind != "blocks" {
		t.Fatalf("dependencies: %+v %v", deps, err)
	}
	if deps, _ := f.repo.Dependencies(ctx, f.other); len(deps) != 0 {
		t.Fatalf("dependencies of the other project: %+v", deps)
	}

	v1, v2, archived := uuid.New(), uuid.New(), uuid.New()
	f.exec(`INSERT INTO versions (id, project_id, name, start_date, release_date, status) VALUES ($1, $3, 'v1', '2026-03-01', '2026-03-31', 'unreleased'), ($2, $3, 'v0', NULL, NULL, 'unreleased')`, v1, v2, p)
	f.exec(`INSERT INTO versions (id, project_id, name, release_date, status, archived_at) VALUES ($1, $2, 'viejo', '2025-01-01', 'archived', now())`, archived, p)
	s1 := f.issue(spec{project: p, kind: "task", summary: "hecha", status: "closed", points: 5.0})
	s2 := f.issue(spec{project: p, kind: "task", summary: "en curso", status: "in_progress"})
	f.exec(`INSERT INTO issue_versions (issue_id, version_id, kind) VALUES ($1, $3, 'fix'), ($2, $3, 'fix'), ($1, $3, 'affects')`, s1, s2, v1)
	versions, err := f.repo.Versions(ctx, p)
	if err != nil || len(versions) != 2 || versions[0].Name != "v1" || versions[1].Name != "v0" {
		t.Fatalf("versions: %+v %v", versions, err)
	}
	if v := versions[0]; v.Total != 2 || v.Done != 1 || v.InProgress != 1 || v.PointsTotal != 5 || v.PointsDone != 5 || v.Kind != "version" ||
		v.Start.Format("2006-01-02") != "2026-03-01" || v.End.Format("2006-01-02") != "2026-03-31" {
		t.Fatalf("version progress: %+v", v)
	}
	if v := versions[1]; v.Total != 0 || v.Start != nil || v.End != nil {
		t.Fatalf("empty version: %+v", v)
	}

	sp := uuid.New()
	f.exec(`INSERT INTO sprints (id, project_id, name, status, start_date, end_date) VALUES ($1, $2, 'Sprint 1', 'active', '2026-03-02', '2026-03-15')`, sp, p)
	f.exec(`INSERT INTO sprints (id, project_id, name, status) VALUES ($1, $2, 'Futuro', 'planned')`, uuid.New(), p)
	f.exec(`UPDATE issues SET sprint_id = $1 WHERE id IN ($2, $3)`, sp, s1, s2)
	sprints, err := f.repo.Sprints(ctx, p)
	if err != nil || len(sprints) != 2 || sprints[0].Name != "Sprint 1" || sprints[0].Total != 2 || sprints[0].Done != 1 || sprints[0].Kind != "sprint" || sprints[1].Name != "Futuro" || sprints[1].Start != nil {
		t.Fatalf("sprints: %+v %v", sprints, err)
	}
	if sprints, _ := f.repo.Sprints(ctx, f.other); len(sprints) != 0 {
		t.Fatalf("sprints of the other project: %+v", sprints)
	}
	var _ = time.Now
}
