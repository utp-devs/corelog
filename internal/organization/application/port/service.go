package port

import (
	"context"

	"github.com/google/uuid"

	"github.com/jhonsferg/corelog/internal/organization/domain"
)

// CreateInput is the input of CreateOrganization. An empty Slug is derived from Name.
type CreateInput struct {
	Name        string
	Slug        string
	Description string
}

// UpdateInput replaces the organization settings.
type UpdateInput struct {
	Settings domain.Settings
}

// Service is the in-port of the organization hexagon. Every method takes the acting user and checks
// the organization role before doing anything else. Users outside the organization get
// domain.ErrNotFound; members without enough rank get domain.ErrForbidden.
type Service interface {
	CreateOrganization(ctx context.Context, actorID uuid.UUID, in CreateInput) (*domain.Organization, error)
	GetOrganization(ctx context.Context, actorID, orgID uuid.UUID) (*domain.Organization, error)
	UpdateSettings(ctx context.Context, actorID, orgID uuid.UUID, in UpdateInput) (*domain.Organization, error)
	ListMine(ctx context.Context, actorID uuid.UUID) ([]Membership, error)
	// DeleteOrganization soft-deletes the organization. Owner only; confirmSlug must equal the slug.
	DeleteOrganization(ctx context.Context, actorID, orgID uuid.UUID, confirmSlug string) error
	// LeaveOrganization removes the actor's membership. The last owner cannot leave.
	LeaveOrganization(ctx context.Context, actorID, orgID uuid.UUID) error
	// TransferOwnership makes newOwnerID (an existing member) the owner and demotes the actor to admin.
	TransferOwnership(ctx context.Context, actorID, orgID, newOwnerID uuid.UUID) error
	// RequireRole returns the caller's role when it has at least min rank. Missing organizations and
	// non-members return domain.ErrNotFound; insufficient rank returns domain.ErrForbidden.
	RequireRole(ctx context.Context, userID, orgID uuid.UUID, min domain.Role) (domain.Role, error)
}
