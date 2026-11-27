package port

import (
	"context"

	"github.com/google/uuid"
)

// Service is the favorites and recents use case. Entity types arrive as strings and are validated inside.
type Service interface {
	RecentRecorder
	Star(ctx context.Context, userID uuid.UUID, entityType string, entityID uuid.UUID) error
	Unstar(ctx context.Context, userID uuid.UUID, entityType string, entityID uuid.UUID) error
	ListFavorites(ctx context.Context, userID uuid.UUID, entityType string) ([]Entry, error)
	// ListRecent treats limit 0 as the default and clamps values above the maximum.
	ListRecent(ctx context.Context, userID uuid.UUID, entityType string, limit int) ([]Entry, error)
}
