package port

import (
	"context"

	"github.com/google/uuid"
	"github.com/jhonsferg/corelog/internal/board/domain"
)

type Repository interface {
	FindDefaultByProject(ctx context.Context, projectID uuid.UUID) (*domain.Board, error)
}
