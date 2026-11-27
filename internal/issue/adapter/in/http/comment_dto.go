package http

import (
	"time"

	"github.com/google/uuid"

	"github.com/jhonsferg/corelog/internal/issue/application/port"
	"github.com/jhonsferg/corelog/internal/issue/domain"
)

// commentCreateRequest is the body of POST /issues/{id}/comments. Unknown fields are rejected by the decoder.
type commentCreateRequest struct {
	Body         string  `json:"body"`
	ParentID     *string `json:"parent_id"`
	Visibility   string  `json:"visibility"`
	InternalRole string  `json:"internal_role"`
}

// commentUpdateRequest is the body of PATCH /comments/{id}.
type commentUpdateRequest struct {
	Body string `json:"body"`
}

type commentAuthorResponse struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type reactionResponse struct {
	Emoji         string `json:"emoji"`
	Count         int    `json:"count"`
	ViewerReacted bool   `json:"viewer_reacted"`
}

// commentResponse is one comment. Root comments of a thread list carry reply_count and replies; replies do not.
type commentResponse struct {
	ID           string                `json:"id"`
	IssueID      string                `json:"issue_id"`
	ParentID     *string               `json:"parent_id"`
	AuthorID     string                `json:"author_id"`
	Author       commentAuthorResponse `json:"author"`
	Body         string                `json:"body"`
	Visibility   string                `json:"visibility"`
	InternalRole *string               `json:"internal_role"`
	Mentions     []string              `json:"mentions"`
	Reactions    []reactionResponse    `json:"reactions"`
	Edited       bool                  `json:"edited"`
	EditedAt     *time.Time            `json:"edited_at"`
	Deleted      bool                  `json:"deleted"`
	CreatedAt    time.Time             `json:"created_at"`
	UpdatedAt    time.Time             `json:"updated_at"`
	DeletedAt    *time.Time            `json:"deleted_at,omitempty"`
	ReplyCount   *int                  `json:"reply_count,omitempty"`
	Replies      *[]commentResponse    `json:"replies,omitempty"`
}

type mentionWarningResponse struct {
	UserID  string `json:"user_id"`
	Code    string `json:"code"`
	Message string `json:"message"`
}

// commentWriteResponse is a written comment with the mention warnings of its body.
type commentWriteResponse struct {
	commentResponse
	Warnings []mentionWarningResponse `json:"warnings"`
}

type commentReactionsResponse struct {
	CommentID string             `json:"comment_id"`
	Reactions []reactionResponse `json:"reactions"`
}

type commentRevisionResponse struct {
	ID       string    `json:"id"`
	Body     string    `json:"body"`
	EditedBy string    `json:"edited_by"`
	EditedAt time.Time `json:"edited_at"`
}

// commentPageMeta is the cursor page info of the thread list.
type commentPageMeta struct {
	Limit      int     `json:"limit"`
	NextCursor *string `json:"next_cursor"`
}

func toCommentResponse(view *port.CommentView) commentResponse {
	c := view.Comment
	out := commentResponse{
		ID: c.ID.String(), IssueID: c.IssueID.String(), AuthorID: c.AuthorID.String(),
		Author:     commentAuthorResponse{ID: c.AuthorID.String(), Name: view.AuthorName},
		Body:       c.Body,
		Visibility: string(c.Visibility),
		Mentions:   uuidStringsOf(c.Mentions),
		Reactions:  reactionResponses(view.Reactions),
		Edited:     c.IsEdited(), EditedAt: c.EditedAt,
		Deleted: c.IsDeleted(), DeletedAt: c.DeletedAt,
		CreatedAt: c.CreatedAt, UpdatedAt: c.UpdatedAt,
	}
	if c.ParentID != nil {
		parent := c.ParentID.String()
		out.ParentID = &parent
	}
	if c.Visibility == domain.CommentVisibilityInternal {
		role := c.InternalRole.Name()
		out.InternalRole = &role
	}
	if c.IsDeleted() {
		out.Body = domain.CommentDeletedPlaceholder
	}
	return out
}

// toThreadResponse adds the reply count and the replies of a root comment.
func toThreadResponse(view *port.CommentView) commentResponse {
	out := toCommentResponse(view)
	count := view.ReplyCount
	out.ReplyCount = &count
	replies := make([]commentResponse, 0, len(view.Replies))
	for _, reply := range view.Replies {
		replies = append(replies, toCommentResponse(reply))
	}
	out.Replies = &replies
	return out
}

func toCommentWriteResponse(result *port.CommentResult) commentWriteResponse {
	warnings := make([]mentionWarningResponse, 0, len(result.Warnings))
	for _, w := range result.Warnings {
		warnings = append(warnings, mentionWarningResponse{UserID: w.UserID.String(), Code: w.Code, Message: w.Message})
	}
	return commentWriteResponse{commentResponse: toCommentResponse(result.Comment), Warnings: warnings}
}

func toReactionsResponse(reactions *port.CommentReactions) commentReactionsResponse {
	return commentReactionsResponse{CommentID: reactions.CommentID.String(), Reactions: reactionResponses(reactions.Reactions)}
}

func toRevisionResponses(revisions []domain.CommentRevision) []commentRevisionResponse {
	out := make([]commentRevisionResponse, 0, len(revisions))
	for _, rev := range revisions {
		out = append(out, commentRevisionResponse{
			ID: rev.ID.String(), Body: rev.Body, EditedBy: rev.EditedBy.String(), EditedAt: rev.EditedAt,
		})
	}
	return out
}

func reactionResponses(counts []port.ReactionCount) []reactionResponse {
	out := make([]reactionResponse, 0, len(counts))
	for _, c := range counts {
		out = append(out, reactionResponse{Emoji: c.Emoji, Count: c.Count, ViewerReacted: c.ViewerReacted})
	}
	return out
}

func uuidStringsOf(ids []uuid.UUID) []string {
	out := make([]string, 0, len(ids))
	for _, id := range ids {
		out = append(out, id.String())
	}
	return out
}
