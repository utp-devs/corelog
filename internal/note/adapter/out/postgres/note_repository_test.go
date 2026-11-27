package postgres

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"

	"github.com/jhonsferg/corelog/internal/note/application/port"
	"github.com/jhonsferg/corelog/internal/note/domain"
	"github.com/jhonsferg/corelog/internal/platform/database/testdb"
)

// seedUser inserts a user with the columns the schema requires.
func seedUser(t *testing.T, db *sqlx.DB, role string) uuid.UUID {
	t.Helper()
	id := uuid.New()
	if _, err := db.Exec(`INSERT INTO users (id, name, display_name, email, password_hash, role)
		VALUES ($1, 'Test', 'Test', $2, 'x', $3)`, id, id.String()+"@example.test", role); err != nil {
		t.Fatal(err)
	}
	return id
}

// seedOrg inserts an organization and returns its id.
func seedOrg(t *testing.T, db *sqlx.DB) uuid.UUID {
	t.Helper()
	id := uuid.New()
	if _, err := db.Exec(`INSERT INTO organizations (id, slug, name) VALUES ($1, $2, 'Org')`,
		id, "org-"+strings.ReplaceAll(id.String(), "-", "")[:12]); err != nil {
		t.Fatal(err)
	}
	return id
}

// seedProject inserts a live project in the organization with the given visibility ('open' or 'private').
func seedProject(t *testing.T, db *sqlx.DB, org, lead uuid.UUID, visibility string) uuid.UUID {
	t.Helper()
	id := uuid.New()
	if _, err := db.Exec(`INSERT INTO projects (id, key, name, lead_id, organization_id, visibility)
		VALUES ($1, $2, 'Project', $3, $4, $5)`,
		id, "P"+strings.ToUpper(strings.ReplaceAll(id.String(), "-", "")[:8]), lead, org, visibility); err != nil {
		t.Fatal(err)
	}
	return id
}

// seedIssue inserts an issue in the project.
func seedIssue(t *testing.T, db *sqlx.DB, project, reporter uuid.UUID) uuid.UUID {
	t.Helper()
	id := uuid.New()
	if _, err := db.Exec(`INSERT INTO issues (id, project_id, issue_type, summary, description, status, priority, reporter_id, key_number)
		VALUES ($1, $2, 'task', 'Issue', '', 'open', 'medium', $3, (SELECT COALESCE(MAX(key_number), 0) + 1 FROM issues WHERE project_id = $2))`, id, project, reporter); err != nil {
		t.Fatal(err)
	}
	return id
}

func newNote(t *testing.T, owner uuid.UUID, f domain.Fields, at time.Time) domain.Note {
	t.Helper()
	n, err := domain.NewNote(owner, f, at)
	if err != nil {
		t.Fatal(err)
	}
	return n
}

func TestRepositoryInsertGetAndOwnerScope(t *testing.T) {
	db := testdb.New(t)
	repo := NewRepository(db)
	ctx := context.Background()
	owner, other := seedUser(t, db, "user"), seedUser(t, db, "admin")
	at := time.Date(2026, 11, 27, 10, 0, 0, 0, time.UTC)

	created, err := repo.Insert(ctx, newNote(t, owner, domain.Fields{Title: "Hola", Tags: []string{"a", "b"}}, at))
	if err != nil {
		t.Fatal(err)
	}
	if created.ID == uuid.Nil {
		t.Fatal("database must assign the id")
	}
	got, err := repo.Get(ctx, owner, created.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Title != "Hola" || len(got.Tags) != 2 || got.Tags[0] != "a" || got.Color != domain.DefaultColor {
		t.Fatalf("round trip mismatch: %+v", got)
	}
	if _, err := repo.Get(ctx, other, created.ID); err != domain.ErrNotFound {
		t.Fatalf("admin must not read another owner's note: %v", err)
	}
}

func TestRepositoryEmptyTagsAreNotNull(t *testing.T) {
	db := testdb.New(t)
	repo := NewRepository(db)
	ctx := context.Background()
	owner := seedUser(t, db, "user")
	created, err := repo.Insert(ctx, newNote(t, owner, domain.Fields{}, time.Now()))
	if err != nil {
		t.Fatal(err)
	}
	got, err := repo.Get(ctx, owner, created.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Tags == nil || len(got.Tags) != 0 {
		t.Fatalf("tags must be an empty non-nil slice, got %#v", got.Tags)
	}
}

func TestRepositoryUpdateSoftDeleteAndMissingRows(t *testing.T) {
	db := testdb.New(t)
	repo := NewRepository(db)
	ctx := context.Background()
	owner, other := seedUser(t, db, "user"), seedUser(t, db, "user")
	at := time.Date(2026, 11, 27, 10, 0, 0, 0, time.UTC)
	created, err := repo.Insert(ctx, newNote(t, owner, domain.Fields{Title: "a"}, at))
	if err != nil {
		t.Fatal(err)
	}

	edited := created
	edited.Title = "b"
	edited.Tags = []string{"x"}
	edited.Pinned = true
	edited.UpdatedAt = at.Add(time.Minute)
	if err := repo.Update(ctx, edited); err != nil {
		t.Fatal(err)
	}
	got, err := repo.Get(ctx, owner, created.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Title != "b" || !got.Pinned || len(got.Tags) != 1 || got.Tags[0] != "x" {
		t.Fatalf("update not persisted: %+v", got)
	}

	foreign := created
	foreign.OwnerID = other
	if err := repo.Update(ctx, foreign); err != domain.ErrNotFound {
		t.Fatalf("update by other owner must miss: %v", err)
	}

	if err := repo.SoftDelete(ctx, owner, created.ID, at); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.Get(ctx, owner, created.ID); err != domain.ErrNotFound {
		t.Fatalf("soft deleted note must be hidden: %v", err)
	}
	if err := repo.SoftDelete(ctx, owner, created.ID, at); err != domain.ErrNotFound {
		t.Fatalf("second delete must miss: %v", err)
	}
	if err := repo.Update(ctx, edited); err != domain.ErrNotFound {
		t.Fatalf("update of deleted note must miss: %v", err)
	}
}

func TestRepositoryListOrderingFiltersAndPagination(t *testing.T) {
	db := testdb.New(t)
	repo := NewRepository(db)
	ctx := context.Background()
	owner := seedUser(t, db, "user")
	base := time.Date(2026, 11, 27, 10, 0, 0, 0, time.UTC)

	oldPinned := newNote(t, owner, domain.Fields{Title: "old pinned", Pinned: true, Tags: []string{"work"}}, base)
	newer := newNote(t, owner, domain.Fields{Title: "newer 100% done", Tags: []string{"work"}}, base.Add(time.Hour))
	underscore := newNote(t, owner, domain.Fields{Title: "a_b literal"}, base.Add(2*time.Hour))
	plain := newNote(t, owner, domain.Fields{Title: "plain axb"}, base.Add(3*time.Hour))
	for _, n := range []domain.Note{oldPinned, newer, underscore, plain} {
		if _, err := repo.Insert(ctx, n); err != nil {
			t.Fatal(err)
		}
	}

	all, err := repo.List(ctx, owner, port.ListQuery{Limit: 50})
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 4 || all[0].Title != "old pinned" {
		t.Fatalf("pinned notes must come first: %+v", titles(all))
	}
	if all[1].Title != "plain axb" {
		t.Fatalf("then newest updated first: %v", titles(all))
	}

	byTag, err := repo.List(ctx, owner, port.ListQuery{Tag: "work", Limit: 50})
	if err != nil || len(byTag) != 2 {
		t.Fatalf("tag filter: %v %v", err, titles(byTag))
	}

	pinned := true
	onlyPinned, err := repo.List(ctx, owner, port.ListQuery{Pinned: &pinned, Limit: 50})
	if err != nil || len(onlyPinned) != 1 {
		t.Fatalf("pinned filter: %v %v", err, titles(onlyPinned))
	}

	percent, err := repo.List(ctx, owner, port.ListQuery{Query: "100%", Limit: 50})
	if err != nil || len(percent) != 1 || percent[0].Title != "newer 100% done" {
		t.Fatalf("percent must match literally: %v %v", err, titles(percent))
	}
	underline, err := repo.List(ctx, owner, port.ListQuery{Query: "a_b", Limit: 50})
	if err != nil || len(underline) != 1 {
		t.Fatalf("underscore must match literally: %v %v", err, titles(underline))
	}

	page1, err := repo.List(ctx, owner, port.ListQuery{Limit: 2})
	if err != nil || len(page1) != 3 {
		t.Fatalf("repository returns limit+1 rows: %v %d", err, len(page1))
	}
	last := page1[1]
	page2, err := repo.List(ctx, owner, port.ListQuery{Limit: 10, After: &port.Position{
		Pinned: last.Pinned, UpdatedAt: last.UpdatedAt, ID: last.ID,
	}})
	if err != nil {
		t.Fatal(err)
	}
	if len(page2) != 2 {
		t.Fatalf("keyset page after second row: got %v", titles(page2))
	}
	for _, n := range page2 {
		if n.ID == page1[0].ID || n.ID == page1[1].ID {
			t.Fatal("keyset page repeats an earlier note")
		}
	}
}

func TestRepositoryLinkFiltersAndOwnerScope(t *testing.T) {
	db := testdb.New(t)
	repo := NewRepository(db)
	ctx := context.Background()
	owner, other := seedUser(t, db, "user"), seedUser(t, db, "user")
	org := seedOrg(t, db)
	project := seedProject(t, db, org, owner, "open")
	issue := seedIssue(t, db, project, owner)

	linked := newNote(t, owner, domain.Fields{IssueID: &issue, ProjectID: &project}, time.Now())
	if _, err := repo.Insert(ctx, linked); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.Insert(ctx, newNote(t, owner, domain.Fields{Title: "unlinked"}, time.Now())); err != nil {
		t.Fatal(err)
	}

	byIssue, err := repo.List(ctx, owner, port.ListQuery{IssueID: &issue, Limit: 10})
	if err != nil || len(byIssue) != 1 {
		t.Fatalf("issue filter: %v %d", err, len(byIssue))
	}
	byProject, err := repo.List(ctx, owner, port.ListQuery{ProjectID: &project, Limit: 10})
	if err != nil || len(byProject) != 1 {
		t.Fatalf("project filter: %v %d", err, len(byProject))
	}
	foreign, err := repo.List(ctx, other, port.ListQuery{ProjectID: &project, Limit: 10})
	if err != nil || len(foreign) != 0 {
		t.Fatalf("another owner must see nothing: %v %d", err, len(foreign))
	}
}

func TestRepositoryTagCountsAndSearchOwn(t *testing.T) {
	db := testdb.New(t)
	repo := NewRepository(db)
	ctx := context.Background()
	owner, other := seedUser(t, db, "user"), seedUser(t, db, "user")
	now := time.Now().UTC()

	first, err := repo.Insert(ctx, newNote(t, owner, domain.Fields{Title: "Compra 50%", Tags: []string{"work", "home"}}, now))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := repo.Insert(ctx, newNote(t, owner, domain.Fields{Title: "Trabajo", Tags: []string{"work"}}, now)); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.Insert(ctx, newNote(t, other, domain.Fields{Title: "Trabajo ajeno", Tags: []string{"work"}}, now)); err != nil {
		t.Fatal(err)
	}
	tags, err := repo.ListTags(ctx, owner)
	if err != nil {
		t.Fatal(err)
	}
	if len(tags) != 2 || tags[0].Tag != "work" || tags[0].Count != 2 || tags[1].Tag != "home" || tags[1].Count != 1 {
		t.Fatalf("tag counts: %+v", tags)
	}

	if err := repo.SoftDelete(ctx, owner, first.ID, now); err != nil {
		t.Fatal(err)
	}
	tags, err = repo.ListTags(ctx, owner)
	if err != nil {
		t.Fatal(err)
	}
	if len(tags) != 1 || tags[0].Tag != "work" || tags[0].Count != 1 {
		t.Fatalf("deleted notes must not count: %+v", tags)
	}

	hits, err := repo.SearchOwn(ctx, owner, "trabajo", 10)
	if err != nil || len(hits) != 1 || hits[0].Title != "Trabajo" {
		t.Fatalf("search must stay within the owner: %v %+v", err, hits)
	}
	percent, err := repo.SearchOwn(ctx, owner, "50%", 10)
	if err != nil || len(percent) != 0 {
		t.Fatalf("deleted note must not match and %% must be literal: %v %+v", err, percent)
	}
}

func TestLinkAccessFollowsProjectVisibility(t *testing.T) {
	db := testdb.New(t)
	access := NewLinkAccess(db)
	ctx := context.Background()
	lead := seedUser(t, db, "user")
	member := seedUser(t, db, "user")
	outsider := seedUser(t, db, "user")
	admin := seedUser(t, db, "admin")
	org := seedOrg(t, db)
	if _, err := db.Exec(`INSERT INTO organization_members (organization_id, user_id, role) VALUES ($1, $2, 'member'), ($1, $3, 'member')`,
		org, member, outsider); err != nil {
		t.Fatal(err)
	}
	open := seedProject(t, db, org, lead, "open")
	private := seedProject(t, db, org, lead, "private")
	issueOpen := seedIssue(t, db, open, lead)
	issuePrivate := seedIssue(t, db, private, lead)

	check := func(name string, got bool, err error, want bool) {
		t.Helper()
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if got != want {
			t.Fatalf("%s: got %v, want %v", name, got, want)
		}
	}
	ok, err := access.CanBrowseProject(ctx, member, open)
	check("open project for org member", ok, err, true)
	ok, err = access.CanBrowseProject(ctx, outsider, private)
	check("private project for non member", ok, err, false)
	ok, err = access.CanBrowseProject(ctx, admin, private)
	check("platform admin sees private project", ok, err, true)
	ok, err = access.CanBrowseIssue(ctx, member, issueOpen)
	check("issue of open project", ok, err, true)
	ok, err = access.CanBrowseIssue(ctx, outsider, issuePrivate)
	check("issue of private project", ok, err, false)
	ok, err = access.CanBrowseIssue(ctx, member, uuid.New())
	check("missing issue", ok, err, false)

	if _, err := db.Exec(`UPDATE projects SET deleted_at = now() WHERE id = $1`, open); err != nil {
		t.Fatal(err)
	}
	ok, err = access.CanBrowseProject(ctx, member, open)
	check("deleted project", ok, err, false)
	ok, err = access.CanBrowseIssue(ctx, member, issueOpen)
	check("issue of deleted project", ok, err, false)
}

func titles(notes []domain.Note) []string {
	out := make([]string, 0, len(notes))
	for _, n := range notes {
		out = append(out, n.Title)
	}
	return out
}
