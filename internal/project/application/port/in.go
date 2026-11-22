package port

import (
	"context"

	"github.com/google/uuid"

	"github.com/jhonsferg/corelog/internal/project/domain"
)

type Service interface {
	Create(ctx context.Context, key, name, description string, projectType domain.ProjectType, leadID uuid.UUID, visibility domain.Visibility) (*domain.Project, error)
	Get(ctx context.Context, actorID uuid.UUID, globalRole string, id uuid.UUID) (*domain.Project, error)
	List(ctx context.Context, actorID uuid.UUID, globalRole string) ([]*domain.Project, error)
	Update(ctx context.Context, actorID uuid.UUID, globalRole string, id uuid.UUID, input domain.UpdateInput) (*domain.Project, error)
	Delete(ctx context.Context, actorID uuid.UUID, globalRole string, id uuid.UUID) error
	ListMembers(ctx context.Context, actorID uuid.UUID, globalRole string, projectID uuid.UUID) ([]Member, error)
	AddMember(ctx context.Context, actorID uuid.UUID, globalRole string, projectID, userID uuid.UUID, role string) (*Member, error)
	UpdateMemberRole(ctx context.Context, actorID uuid.UUID, globalRole string, projectID, userID uuid.UUID, role string) (*Member, error)
	RemoveMember(ctx context.Context, actorID uuid.UUID, globalRole string, projectID, userID uuid.UUID) error
}
