package port

import (
	"context"

	"github.com/google/uuid"

	"github.com/jhonsferg/corelog/internal/ticket/domain"
)

type Repository interface {
	Save(ctx context.Context, t *domain.Ticket) error
	FindByID(ctx context.Context, id uuid.UUID) (*domain.Ticket, error)
	FindAll(ctx context.Context, filter ListFilter) ([]*domain.Ticket, error)
	Update(ctx context.Context, t *domain.Ticket) error
}

type UserDirectory interface {
	TeamOf(ctx context.Context, userID uuid.UUID) (*uuid.UUID, error)
}

type CollaborationRepository interface {
	SaveComment(ctx context.Context, comment *domain.Comment) error
	FindComments(ctx context.Context, issueID uuid.UUID) ([]*domain.Comment, error)
	UpdateComment(ctx context.Context, comment *domain.Comment) error
	DeleteComment(ctx context.Context, comment *domain.Comment) error
	SaveActivity(ctx context.Context, activity *domain.Activity) error
	FindActivities(ctx context.Context, issueID uuid.UUID) ([]*domain.Activity, error)
	FindMentionIDs(ctx context.Context, issueID uuid.UUID, names []string) ([]uuid.UUID, error)
}
