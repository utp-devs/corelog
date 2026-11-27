package domain

import (
	"time"

	"github.com/google/uuid"
)

// NewLinkActivity records a link added or removed on an issue. other is the key of the issue at the far end.
func NewLinkActivity(issueID, actorID uuid.UUID, action string, kind IssueLinkKind, other string, at time.Time) *Activity {
	field := "links"
	value := string(kind) + " " + other
	return &Activity{
		IssueID:   issueID,
		ActorID:   actorID,
		Action:    action,
		FieldName: &field,
		NewValue:  &value,
		CreatedAt: at.UTC(),
	}
}
