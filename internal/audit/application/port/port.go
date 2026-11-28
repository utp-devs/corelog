// Package port declares the audit log ports: the repository used by the
// consumer, the retention job and the future HTTP list endpoint.
package port

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/jhonsferg/corelog/internal/audit/domain"
)

// ListFilter narrows an audit list. Zero values mean "no filter". Entries are
// returned newest first; Cursor, when set, returns only entries with id < Cursor.
type ListFilter struct {
	OrganizationID *uuid.UUID
	ActorID        *uuid.UUID
	Action         string
	EntityType     string
	EntityID       *uuid.UUID
	Since          *time.Time // inclusive lower bound on created_at
	Until          *time.Time // exclusive upper bound on created_at
	Cursor         int64
	Limit          int // 0 selects DefaultLimit; values above MaxLimit are clamped
}

const (
	// DefaultLimit is the page size when ListFilter.Limit is zero.
	DefaultLimit = 50
	// MaxLimit is the largest page size the repository returns.
	MaxLimit = 200
)

// ListPage is one page of audit entries. NextCursor is zero when there are no
// more entries; otherwise pass it back as ListFilter.Cursor.
type ListPage struct {
	Entries    []domain.Entry
	NextCursor int64
}

// Repository stores and reads audit entries. Entries are never updated or
// deleted through this port, except PurgeOlderThan for retention.
type Repository interface {
	// Save stores the entry. An entry whose SourceEventID already exists is
	// ignored without error, so redelivered events do not duplicate entries.
	Save(ctx context.Context, e domain.Entry) error
	// List returns one page of entries matching the filter, newest first.
	List(ctx context.Context, f ListFilter) (ListPage, error)
	// PurgeOlderThan deletes entries created before cutoff and returns how many
	// were removed. It is the only path that may delete rows.
	PurgeOlderThan(ctx context.Context, cutoff time.Time) (int64, error)
	// ActorName returns the display name of a user, or an empty string when it is unknown.
	ActorName(ctx context.Context, id uuid.UUID) (string, error)
}
