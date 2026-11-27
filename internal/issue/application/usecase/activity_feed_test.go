package usecase

import (
	"context"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/jhonsferg/corelog/internal/issue/application/port"
	"github.com/jhonsferg/corelog/internal/issue/domain"
	"github.com/jhonsferg/corelog/internal/platform/apperror"
)

// memLabels is an in-memory label store for one project: it implements the issue label repository.
type memLabels struct {
	projectID  uuid.UUID
	names      map[uuid.UUID]string
	issueLinks map[uuid.UUID][]uuid.UUID
}

func newMemLabels(projectID uuid.UUID) *memLabels {
	return &memLabels{projectID: projectID, names: map[uuid.UUID]string{}, issueLinks: map[uuid.UUID][]uuid.UUID{}}
}

func (m *memLabels) IssueLabelIDs(_ context.Context, issueID uuid.UUID) ([]uuid.UUID, error) {
	return append([]uuid.UUID(nil), m.issueLinks[issueID]...), nil
}

func (m *memLabels) ResolveLabelNames(_ context.Context, projectID uuid.UUID, names []string) ([]uuid.UUID, error) {
	ids := make([]uuid.UUID, 0, len(names))
	for _, name := range names {
		var found uuid.UUID
		for id, existing := range m.names {
			if existing == name && projectID == m.projectID {
				found = id
			}
		}
		if found == uuid.Nil {
			found = uuid.New()
			m.names[found] = name
		}
		ids = append(ids, found)
	}
	return ids, nil
}

func (m *memLabels) ReplaceIssueLabels(_ context.Context, issueID uuid.UUID, labelIDs []uuid.UUID) error {
	m.issueLinks[issueID] = append([]uuid.UUID(nil), labelIDs...)
	return nil
}

// memFeed serves the feed from the in-memory collaboration store, applying the same ordering and cursor rules
// as the SQL reader.
type memFeed struct {
	collab *memCollab
	labels *memLabels
}

func (m *memFeed) FeedRefs(_ context.Context, issueID uuid.UUID, filter string, viewer domain.CommentLevel, after *domain.ActivityCursor, limit int) ([]port.FeedRef, error) {
	var refs []port.FeedRef
	if filter != domain.ActivityFilterComments {
		units := map[uuid.UUID]*port.FeedRef{}
		for _, a := range m.collab.activities {
			if a.IssueID != issueID {
				continue
			}
			key := a.ID
			if a.BatchID != nil {
				key = *a.BatchID
			}
			ref, ok := units[key]
			if !ok {
				ref = &port.FeedRef{Kind: domain.FeedKindHistory, ID: key, At: a.CreatedAt}
				units[key] = ref
			}
			if a.CreatedAt.After(ref.At) {
				ref.At = a.CreatedAt
			}
		}
		for _, ref := range units {
			refs = append(refs, *ref)
		}
	}
	if filter != domain.ActivityFilterHistory {
		for _, c := range m.collab.comments {
			if c.IssueID == issueID && (c.Visibility == domain.CommentVisibilityPublic || domain.CanReadComment(viewer, c.Visibility, c.InternalRole)) {
				refs = append(refs, port.FeedRef{Kind: domain.FeedKindComment, ID: c.ID, At: c.CreatedAt})
			}
		}
	}
	sort.Slice(refs, func(i, j int) bool {
		a, b := refs[i], refs[j]
		if !a.At.Equal(b.At) {
			return a.At.After(b.At)
		}
		if a.Kind != b.Kind {
			return a.Kind > b.Kind
		}
		return a.ID.String() > b.ID.String()
	})
	var out []port.FeedRef
	for _, ref := range refs {
		if after != nil && !isBefore(ref, *after) {
			continue
		}
		out = append(out, ref)
		if len(out) == limit {
			break
		}
	}
	return out, nil
}

// isBefore reports whether ref sorts strictly after the cursor position in the descending order.
func isBefore(ref port.FeedRef, c domain.ActivityCursor) bool {
	if !ref.At.Equal(c.At) {
		return ref.At.Before(c.At)
	}
	if ref.Kind != c.Kind {
		return ref.Kind < c.Kind
	}
	return ref.ID.String() < c.ID.String()
}

func (m *memFeed) FeedContent(_ context.Context, _ uuid.UUID, refs []port.FeedRef) (port.FeedContent, error) {
	content := port.FeedContent{Comments: map[uuid.UUID]*domain.Comment{}, Activities: map[uuid.UUID][]*domain.Activity{}}
	for _, ref := range refs {
		if ref.Kind == domain.FeedKindComment {
			for _, c := range m.collab.comments {
				if c.ID == ref.ID {
					content.Comments[c.ID] = c
				}
			}
			continue
		}
		for _, a := range m.collab.activities {
			key := a.ID
			if a.BatchID != nil {
				key = *a.BatchID
			}
			if key == ref.ID {
				content.Activities[ref.ID] = append(content.Activities[ref.ID], a)
			}
		}
	}
	return content, nil
}

func (m *memFeed) ReferenceNames(_ context.Context, kind string, ids []uuid.UUID) (map[uuid.UUID]string, error) {
	out := map[uuid.UUID]string{}
	if kind != port.ReferenceLabel {
		return out, nil
	}
	for _, id := range ids {
		if name, ok := m.labels.names[id]; ok {
			out[id] = name
		}
	}
	return out, nil
}

// feedFixture wires the patch fixture with the label store and the in-memory feed.
func feedFixture(t *testing.T) (*patchFixture, *memLabels, *memFeed) {
	t.Helper()
	f := newPatchFixture(t)
	labels := newMemLabels(f.issue.ProjectID)
	feed := &memFeed{collab: f.collab, labels: labels}
	f.svc.WithActivityFeed(feed).WithIssueLabels(labels)
	return f, labels, feed
}

func listFeed(t *testing.T, f *patchFixture, actor uuid.UUID, q port.ActivityQuery) *port.ActivityFeedPage {
	t.Helper()
	page, err := f.svc.ListActivityFeed(context.Background(), f.issue.ID, actor, "user", q)
	if err != nil {
		t.Fatalf("ListActivityFeed: %v", err)
	}
	return page
}

func TestActivityFeed_GroupsOnePatchIntoOneEntry(t *testing.T) {
	f, _, _ := feedFixture(t)
	ctx := context.Background()
	f.clock.Advance(time.Minute)
	if _, err := f.svc.Patch(ctx, f.issue.ID, f.lead, "user", port.PatchInput{
		Summary:     port.Some("Nuevo resumen"),
		Description: port.Some("Nueva descripción"),
	}); err != nil {
		t.Fatal(err)
	}
	page := listFeed(t, f, f.lead, port.ActivityQuery{})
	if len(page.Entries) != 1 {
		t.Fatalf("entries = %d, want 1 grouped entry", len(page.Entries))
	}
	entry := page.Entries[0]
	if entry.Kind != domain.FeedKindHistory || len(entry.Changes) != 2 {
		t.Fatalf("entry = %+v, want history with 2 changes", entry)
	}
	labels := []string{entry.Changes[0].Label, entry.Changes[1].Label}
	sort.Strings(labels)
	if labels[0] != "Descripción" || labels[1] != "Resumen" {
		t.Errorf("labels = %v, want Spanish field names", labels)
	}
	if entry.ActorName == "" {
		t.Error("actor name must be filled")
	}
}

func TestActivityFeed_NewestFirstAndCommentsMerged(t *testing.T) {
	f, _, _ := feedFixture(t)
	ctx := context.Background()
	f.clock.Advance(time.Minute)
	if _, err := f.svc.Patch(ctx, f.issue.ID, f.lead, "user", port.PatchInput{Summary: port.Some("Uno")}); err != nil {
		t.Fatal(err)
	}
	f.clock.Advance(time.Minute)
	seedComment(t, f, f.lead, "Hola")
	page := listFeed(t, f, f.lead, port.ActivityQuery{})
	if len(page.Entries) != 2 || page.Entries[0].Kind != domain.FeedKindComment || page.Entries[1].Kind != domain.FeedKindHistory {
		t.Fatalf("entries = %+v, want comment then history", page.Entries)
	}
	if page.Entries[0].Comment == nil || page.Entries[0].Comment.Body != "Hola" {
		t.Errorf("comment body missing: %+v", page.Entries[0].Comment)
	}
}

func TestActivityFeed_CursorPagesWithEqualTimestamps(t *testing.T) {
	f, _, _ := feedFixture(t)
	ctx := context.Background()
	// The fixed clock gives every row the same timestamp: the order must still be total and stable.
	for i := 0; i < 3; i++ {
		seedComment(t, f, f.lead, "c")
	}
	for _, summary := range []string{"a", "b"} {
		if _, err := f.svc.Patch(ctx, f.issue.ID, f.lead, "user", port.PatchInput{Summary: port.Some(summary)}); err != nil {
			t.Fatal(err)
		}
	}
	seen := map[string]bool{}
	cursor := ""
	pages := 0
	for {
		page := listFeed(t, f, f.lead, port.ActivityQuery{Cursor: cursor, Limit: 2, LimitGiven: true})
		for _, e := range page.Entries {
			if seen[e.ID.String()] {
				t.Fatalf("entry %s returned twice", e.ID)
			}
			seen[e.ID.String()] = true
		}
		pages++
		if page.NextCursor == "" {
			break
		}
		if pages > 5 {
			t.Fatal("paging does not end")
		}
		cursor = page.NextCursor
	}
	if len(seen) != 5 || pages != 3 {
		t.Errorf("seen %d entries in %d pages, want 5 in 3", len(seen), pages)
	}
}

func TestActivityFeed_FilterSelectsKinds(t *testing.T) {
	f, _, _ := feedFixture(t)
	ctx := context.Background()
	seedComment(t, f, f.lead, "c")
	if _, err := f.svc.Patch(ctx, f.issue.ID, f.lead, "user", port.PatchInput{Summary: port.Some("x")}); err != nil {
		t.Fatal(err)
	}
	comments := listFeed(t, f, f.lead, port.ActivityQuery{Filter: domain.ActivityFilterComments})
	history := listFeed(t, f, f.lead, port.ActivityQuery{Filter: domain.ActivityFilterHistory})
	if len(comments.Entries) != 1 || comments.Entries[0].Kind != domain.FeedKindComment {
		t.Errorf("comments filter = %+v", comments.Entries)
	}
	if len(history.Entries) != 1 || history.Entries[0].Kind != domain.FeedKindHistory {
		t.Errorf("history filter = %+v", history.Entries)
	}
}

func TestActivityFeed_ResolvesUserNamesAfterRenameAndDeletion(t *testing.T) {
	f, _, _ := feedFixture(t)
	ctx := context.Background()
	dir := f.svc.directory.(*fakeUserDirectory)
	dir.names = map[uuid.UUID]string{f.owner: "Ana"}
	f.clock.Advance(time.Minute)
	if _, err := f.svc.Patch(ctx, f.issue.ID, f.lead, "user", port.PatchInput{AssigneeID: port.Some(f.owner)}); err != nil {
		t.Fatal(err)
	}
	change := func() port.ActivityChange {
		page := listFeed(t, f, f.lead, port.ActivityQuery{})
		return page.Entries[0].Changes[0]
	}
	if got := change(); got.Label != "Responsable" || got.To == nil || *got.To != "Ana" {
		t.Fatalf("change = %+v, want Responsable Ana", got)
	}
	dir.names[f.owner] = "Ana García"
	if got := change(); got.To == nil || *got.To != "Ana García" {
		t.Errorf("after rename to = %v, want current name", got.To)
	}
	delete(dir.names, f.owner)
	if got := change(); got.To == nil || *got.To != deletedUserName {
		t.Errorf("after deletion to = %v, want %q", got.To, deletedUserName)
	}
}

func TestActivityFeed_LabelsCreatedOnTypeAndResolvedByName(t *testing.T) {
	f, labels, _ := feedFixture(t)
	ctx := context.Background()
	f.clock.Advance(time.Minute)
	if _, err := f.svc.Patch(ctx, f.issue.ID, f.lead, "user", port.PatchInput{
		Labels: port.Some([]string{"bug", " ux ", "bug"}),
	}); err != nil {
		t.Fatalf("patch labels: %v", err)
	}
	if len(labels.names) != 2 {
		t.Fatalf("labels created = %d, want 2 (duplicate ignored)", len(labels.names))
	}
	page := listFeed(t, f, f.lead, port.ActivityQuery{})
	change := page.Entries[0].Changes[0]
	if change.Label != "Etiquetas" || change.To == nil || *change.To != "bug, ux" {
		t.Fatalf("change = %+v, want Etiquetas bug, ux", change)
	}
	for id, name := range labels.names {
		if name == "bug" {
			labels.names[id] = "defecto"
		}
	}
	page = listFeed(t, f, f.lead, port.ActivityQuery{})
	if got := *page.Entries[0].Changes[0].To; got != "defecto, ux" {
		t.Errorf("after rename = %q, want current name", got)
	}
	for id, name := range labels.names {
		if name == "ux" {
			delete(labels.names, id)
		}
	}
	page = listFeed(t, f, f.lead, port.ActivityQuery{})
	if got := *page.Entries[0].Changes[0].To; got != deletedLabelName+", defecto" {
		t.Errorf("after delete = %q", got)
	}
}

func TestPatchLabels_NoChangeForSameSetAndClear(t *testing.T) {
	f, _, _ := feedFixture(t)
	ctx := context.Background()
	res, err := f.svc.Patch(ctx, f.issue.ID, f.lead, "user", port.PatchInput{Labels: port.Some([]string{"a", "b"})})
	if err != nil || len(res.Changes) != 1 {
		t.Fatalf("first patch: err=%v", err)
	}
	res, err = f.svc.Patch(ctx, f.issue.ID, f.lead, "user", port.PatchInput{Labels: port.Some([]string{"b", "a"})})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Changes) != 0 {
		t.Errorf("same set must not change: %+v", res.Changes)
	}
	res, err = f.svc.Patch(ctx, f.issue.ID, f.lead, "user", port.PatchInput{Labels: port.Null[[]string]()})
	if err != nil || len(res.Changes) != 1 || res.Changes[0].To != nil {
		t.Fatalf("clear: err=%v", err)
	}
}

func TestPatchLabels_RejectsInvalidNames(t *testing.T) {
	f, _, _ := feedFixture(t)
	ctx := context.Background()
	tooLong := strings.Repeat("x", domain.MaxLabelNameLength+1)
	_, err := f.svc.Patch(ctx, f.issue.ID, f.lead, "user", port.PatchInput{Labels: port.Some([]string{"  "})})
	if apperror.CodeOf(err) != "issue.invalid_label" {
		t.Errorf("blank name: code = %q", apperror.CodeOf(err))
	}
	_, err = f.svc.Patch(ctx, f.issue.ID, f.lead, "user", port.PatchInput{Labels: port.Some([]string{tooLong})})
	if apperror.KindOf(err) != apperror.KindValidation {
		t.Errorf("long name: kind = %v", apperror.KindOf(err))
	}
}

func TestActivityFeed_HiddenIssueIsNotFound(t *testing.T) {
	f, _, _ := feedFixture(t)
	_, err := f.svc.ListActivityFeed(context.Background(), f.issue.ID, uuid.New(), "user", port.ActivityQuery{})
	if apperror.KindOf(err) != apperror.KindNotFound || apperror.CodeOf(err) != "issue.not_found" {
		t.Errorf("hidden issue: kind=%v code=%q", apperror.KindOf(err), apperror.CodeOf(err))
	}
}

func TestActivityFeed_RejectsBadQuery(t *testing.T) {
	f, _, _ := feedFixture(t)
	cases := map[string]port.ActivityQuery{
		"issue.invalid_activity_filter": {Filter: "worklog"},
		"issue.invalid_activity_limit":  {Limit: 0, LimitGiven: true},
		"issue.invalid_activity_cursor": {Cursor: "!!!"},
	}
	for code, q := range cases {
		_, err := f.svc.ListActivityFeed(context.Background(), f.issue.ID, f.lead, "user", q)
		if apperror.CodeOf(err) != code {
			t.Errorf("query %+v: code = %q, want %q", q, apperror.CodeOf(err), code)
		}
	}
	_, err := f.svc.ListActivityFeed(context.Background(), f.issue.ID, f.lead, "user", port.ActivityQuery{Limit: 101, LimitGiven: true})
	if apperror.KindOf(err) != apperror.KindValidation {
		t.Errorf("limit 101 kind = %v", apperror.KindOf(err))
	}
}

func TestActivityCursor_RoundTrip(t *testing.T) {
	want := domain.ActivityCursor{At: time.Date(2026, 11, 27, 10, 0, 0, 123456000, time.UTC), Kind: domain.FeedKindHistory, ID: uuid.New()}
	got, err := domain.DecodeActivityCursor(domain.EncodeActivityCursor(want))
	if err != nil || got == nil {
		t.Fatalf("decode: %v", err)
	}
	if !got.At.Equal(want.At) || got.Kind != want.Kind || got.ID != want.ID {
		t.Errorf("round trip = %+v, want %+v", got, want)
	}
}

// seedComment stores a public comment through the in-memory collaboration store.
func seedComment(t *testing.T, f *patchFixture, author uuid.UUID, body string) {
	t.Helper()
	comment, err := domain.NewComment(f.issue.ID, author, body, f.clock.Now())
	if err != nil {
		t.Fatal(err)
	}
	if err := f.collab.SaveComment(context.Background(), comment); err != nil {
		t.Fatal(err)
	}
}

func TestActivityFeed_DeletedCommentsShowPlaceholderAndInternalOnesFollowRole(t *testing.T) {
	f, _, _ := feedFixture(t)
	ctx := context.Background()
	seedComment(t, f, f.lead, "Visible y luego borrado")
	internal, err := domain.NewComment(f.issue.ID, f.lead, "Nota interna", f.clock.Now())
	if err != nil {
		t.Fatal(err)
	}
	internal.Visibility = domain.CommentVisibilityInternal
	internal.InternalRole = domain.CommentLevelContributor
	if err := f.collab.SaveComment(ctx, internal); err != nil {
		t.Fatal(err)
	}
	for _, c := range f.collab.comments {
		if c.Body == "Visible y luego borrado" {
			f.clock.Advance(time.Minute)
			c.Delete(f.clock.Now())
		}
	}

	// The fixture actor is a lead, so it reads the internal comment too; the placeholder replaces the deleted body.
	page := listFeed(t, f, f.lead, port.ActivityQuery{})
	if len(page.Entries) != 2 {
		t.Fatalf("entries = %d, want the placeholder and the internal comment", len(page.Entries))
	}
	var placeholder, internalSeen bool
	for _, entry := range page.Entries {
		if entry.Comment == nil {
			t.Fatalf("unexpected entry %+v", entry)
		}
		if entry.Comment.Deleted && entry.Comment.Body == domain.CommentDeletedPlaceholder {
			placeholder = true
		}
		if entry.Comment.Visibility == string(domain.CommentVisibilityInternal) && entry.Comment.Body == "Nota interna" {
			internalSeen = true
		}
	}
	if !placeholder || !internalSeen {
		t.Fatalf("placeholder=%v internal=%v in %+v", placeholder, internalSeen, page.Entries)
	}
}
