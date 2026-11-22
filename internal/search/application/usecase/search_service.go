package usecase

import (
	"context"

	"github.com/google/uuid"

	"github.com/jhonsferg/corelog/internal/search/application/port"
	"github.com/jhonsferg/corelog/internal/search/domain"
)

type SearchService struct {
	index port.Index
}

func NewSearchService(index port.Index) *SearchService {
	return &SearchService{index: index}
}

var _ port.Service = (*SearchService)(nil)

// Search valida la consulta y delega en el indice. El administrador global es el unico que ve todos los proyectos.
func (s *SearchService) Search(ctx context.Context, actorID uuid.UUID, globalRole string, query string, limit int) ([]domain.Hit, error) {
	normalized, err := domain.NormalizeQuery(query)
	if err != nil {
		return nil, err
	}
	return s.index.Search(ctx, actorID, globalRole == "admin", normalized, domain.ClampLimit(limit))
}
