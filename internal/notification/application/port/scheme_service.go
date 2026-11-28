package port

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/jhonsferg/corelog/internal/notification/domain"
)

// ProjectAuthorizer decides project actions; it answers not found for hidden projects and forbidden otherwise.
type ProjectAuthorizer interface {
	Authorize(ctx context.Context, action string, actorID uuid.UUID, globalRole string, projectID uuid.UUID) error
}

// OrgRoles checks the role of a user in an organization (owner, admin, member, guest).
type OrgRoles interface {
	RequireRole(ctx context.Context, userID, orgID uuid.UUID, min string) error
}

// SchemeView is a scheme as the API shows it: the rules in force and which events the scope overrides.
type SchemeView struct {
	OrganizationID uuid.UUID
	ProjectID      *uuid.UUID
	Scheme         domain.Scheme
	Overridden     map[domain.Event]bool
	UpdatedAt      *time.Time
}

// SchemeService manages the notification schemes.
type SchemeService interface {
	GetProject(ctx context.Context, actorID uuid.UUID, globalRole string, projectID uuid.UUID) (SchemeView, error)
	PutProject(ctx context.Context, actorID uuid.UUID, globalRole string, projectID uuid.UUID, scheme domain.Scheme) (SchemeView, error)
	ResetProject(ctx context.Context, actorID uuid.UUID, globalRole string, projectID uuid.UUID) (SchemeView, error)
	GetOrganization(ctx context.Context, actorID, orgID uuid.UUID) (SchemeView, error)
	PutOrganization(ctx context.Context, actorID, orgID uuid.UUID, scheme domain.Scheme) (SchemeView, error)
	ResetOrganization(ctx context.Context, actorID, orgID uuid.UUID) (SchemeView, error)
}

// OutboxAdmin is the platform administration of the email queue.
type OutboxAdmin interface {
	List(ctx context.Context, status string, limit, offset int) ([]OutboxEntry, int, error)
	Retry(ctx context.Context, id uuid.UUID) error
}
