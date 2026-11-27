package usecase

import (
	"context"

	"github.com/jhonsferg/corelog/internal/search/application/port"
	"github.com/jhonsferg/corelog/internal/search/domain"
)

// GlobalService searches every kind of entity the viewer can see. Notes come from a separate source because
// they are private to their owner.
type GlobalService struct {
	index port.GlobalIndex
	notes port.NoteSource
}

// NewGlobalService builds the service. notes may be nil.
func NewGlobalService(index port.GlobalIndex, notes port.NoteSource) *GlobalService {
	return &GlobalService{index: index, notes: notes}
}

var _ port.GlobalService = (*GlobalService)(nil)

// SearchAll validates the query, asks the index and the notes source, and returns the hits ordered by kind in
// display order, each kind in its own ranking.
func (s *GlobalService) SearchAll(ctx context.Context, viewer port.Viewer, query string, kinds []domain.Kind, limit int) ([]domain.GlobalHit, error) {
	normalized, err := domain.NormalizeQuery(query)
	if err != nil {
		return nil, err
	}
	limit = domain.ClampLimit(limit)
	if len(kinds) == 0 {
		kinds = domain.DefaultKinds
	}
	var indexKinds []domain.Kind
	wantNotes := false
	for _, k := range kinds {
		if k == domain.KindNote {
			wantNotes = true
			continue
		}
		indexKinds = append(indexKinds, k)
	}
	var hits []domain.GlobalHit
	if len(indexKinds) > 0 {
		hits, err = s.index.SearchAll(ctx, port.GlobalRequest{Viewer: viewer, Query: normalized, Kinds: indexKinds, Limit: limit})
		if err != nil {
			return nil, err
		}
	}
	if wantNotes && s.notes != nil {
		notes, err := s.notes.SearchNotes(ctx, viewer.UserID, normalized, limit)
		if err != nil {
			return nil, err
		}
		hits = append(hits, notes...)
	}
	return hits, nil
}
