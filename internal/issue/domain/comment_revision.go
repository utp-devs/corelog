package domain

import (
	"time"

	"github.com/google/uuid"
)

// CommentRevision is a body that a comment had before an edit. The current body lives in the comment itself.
type CommentRevision struct {
	ID        uuid.UUID
	CommentID uuid.UUID
	Body      string
	EditedBy  uuid.UUID
	// EditedAt is when the body was replaced by a newer one.
	EditedAt time.Time
}
