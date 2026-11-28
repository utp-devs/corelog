// Package domain holds the audit log model: immutable entries built from
// domain events through a table of mapping rules.
package domain

import (
	"time"

	"github.com/google/uuid"
)

// Entry is one immutable audit record.
type Entry struct {
	ID             int64
	OrganizationID *uuid.UUID
	ProjectID      *uuid.UUID
	ActorID        *uuid.UUID
	Action         string
	EntityType     string
	EntityID       *uuid.UUID
	Summary        string
	Before         map[string]any
	After          map[string]any
	IP             string
	UserAgent      string
	SourceEventID  *int64
	// RequestID is the id of the HTTP request that caused the event.
	RequestID string
	// ActorName is read-only: it is joined from users when listing.
	ActorName string
	CreatedAt time.Time
}

// SourceEvent is the part of a domain event the audit mapper needs. It keeps
// this package independent of the events platform package.
type SourceEvent struct {
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
