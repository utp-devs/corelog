package port

import (
	"context"

	"github.com/google/uuid"

	"github.com/jhonsferg/corelog/internal/search/domain"
)

// GlobalRequest is a global search: the normalized query, the kinds to search and the limit per kind.
type GlobalRequest struct {
	Viewer Viewer
	Query  string
	Kinds  []domain.Kind
	Limit  int
}

// GlobalIndex searches projects, issues, people, teams and filters visible to the viewer. Each kind is
// ranked by itself and capped at Limit. Hits come back with the title highlight and, for issues, a snippet.
type GlobalIndex interface {
	SearchAll(ctx context.Context, req GlobalRequest) ([]domain.GlobalHit, error)
}

// NoteSource is the narrow port to the notes of the viewer (private to their owner).
type NoteSource interface {
	SearchNotes(ctx context.Context, userID uuid.UUID, query string, limit int) ([]domain.GlobalHit, error)
}

// GlobalService is the global search use case.
type GlobalService interface {
	SearchAll(ctx context.Context, viewer Viewer, query string, kinds []domain.Kind, limit int) ([]domain.GlobalHit, error)
}
