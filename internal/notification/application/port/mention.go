package port

import (
	"context"

	"github.com/google/uuid"
)

// ProjectBrowser reports whether a user can still browse a project. Mention notices are sent only while it holds.
type ProjectBrowser interface {
	CanBrowse(ctx context.Context, userID, projectID uuid.UUID) (bool, error)
}

// MentionDeliveries records the in-app notices sent for comments, so that a redelivered event notifies once, and
// reads the mentions stored for a comment.
type MentionDeliveries interface {
	NoticeSent(ctx context.Context, commentID, userID uuid.UUID, kind string) (bool, error)
	RecordNotice(ctx context.Context, commentID, userID uuid.UUID, kind string) error
	IsMentioned(ctx context.Context, commentID, userID uuid.UUID) (bool, error)
}
