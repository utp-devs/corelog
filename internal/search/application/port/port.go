package port

import (
	"context"

	"github.com/google/uuid"

	"github.com/jhonsferg/corelog/internal/search/domain"
)

// Index busca proyectos e incidencias visibles para el actor. Un administrador global ve todo;
// el resto solo los proyectos en los que es miembro (ADR-009: LIKE detras de este puerto).
type Index interface {
	Search(ctx context.Context, actorID uuid.UUID, admin bool, query string, limit int) ([]domain.Hit, error)
}

// Service es el caso de uso de busqueda global.
type Service interface {
	Search(ctx context.Context, actorID uuid.UUID, globalRole string, query string, limit int) ([]domain.Hit, error)
}
