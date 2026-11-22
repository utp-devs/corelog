package port

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/jhonsferg/corelog/internal/project/domain"
)

type Repository interface {
	Save(ctx context.Context, project *domain.Project) error
	FindByID(ctx context.Context, id uuid.UUID) (*domain.Project, error)
	FindByKey(ctx context.Context, key string) (*domain.Project, error)
	FindAll(ctx context.Context) ([]*domain.Project, error)
	Update(ctx context.Context, project *domain.Project) error
	Delete(ctx context.Context, id uuid.UUID) error
}

// ProjectCreationRepository persists a project and its required initial relations atomically.
type ProjectCreationRepository interface {
	CreateWithDefaults(ctx context.Context, project *domain.Project, lead Member) error
}

type Member struct {
	ProjectID uuid.UUID
	UserID    uuid.UUID
	Role      string
	CreatedAt time.Time
}

type MemberRepository interface {
	FindRole(ctx context.Context, projectID, userID uuid.UUID) (*string, error)
	ListProjectIDs(ctx context.Context, userID uuid.UUID) ([]uuid.UUID, error)
	List(ctx context.Context, projectID uuid.UUID) ([]Member, error)
	Add(ctx context.Context, member Member) error
	UpdateRole(ctx context.Context, projectID, userID uuid.UUID, role string) error
	Remove(ctx context.Context, projectID, userID uuid.UUID) error
}

type CatalogRepository interface {
	ListLabels(ctx context.Context, projectID uuid.UUID) ([]domain.Label, error)
	CreateLabel(ctx context.Context, label *domain.Label) error
	DeleteLabel(ctx context.Context, projectID, labelID uuid.UUID) error
	ListComponents(ctx context.Context, projectID uuid.UUID) ([]domain.Component, error)
	CreateComponent(ctx context.Context, component *domain.Component) error
	DeleteComponent(ctx context.Context, projectID, componentID uuid.UUID) error
}
