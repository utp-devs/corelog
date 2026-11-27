package postgres

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/jhonsferg/corelog/internal/issue/domain"
)

// commentUser inserts a second user, needed for mentions and reactions.
func commentUser(t *testing.T, e v2Env, name string) uuid.UUID {
	t.Helper()
	id := uuid.New()
	if _, err := e.db.Exec(`INSERT INTO users (id, name, display_name, email, password_hash, role)
		VALUES ($1, $2, $2, $3, 'x', 'user')`, id, name, id.String()+"@example.test"); err != nil {
		t.Fatal(err)
	}
	return id
}

func storeComment(t *testing.T, e v2Env, store *CommentRepository, issueID uuid.UUID, body string, at time.Time) *domain.Comment {
	t.Helper()
	c, err := domain.NewComment(issueID, e.lead, body, at)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.InsertComment(context.Background(), c); err != nil {
		t.Fatal(err)
	}
	return c
}

func rootIDs(comments []*domain.Comment) map[uuid.UUID]bool {
	out := map[uuid.UUID]bool{}
	for _, c := range comments {
		out[c.ID] = true
	}
	return out
}

// TestCommentRepositoryThreadsFollowVisibility checks that roots and replies come back only for the viewer level
// that can read them, that a deleted root stays while it has a live reply, and that other issues never leak in.
func TestCommentRepositoryThreadsFollowVisibility(t *testing.T) {
	e := newV2Env(t)
	ctx := context.Background()
	store := NewCommentRepository(e.db)
	issue := e.newIssue(t, "Hilos")
	other := e.newIssue(t, "Otra")
	base := time.Date(2026, 11, 27, 9, 0, 0, 0, time.UTC)

	public := storeComment(t, e, store, issue.ID, "publico", base)
	reply, _ := domain.NewComment(issue.ID, e.lead, "respuesta", base.Add(time.Second))
	reply.ParentID = &public.ID
	if err := store.InsertComment(ctx, reply); err != nil {
		t.Fatal(err)
	}

	internal, _ := domain.NewComment(issue.ID, e.lead, "interno", base.Add(2*time.Second))
	internal.Visibility = domain.CommentVisibilityInternal
	internal.InternalRole = domain.CommentLevelContributor
	if err := store.InsertComment(ctx, internal); err != nil {
		t.Fatal(err)
	}
	leadOnly, _ := domain.NewComment(issue.ID, e.lead, "solo leads", base.Add(3*time.Second))
	leadOnly.Visibility = domain.CommentVisibilityInternal
	leadOnly.InternalRole = domain.CommentLevelLead
	if err := store.InsertComment(ctx, leadOnly); err != nil {
		t.Fatal(err)
	}

	deletedWithReply := storeComment(t, e, store, issue.ID, "borrado con respuesta", base.Add(4*time.Second))
	child, _ := domain.NewComment(issue.ID, e.lead, "hija viva", base.Add(5*time.Second))
	child.ParentID = &deletedWithReply.ID
	if err := store.InsertComment(ctx, child); err != nil {
		t.Fatal(err)
	}
	deletedWithReply.Delete(base.Add(6 * time.Second))
	if err := store.SoftDeleteComment(ctx, deletedWithReply); err != nil {
		t.Fatal(err)
	}
	lonely := storeComment(t, e, store, issue.ID, "borrado sin respuesta", base.Add(7*time.Second))
	lonely.Delete(base.Add(8 * time.Second))
	if err := store.SoftDeleteComment(ctx, lonely); err != nil {
		t.Fatal(err)
	}
	storeComment(t, e, store, other.ID, "de otra incidencia", base.Add(9*time.Second))

	viewer, err := store.ListThreadRoots(ctx, issue.ID, domain.CommentLevelViewer, domain.CommentSortAsc, nil, 50)
	if err != nil {
		t.Fatal(err)
	}
	got := rootIDs(viewer)
	if !got[public.ID] || !got[deletedWithReply.ID] || got[internal.ID] || got[leadOnly.ID] || got[lonely.ID] || len(viewer) != 2 {
		t.Fatalf("viewer roots = %v, want the public root and the deleted root with a reply", got)
	}

	contributor, err := store.ListThreadRoots(ctx, issue.ID, domain.CommentLevelContributor, domain.CommentSortAsc, nil, 50)
	if err != nil {
		t.Fatal(err)
	}
	if !rootIDs(contributor)[internal.ID] || rootIDs(contributor)[leadOnly.ID] {
		t.Fatalf("contributor roots = %d, internal must show and lead-only must not", len(contributor))
	}

	replies, err := store.ListReplies(ctx, issue.ID, []uuid.UUID{public.ID, deletedWithReply.ID}, domain.CommentLevelViewer)
	if err != nil {
		t.Fatal(err)
	}
	if len(replies) != 2 {
		t.Fatalf("replies for viewer = %d, want the public reply and the live child", len(replies))
	}
	for _, r := range replies {
		if r.DeletedAt != nil {
			t.Error("deleted replies must not be listed")
		}
	}

	paged, err := store.ListThreadRoots(ctx, issue.ID, domain.CommentLevelLead, domain.CommentSortDesc, &domain.CommentCursor{CreatedAt: internal.CreatedAt, ID: internal.ID}, 50)
	if err != nil {
		t.Fatal(err)
	}
	if rootIDs(paged)[internal.ID] || !rootIDs(paged)[public.ID] {
		t.Fatalf("desc cursor page = %d roots, want the older ones only", len(paged))
	}
}

// TestCommentRepositoryEditDeleteMentionsAndReactions covers revisions, mention replacement, reaction toggles and
// the cleanup done by a soft delete.
func TestCommentRepositoryEditDeleteMentionsAndReactions(t *testing.T) {
	e := newV2Env(t)
	ctx := context.Background()
	store := NewCommentRepository(e.db)
	issue := e.newIssue(t, "Ediciones")
	bob := commentUser(t, e, "Bob")
	base := time.Date(2026, 11, 27, 9, 0, 0, 0, time.UTC)

	c := storeComment(t, e, store, issue.ID, "viejo", base)
	if err := store.ReplaceMentions(ctx, c.ID, []uuid.UUID{bob}); err != nil {
		t.Fatal(err)
	}
	if err := c.Update("nuevo", base.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	if err := store.UpdateCommentBody(ctx, c, "viejo", e.lead); err != nil {
		t.Fatal(err)
	}
	revisions, err := store.ListRevisions(ctx, c.ID)
	if err != nil || len(revisions) != 1 || revisions[0].Body != "viejo" || revisions[0].EditedBy != e.lead {
		t.Fatalf("revisions = %+v, %v", revisions, err)
	}
	stored, err := store.FindComment(ctx, c.ID)
	if err != nil || stored.Body != "nuevo" || !stored.IsEdited() {
		t.Fatalf("stored edit = %+v, %v", stored, err)
	}

	mentioned, err := store.MentionedUserIDs(ctx, c.ID)
	if err != nil || len(mentioned) != 1 || mentioned[0] != bob {
		t.Fatalf("mentions = %v, %v", mentioned, err)
	}
	if err := store.ReplaceMentions(ctx, c.ID, nil); err != nil {
		t.Fatal(err)
	}
	if mentioned, _ := store.MentionedUserIDs(ctx, c.ID); len(mentioned) != 0 {
		t.Fatalf("mentions after replace = %v", mentioned)
	}

	for i := 0; i < 2; i++ {
		if err := store.SetReaction(ctx, c.ID, e.lead, "rocket", true); err != nil {
			t.Fatal(err)
		}
	}
	if err := store.SetReaction(ctx, c.ID, bob, "rocket", true); err != nil {
		t.Fatal(err)
	}
	if err := store.SetReaction(ctx, c.ID, bob, "eyes", true); err != nil {
		t.Fatal(err)
	}
	counts, err := store.ReactionCounts(ctx, []uuid.UUID{c.ID}, e.lead)
	if err != nil {
		t.Fatal(err)
	}
	list := counts[c.ID]
	if len(list) != 2 || list[0].Emoji != "eyes" || list[0].Count != 1 || list[1].Emoji != "rocket" || list[1].Count != 2 || !list[1].ViewerReacted {
		t.Fatalf("reaction counts = %+v, want eyes 1 then rocket 2 (viewer), in the fixed order", list)
	}
	if err := store.SetReaction(ctx, c.ID, bob, "rocket", false); err != nil {
		t.Fatal(err)
	}
	if counts, _ := store.ReactionCounts(ctx, []uuid.UUID{c.ID}, bob); counts[c.ID][1].Count != 1 {
		t.Fatalf("after removal = %+v", counts[c.ID])
	}

	if err := store.ReplaceMentions(ctx, c.ID, []uuid.UUID{bob}); err != nil {
		t.Fatal(err)
	}
	c.Delete(base.Add(2 * time.Minute))
	if err := store.SoftDeleteComment(ctx, c); err != nil {
		t.Fatal(err)
	}
	deleted, err := store.FindComment(ctx, c.ID)
	if err != nil || deleted.Body != "" || !deleted.IsDeleted() {
		t.Fatalf("deleted = %+v, %v", deleted, err)
	}
	if revisions, _ := store.ListRevisions(ctx, c.ID); len(revisions) != 0 {
		t.Error("a deleted comment keeps no revisions")
	}
	if counts, _ := store.ReactionCounts(ctx, []uuid.UUID{c.ID}, e.lead); len(counts[c.ID]) != 0 {
		t.Error("a deleted comment keeps no reactions")
	}
	if mentioned, _ := store.MentionedUserIDs(ctx, c.ID); len(mentioned) != 0 {
		t.Error("a deleted comment keeps no mentions")
	}
	if err := store.UpdateCommentBody(ctx, c, "", e.lead); err == nil {
		t.Error("updating a deleted comment must fail")
	}
}

// TestCommentRepositoryFeedHidesInternalForLowerViewers checks the activity feed visibility rule in SQL.
func TestCommentRepositoryFeedHidesInternalForLowerViewers(t *testing.T) {
	e := newV2Env(t)
	ctx := context.Background()
	store := NewCommentRepository(e.db)
	issue := e.newIssue(t, "Feed")
	base := time.Date(2026, 11, 27, 9, 0, 0, 0, time.UTC)
	storeComment(t, e, store, issue.ID, "publico", base)
	internal, _ := domain.NewComment(issue.ID, e.lead, "interno", base.Add(time.Second))
	internal.Visibility = domain.CommentVisibilityInternal
	internal.InternalRole = domain.CommentLevelContributor
	if err := store.InsertComment(ctx, internal); err != nil {
		t.Fatal(err)
	}

	reader := NewActivityFeedRepository(e.db)
	for _, tc := range []struct {
		level domain.CommentLevel
		want  int
	}{{domain.CommentLevelViewer, 1}, {domain.CommentLevelContributor, 2}} {
		refs, err := reader.FeedRefs(ctx, issue.ID, domain.ActivityFilterComments, tc.level, nil, 10)
		if err != nil {
			t.Fatal(err)
		}
		if len(refs) != tc.want {
			t.Errorf("level %d sees %d comments, want %d", tc.level, len(refs), tc.want)
		}
	}
}
