package domain

import (
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/jhonsferg/corelog/internal/platform/apperror"
)

var ErrInvalidComment = apperror.WithCode(apperror.KindValidation, "issue.invalid_comment", "comment body is required")

type Comment struct {
	ID        uuid.UUID
	IssueID   uuid.UUID
	AuthorID  uuid.UUID
	Body      string
	Mentions  []uuid.UUID
	CreatedAt time.Time
	UpdatedAt time.Time
	DeletedAt *time.Time
}

func NewComment(issueID, authorID uuid.UUID, body string) (*Comment, error) {
	body = strings.TrimSpace(body)
	if issueID == uuid.Nil || authorID == uuid.Nil || body == "" {
		return nil, ErrInvalidComment
	}
	now := time.Now().UTC()
	return &Comment{
		IssueID: issueID, AuthorID: authorID, Body: body,
		CreatedAt: now, UpdatedAt: now,
	}, nil
}

func (c *Comment) Update(body string) error {
	body = strings.TrimSpace(body)
	if body == "" {
		return ErrInvalidComment
	}
	c.Body = body
	c.UpdatedAt = time.Now().UTC()
	return nil
}

func (c *Comment) Delete() {
	now := time.Now().UTC()
	c.DeletedAt = &now
	c.UpdatedAt = now
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
