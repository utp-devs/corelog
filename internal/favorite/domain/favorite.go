package domain

import (
	"time"

	"github.com/google/uuid"

	"github.com/jhonsferg/corelog/internal/platform/apperror"
)

// EntityType identifies the kind of element a user can star or visit.
type EntityType string

const (
	EntityProject   EntityType = "project"
	EntityFilter    EntityType = "filter"
	EntityBoard     EntityType = "board"
	EntityDashboard EntityType = "dashboard"
	EntityIssue     EntityType = "issue"
	EntityTeam      EntityType = "team"
)

const (
	// DefaultRecentLimit is the number of recent items returned when the caller does not pass a limit.
	DefaultRecentLimit = 10
	// MaxRecentLimit caps the number of recent items returned by one request.
	MaxRecentLimit = 50
	// RecentRetention is the number of recent items kept per user and entity type.
	RecentRetention = 50
)

var (
	ErrInvalidType      = apperror.WithCode(apperror.KindValidation, "favorite.invalid_type", "tipo de elemento no valido")
	ErrTypeNotSupported = apperror.WithCode(apperror.KindValidation, "favorite.type_not_supported", "este tipo de elemento todavia no admite favoritos ni recientes")
	ErrInvalidID        = apperror.WithCode(apperror.KindValidation, "favorite.invalid_id", "identificador no valido")
	ErrInvalidLimit     = apperror.WithCode(apperror.KindValidation, "favorite.invalid_limit", "el limite debe ser un entero positivo")
	ErrEntityNotFound   = apperror.WithCode(apperror.KindNotFound, "favorite.entity_not_found", "elemento no encontrado")
)

// ParseEntityType validates the type received from the caller.
func ParseEntityType(raw string) (EntityType, error) {
	switch EntityType(raw) {
	case EntityProject, EntityFilter, EntityBoard, EntityDashboard, EntityIssue, EntityTeam:
		return EntityType(raw), nil
	default:
		return "", ErrInvalidType
	}
}

// Favorite is a star placed by a user on one entity.
type Favorite struct {
	UserID    uuid.UUID
	Type      EntityType
	EntityID  uuid.UUID
	CreatedAt time.Time
}

// NewFavorite builds a star for the current time after checking the identifiers.
func NewFavorite(userID uuid.UUID, entityType EntityType, entityID uuid.UUID, now time.Time) (Favorite, error) {
	if userID == uuid.Nil || entityID == uuid.Nil {
		return Favorite{}, ErrInvalidID
	}
	return Favorite{UserID: userID, Type: entityType, EntityID: entityID, CreatedAt: now.UTC()}, nil
}

// RecentItem records the last time a user viewed an entity.
type RecentItem struct {
	UserID   uuid.UUID
	Type     EntityType
	EntityID uuid.UUID
	ViewedAt time.Time
}

// NewRecentItem builds a view record for the current time after checking the identifiers.
func NewRecentItem(userID uuid.UUID, entityType EntityType, entityID uuid.UUID, now time.Time) (RecentItem, error) {
	if userID == uuid.Nil || entityID == uuid.Nil {
		return RecentItem{}, ErrInvalidID
	}
	return RecentItem{UserID: userID, Type: entityType, EntityID: entityID, ViewedAt: now.UTC()}, nil
}
