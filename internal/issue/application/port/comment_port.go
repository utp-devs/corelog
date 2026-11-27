package port

import (
	"context"

	"github.com/google/uuid"

	"github.com/jhonsferg/corelog/internal/issue/domain"
)

// CommentStore persists comments with their mentions, revisions, reactions and thread reads. Reads that depend on
// the viewer take the comment level of the viewer and only return the comments that viewer can read.
type CommentStore interface {
	// FindComment returns the comment, deleted or not. A missing comment returns domain.ErrCommentNotFound.
	FindComment(ctx context.Context, id uuid.UUID) (*domain.Comment, error)
	// InsertComment stores the comment, its mentions and the follow of its author. It sets the id.
	InsertComment(ctx context.Context, comment *domain.Comment) error
	// UpdateCommentBody stores the new body of the comment and a revision with the previous body.
	UpdateCommentBody(ctx context.Context, comment *domain.Comment, previousBody string, editedBy uuid.UUID) error
	// SoftDeleteComment marks the comment deleted and removes its body, revisions, reactions and mentions.
	SoftDeleteComment(ctx context.Context, comment *domain.Comment) error
	// ReplaceMentions sets the mentioned users of the comment.
	ReplaceMentions(ctx context.Context, commentID uuid.UUID, userIDs []uuid.UUID) error
	// MentionedUserIDs returns the users currently mentioned by the comment.
	MentionedUserIDs(ctx context.Context, commentID uuid.UUID) ([]uuid.UUID, error)
	// ListMentions returns the mentioned users of each comment in one statement.
	ListMentions(ctx context.Context, commentIDs []uuid.UUID) (map[uuid.UUID][]uuid.UUID, error)
	// ListRevisions returns the previous bodies of the comment, newest first.
	ListRevisions(ctx context.Context, commentID uuid.UUID) ([]domain.CommentRevision, error)
	// ListThreadRoots returns up to limit root comments of the issue after the cursor, in the given sort order.
	// A deleted root is returned only when it still has a readable reply.
	ListThreadRoots(ctx context.Context, issueID uuid.UUID, viewer domain.CommentLevel, sort string, after *domain.CommentCursor, limit int) ([]*domain.Comment, error)
	// ListReplies returns the readable live replies of the given roots, oldest first.
	ListReplies(ctx context.Context, issueID uuid.UUID, rootIDs []uuid.UUID, viewer domain.CommentLevel) ([]*domain.Comment, error)
	// ReactionCounts returns the reaction counts of each comment, in the fixed reaction order, and whether the viewer
	// reacted with each one.
	ReactionCounts(ctx context.Context, commentIDs []uuid.UUID, viewerID uuid.UUID) (map[uuid.UUID][]ReactionCount, error)
	// SetReaction adds (on) or removes (!on) the reaction of the user. Repeating the call is harmless.
	SetReaction(ctx context.Context, commentID, userID uuid.UUID, emoji string, on bool) error
}

// CommentPermissions answers the project permission questions of the comment use cases. It matches the per-permission
// checker of the issue module.
type CommentPermissions interface {
	Can(ctx context.Context, actorID uuid.UUID, globalRole string, projectID uuid.UUID, permission string, holders ...string) (bool, error)
}

// Permission names used by the comment use cases (catalog names of the authorization module).
const (
	PermissionBrowseProject    = "browse_project"
	PermissionCommentIssue     = "comment_issue"
	PermissionEditOwnComment   = "edit_own_comment"
	PermissionDeleteOwnComment = "delete_own_comment"
	PermissionManageComments   = "manage_comments"
)

// CommentActor is the authenticated caller of a comment operation.
type CommentActor struct {
	ID         uuid.UUID
	GlobalRole string
}

// CommentCreateInput is a new comment. Visibility is "public" or "internal" ("" means public); InternalRole is
// "contributor" or "lead" ("" means contributor) and only applies to internal comments.
type CommentCreateInput struct {
	IssueID      uuid.UUID
	Body         string
	ParentID     *uuid.UUID
	Visibility   string
	InternalRole string
}

// ThreadPageInput is the validated query of the thread list.
type ThreadPageInput struct {
	Sort       string
	Cursor     string
	Limit      int
	LimitGiven bool
}

// ThreadPage is one page of root comments with their replies.
type ThreadPage struct {
	Threads    []*CommentView
	NextCursor string
}

// CommentView is a comment prepared for the API: author name, mentions, reactions and, for roots, the replies.
type CommentView struct {
	Comment    *domain.Comment
	AuthorName string
	Reactions  []ReactionCount
	ReplyCount int
	Replies    []*CommentView
}

// ReactionCount is the number of users who reacted with one emoji and whether the viewer is one of them.
type ReactionCount struct {
	Emoji         string
	Count         int
	ViewerReacted bool
}

// MentionWarning reports a mention that was kept as plain text because the user cannot be mentioned there. The
// message is the same for unknown users and for users without access, so it does not reveal who exists.
type MentionWarning struct {
	UserID  uuid.UUID
	Code    string
	Message string
}

// CommentResult is a written comment with the mention warnings of its body.
type CommentResult struct {
	Comment  *CommentView
	Warnings []MentionWarning
}

// CommentReactions is the reaction summary of one comment.
type CommentReactions struct {
	CommentID uuid.UUID
	Reactions []ReactionCount
}

// CommentService is the in-port of comments: threads, writes, revisions and reactions. Callers that cannot see the
// issue or the comment get not found; visible but denied operations get forbidden.
type CommentService interface {
	ListThreads(ctx context.Context, actor CommentActor, issueID uuid.UUID, query ThreadPageInput) (*ThreadPage, error)
	CreateComment(ctx context.Context, actor CommentActor, in CommentCreateInput) (*CommentResult, error)
	UpdateComment(ctx context.Context, actor CommentActor, commentID uuid.UUID, body string) (*CommentResult, error)
	DeleteComment(ctx context.Context, actor CommentActor, commentID uuid.UUID) error
	ListRevisions(ctx context.Context, actor CommentActor, commentID uuid.UUID) ([]domain.CommentRevision, error)
	SetReaction(ctx context.Context, actor CommentActor, commentID uuid.UUID, emoji string, on bool) (*CommentReactions, error)
}
