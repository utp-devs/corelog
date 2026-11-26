package port

import (
	"context"

	"github.com/google/uuid"

	"github.com/jhonsferg/corelog/internal/project/domain"
)

type Service interface {
	Create(ctx context.Context, actorID, orgID uuid.UUID, key, name, description string, projectType domain.ProjectType, visibility domain.Visibility) (*domain.Project, error)
	Get(ctx context.Context, actorID uuid.UUID, globalRole string, id uuid.UUID) (*domain.Project, error)
	List(ctx context.Context, actorID uuid.UUID, globalRole string, orgID uuid.UUID) ([]*domain.Project, error)
	Update(ctx context.Context, actorID uuid.UUID, globalRole string, id uuid.UUID, input domain.UpdateInput) (*domain.Project, error)
	Delete(ctx context.Context, actorID uuid.UUID, globalRole string, id uuid.UUID) error
	ListMembers(ctx context.Context, actorID uuid.UUID, globalRole string, projectID uuid.UUID) ([]Member, error)
	AddMember(ctx context.Context, actorID uuid.UUID, globalRole string, projectID, userID uuid.UUID, role string) (*Member, error)
	UpdateMemberRole(ctx context.Context, actorID uuid.UUID, globalRole string, projectID, userID uuid.UUID, role string) (*Member, error)
	RemoveMember(ctx context.Context, actorID uuid.UUID, globalRole string, projectID, userID uuid.UUID) error
	ListLabels(ctx context.Context, actorID uuid.UUID, globalRole string, projectID uuid.UUID) ([]domain.Label, error)
	CreateLabel(ctx context.Context, actorID uuid.UUID, globalRole string, projectID uuid.UUID, name, color string) (*domain.Label, error)
	DeleteLabel(ctx context.Context, actorID uuid.UUID, globalRole string, projectID, labelID uuid.UUID) error
	ListComponents(ctx context.Context, actorID uuid.UUID, globalRole string, projectID uuid.UUID) ([]domain.Component, error)
	CreateComponent(ctx context.Context, actorID uuid.UUID, globalRole string, projectID uuid.UUID, name, description string) (*domain.Component, error)
	DeleteComponent(ctx context.Context, actorID uuid.UUID, globalRole string, projectID, componentID uuid.UUID) error
}
