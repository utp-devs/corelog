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
	"github.com/jhonsferg/corelog/internal/platform/events"
)

// cmtRoles maps users to their project role. A missing user has no role on the project.
type cmtRoles map[uuid.UUID]string

// roleAllows is the permission matrix of the fakes: lead manages comments, contributors comment and edit their own,
// viewers only browse.
func roleAllows(role, permission string) bool {
	switch permission {
	case port.PermissionBrowseProject:
		return role == "lead" || role == "contributor" || role == "viewer"
	case port.PermissionCommentIssue, port.PermissionEditOwnComment, port.PermissionDeleteOwnComment:
		return role == "lead" || role == "contributor"
	case port.PermissionManageComments:
		return role == "lead"
	}
	return false
}

func (r cmtRoles) Can(_ context.Context, actor uuid.UUID, _ string, _ uuid.UUID, permission string, _ ...string) (bool, error) {
	return roleAllows(r[actor], permission), nil
}

// cmtAuthz is the write gate: forbidden without the permission, a conflict on archived projects.
type cmtAuthz struct {
	roles    cmtRoles
	archived bool
}

func (a *cmtAuthz) Authorize(_ context.Context, action string, actor uuid.UUID, _ string, _ uuid.UUID) error {
	if !roleAllows(a.roles[actor], action) {
		return apperror.New(apperror.KindForbidden, "forbidden")
	}
	if a.archived {
		return apperror.WithCode(apperror.KindConflict, "project.archived", "el proyecto esta archivado y no admite cambios")
	}
	return nil
}

type cmtClock struct{ at time.Time }

func (c *cmtClock) Now() time.Time { return c.at }

type cmtEvents struct{ list []events.Event }

func (e *cmtEvents) Publish(_ context.Context, ev events.Event) error {
	e.list = append(e.list, ev)
	return nil
}

// cmtStore is an in-memory port.CommentStore with the same visibility and ordering rules as the SQL store.
type cmtStore struct {
	comments  map[uuid.UUID]*domain.Comment
	mentions  map[uuid.UUID][]uuid.UUID
	revisions map[uuid.UUID][]domain.CommentRevision
	reactions map[uuid.UUID]map[string]map[uuid.UUID]bool
}

func newCmtStore() *cmtStore {
	return &cmtStore{
		comments:  map[uuid.UUID]*domain.Comment{},
		mentions:  map[uuid.UUID][]uuid.UUID{},
		revisions: map[uuid.UUID][]domain.CommentRevision{},
		reactions: map[uuid.UUID]map[string]map[uuid.UUID]bool{},
	}
}

func cmtCopy(c *domain.Comment) *domain.Comment {
	cp := *c
	cp.Mentions = append([]uuid.UUID(nil), c.Mentions...)
	return &cp
}

func cmtReadable(c *domain.Comment, viewer domain.CommentLevel) bool {
	return c.Visibility == domain.CommentVisibilityPublic || domain.CanReadComment(viewer, c.Visibility, c.InternalRole)
}

func (s *cmtStore) FindComment(_ context.Context, id uuid.UUID) (*domain.Comment, error) {
	c, ok := s.comments[id]
	if !ok {
		return nil, domain.ErrCommentNotFound
	}
	return cmtCopy(c), nil
}

func (s *cmtStore) InsertComment(_ context.Context, c *domain.Comment) error {
	c.ID = uuid.New()
	stored := cmtCopy(c)
	s.comments[c.ID] = stored
	s.mentions[c.ID] = append([]uuid.UUID(nil), c.Mentions...)
	return nil
}

func (s *cmtStore) UpdateCommentBody(_ context.Context, c *domain.Comment, previous string, editedBy uuid.UUID) error {
	s.revisions[c.ID] = append(s.revisions[c.ID], domain.CommentRevision{
		ID: uuid.New(), CommentID: c.ID, Body: previous, EditedBy: editedBy, EditedAt: *c.EditedAt,
	})
	s.comments[c.ID] = cmtCopy(c)
	return nil
}

func (s *cmtStore) SoftDeleteComment(_ context.Context, c *domain.Comment) error {
	delete(s.revisions, c.ID)
	delete(s.reactions, c.ID)
	delete(s.mentions, c.ID)
	s.comments[c.ID] = cmtCopy(c)
	return nil
}

func (s *cmtStore) ReplaceMentions(_ context.Context, id uuid.UUID, userIDs []uuid.UUID) error {
	s.mentions[id] = append([]uuid.UUID(nil), userIDs...)
	return nil
}

func (s *cmtStore) MentionedUserIDs(_ context.Context, id uuid.UUID) ([]uuid.UUID, error) {
	return append([]uuid.UUID(nil), s.mentions[id]...), nil
}

func (s *cmtStore) ListMentions(_ context.Context, ids []uuid.UUID) (map[uuid.UUID][]uuid.UUID, error) {
	out := map[uuid.UUID][]uuid.UUID{}
	for _, id := range ids {
		if len(s.mentions[id]) > 0 {
			out[id] = append([]uuid.UUID(nil), s.mentions[id]...)
		}
	}
	return out, nil
}

func (s *cmtStore) ListRevisions(_ context.Context, id uuid.UUID) ([]domain.CommentRevision, error) {
	revs := append([]domain.CommentRevision(nil), s.revisions[id]...)
	sort.SliceStable(revs, func(i, j int) bool { return revs[i].EditedAt.After(revs[j].EditedAt) })
	return revs, nil
}

func (s *cmtStore) live(c *domain.Comment, issueID uuid.UUID) bool {
	return c.IssueID == issueID && c.DeletedAt == nil
}

func (s *cmtStore) ListThreadRoots(_ context.Context, issueID uuid.UUID, viewer domain.CommentLevel, sortOrder string, after *domain.CommentCursor, limit int) ([]*domain.Comment, error) {
	var roots []*domain.Comment
	for _, c := range s.comments {
		if c.IssueID != issueID || c.ParentID != nil || !cmtReadable(c, viewer) {
			continue
		}
		if c.DeletedAt != nil && len(s.readableReplies(issueID, c.ID, viewer)) == 0 {
			continue
		}
		roots = append(roots, cmtCopy(c))
	}
	sort.Slice(roots, func(i, j int) bool {
		if !roots[i].CreatedAt.Equal(roots[j].CreatedAt) {
			return roots[i].CreatedAt.Before(roots[j].CreatedAt)
		}
		return roots[i].ID.String() < roots[j].ID.String()
	})
	if sortOrder == domain.CommentSortDesc {
		for i, j := 0, len(roots)-1; i < j; i, j = i+1, j-1 {
			roots[i], roots[j] = roots[j], roots[i]
		}
	}
	var out []*domain.Comment
	for _, c := range roots {
		if after != nil && !cmtAfter(c, *after, sortOrder) {
			continue
		}
		out = append(out, c)
		if len(out) == limit {
			break
		}
	}
	return out, nil
}

// cmtAfter reports whether c comes after the cursor in the given order.
func cmtAfter(c *domain.Comment, cursor domain.CommentCursor, sortOrder string) bool {
	if sortOrder == domain.CommentSortDesc {
		return c.CreatedAt.Before(cursor.CreatedAt) || (c.CreatedAt.Equal(cursor.CreatedAt) && c.ID.String() < cursor.ID.String())
	}
	return c.CreatedAt.After(cursor.CreatedAt) || (c.CreatedAt.Equal(cursor.CreatedAt) && c.ID.String() > cursor.ID.String())
}

func (s *cmtStore) readableReplies(issueID, rootID uuid.UUID, viewer domain.CommentLevel) []*domain.Comment {
	var out []*domain.Comment
	for _, c := range s.comments {
		if c.ParentID != nil && *c.ParentID == rootID && s.live(c, issueID) && cmtReadable(c, viewer) {
			out = append(out, cmtCopy(c))
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt.Before(out[j].CreatedAt) })
	return out
}

func (s *cmtStore) ListReplies(_ context.Context, issueID uuid.UUID, rootIDs []uuid.UUID, viewer domain.CommentLevel) ([]*domain.Comment, error) {
	out := []*domain.Comment{}
	for _, root := range rootIDs {
		out = append(out, s.readableReplies(issueID, root, viewer)...)
	}
	return out, nil
}

func (s *cmtStore) ReactionCounts(_ context.Context, ids []uuid.UUID, viewerID uuid.UUID) (map[uuid.UUID][]port.ReactionCount, error) {
	out := map[uuid.UUID][]port.ReactionCount{}
	for _, id := range ids {
		for _, emoji := range domain.CommentReactions {
			users := s.reactions[id][emoji]
			if len(users) == 0 {
				continue
			}
			out[id] = append(out[id], port.ReactionCount{Emoji: emoji, Count: len(users), ViewerReacted: users[viewerID]})
		}
	}
	return out, nil
}

func (s *cmtStore) SetReaction(_ context.Context, commentID, userID uuid.UUID, emoji string, on bool) error {
	if s.reactions[commentID] == nil {
		s.reactions[commentID] = map[string]map[uuid.UUID]bool{}
	}
	if s.reactions[commentID][emoji] == nil {
		s.reactions[commentID][emoji] = map[uuid.UUID]bool{}
	}
	if on {
		s.reactions[commentID][emoji][userID] = true
	} else {
		delete(s.reactions[commentID][emoji], userID)
	}
	return nil
}

// cmtFixture is one project with an issue and users in every role that matters for comments.
type cmtFixture struct {
	t        *testing.T
	ctx      context.Context
	svc      *CommentService
	store    *cmtStore
	roles    cmtRoles
	authz    *cmtAuthz
	events   *cmtEvents
	clock    *cmtClock
	issue    *domain.Issue
	lead     uuid.UUID
	author   uuid.UUID
	bob      uuid.UUID
	viewer   uuid.UUID
	outsider uuid.UUID
}

func newCmtFixture(t *testing.T) *cmtFixture {
	t.Helper()
	ctx := context.Background()
	repo := newFakeIssueRepo()
	issue := &domain.Issue{ProjectID: uuid.New(), Summary: "Comentarios", ProjectKey: "CORE", KeyNumber: 1}
	if err := repo.Save(ctx, issue); err != nil {
		t.Fatal(err)
	}
	f := &cmtFixture{
		t: t, ctx: ctx, store: newCmtStore(), roles: cmtRoles{}, events: &cmtEvents{},
		clock: &cmtClock{at: time.Date(2026, 11, 27, 9, 0, 0, 0, time.UTC)}, issue: issue,
		lead: uuid.New(), author: uuid.New(), bob: uuid.New(), viewer: uuid.New(), outsider: uuid.New(),
	}
	f.roles[f.lead] = "lead"
	f.roles[f.author] = "contributor"
	f.roles[f.bob] = "contributor"
	f.roles[f.viewer] = "viewer"
	f.authz = &cmtAuthz{roles: f.roles}
	names := newFakeUserDirectory()
	names.names[f.lead] = "Lia Lead"
	names.names[f.author] = "Ana Autora"
	names.names[f.bob] = "Bob Bravo"
	names.names[f.viewer] = "Vera Viewer"
	f.svc = NewCommentService(repo, f.store, f.roles, f.authz, names, nil, f.events, f.clock)
	return f
}

func (f *cmtFixture) actor(id uuid.UUID) port.CommentActor {
	return port.CommentActor{ID: id, GlobalRole: "user"}
}

func (f *cmtFixture) create(by uuid.UUID, body string) *port.CommentResult {
	f.t.Helper()
	result, err := f.svc.CreateComment(f.ctx, f.actor(by), port.CommentCreateInput{IssueID: f.issue.ID, Body: body})
	if err != nil {
		f.t.Fatalf("create %q: %v", body, err)
	}
	return result
}

func (f *cmtFixture) reply(by uuid.UUID, parent uuid.UUID, body string) *port.CommentResult {
	f.t.Helper()
	result, err := f.svc.CreateComment(f.ctx, f.actor(by), port.CommentCreateInput{IssueID: f.issue.ID, Body: body, ParentID: &parent})
	if err != nil {
		f.t.Fatalf("reply %q: %v", body, err)
	}
	return result
}

func (f *cmtFixture) threads(by uuid.UUID, in port.ThreadPageInput) *port.ThreadPage {
	f.t.Helper()
	page, err := f.svc.ListThreads(f.ctx, f.actor(by), f.issue.ID, in)
	if err != nil {
		f.t.Fatalf("list threads: %v", err)
	}
	return page
}

func mentionOf(id uuid.UUID, name string) string {
	return "@[" + name + "](user:" + id.String() + ")"
}

func TestCommentRepliesAttachToTheRoot(t *testing.T) {
	f := newCmtFixture(t)
	root := f.create(f.author, "Raiz").Comment.Comment
	f.clock.at = f.clock.at.Add(time.Minute)
	first := f.reply(f.bob, root.ID, "Respuesta").Comment.Comment
	f.clock.at = f.clock.at.Add(time.Minute)
	nested := f.reply(f.lead, first.ID, "Respuesta a la respuesta").Comment.Comment

	if nested.ParentID == nil || *nested.ParentID != root.ID {
		t.Fatalf("reply to a reply must attach to the root, got parent %v", nested.ParentID)
	}
	page := f.threads(f.author, port.ThreadPageInput{})
	if len(page.Threads) != 1 || page.Threads[0].ReplyCount != 2 || len(page.Threads[0].Replies) != 2 {
		t.Fatalf("thread = %+v, want one root with two replies", page.Threads)
	}
	if page.Threads[0].Replies[0].Comment.ID != first.ID {
		t.Error("replies must be oldest first")
	}
}

func TestCommentThreadPaginationAndSort(t *testing.T) {
	f := newCmtFixture(t)
	for _, body := range []string{"uno", "dos", "tres"} {
		f.clock.at = f.clock.at.Add(time.Minute)
		f.create(f.author, body)
	}
	first := f.threads(f.author, port.ThreadPageInput{Sort: "desc", Limit: 2, LimitGiven: true})
	if len(first.Threads) != 2 || first.NextCursor == "" {
		t.Fatalf("first page = %d threads, cursor %q", len(first.Threads), first.NextCursor)
	}
	if first.Threads[0].Comment.Body != "tres" {
		t.Fatalf("desc sort starts with %q, want the newest", first.Threads[0].Comment.Body)
	}
	second := f.threads(f.author, port.ThreadPageInput{Sort: "desc", Limit: 2, LimitGiven: true, Cursor: first.NextCursor})
	if len(second.Threads) != 1 || second.Threads[0].Comment.Body != "uno" || second.NextCursor != "" {
		t.Fatalf("second page = %+v", second.Threads)
	}
	if _, err := f.svc.ListThreads(f.ctx, f.actor(f.author), f.issue.ID, port.ThreadPageInput{Cursor: "no-es-cursor"}); apperror.CodeOf(err) != "issue.comment_cursor_invalid" {
		t.Fatalf("bad cursor: %v", err)
	}
	if _, err := f.svc.ListThreads(f.ctx, f.actor(f.author), f.issue.ID, port.ThreadPageInput{Sort: "x"}); apperror.CodeOf(err) != "issue.comment_sort_invalid" {
		t.Fatalf("bad sort: %v", err)
	}
	if _, err := f.svc.ListThreads(f.ctx, f.actor(f.author), f.issue.ID, port.ThreadPageInput{Limit: 101, LimitGiven: true}); apperror.CodeOf(err) != "issue.comment_limit_invalid" {
		t.Fatalf("bad limit: %v", err)
	}
}

func TestCommentReplyRules(t *testing.T) {
	f := newCmtFixture(t)
	missing := uuid.New()
	_, err := f.svc.CreateComment(f.ctx, f.actor(f.author), port.CommentCreateInput{IssueID: f.issue.ID, Body: "x", ParentID: &missing})
	if apperror.CodeOf(err) != "issue.comment_parent_invalid" {
		t.Fatalf("reply to missing parent: %v", err)
	}

	root := f.create(f.author, "Raiz").Comment.Comment
	f.svc.DeleteComment(f.ctx, f.actor(f.author), root.ID)
	_, err = f.svc.CreateComment(f.ctx, f.actor(f.bob), port.CommentCreateInput{IssueID: f.issue.ID, Body: "x", ParentID: &root.ID})
	if apperror.CodeOf(err) != "issue.comment_parent_invalid" {
		t.Fatalf("reply to deleted parent: %v", err)
	}

	internal := f.createWith(f.lead, port.CommentCreateInput{Body: "Interno", Visibility: "internal"}).Comment.Comment
	if _, err := f.svc.CreateComment(f.ctx, f.actor(f.lead), port.CommentCreateInput{IssueID: f.issue.ID, Body: "x", ParentID: &internal.ID, Visibility: "public"}); apperror.CodeOf(err) != "issue.comment_reply_visibility" {
		t.Fatalf("public reply to internal root: %v", err)
	}
}

func (f *cmtFixture) createWith(by uuid.UUID, in port.CommentCreateInput) *port.CommentResult {
	f.t.Helper()
	in.IssueID = f.issue.ID
	result, err := f.svc.CreateComment(f.ctx, f.actor(by), in)
	if err != nil {
		f.t.Fatalf("create: %v", err)
	}
	return result
}

func TestCommentMentionsAreValidated(t *testing.T) {
	f := newCmtFixture(t)
	ghost := uuid.New()
	body := strings.Join([]string{
		"Mirar " + mentionOf(f.bob, "Bob"),
		"y " + mentionOf(ghost, "Nadie"),
		"y " + mentionOf(f.outsider, "Externo"),
		"y yo " + mentionOf(f.author, "Ana"),
		"otra vez " + mentionOf(f.bob, "Bob"),
		"en codigo `" + mentionOf(f.lead, "Lia") + "`",
	}, "\n")
	result := f.create(f.author, body)

	if len(result.Comment.Comment.Mentions) != 1 || result.Comment.Comment.Mentions[0] != f.bob {
		t.Fatalf("mentions = %v, want only bob", result.Comment.Comment.Mentions)
	}
	if len(result.Warnings) != 2 {
		t.Fatalf("warnings = %+v, want unknown and outsider only", result.Warnings)
	}
	for _, w := range result.Warnings {
		if w.Code != "issue.comment_mention_invalid" || w.Message == "" {
			t.Errorf("warning %+v without stable code", w)
		}
	}
	if len(f.events.list) != 1 || f.events.list[0].Type != EventCommentMentioned {
		t.Fatalf("events = %+v, want one mention event", f.events.list)
	}
	ids, _ := f.events.list[0].Payload["user_ids"].([]string)
	if len(ids) != 1 || ids[0] != f.bob.String() {
		t.Fatalf("mention event users = %v", f.events.list[0].Payload["user_ids"])
	}
}

func TestInternalCommentVisibilityMatrix(t *testing.T) {
	f := newCmtFixture(t)
	f.create(f.author, "Publico")
	internal := f.createWith(f.lead, port.CommentCreateInput{Body: "Nota interna", Visibility: "internal"})
	if internal.Comment.Comment.Visibility != domain.CommentVisibilityInternal || internal.Comment.Comment.InternalRole.Name() != "contributor" {
		t.Fatalf("internal default = %+v", internal.Comment.Comment)
	}

	if page := f.threads(f.lead, port.ThreadPageInput{}); len(page.Threads) != 2 {
		t.Errorf("lead sees %d threads, want 2", len(page.Threads))
	}
	if page := f.threads(f.author, port.ThreadPageInput{}); len(page.Threads) != 2 {
		t.Errorf("contributor sees %d threads, want 2", len(page.Threads))
	}
	if page := f.threads(f.viewer, port.ThreadPageInput{}); len(page.Threads) != 1 || page.Threads[0].Comment.Body != "Publico" {
		t.Errorf("viewer must not see the internal comment: %+v", page.Threads)
	}
	if _, err := f.svc.ListThreads(f.ctx, f.actor(f.outsider), f.issue.ID, port.ThreadPageInput{}); apperror.KindOf(err) != apperror.KindNotFound {
		t.Errorf("outsider lists threads: %v, want not found", err)
	}

	leadOnly := f.createWith(f.lead, port.CommentCreateInput{Body: "Solo leads", Visibility: "internal", InternalRole: "lead"})
	if page := f.threads(f.author, port.ThreadPageInput{}); len(page.Threads) != 2 {
		t.Errorf("contributor sees lead-only comment: %d threads", len(page.Threads))
	}
	if page := f.threads(f.lead, port.ThreadPageInput{}); len(page.Threads) != 3 {
		t.Errorf("lead sees %d threads, want 3", len(page.Threads))
	}
	if _, err := f.svc.UpdateComment(f.ctx, f.actor(f.viewer), leadOnly.Comment.Comment.ID, "x"); apperror.KindOf(err) != apperror.KindNotFound {
		t.Errorf("viewer edits hidden comment: %v, want not found", err)
	}
	if _, err := f.svc.SetReaction(f.ctx, f.actor(f.viewer), leadOnly.Comment.Comment.ID, "heart", true); apperror.KindOf(err) != apperror.KindNotFound {
		t.Errorf("viewer reacts to hidden comment: %v, want not found", err)
	}

	if _, err := f.svc.CreateComment(f.ctx, f.actor(f.author), port.CommentCreateInput{IssueID: f.issue.ID, Body: "x", Visibility: "internal", InternalRole: "lead"}); apperror.CodeOf(err) != "issue.comment_visibility_invalid" {
		t.Errorf("contributor creates lead-only internal comment: %v", err)
	}
	if _, err := f.svc.CreateComment(f.ctx, f.actor(f.viewer), port.CommentCreateInput{IssueID: f.issue.ID, Body: "x"}); apperror.KindOf(err) != apperror.KindForbidden {
		t.Errorf("viewer comments: %v, want forbidden", err)
	}
	if _, err := f.svc.CreateComment(f.ctx, f.actor(f.author), port.CommentCreateInput{IssueID: f.issue.ID, Body: "x", Visibility: "secreto"}); apperror.CodeOf(err) != "issue.comment_visibility_invalid" {
		t.Errorf("unknown visibility: %v", err)
	}
}

func TestInternalCommentMentionsMustReachTheRole(t *testing.T) {
	f := newCmtFixture(t)
	result := f.createWith(f.author, port.CommentCreateInput{
		Body: "Para " + mentionOf(f.viewer, "Vera") + " y " + mentionOf(f.bob, "Bob"), Visibility: "internal",
	})
	if len(result.Comment.Comment.Mentions) != 1 || result.Comment.Comment.Mentions[0] != f.bob {
		t.Fatalf("mentions = %v, a viewer must not be mentioned in an internal comment", result.Comment.Comment.Mentions)
	}
	if len(result.Warnings) != 1 {
		t.Fatalf("warnings = %+v, want one for the viewer", result.Warnings)
	}
}

func TestCommentEditKeepsRevisionsAndNotifiesNewMentionsOnly(t *testing.T) {
	f := newCmtFixture(t)
	created := f.create(f.author, "uno "+mentionOf(f.bob, "Bob")).Comment.Comment
	f.clock.at = f.clock.at.Add(time.Minute)

	if _, err := f.svc.UpdateComment(f.ctx, f.actor(f.author), created.ID, "dos "+mentionOf(f.bob, "Bob")); err != nil {
		t.Fatal(err)
	}
	if len(f.events.list) != 1 {
		t.Fatalf("re-mentioning an existing user must not notify again: %d events", len(f.events.list))
	}
	f.clock.at = f.clock.at.Add(time.Minute)
	updated, err := f.svc.UpdateComment(f.ctx, f.actor(f.author), created.ID, "tres "+mentionOf(f.bob, "Bob")+" "+mentionOf(f.lead, "Lia"))
	if err != nil {
		t.Fatal(err)
	}
	if !updated.Comment.Comment.IsEdited() || updated.Comment.Comment.Body != "tres "+mentionOf(f.bob, "Bob")+" "+mentionOf(f.lead, "Lia") {
		t.Fatalf("edit not applied: %+v", updated.Comment.Comment)
	}
	if len(f.events.list) != 2 {
		t.Fatalf("events = %d, want one more for the new mention", len(f.events.list))
	}
	newIDs, _ := f.events.list[1].Payload["user_ids"].([]string)
	if len(newIDs) != 1 || newIDs[0] != f.lead.String() {
		t.Fatalf("new mention event = %v, want only the lead", f.events.list[1].Payload["user_ids"])
	}

	f.clock.at = f.clock.at.Add(time.Minute)
	if _, err := f.svc.UpdateComment(f.ctx, f.actor(f.lead), created.ID, "moderado"); err != nil {
		t.Fatalf("manager edit: %v", err)
	}
	if _, err := f.svc.UpdateComment(f.ctx, f.actor(f.bob), created.ID, "ajeno"); apperror.KindOf(err) != apperror.KindForbidden {
		t.Errorf("other contributor edits: %v, want forbidden", err)
	}
	revisions, err := f.svc.ListRevisions(f.ctx, f.actor(f.author), created.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(revisions) != 3 || revisions[0].EditedBy != f.lead || revisions[len(revisions)-1].Body != "uno "+mentionOf(f.bob, "Bob") {
		t.Fatalf("revisions = %+v, want three newest first with the lead's edit on top", revisions)
	}
	if _, err := f.svc.ListRevisions(f.ctx, f.actor(f.viewer), created.ID); apperror.KindOf(err) != apperror.KindForbidden {
		t.Errorf("viewer lists revisions: %v, want forbidden", err)
	}
}

func TestCommentDeletePlaceholderAndPermissions(t *testing.T) {
	f := newCmtFixture(t)
	root := f.create(f.author, "Raiz").Comment.Comment
	reply := f.reply(f.bob, root.ID, "Respuesta").Comment.Comment
	f.clock.at = f.clock.at.Add(time.Minute)
	lonely := f.create(f.author, "Sin respuestas").Comment.Comment

	if err := f.svc.DeleteComment(f.ctx, f.actor(f.author), root.ID); err != nil {
		t.Fatal(err)
	}
	page := f.threads(f.lead, port.ThreadPageInput{})
	if len(page.Threads) != 2 || page.Threads[0].Comment.ID != root.ID || !page.Threads[0].Comment.IsDeleted() || page.Threads[0].Comment.Body != "" {
		t.Fatalf("deleted root with a reply must keep its place: %+v", page.Threads)
	}
	if len(page.Threads[0].Replies) != 1 || page.Threads[0].Replies[0].Comment.ID != reply.ID {
		t.Fatal("replies of a deleted root must stay")
	}
	if err := f.svc.DeleteComment(f.ctx, f.actor(f.bob), lonely.ID); apperror.KindOf(err) != apperror.KindForbidden {
		t.Fatalf("contributor deletes someone else's comment: %v, want forbidden", err)
	}
	if err := f.svc.DeleteComment(f.ctx, f.actor(f.lead), lonely.ID); err != nil {
		t.Fatalf("manager deletes: %v", err)
	}
	if err := f.svc.DeleteComment(f.ctx, f.actor(f.lead), lonely.ID); apperror.KindOf(err) != apperror.KindNotFound {
		t.Fatalf("second delete: %v, want not found", err)
	}
	if _, err := f.svc.UpdateComment(f.ctx, f.actor(f.author), root.ID, "tarde"); apperror.KindOf(err) != apperror.KindNotFound {
		t.Fatalf("edit after delete: %v, want not found", err)
	}
	if page := f.threads(f.author, port.ThreadPageInput{}); len(page.Threads) != 1 {
		t.Fatalf("comment deleted without replies must disappear: %d threads", len(page.Threads))
	}
}

func TestCommentReactionsAreIdempotentAndScoped(t *testing.T) {
	f := newCmtFixture(t)
	comment := f.create(f.author, "Reaccionen").Comment.Comment

	for i := 0; i < 2; i++ {
		if _, err := f.svc.SetReaction(f.ctx, f.actor(f.author), comment.ID, "thumbs_up", true); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := f.svc.SetReaction(f.ctx, f.actor(f.bob), comment.ID, "heart", true); err != nil {
		t.Fatal(err)
	}
	summary, err := f.svc.SetReaction(f.ctx, f.actor(f.bob), comment.ID, "thumbs_up", true)
	if err != nil {
		t.Fatal(err)
	}
	if len(summary.Reactions) != 2 || summary.Reactions[0].Emoji != "thumbs_up" || summary.Reactions[0].Count != 2 {
		t.Fatalf("summary = %+v, want thumbs_up 2 then heart 1", summary.Reactions)
	}
	page := f.threads(f.viewer, port.ThreadPageInput{})
	if page.Threads[0].Reactions[0].ViewerReacted {
		t.Error("viewer_reacted must be per viewer")
	}
	page = f.threads(f.author, port.ThreadPageInput{})
	if !page.Threads[0].Reactions[0].ViewerReacted || page.Threads[0].Reactions[0].Count != 2 {
		t.Errorf("author view = %+v", page.Threads[0].Reactions)
	}

	if _, err := f.svc.SetReaction(f.ctx, f.actor(f.author), comment.ID, "laugh", true); apperror.CodeOf(err) != "issue.comment_reaction_invalid" {
		t.Errorf("unknown emoji: %v", err)
	}
	if _, err := f.svc.SetReaction(f.ctx, f.actor(f.viewer), comment.ID, "heart", true); apperror.KindOf(err) != apperror.KindForbidden {
		t.Errorf("viewer reacts: %v, want forbidden", err)
	}
	for i := 0; i < 2; i++ {
		if _, err := f.svc.SetReaction(f.ctx, f.actor(f.author), comment.ID, "thumbs_up", false); err != nil {
			t.Fatal(err)
		}
	}
	summary, _ = f.svc.SetReaction(f.ctx, f.actor(f.bob), comment.ID, "heart", false)
	if len(summary.Reactions) != 1 || summary.Reactions[0].Emoji != "thumbs_up" || summary.Reactions[0].Count != 1 {
		t.Errorf("after removals = %+v", summary.Reactions)
	}
}

func TestCommentOutsidersAndArchivedProjects(t *testing.T) {
	f := newCmtFixture(t)
	comment := f.create(f.author, "Hola").Comment.Comment

	if _, err := f.svc.CreateComment(f.ctx, f.actor(f.outsider), port.CommentCreateInput{IssueID: f.issue.ID, Body: "x"}); apperror.KindOf(err) != apperror.KindNotFound {
		t.Errorf("outsider comments: %v, want not found", err)
	}
	if _, err := f.svc.UpdateComment(f.ctx, f.actor(f.outsider), comment.ID, "x"); apperror.KindOf(err) != apperror.KindNotFound {
		t.Errorf("outsider edits: %v, want not found", err)
	}

	f.authz.archived = true
	if _, err := f.svc.CreateComment(f.ctx, f.actor(f.author), port.CommentCreateInput{IssueID: f.issue.ID, Body: "x"}); apperror.CodeOf(err) != "project.archived" {
		t.Errorf("archived create: %v, want project.archived", err)
	}
	if _, err := f.svc.SetReaction(f.ctx, f.actor(f.author), comment.ID, "heart", true); apperror.CodeOf(err) != "project.archived" {
		t.Errorf("archived reaction: %v", err)
	}
	if page := f.threads(f.author, port.ThreadPageInput{}); len(page.Threads) != 1 {
		t.Error("archived projects stay readable")
	}
}

func TestCommentEventsForReplies(t *testing.T) {
	f := newCmtFixture(t)
	root := f.create(f.author, "Raiz").Comment.Comment
	f.reply(f.bob, root.ID, "Respuesta de Bob")
	f.reply(f.author, root.ID, "Mi propia respuesta")

	if len(f.events.list) != 2 {
		t.Fatalf("events = %d, want two reply events", len(f.events.list))
	}
	first := f.events.list[0]
	if first.Type != EventCommentCreated || first.Payload["reply_to_user_id"] != f.author.String() {
		t.Errorf("reply event = %+v, want the parent author", first)
	}
	second := f.events.list[1]
	if _, ok := second.Payload["reply_to_user_id"]; ok {
		t.Error("replying to your own comment must not name a recipient")
	}
	if first.ProjectID == nil || *first.ProjectID != f.issue.ProjectID || first.EntityType != "comment" {
		t.Errorf("event scope = %+v", first)
	}
}

func TestCommentWriteValidation(t *testing.T) {
	f := newCmtFixture(t)
	if _, err := f.svc.CreateComment(f.ctx, f.actor(f.author), port.CommentCreateInput{IssueID: f.issue.ID, Body: "   "}); apperror.CodeOf(err) != "issue.invalid_comment" {
		t.Errorf("blank body: %v", err)
	}
	long := strings.Repeat("x", domain.CommentMaxLength+1)
	if _, err := f.svc.CreateComment(f.ctx, f.actor(f.author), port.CommentCreateInput{IssueID: f.issue.ID, Body: long}); apperror.CodeOf(err) != "issue.comment_too_long" {
		t.Errorf("long body: %v", err)
	}
	var many strings.Builder
	for i := 0; i <= domain.CommentMaxMentions; i++ {
		many.WriteString(mentionOf(uuid.New(), "n") + " ")
	}
	if _, err := f.svc.CreateComment(f.ctx, f.actor(f.author), port.CommentCreateInput{IssueID: f.issue.ID, Body: many.String()}); apperror.CodeOf(err) != "issue.comment_too_many_mentions" {
		t.Errorf("too many mentions: %v", err)
	}
}
