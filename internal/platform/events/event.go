// Package events implements the domain event outbox: producers publish
// events inside the same transaction as their change, and dispatchers deliver
// them to consumers at least once.
package events

import (
	"time"

	"github.com/google/uuid"
)

// Event is a domain event as stored in the domain_events table.
type Event struct {
	ID         int64
	OrgID      *uuid.UUID
	ProjectID  *uuid.UUID
	EntityID   *uuid.UUID
	ActorID    *uuid.UUID
	Type       string
	EntityType string
	Payload    map[string]any
	CreatedAt  time.Time
}
