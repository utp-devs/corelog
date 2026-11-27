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
	// Delete is the legacy entry point: it soft deletes without the typed confirmation. New callers
	// use ProjectManager.DeleteProject.
	Delete(ctx context.Context, actorID uuid.UUID, globalRole string, id uuid.UUID) error
	// ListMembers returns the direct members and the users reached through team grants.
	ListMembers(ctx context.Context, actorID uuid.UUID, globalRole string, projectID uuid.UUID) ([]MemberEntry, error)
	AddMember(ctx context.Context, actorID uuid.UUID, globalRole string, projectID, userID uuid.UUID, role string) (*Member, error)
	UpdateMemberRole(ctx context.Context, actorID uuid.UUID, globalRole string, projectID, userID uuid.UUID, role string) (*Member, error)
	RemoveMember(ctx context.Context, actorID uuid.UUID, globalRole string, projectID, userID uuid.UUID) error
	// ListTeamGrants returns the team roles of the project. Requires manage_members.
	ListTeamGrants(ctx context.Context, actorID uuid.UUID, globalRole string, projectID uuid.UUID) ([]TeamGrant, error)
	// SetTeamGrant creates or replaces the role of a team on the project. The team must belong to the
	// project organization.
	SetTeamGrant(ctx context.Context, actorID uuid.UUID, globalRole string, projectID, teamID uuid.UUID, role string) (*TeamGrant, error)
	// RemoveTeamGrant deletes the team role of the project.
	RemoveTeamGrant(ctx context.Context, actorID uuid.UUID, globalRole string, projectID, teamID uuid.UUID) error
	ListLabels(ctx context.Context, actorID uuid.UUID, globalRole string, projectID uuid.UUID) ([]domain.Label, error)
	CreateLabel(ctx context.Context, actorID uuid.UUID, globalRole string, projectID uuid.UUID, name, color string) (*domain.Label, error)
	DeleteLabel(ctx context.Context, actorID uuid.UUID, globalRole string, projectID, labelID uuid.UUID) error
	// UpdateLabel renames or recolors a label. Nil arguments keep the current value.
	UpdateLabel(ctx context.Context, actorID uuid.UUID, globalRole string, projectID, labelID uuid.UUID, name, color *string) (*domain.Label, error)
	ListComponents(ctx context.Context, actorID uuid.UUID, globalRole string, projectID uuid.UUID) ([]domain.Component, error)
	CreateComponent(ctx context.Context, actorID uuid.UUID, globalRole string, projectID uuid.UUID, in domain.ComponentInput) (*domain.Component, error)
	// UpdateComponent changes the name, description, lead or default assignee policy of a component.
	UpdateComponent(ctx context.Context, actorID uuid.UUID, globalRole string, projectID, componentID uuid.UUID, patch domain.ComponentPatch) (*domain.Component, error)
	// DeleteComponent removes a component. moveTo, when set, takes over the issues of the component.
	DeleteComponent(ctx context.Context, actorID uuid.UUID, globalRole string, projectID, componentID uuid.UUID, moveTo *uuid.UUID) error
}

// CreateInput is the input of a project created from a template.
type CreateInput struct {
	Key                   string
	Name                  string
	Description           string
	Template              domain.Template
	Visibility            domain.Visibility
	Color                 string
	Icon                  string
	DefaultAssigneePolicy domain.AssigneePolicy
	// MemberIDs are added with the contributor role; the creator is always lead.
	MemberIDs []uuid.UUID
	// TeamIDs are granted the contributor role on the project.
	TeamIDs []uuid.UUID
}

// ProjectManager holds the project model v2 operations. The HTTP layer calls them in a later step.
type ProjectManager interface {
	// CreateProject creates a project from a template. Key conflicts return project.key_taken.
	CreateProject(ctx context.Context, actorID, orgID uuid.UUID, input CreateInput) (*domain.Project, error)
	// ListProjects returns one page of the projects visible to the actor.
	ListProjects(ctx context.Context, actorID uuid.UUID, globalRole string, orgID uuid.UUID, filter ProjectFilter) (ProjectPage, error)
	// Archive makes the project read-only.
	Archive(ctx context.Context, actorID uuid.UUID, globalRole string, id uuid.UUID) (*domain.Project, error)
	// Restore reverts Archive.
	Restore(ctx context.Context, actorID uuid.UUID, globalRole string, id uuid.UUID) (*domain.Project, error)
	// DeleteProject soft deletes the project. confirmation must equal the project name.
	DeleteProject(ctx context.Context, actorID uuid.UUID, globalRole string, id uuid.UUID, confirmation string) error
	// FindByKey resolves a project key for the actor among the projects the actor can see. orgID is
	// optional (uuid.Nil): when a key exists in several organizations the actor belongs to and orgID is
	// not given, the call returns domain.ErrProjectKeyAmbiguous.
	FindByKey(ctx context.Context, actorID uuid.UUID, globalRole string, orgID uuid.UUID, key string) (*domain.Project, error)
	// KeyAvailable reports whether a project key is valid and free in the organization.
	KeyAvailable(ctx context.Context, actorID uuid.UUID, globalRole string, orgID uuid.UUID, key string) (bool, error)
}
