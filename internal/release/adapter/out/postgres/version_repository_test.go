package postgres_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"

	"github.com/jhonsferg/corelog/internal/platform/apperror"
	"github.com/jhonsferg/corelog/internal/platform/database/testdb"
	"github.com/jhonsferg/corelog/internal/release/adapter/out/postgres"
	"github.com/jhonsferg/corelog/internal/release/domain"
)

// testTime is the fixed clock of the repository tests.
func testTime() time.Time { return time.Date(2026, 11, 27, 9, 0, 0, 0, time.UTC) }

// testOrg is the organization the test database seeds.
const testOrg = "00000000-0000-4000-8000-000000000002"

type repoEnv struct {
	t       *testing.T
	db      *sqlx.DB
	repo    *postgres.Repository
	ctx     context.Context
	lead    uuid.UUID
	project uuid.UUID
	keyNum  int
}

func newRepoEnv(t *testing.T) *repoEnv {
	t.Helper()
	db := testdb.New(t)
	e := &repoEnv{t: t, db: db, repo: postgres.NewRepository(db), ctx: context.Background(), lead: uuid.New(), project: uuid.New()}
	e.exec(`INSERT INTO users (id, name, display_name, email, password_hash, role) VALUES ($1, 'lead', 'lead', 'lead@example.test', 'x', 'user')`, e.lead)
	e.exec(`INSERT INTO projects (id, organization_id, key, name, type, template, lead_id, visibility)
		VALUES ($1, $2, 'REL', 'Releases', 'scrum', 'scrum', $3, 'private')`, e.project, testOrg, e.lead)
	e.exec(`SELECT corelog_ensure_workflow($1)`, e.project)
	return e
}

func (e *repoEnv) exec(query string, args ...any) {
	e.t.Helper()
	if _, err := e.db.Exec(query, args...); err != nil {
		e.t.Fatalf("%v\n%s", err, query)
	}
}

// issue inserts a live issue whose category follows the legacy status ("open", "in_progress" or "closed").
func (e *repoEnv) issue(status string, points float64) uuid.UUID {
	e.t.Helper()
	e.keyNum++
	id := uuid.New()
	e.exec(`INSERT INTO issues (id, project_id, summary, description, status, priority, reporter_id, issue_type, key_number, rank, story_points, issue_type_id, priority_id)
		VALUES ($1, $2, $3, '', $4, 'medium', $5, 'task', $6, $7, $8,
			(SELECT id FROM issue_types WHERE organization_id = $9 AND legacy_key = 'task'),
			(SELECT id FROM priorities WHERE organization_id = $9 AND legacy_key = 'medium'))`,
		id, e.project, "Incidencia", status, e.lead, e.keyNum, "r"+string(rune('a'+e.keyNum)), points, testOrg)
	return id
}

func (e *repoEnv) newVersion(name string) *domain.Version {
	e.t.Helper()
	v, err := domain.NewVersion(e.project, name, "", nil, nil, testTime())
	if err != nil {
		e.t.Fatal(err)
	}
	if err := e.repo.Create(e.ctx, v); err != nil {
		e.t.Fatalf("create %s: %v", name, err)
	}
	return v
}

func (e *repoEnv) link(issue, version uuid.UUID, kind string) {
	e.t.Helper()
	e.exec(`INSERT INTO issue_versions (issue_id, version_id, kind) VALUES ($1, $2, $3)`, issue, version, kind)
}

func TestVersionRepositoryRoundTripAndNameRules(t *testing.T) {
	e := newRepoEnv(t)
	v := e.newVersion("1.0.0")
	if v.ID == uuid.Nil {
		t.Fatal("id not assigned")
	}
	got, err := e.repo.FindByID(e.ctx, v.ID)
	if err != nil || got.Name != "1.0.0" || got.Status != domain.StatusUnreleased {
		t.Fatalf("find: %v %+v", err, got)
	}
	if _, err := e.repo.FindByID(e.ctx, uuid.New()); apperror.CodeOf(err) != "version.not_found" {
		t.Fatalf("missing version: %v", err)
	}

	// Names are unique per project ignoring case.
	dup, _ := domain.NewVersion(e.project, "1.0.0", "", nil, nil, testTime())
	if err := e.repo.Create(e.ctx, dup); apperror.CodeOf(err) != "version.name_taken" {
		t.Fatalf("duplicate name accepted: %v", err)
	}
	taken, err := e.repo.NameTaken(e.ctx, e.project, "1.0.0", uuid.Nil)
	if err != nil || !taken {
		t.Fatalf("NameTaken = %v %v", taken, err)
	}
	if taken, _ := e.repo.NameTaken(e.ctx, e.project, "1.0.0", v.ID); taken {
		t.Fatal("a version does not clash with itself")
	}

	// Other projects may reuse the name.
	other := uuid.New()
	e.exec(`INSERT INTO projects (id, organization_id, key, name, type, template, lead_id, visibility)
		VALUES ($1, $2, 'OTH', 'Other', 'scrum', 'scrum', $3, 'private')`, other, testOrg, e.lead)
	foreign, _ := domain.NewVersion(other, "1.0.0", "", nil, nil, testTime())
	if err := e.repo.Create(e.ctx, foreign); err != nil {
		t.Fatalf("same name in another project: %v", err)
	}
}

func TestVersionRepositoryLockAndSave(t *testing.T) {
	e := newRepoEnv(t)
	v := e.newVersion("2.0")
	locked, err := e.repo.LockByID(e.ctx, v.ID)
	if err != nil || locked.ID != v.ID {
		t.Fatalf("lock: %v", err)
	}
	if err := locked.Archive(testTime()); err != nil {
		t.Fatal(err)
	}
	if err := e.repo.Save(e.ctx, locked); err != nil {
		t.Fatal(err)
	}
	got, _ := e.repo.FindByID(e.ctx, v.ID)
	if got.Status != domain.StatusArchived || got.ArchivedAt == nil {
		t.Fatalf("archived not stored: %+v", got)
	}
}

func TestProgressCountsFixIssuesByCategory(t *testing.T) {
	e := newRepoEnv(t)
	v := e.newVersion("3.0")
	e.link(e.issue("closed", 5), v.ID, "fix")
	e.link(e.issue("open", 3), v.ID, "fix")
	e.link(e.issue("in_progress", 2), v.ID, "fix")
	e.link(e.issue("open", 13), v.ID, "affects") // affects links do not count as progress

	rows, err := e.repo.Progress(e.ctx, []uuid.UUID{v.ID})
	if err != nil {
		t.Fatal(err)
	}
	p := domain.NewProgress(rows[v.ID])
	if p.IssuesTotal != 3 || p.IssuesDone != 1 || p.PointsTotal != 10 || p.PointsDone != 5 {
		t.Fatalf("progress %+v", p)
	}
	if p.ByCategory[domain.CategoryTodo] != 1 || p.ByCategory[domain.CategoryInProgress] != 1 {
		t.Fatalf("by category %v", p.ByCategory)
	}
}

func TestMoveUnresolvedKeepsDoneIssues(t *testing.T) {
	e := newRepoEnv(t)
	source := e.newVersion("4.0")
	target := e.newVersion("4.1")
	done := e.issue("closed", 1)
	open := e.issue("open", 1)
	e.link(done, source.ID, "fix")
	e.link(open, source.ID, "fix")

	count, err := e.repo.UnresolvedFixCount(e.ctx, source.ID)
	if err != nil || count != 1 {
		t.Fatalf("unresolved = %d %v", count, err)
	}
	moved, err := e.repo.MoveUnresolvedFixes(e.ctx, source.ID, target.ID)
	if err != nil || moved != 1 {
		t.Fatalf("moved = %d %v", moved, err)
	}
	assertLink(t, e, done, source.ID, "fix", true)
	assertLink(t, e, done, target.ID, "fix", false)
	assertLink(t, e, open, source.ID, "fix", false)
	assertLink(t, e, open, target.ID, "fix", true)
}

func TestMoveLinksAndDeleteCascade(t *testing.T) {
	e := newRepoEnv(t)
	from := e.newVersion("5.0")
	to := e.newVersion("5.1")
	issue := e.issue("open", 1)
	e.link(issue, from.ID, "fix")
	e.link(issue, from.ID, "affects")
	if err := e.repo.MoveLinks(e.ctx, from.ID, to.ID); err != nil {
		t.Fatal(err)
	}
	assertLink(t, e, issue, to.ID, "fix", true)
	assertLink(t, e, issue, to.ID, "affects", true)
	if err := e.repo.Delete(e.ctx, from.ID); err != nil {
		t.Fatal(err)
	}
	assertLink(t, e, issue, from.ID, "fix", false)
	if err := e.repo.Delete(e.ctx, from.ID); apperror.CodeOf(err) != "version.not_found" {
		t.Fatalf("second delete: %v", err)
	}
}

func TestNotesReadFixIssuesOfTheVersion(t *testing.T) {
	e := newRepoEnv(t)
	v := e.newVersion("6.0")
	fixed := e.issue("closed", 1)
	e.link(fixed, v.ID, "fix")
	e.link(e.issue("open", 1), v.ID, "affects")
	notes, err := e.repo.NotesIssues(e.ctx, v.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(notes) != 1 || notes[0].ID != fixed || !notes[0].Done || notes[0].Key == "" {
		t.Fatalf("notes = %+v", notes)
	}
}

func assertLink(t *testing.T, e *repoEnv, issue, version uuid.UUID, kind string, want bool) {
	t.Helper()
	var found bool
	if err := e.db.Get(&found, `SELECT EXISTS (SELECT 1 FROM issue_versions WHERE issue_id = $1 AND version_id = $2 AND kind = $3)`, issue, version, kind); err != nil {
		t.Fatal(err)
	}
	if found != want {
		t.Fatalf("link %s issue=%s version=%s = %v, want %v", kind, issue, version, found, want)
	}
}
