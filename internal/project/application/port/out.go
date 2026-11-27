package port

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/jhonsferg/corelog/internal/project/domain"
)

// VisibilityScope selects the projects of an organization that a viewer may list.
// All: every project (org owner/admin, platform admin). Open: also the open projects.
// Projects where UserID is an explicit project member, or a member of a team granted on the
// project, are always included.
type VisibilityScope struct {
	UserID uuid.UUID
	All    bool
	Open   bool
}

// ArchiveFilter selects archived, active or all projects in a listing.
type ArchiveFilter string

const (
	ArchiveFilterExclude ArchiveFilter = "exclude"
	ArchiveFilterInclude ArchiveFilter = "include"
	ArchiveFilterOnly    ArchiveFilter = "only"
)

// SortField is a whitelisted sort column of the project listing.
type SortField string

const (
	SortByName      SortField = "name"
	SortByKey       SortField = "key"
	SortByCreatedAt SortField = "created_at"
	SortByUpdatedAt SortField = "updated_at"
)

// ProjectFilter is the query of a project listing. The use case fills OrganizationID and Scope from
// the caller; the rest comes from the request.
type ProjectFilter struct {
	OrganizationID uuid.UUID
	Scope          VisibilityScope
	// Q matches the name or the key, case insensitive.
	Q        string
	Template domain.Template
	// LeadID filters by lead when not nil.
	LeadID *uuid.UUID
	// Starred keeps only the projects the viewer starred.
	Starred  bool
	Archived ArchiveFilter
	Sort     SortField
	Desc     bool
	Page     int
	PageSize int
}

// ProjectPage is one page of a listing with the total number of matches.
type ProjectPage struct {
	Items []*domain.Project
	Total int
}

type Repository interface {
	Save(ctx context.Context, project *domain.Project) error
	FindByID(ctx context.Context, id uuid.UUID) (*domain.Project, error)
	FindByKey(ctx context.Context, orgID uuid.UUID, key string) (*domain.Project, error)
	FindByOrganization(ctx context.Context, orgID uuid.UUID, scope VisibilityScope) ([]*domain.Project, error)
	// ListPage returns one page of the filtered listing and the total count.
	ListPage(ctx context.Context, filter ProjectFilter) (ProjectPage, error)
	Update(ctx context.Context, project *domain.Project) error
	// Delete soft deletes the project: it sets deleted_at and every read stops seeing it.
	Delete(ctx context.Context, orgID, id uuid.UUID) error
	// KeyAvailable reports whether no live (not deleted) project of the organization uses key.
	KeyAvailable(ctx context.Context, orgID uuid.UUID, key string) (bool, error)
	// AllocateIssueNumber atomically takes the next issue number of the project and returns it.
	AllocateIssueNumber(ctx context.Context, projectID uuid.UUID) (int, error)
}

// ProjectCreationRepository persists a project and its required initial relations atomically.
type ProjectCreationRepository interface {
	CreateWithDefaults(ctx context.Context, project *domain.Project, lead Member) error
	// CreateWithMembers is CreateWithDefaults plus initial members and team grants in the same
	// transaction. Teams must belong to the project organization.
	CreateWithMembers(ctx context.Context, project *domain.Project, lead Member, members []Member, teamIDs []uuid.UUID) error
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

// Acciones de proyecto que el modulo delega al autorizador. Usan los nombres de la matriz de permisos.
const (
	ActionViewProject   = "view_project"
	ActionEditProject   = "edit_project"
	ActionManageMembers = "manage_members"
	ActionEditIssue     = "edit_issue"
)

// ProjectAuthorizer decide si un actor puede ejecutar una accion sobre un proyecto.
type ProjectAuthorizer interface {
	Authorize(ctx context.Context, action string, actorID uuid.UUID, globalRole string, projectID uuid.UUID) error
}

// EventPublisher records domain events. It is the outbox writer from the platform events package,
// declared here as a narrow port.
type EventPublisher interface {
	PublishProjectEvent(ctx context.Context, event ProjectEvent) error
}

// ProjectEvent is a project lifecycle event. Consumers read it through the outbox.
type ProjectEvent struct {
	OrganizationID uuid.UUID
	ProjectID      uuid.UUID
	ActorID        uuid.UUID
	Type           string
	Payload        map[string]any
}
