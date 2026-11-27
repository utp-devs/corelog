package domain

import (
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"

	"github.com/jhonsferg/corelog/internal/platform/apperror"
)

var ErrInvalidComment = apperror.WithCode(apperror.KindValidation, "issue.invalid_comment", "comment body is required")

// CommentMaxLength is the longest comment body, in characters.
const CommentMaxLength = 20000

type Comment struct {
	ID       uuid.UUID
	IssueID  uuid.UUID
	AuthorID uuid.UUID
	// ParentID is the root comment of the thread when the comment is a reply; threads have a single level.
	ParentID   *uuid.UUID
	Body       string
	Visibility CommentVisibility
	// InternalRole is the lowest project role that can read an internal comment. It is CommentLevelNone for
	// public comments.
	InternalRole CommentLevel
	// Mentions are the users mentioned by the body that were valid when the body was saved.
	Mentions  []uuid.UUID
	CreatedAt time.Time
	UpdatedAt time.Time
	EditedAt  *time.Time
	DeletedAt *time.Time
}

// NewComment builds a public root comment. The body is trimmed; it must not be empty and must fit CommentMaxLength.
func NewComment(issueID, authorID uuid.UUID, body string, at time.Time) (*Comment, error) {
	body = strings.TrimSpace(body)
	if issueID == uuid.Nil || authorID == uuid.Nil || body == "" {
		return nil, ErrInvalidComment
	}
	if utf8.RuneCountInString(body) > CommentMaxLength {
		return nil, ErrCommentTooLong
	}
	at = at.UTC()
	return &Comment{
		IssueID: issueID, AuthorID: authorID, Body: body, Visibility: CommentVisibilityPublic,
		CreatedAt: at, UpdatedAt: at,
	}, nil
}

// Update replaces the body and marks the comment as edited at the given time.
func (c *Comment) Update(body string, at time.Time) error {
	body = strings.TrimSpace(body)
	if body == "" {
		return ErrInvalidComment
	}
	if utf8.RuneCountInString(body) > CommentMaxLength {
		return ErrCommentTooLong
	}
	at = at.UTC()
	c.Body = body
	c.UpdatedAt = at
	c.EditedAt = &at
	return nil
}

// Delete soft-deletes the comment. The body is cleared: the thread keeps the place with a placeholder.
func (c *Comment) Delete(at time.Time) {
	at = at.UTC()
	c.Body = ""
	c.DeletedAt = &at
	c.UpdatedAt = at
}

// IsDeleted reports whether the comment was soft-deleted.
func (c *Comment) IsDeleted() bool {
	return c.DeletedAt != nil
}

// IsReply reports whether the comment belongs to a thread (it has a root).
func (c *Comment) IsReply() bool {
	return c.ParentID != nil
}

// IsEdited reports whether the body was changed after creation.
func (c *Comment) IsEdited() bool {
	return c.EditedAt != nil
}

// NewAssigneeActivity registra un cambio de persona asignada. previous y next son nil cuando no hay persona
// asignada antes o despues (asignar desde vacio, desasignar).
func NewAssigneeActivity(issueID, actorID uuid.UUID, previous, next *uuid.UUID) *Activity {
	field := "assignee_id"
	activity := &Activity{
		IssueID:   issueID,
		ActorID:   actorID,
		Action:    "assignee_changed",
		FieldName: &field,
		CreatedAt: time.Now().UTC(),
	}
	if previous != nil {
		old := previous.String()
		activity.OldValue = &old
	}
	if next != nil {
		value := next.String()
		activity.NewValue = &value
	}
	return activity
}

type Activity struct {
	ID        uuid.UUID
	IssueID   uuid.UUID
	ActorID   uuid.UUID
	Action    string
	FieldName *string
	OldValue  *string
	NewValue  *string
	// BatchID groups the activities written by one patch.
	BatchID   *uuid.UUID
	CreatedAt time.Time
}

// NewFieldActivity records one field change of a patch. Every activity of one patch shares batchID. For custom
// fields the field name is the field id.
func NewFieldActivity(issueID, actorID, batchID uuid.UUID, change Change, at time.Time) *Activity {
	field := change.Field
	return &Activity{
		IssueID:   issueID,
		ActorID:   actorID,
		BatchID:   &batchID,
		Action:    "field_changed",
		FieldName: &field,
		OldValue:  change.From,
		NewValue:  change.To,
		CreatedAt: at,
	}
}
