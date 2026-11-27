package usecase

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/jhonsferg/corelog/internal/issue/application/port"
	"github.com/jhonsferg/corelog/internal/issue/domain"
	"github.com/jhonsferg/corelog/internal/platform/apperror"
	"github.com/jhonsferg/corelog/internal/platform/clock"
	"github.com/jhonsferg/corelog/internal/platform/database"
	"github.com/jhonsferg/corelog/internal/platform/events"
	"github.com/jhonsferg/corelog/internal/platform/pagination"
)

// Event types published by comment writes. The notification module consumes them.
const (
	EventCommentMentioned = "comment.mentioned"
	EventCommentCreated   = "comment.created"
)

// CommentService implements the comment in-port: threads, writes, revisions and reactions.
type CommentService struct {
	issues     port.Repository
	store      port.CommentStore
	perms      port.CommentPermissions
	authorizer port.ProjectAuthorizer
	names      port.UserDirectory
	tx         database.TxManager
	pub        events.Publisher
	clock      clock.Clock
}

// NewCommentService wires the comment use cases. The clock is required; the other ports may be nil in tests that
// do not use them (a nil permission checker denies everything, a nil directory shows no author names).
func NewCommentService(issues port.Repository, store port.CommentStore, perms port.CommentPermissions, authorizer port.ProjectAuthorizer, names port.UserDirectory, tx database.TxManager, pub events.Publisher, clk clock.Clock) *CommentService {
	return &CommentService{issues: issues, store: store, perms: perms, authorizer: authorizer, names: names, tx: tx, pub: pub, clock: clk}
}

var _ port.CommentService = (*CommentService)(nil)

func (s *CommentService) now() time.Time {
	return s.clock.Now().UTC().Truncate(time.Microsecond)
}

// issueScope loads the issue and the comment level of the actor. An actor who cannot browse the project gets not
// found.
func (s *CommentService) issueScope(ctx context.Context, actor port.CommentActor, issueID uuid.UUID) (*domain.Issue, domain.CommentLevel, error) {
	issue, err := s.issues.FindByID(ctx, issueID)
	if err != nil {
		return nil, domain.CommentLevelNone, err
	}
	level, err := commentLevelOf(ctx, s.perms, actor.ID, actor.GlobalRole, issue.ProjectID)
	if err != nil {
		return nil, domain.CommentLevelNone, err
	}
	if level == domain.CommentLevelNone {
		return nil, domain.CommentLevelNone, domain.ErrIssueNotFound
	}
	return issue, level, nil
}

// commentScope loads a comment the actor can read. Comments the actor cannot read are not found.
func (s *CommentService) commentScope(ctx context.Context, actor port.CommentActor, commentID uuid.UUID) (*domain.Comment, *domain.Issue, error) {
	comment, err := s.store.FindComment(ctx, commentID)
	if err != nil {
		return nil, nil, err
	}
	issue, level, err := s.issueScope(ctx, actor, comment.IssueID)
	if err != nil {
		if errors.Is(err, domain.ErrIssueNotFound) {
			return nil, nil, domain.ErrCommentNotFound
		}
		return nil, nil, err
	}
	if !domain.CanReadComment(level, comment.Visibility, comment.InternalRole) {
		return nil, nil, domain.ErrCommentNotFound
	}
	return comment, issue, nil
}

// allow runs a write action through the project authorizer. Denied actors get forbidden; writes on archived projects
// get the archive conflict, as in the rest of the issue module.
func (s *CommentService) allow(ctx context.Context, actor port.CommentActor, projectID uuid.UUID, action string) error {
	if s.authorizer == nil {
		return domain.ErrCommentForbidden
	}
	return s.authorizer.Authorize(ctx, action, actor.ID, actor.GlobalRole, projectID)
}

// allowOwn lets the author use the own-comment action and anyone else use manage_comments.
func (s *CommentService) allowOwn(ctx context.Context, actor port.CommentActor, projectID, authorID uuid.UUID, ownAction string) error {
	if actor.ID == authorID {
		err := s.allow(ctx, actor, projectID, ownAction)
		if err == nil || apperror.KindOf(err) != apperror.KindForbidden {
			return err
		}
	}
	return s.allow(ctx, actor, projectID, port.PermissionManageComments)
}

// ListThreads returns one page of root comments with their readable replies.
func (s *CommentService) ListThreads(ctx context.Context, actor port.CommentActor, issueID uuid.UUID, query port.ThreadPageInput) (*port.ThreadPage, error) {
	sort, err := domain.ParseCommentSort(query.Sort)
	if err != nil {
		return nil, err
	}
	limit, err := domain.NormalizeCommentLimit(query.Limit, query.LimitGiven)
	if err != nil {
		return nil, err
	}
	var after *domain.CommentCursor
	if query.Cursor != "" {
		var cursor domain.CommentCursor
		if err := pagination.DecodeCursor(query.Cursor, &cursor); err != nil {
			return nil, domain.ErrCommentCursorInvalid
		}
		after = &cursor
	}
	issue, level, err := s.issueScope(ctx, actor, issueID)
	if err != nil {
		return nil, err
	}
	// One extra root tells whether another page exists.
	roots, err := s.store.ListThreadRoots(ctx, issue.ID, level, sort, after, limit+1)
	if err != nil {
		return nil, err
	}
	page := &port.ThreadPage{Threads: []*port.CommentView{}}
	if len(roots) > limit {
		roots = roots[:limit]
		last := roots[len(roots)-1]
		cursor, err := pagination.EncodeCursor(domain.CommentCursor{CreatedAt: last.CreatedAt, ID: last.ID})
		if err != nil {
			return nil, err
		}
		page.NextCursor = cursor
	}
	if len(roots) == 0 {
		return page, nil
	}
	rootIDs := make([]uuid.UUID, 0, len(roots))
	for _, root := range roots {
		rootIDs = append(rootIDs, root.ID)
	}
	replies, err := s.store.ListReplies(ctx, issue.ID, rootIDs, level)
	if err != nil {
		return nil, err
	}
	all := append(append([]*domain.Comment{}, roots...), replies...)
	ids := make([]uuid.UUID, 0, len(all))
	authorIDs := make([]uuid.UUID, 0, len(all))
	for _, c := range all {
		ids = append(ids, c.ID)
		authorIDs = append(authorIDs, c.AuthorID)
	}
	mentions, err := s.store.ListMentions(ctx, ids)
	if err != nil {
		return nil, err
	}
	reactions, err := s.store.ReactionCounts(ctx, ids, actor.ID)
	if err != nil {
		return nil, err
	}
	names, err := s.authorNames(ctx, authorIDs)
	if err != nil {
		return nil, err
	}
	views := make(map[uuid.UUID]*port.CommentView, len(all))
	for _, c := range all {
		c.Mentions = mentions[c.ID]
		views[c.ID] = &port.CommentView{Comment: c, AuthorName: names[c.AuthorID], Reactions: reactions[c.ID]}
	}
	for _, reply := range replies {
		root := views[*reply.ParentID]
		if root == nil {
			continue
		}
		root.Replies = append(root.Replies, views[reply.ID])
	}
	for _, root := range roots {
		view := views[root.ID]
		view.ReplyCount = len(view.Replies)
		page.Threads = append(page.Threads, view)
	}
	return page, nil
}

// CreateComment stores a root comment or a reply. A reply attaches to the root of its thread and inherits the
// internal visibility of an internal root.
func (s *CommentService) CreateComment(ctx context.Context, actor port.CommentActor, in port.CommentCreateInput) (*port.CommentResult, error) {
	issue, level, err := s.issueScope(ctx, actor, in.IssueID)
	if err != nil {
		return nil, err
	}
	if err := s.allow(ctx, actor, issue.ProjectID, port.PermissionCommentIssue); err != nil {
		return nil, err
	}
	visibility, role, err := parseVisibilityInput(in.Visibility, in.InternalRole)
	if err != nil {
		return nil, err
	}
	var parent, root *domain.Comment
	if in.ParentID != nil {
		if parent, root, err = s.replyTarget(ctx, issue, level, *in.ParentID); err != nil {
			return nil, err
		}
		if root.Visibility == domain.CommentVisibilityInternal {
			if visibility == domain.CommentVisibilityPublic {
				return nil, domain.ErrCommentReplyVisibility
			}
			if role < root.InternalRole {
				role = root.InternalRole
			}
		}
	}
	if visibility == domain.CommentVisibilityInternal && role > level {
		return nil, domain.ErrCommentVisibilityInvalid
	}
	comment, err := domain.NewComment(issue.ID, actor.ID, in.Body, s.now())
	if err != nil {
		return nil, err
	}
	if visibility == domain.CommentVisibilityInternal {
		comment.Visibility = domain.CommentVisibilityInternal
		comment.InternalRole = role
	}
	if root != nil {
		rootID := root.ID
		comment.ParentID = &rootID
	}
	resolved, err := s.resolveMentions(ctx, issue.ProjectID, actor.ID, comment.Body, comment.Visibility, comment.InternalRole)
	if err != nil {
		return nil, err
	}
	comment.Mentions = resolved.ids

	// A reply notifies the parent author only when that author can read the reply.
	replyTo := uuid.Nil
	if parent != nil && parent.AuthorID != actor.ID {
		parentLevel, err := commentLevelOf(ctx, s.perms, parent.AuthorID, "user", issue.ProjectID)
		if err != nil {
			return nil, err
		}
		if domain.CanReadComment(parentLevel, comment.Visibility, comment.InternalRole) {
			replyTo = parent.AuthorID
		}
	}

	err = s.inTx(ctx, func(ctx context.Context) error {
		if err := s.store.InsertComment(ctx, comment); err != nil {
			return err
		}
		if len(comment.Mentions) > 0 {
			if err := s.publish(ctx, EventCommentMentioned, issue, comment, actor.ID, map[string]any{
				"user_ids": idStrings(comment.Mentions),
			}); err != nil {
				return err
			}
		}
		if parent != nil {
			payload := map[string]any{}
			if replyTo != uuid.Nil {
				payload["reply_to_user_id"] = replyTo.String()
			}
			return s.publish(ctx, EventCommentCreated, issue, comment, actor.ID, payload)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	view, err := s.view(ctx, comment)
	if err != nil {
		return nil, err
	}
	return &port.CommentResult{Comment: view, Warnings: resolved.warnings}, nil
}

// replyTarget loads the comment a reply points to and its root. Any comment that is missing, deleted, from another
// issue or unreadable by the actor makes the reply invalid.
func (s *CommentService) replyTarget(ctx context.Context, issue *domain.Issue, level domain.CommentLevel, parentID uuid.UUID) (*domain.Comment, *domain.Comment, error) {
	parent, err := s.store.FindComment(ctx, parentID)
	if err != nil {
		if errors.Is(err, domain.ErrCommentNotFound) {
			return nil, nil, domain.ErrCommentParentInvalid
		}
		return nil, nil, err
	}
	if parent.IssueID != issue.ID || parent.IsDeleted() || !domain.CanReadComment(level, parent.Visibility, parent.InternalRole) {
		return nil, nil, domain.ErrCommentParentInvalid
	}
	root := parent
	if parent.IsReply() {
		if root, err = s.store.FindComment(ctx, *parent.ParentID); err != nil {
			return nil, nil, err
		}
		if root.IsDeleted() || !domain.CanReadComment(level, root.Visibility, root.InternalRole) {
			return nil, nil, domain.ErrCommentParentInvalid
		}
	}
	return parent, root, nil
}

// UpdateComment replaces the body of a live comment. The previous body is kept as a revision, and only the newly
// mentioned users are notified.
func (s *CommentService) UpdateComment(ctx context.Context, actor port.CommentActor, commentID uuid.UUID, body string) (*port.CommentResult, error) {
	comment, issue, err := s.commentScope(ctx, actor, commentID)
	if err != nil {
		return nil, err
	}
	if comment.IsDeleted() {
		return nil, domain.ErrCommentNotFound
	}
	if err := s.allowOwn(ctx, actor, issue.ProjectID, comment.AuthorID, port.PermissionEditOwnComment); err != nil {
		return nil, err
	}
	previous := comment.Body
	if err := comment.Update(body, s.now()); err != nil {
		return nil, err
	}
	resolved, err := s.resolveMentions(ctx, issue.ProjectID, comment.AuthorID, comment.Body, comment.Visibility, comment.InternalRole)
	if err != nil {
		return nil, err
	}
	existing, err := s.store.MentionedUserIDs(ctx, comment.ID)
	if err != nil {
		return nil, err
	}
	comment.Mentions = resolved.ids
	added := newlyMentioned(existing, resolved.ids)
	err = s.inTx(ctx, func(ctx context.Context) error {
		if comment.Body != previous {
			if err := s.store.UpdateCommentBody(ctx, comment, previous, actor.ID); err != nil {
				return err
			}
		}
		if err := s.store.ReplaceMentions(ctx, comment.ID, resolved.ids); err != nil {
			return err
		}
		if len(added) > 0 {
			return s.publish(ctx, EventCommentMentioned, issue, comment, actor.ID, map[string]any{
				"user_ids": idStrings(added),
			})
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	view, err := s.view(ctx, comment)
	if err != nil {
		return nil, err
	}
	return &port.CommentResult{Comment: view, Warnings: resolved.warnings}, nil
}

// DeleteComment soft-deletes a live comment. The thread keeps its place with a placeholder.
func (s *CommentService) DeleteComment(ctx context.Context, actor port.CommentActor, commentID uuid.UUID) error {
	comment, issue, err := s.commentScope(ctx, actor, commentID)
	if err != nil {
		return err
	}
	if comment.IsDeleted() {
		return domain.ErrCommentNotFound
	}
	if err := s.allowOwn(ctx, actor, issue.ProjectID, comment.AuthorID, port.PermissionDeleteOwnComment); err != nil {
		return err
	}
	comment.Delete(s.now())
	return s.inTx(ctx, func(ctx context.Context) error {
		return s.store.SoftDeleteComment(ctx, comment)
	})
}

// ListRevisions returns the previous bodies of a live comment for its author or a manager of comments.
func (s *CommentService) ListRevisions(ctx context.Context, actor port.CommentActor, commentID uuid.UUID) ([]domain.CommentRevision, error) {
	comment, issue, err := s.commentScope(ctx, actor, commentID)
	if err != nil {
		return nil, err
	}
	if comment.IsDeleted() {
		return nil, domain.ErrCommentNotFound
	}
	if err := s.allowOwn(ctx, actor, issue.ProjectID, comment.AuthorID, port.PermissionEditOwnComment); err != nil {
		return nil, err
	}
	return s.store.ListRevisions(ctx, comment.ID)
}

// SetReaction adds or removes the actor's reaction. Adding twice or removing twice leaves the same state.
func (s *CommentService) SetReaction(ctx context.Context, actor port.CommentActor, commentID uuid.UUID, emoji string, on bool) (*port.CommentReactions, error) {
	if !domain.IsCommentReaction(emoji) {
		return nil, domain.ErrCommentReactionInvalid
	}
	comment, issue, err := s.commentScope(ctx, actor, commentID)
	if err != nil {
		return nil, err
	}
	if comment.IsDeleted() {
		return nil, domain.ErrCommentNotFound
	}
	if err := s.allow(ctx, actor, issue.ProjectID, port.PermissionCommentIssue); err != nil {
		return nil, err
	}
	if err := s.store.SetReaction(ctx, comment.ID, actor.ID, emoji, on); err != nil {
		return nil, err
	}
	counts, err := s.store.ReactionCounts(ctx, []uuid.UUID{comment.ID}, actor.ID)
	if err != nil {
		return nil, err
	}
	reactions := counts[comment.ID]
	if reactions == nil {
		reactions = []port.ReactionCount{}
	}
	return &port.CommentReactions{CommentID: comment.ID, Reactions: reactions}, nil
}

// view prepares one comment for a write response: author name, mentions and reactions of the author's viewer.
func (s *CommentService) view(ctx context.Context, comment *domain.Comment) (*port.CommentView, error) {
	names, err := s.authorNames(ctx, []uuid.UUID{comment.AuthorID})
	if err != nil {
		return nil, err
	}
	counts, err := s.store.ReactionCounts(ctx, []uuid.UUID{comment.ID}, comment.AuthorID)
	if err != nil {
		return nil, err
	}
	return &port.CommentView{Comment: comment, AuthorName: names[comment.AuthorID], Reactions: counts[comment.ID]}, nil
}

func (s *CommentService) authorNames(ctx context.Context, ids []uuid.UUID) (map[uuid.UUID]string, error) {
	if s.names == nil || len(ids) == 0 {
		return map[uuid.UUID]string{}, nil
	}
	return s.names.NamesOf(ctx, ids)
}

func (s *CommentService) inTx(ctx context.Context, fn func(ctx context.Context) error) error {
	if s.tx == nil {
		return fn(ctx)
	}
	return s.tx.InTx(ctx, fn)
}

// publish emits a comment event with the issue and the comment identifiers filled in.
func (s *CommentService) publish(ctx context.Context, eventType string, issue *domain.Issue, comment *domain.Comment, actorID uuid.UUID, payload map[string]any) error {
	if s.pub == nil {
		return nil
	}
	projectID := issue.ProjectID
	entityID := comment.ID
	actor := actorID
	payload["comment_id"] = comment.ID.String()
	payload["issue_id"] = issue.ID.String()
	payload["issue_key"] = commentIssueKey(issue.ProjectKey, issue.KeyNumber)
	return s.pub.Publish(ctx, events.Event{
		ProjectID:  &projectID,
		EntityID:   &entityID,
		ActorID:    &actor,
		Type:       eventType,
		EntityType: "comment",
		Payload:    payload,
	})
}

// parseVisibilityInput maps the visibility and internal role of a new comment. The default is public; an internal
// comment defaults to the contributor role.
func parseVisibilityInput(visibility, internalRole string) (domain.CommentVisibility, domain.CommentLevel, error) {
	switch strings.TrimSpace(visibility) {
	case "", string(domain.CommentVisibilityPublic):
		if internalRole != "" {
			return "", domain.CommentLevelNone, domain.ErrCommentVisibilityInvalid
		}
		return domain.CommentVisibilityPublic, domain.CommentLevelNone, nil
	case string(domain.CommentVisibilityInternal):
		if internalRole == "" {
			return domain.CommentVisibilityInternal, domain.CommentLevelContributor, nil
		}
		role, err := domain.ParseInternalRole(internalRole)
		if err != nil {
			return "", domain.CommentLevelNone, err
		}
		return domain.CommentVisibilityInternal, role, nil
	default:
		return "", domain.CommentLevelNone, domain.ErrCommentVisibilityInvalid
	}
}

func idStrings(ids []uuid.UUID) []string {
	out := make([]string, 0, len(ids))
	for _, id := range ids {
		out = append(out, id.String())
	}
	return out
}
