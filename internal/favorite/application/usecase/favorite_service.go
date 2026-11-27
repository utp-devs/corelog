package usecase

import (
	"context"

	"github.com/google/uuid"

	"github.com/jhonsferg/corelog/internal/favorite/application/port"
	"github.com/jhonsferg/corelog/internal/favorite/domain"
	"github.com/jhonsferg/corelog/internal/platform/clock"
)

// Service implements favorites and recents. Visibility is checked per entity type through the registry;
// entity types without a registered visibility rule are rejected as unsupported.
type Service struct {
	repo       port.Repository
	visibility map[domain.EntityType]port.EntityVisibility
	clock      clock.Clock
}

var _ port.Service = (*Service)(nil)

// NewService builds the use case. visibility maps each supported entity type to its visibility rule.
func NewService(repo port.Repository, visibility map[domain.EntityType]port.EntityVisibility, clk clock.Clock) *Service {
	return &Service{repo: repo, visibility: visibility, clock: clk}
}

// resolve validates the type string and returns the visibility rule registered for it.
func (s *Service) resolve(raw string) (domain.EntityType, port.EntityVisibility, error) {
	entityType, err := domain.ParseEntityType(raw)
	if err != nil {
		return "", nil, err
	}
	rule, ok := s.visibility[entityType]
	if !ok {
		return "", nil, domain.ErrTypeNotSupported
	}
	return entityType, rule, nil
}

// Star marks an entity as favorite. Starring an entity the user cannot see answers not found.
// Starring twice is a no-op.
func (s *Service) Star(ctx context.Context, userID uuid.UUID, raw string, entityID uuid.UUID) error {
	entityType, rule, err := s.resolve(raw)
	if err != nil {
		return err
	}
	favorite, err := domain.NewFavorite(userID, entityType, entityID, s.clock.Now())
	if err != nil {
		return err
	}
	if err := s.requireVisible(ctx, rule, userID, entityID); err != nil {
		return err
	}
	return s.repo.AddFavorite(ctx, favorite)
}

// Unstar removes a star. It does not check visibility, so a user can drop a star on an entity that
// became invisible or deleted.
func (s *Service) Unstar(ctx context.Context, userID uuid.UUID, raw string, entityID uuid.UUID) error {
	entityType, _, err := s.resolve(raw)
	if err != nil {
		return err
	}
	if userID == uuid.Nil || entityID == uuid.Nil {
		return domain.ErrInvalidID
	}
	return s.repo.RemoveFavorite(ctx, userID, entityType, entityID)
}

// ListFavorites returns the starred entities the user can still see.
func (s *Service) ListFavorites(ctx context.Context, userID uuid.UUID, raw string) ([]port.Entry, error) {
	entityType, rule, err := s.resolve(raw)
	if err != nil {
		return nil, err
	}
	entries, err := s.repo.ListFavorites(ctx, userID, entityType)
	if err != nil {
		return nil, err
	}
	return s.filterVisible(ctx, rule, userID, entries)
}

// ListRecent returns up to limit recent entities the user can still see. The repository is asked for the
// full retention window so that hidden entries do not shorten the result below the requested size.
func (s *Service) ListRecent(ctx context.Context, userID uuid.UUID, raw string, limit int) ([]port.Entry, error) {
	entityType, rule, err := s.resolve(raw)
	if err != nil {
		return nil, err
	}
	switch {
	case limit < 0:
		return nil, domain.ErrInvalidLimit
	case limit == 0:
		limit = domain.DefaultRecentLimit
	case limit > domain.MaxRecentLimit:
		limit = domain.MaxRecentLimit
	}
	entries, err := s.repo.ListRecent(ctx, userID, entityType, domain.RecentRetention)
	if err != nil {
		return nil, err
	}
	visible, err := s.filterVisible(ctx, rule, userID, entries)
	if err != nil {
		return nil, err
	}
	if len(visible) > limit {
		visible = visible[:limit]
	}
	return visible, nil
}

// RecordView stores a view of an entity. The caller has normally checked visibility already; it is
// checked again here so that the recents list never holds entities the user cannot open.
func (s *Service) RecordView(ctx context.Context, userID uuid.UUID, raw string, entityID uuid.UUID) error {
	entityType, rule, err := s.resolve(raw)
	if err != nil {
		return err
	}
	item, err := domain.NewRecentItem(userID, entityType, entityID, s.clock.Now())
	if err != nil {
		return err
	}
	if err := s.requireVisible(ctx, rule, userID, entityID); err != nil {
		return err
	}
	return s.repo.TouchRecent(ctx, item)
}

func (s *Service) requireVisible(ctx context.Context, rule port.EntityVisibility, userID, entityID uuid.UUID) error {
	ok, err := rule.CanView(ctx, userID, entityID)
	if err != nil {
		return err
	}
	if !ok {
		return domain.ErrEntityNotFound
	}
	return nil
}

func (s *Service) filterVisible(ctx context.Context, rule port.EntityVisibility, userID uuid.UUID, entries []port.Entry) ([]port.Entry, error) {
	visible := make([]port.Entry, 0, len(entries))
	for _, entry := range entries {
		ok, err := rule.CanView(ctx, userID, entry.ID)
		if err != nil {
			return nil, err
		}
		if ok {
			visible = append(visible, entry)
		}
	}
	return visible, nil
}
