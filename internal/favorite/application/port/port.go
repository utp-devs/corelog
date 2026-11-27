package port

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/jhonsferg/corelog/internal/favorite/domain"
)

// Entry is the display view of one starred or recently viewed entity.
// CreatedAt is the time the star was placed for favorites and the creation time of the entity for recents.
type Entry struct {
	Type      domain.EntityType
	ID        uuid.UUID
	Name      string
	Key       string
	Color     string
	CreatedAt time.Time
	ViewedAt  *time.Time
}

// Repository persists stars and recent views. Reads resolve display data for the entity type.
type Repository interface {
	// AddFavorite stores the star. Adding a star that already exists is not an error.
	AddFavorite(ctx context.Context, favorite domain.Favorite) error
	// RemoveFavorite deletes the star. Removing a missing star is not an error.
	RemoveFavorite(ctx context.Context, userID uuid.UUID, entityType domain.EntityType, entityID uuid.UUID) error
	// ListFavorites returns the starred entities of one type, newest star first, skipping deleted entities.
	ListFavorites(ctx context.Context, userID uuid.UUID, entityType domain.EntityType) ([]Entry, error)
	// TouchRecent upserts the view time and prunes the user's list of this type to domain.RecentRetention
	// entries in one transaction.
	TouchRecent(ctx context.Context, item domain.RecentItem) error
	// ListRecent returns up to limit recent entities of one type, most recent view first, skipping deleted entities.
	ListRecent(ctx context.Context, userID uuid.UUID, entityType domain.EntityType, limit int) ([]Entry, error)
}

// EntityVisibility decides whether a user may see one entity. One implementation is registered per entity type.
type EntityVisibility interface {
	CanView(ctx context.Context, userID, entityID uuid.UUID) (bool, error)
}

// RecentRecorder is called by other modules when an entity is viewed. The entity type is passed as its
// string value (for example "project") so that callers do not depend on this package's domain types.
// Callers should treat failures as non-fatal: the view itself has already succeeded.
type RecentRecorder interface {
	RecordView(ctx context.Context, userID uuid.UUID, entityType string, entityID uuid.UUID) error
}
