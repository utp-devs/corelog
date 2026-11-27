package usecase

import (
	"context"
	"errors"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/jhonsferg/corelog/internal/note/application/port"
	"github.com/jhonsferg/corelog/internal/note/domain"
	"github.com/jhonsferg/corelog/internal/platform/apperror"
	"github.com/jhonsferg/corelog/internal/platform/clock"
	"github.com/jhonsferg/corelog/internal/platform/pagination"
)

// fakeRepo keeps notes in memory and scopes every call to the owner, like the real repository.
type fakeRepo struct {
	notes map[uuid.UUID]domain.Note
	tick  int
}

func newFakeRepo() *fakeRepo { return &fakeRepo{notes: map[uuid.UUID]domain.Note{}} }

func (f *fakeRepo) Insert(_ context.Context, n domain.Note) (domain.Note, error) {
	n.ID = uuid.New()
	f.notes[n.ID] = n
	return n, nil
}

func (f *fakeRepo) Update(_ context.Context, n domain.Note) error {
	cur, ok := f.notes[n.ID]
	if !ok || cur.OwnerID != n.OwnerID {
		return domain.ErrNotFound
	}
	f.notes[n.ID] = n
	return nil
}

func (f *fakeRepo) Get(_ context.Context, ownerID, id uuid.UUID) (domain.Note, error) {
	n, ok := f.notes[id]
	if !ok || n.OwnerID != ownerID {
		return domain.Note{}, domain.ErrNotFound
	}
	return n, nil
}

func (f *fakeRepo) SoftDelete(_ context.Context, ownerID, id uuid.UUID, _ time.Time) error {
	n, ok := f.notes[id]
	if !ok || n.OwnerID != ownerID {
		return domain.ErrNotFound
	}
	delete(f.notes, id)
	return nil
}

func (f *fakeRepo) List(_ context.Context, ownerID uuid.UUID, q port.ListQuery) ([]domain.Note, error) {
	var out []domain.Note
	for _, n := range f.notes {
		if n.OwnerID != ownerID {
			continue
		}
		if q.Query != "" && !strings.Contains(strings.ToLower(n.Title+" "+n.Body), strings.ToLower(q.Query)) {
			continue
		}
		if q.Tag != "" && !contains(n.Tags, q.Tag) {
			continue
		}
		if q.Pinned != nil && n.Pinned != *q.Pinned {
			continue
		}
		out = append(out, n)
	}
	sort.Slice(out, func(i, j int) bool {
		a, b := out[i], out[j]
		if a.Pinned != b.Pinned {
			return a.Pinned
		}
		if !a.UpdatedAt.Equal(b.UpdatedAt) {
			return a.UpdatedAt.After(b.UpdatedAt)
		}
		return a.ID.String() > b.ID.String()
	})
	if q.After != nil {
		var rest []domain.Note
		for _, n := range out {
			if sortsAfter(n, q.After) {
				rest = append(rest, n)
			}
		}
		out = rest
	}
	if len(out) > q.Limit+1 {
		out = out[:q.Limit+1]
	}
	return out, nil
}

// sortsAfter reports whether n comes later than p in the list order (pinned, updated_at, id; descending).
func sortsAfter(n domain.Note, p *port.Position) bool {
	if n.Pinned != p.Pinned {
		return !n.Pinned && p.Pinned
	}
	if !n.UpdatedAt.Equal(p.UpdatedAt) {
		return n.UpdatedAt.Before(p.UpdatedAt)
	}
	return n.ID.String() < p.ID.String()
}

func (f *fakeRepo) ListTags(_ context.Context, ownerID uuid.UUID) ([]port.TagCount, error) {
	counts := map[string]int{}
	for _, n := range f.notes {
		if n.OwnerID != ownerID {
			continue
		}
		for _, t := range n.Tags {
			counts[t]++
		}
	}
	var out []port.TagCount
	for t, c := range counts {
		out = append(out, port.TagCount{Tag: t, Count: c})
	}
	return out, nil
}

func (f *fakeRepo) SearchOwn(_ context.Context, ownerID uuid.UUID, query string, limit int) ([]port.SearchHit, error) {
	var out []port.SearchHit
	for _, n := range f.notes {
		if n.OwnerID == ownerID && strings.Contains(strings.ToLower(n.Title+" "+n.Body), strings.ToLower(query)) {
			out = append(out, port.SearchHit{ID: n.ID, Title: n.Title, UpdatedAt: n.UpdatedAt})
		}
	}
	if len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

func contains(items []string, v string) bool {
	for _, i := range items {
		if i == v {
			return true
		}
	}
	return false
}

// fakeLinks grants browsing to the listed identifiers only.
type fakeLinks struct {
	issues   map[uuid.UUID]bool
	projects map[uuid.UUID]bool
	err      error
}

func (f *fakeLinks) CanBrowseIssue(_ context.Context, _, issueID uuid.UUID) (bool, error) {
	return f.issues[issueID], f.err
}

func (f *fakeLinks) CanBrowseProject(_ context.Context, _, projectID uuid.UUID) (bool, error) {
	return f.projects[projectID], f.err
}

func newTestService() (*Service, *fakeRepo, *fakeLinks, *clock.FakeClock) {
	repo := newFakeRepo()
	links := &fakeLinks{issues: map[uuid.UUID]bool{}, projects: map[uuid.UUID]bool{}}
	clk := &clock.FakeClock{T: time.Date(2026, 11, 27, 10, 0, 0, 0, time.UTC)}
	return NewService(repo, links, clk), repo, links, clk
}

func assertErr(t *testing.T, err error, code string, kind apperror.Kind) {
	t.Helper()
	if err == nil {
		t.Fatalf("expected %s", code)
	}
	if apperror.CodeOf(err) != code || apperror.KindOf(err) != kind {
		t.Fatalf("got code %q kind %s, want %q %s (%v)", apperror.CodeOf(err), apperror.KindOf(err), code, kind, err)
	}
}

func TestOtherUsersAndAdminsGetNotFound(t *testing.T) {
	svc, _, _, _ := newTestService()
	ctx := context.Background()
	owner, other := uuid.New(), uuid.New()
	created, err := svc.Create(ctx, owner, domain.Fields{Title: "secret"})
	if err != nil {
		t.Fatal(err)
	}
	title := "hijack"
	if _, err := svc.Get(ctx, other, created.ID); err != domain.ErrNotFound {
		t.Fatalf("get by other user: %v", err)
	}
	if _, err := svc.Update(ctx, other, created.ID, domain.Patch{Title: &title}); err != domain.ErrNotFound {
		t.Fatalf("update by other user: %v", err)
	}
	if err := svc.Delete(ctx, other, created.ID); err != domain.ErrNotFound {
		t.Fatalf("delete by other user: %v", err)
	}
	if _, err := svc.SetPinned(ctx, other, created.ID, true); err != domain.ErrNotFound {
		t.Fatalf("pin by other user: %v", err)
	}
	page, err := svc.List(ctx, other, port.ListQuery{})
	if err != nil || len(page.Items) != 0 {
		t.Fatalf("other user must not list notes: %v %d", err, len(page.Items))
	}
	hits, err := svc.SearchOwn(ctx, other, "secret", 10)
	if err != nil || len(hits) != 0 {
		t.Fatalf("other user must not find notes: %v %d", err, len(hits))
	}
}

func TestCreateWithLinksRequiresBrowseAccess(t *testing.T) {
	svc, _, links, _ := newTestService()
	ctx := context.Background()
	user := uuid.New()
	issue, project := uuid.New(), uuid.New()

	_, err := svc.Create(ctx, user, domain.Fields{IssueID: &issue})
	assertErr(t, err, "note.link_forbidden", apperror.KindNotFound)

	links.issues[issue] = true
	_, err = svc.Create(ctx, user, domain.Fields{IssueID: &issue})
	if err != nil {
		t.Fatalf("linked issue should be allowed: %v", err)
	}

	_, err = svc.Create(ctx, user, domain.Fields{ProjectID: &project})
	assertErr(t, err, "note.link_forbidden", apperror.KindNotFound)

	links.err = errors.New("db down")
	_, err = svc.Create(ctx, user, domain.Fields{ProjectID: &project})
	if err == nil || apperror.KindOf(err) != apperror.KindInternal {
		t.Fatalf("access check failure must be internal, got %v", err)
	}
}

func TestUpdateChecksOnlyChangedLinks(t *testing.T) {
	svc, _, links, _ := newTestService()
	ctx := context.Background()
	user := uuid.New()
	issue := uuid.New()
	links.issues[issue] = true
	n, err := svc.Create(ctx, user, domain.Fields{IssueID: &issue})
	if err != nil {
		t.Fatal(err)
	}
	// Access to the issue is lost; editing the title must keep working.
	delete(links.issues, issue)
	title := "still mine"
	if _, err := svc.Update(ctx, user, n.ID, domain.Patch{Title: &title}); err != nil {
		t.Fatalf("editing an unchanged link must not be blocked: %v", err)
	}
	// Re-linking to the same issue is not a new link either.
	if _, err := svc.Update(ctx, user, n.ID, domain.Patch{IssueID: &issue}); err != nil {
		t.Fatalf("same link must not be rechecked: %v", err)
	}
	other := uuid.New()
	_, err = svc.Update(ctx, user, n.ID, domain.Patch{IssueID: &other})
	assertErr(t, err, "note.link_forbidden", apperror.KindNotFound)
}

func TestCreateLimitsAndTagNormalization(t *testing.T) {
	svc, _, _, _ := newTestService()
	ctx := context.Background()
	user := uuid.New()

	n, err := svc.Create(ctx, user, domain.Fields{Tags: []string{" Ideas ", "ideas", "Work"}})
	if err != nil {
		t.Fatal(err)
	}
	if len(n.Tags) != 2 || n.Tags[0] != "ideas" || n.Tags[1] != "work" {
		t.Fatalf("tags not normalized: %v", n.Tags)
	}

	_, err = svc.Create(ctx, user, domain.Fields{Title: strings.Repeat("t", 121)})
	assertErr(t, err, "note.invalid", apperror.KindValidation)

	_, err = svc.Create(ctx, user, domain.Fields{Body: strings.Repeat("b", 20001)})
	assertErr(t, err, "note.invalid", apperror.KindValidation)

	_, err = svc.Create(ctx, user, domain.Fields{Color: "magenta"})
	assertErr(t, err, "note.invalid", apperror.KindValidation)
}

func TestListLimitBounds(t *testing.T) {
	svc, _, _, _ := newTestService()
	ctx := context.Background()
	user := uuid.New()
	for _, limit := range []int{-1, 101} {
		_, err := svc.List(ctx, user, port.ListQuery{Limit: limit})
		assertErr(t, err, "note.invalid", apperror.KindValidation)
	}
}

func TestListPaginatesWithCursor(t *testing.T) {
	svc, _, _, clk := newTestService()
	ctx := context.Background()
	user := uuid.New()
	for i := 0; i < 5; i++ {
		clk.Advance(time.Minute)
		if _, err := svc.Create(ctx, user, domain.Fields{Title: "n"}); err != nil {
			t.Fatal(err)
		}
	}
	seen := map[uuid.UUID]bool{}
	var cursor string
	pages := 0
	for {
		var pos *port.Position
		if cursor != "" {
			var p port.Position
			if err := pagination.DecodeCursor(cursor, &p); err != nil {
				t.Fatal(err)
			}
			pos = &p
		}
		page, err := svc.List(ctx, user, port.ListQuery{Limit: 2, After: pos})
		if err != nil {
			t.Fatal(err)
		}
		pages++
		for _, n := range page.Items {
			if seen[n.ID] {
				t.Fatalf("note %s returned twice", n.ID)
			}
			seen[n.ID] = true
		}
		if page.NextCursor == "" {
			break
		}
		cursor = page.NextCursor
		if pages > 5 {
			t.Fatal("pagination does not terminate")
		}
	}
	if len(seen) != 5 || pages != 3 {
		t.Fatalf("saw %d notes in %d pages, want 5 in 3", len(seen), pages)
	}
}

func TestPinIsIdempotentAndTouchesUpdatedAt(t *testing.T) {
	svc, _, _, clk := newTestService()
	ctx := context.Background()
	user := uuid.New()
	n, err := svc.Create(ctx, user, domain.Fields{})
	if err != nil {
		t.Fatal(err)
	}
	clk.Advance(time.Hour)
	pinned, err := svc.SetPinned(ctx, user, n.ID, true)
	if err != nil || !pinned.Pinned || !pinned.UpdatedAt.Equal(clk.T) {
		t.Fatalf("pin: %v %+v", err, pinned)
	}
	clk.Advance(time.Hour)
	again, err := svc.SetPinned(ctx, user, n.ID, true)
	if err != nil || !again.UpdatedAt.Equal(pinned.UpdatedAt) {
		t.Fatalf("second pin must not change the note: %v %+v", err, again)
	}
}

func TestDeleteIsSoft(t *testing.T) {
	svc, repo, _, _ := newTestService()
	ctx := context.Background()
	user := uuid.New()
	n, err := svc.Create(ctx, user, domain.Fields{Title: "gone"})
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.Delete(ctx, user, n.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.Get(ctx, user, n.ID); err != domain.ErrNotFound {
		t.Fatalf("deleted note must be hidden: %v", err)
	}
	assertErr(t, svc.Delete(ctx, user, n.ID), "note.not_found", apperror.KindNotFound)
}

func TestSearchOwnRules(t *testing.T) {
	svc, _, _, _ := newTestService()
	ctx := context.Background()
	user := uuid.New()
	if _, err := svc.Create(ctx, user, domain.Fields{Title: "Plan de compras"}); err != nil {
		t.Fatal(err)
	}
	hits, err := svc.SearchOwn(ctx, user, "   ", 10)
	if err != nil || len(hits) != 0 {
		t.Fatalf("blank query must return no hits: %v %d", err, len(hits))
	}
	hits, err = svc.SearchOwn(ctx, user, "compras", 0)
	if err != nil || len(hits) != 1 {
		t.Fatalf("expected one hit with default limit: %v %d", err, len(hits))
	}
}

func TestListTagsDelegates(t *testing.T) {
	svc, _, _, _ := newTestService()
	ctx := context.Background()
	user := uuid.New()
	if _, err := svc.Create(ctx, user, domain.Fields{Tags: []string{"a", "b"}}); err != nil {
		t.Fatal(err)
	}
	tags, err := svc.ListTags(ctx, user)
	if err != nil || len(tags) != 2 {
		t.Fatalf("tags: %v %+v", err, tags)
	}
}
