package port

import (
	"context"

	"github.com/google/uuid"

	"github.com/jhonsferg/corelog/internal/team/domain"
)

// Repository persists teams and memberships. Every organization-owned query is
// filtered by organization_id. Member queries join through teams so that a team
// id from another organization never matches.
type Repository interface {
	Save(ctx context.Context, t *domain.Team) error
	FindByID(ctx context.Context, id uuid.UUID) (*domain.Team, error)
	FindByName(ctx context.Context, name string) (*domain.Team, error)
	FindByOrganization(ctx context.Context, organizationID uuid.UUID) ([]*domain.Team, error)
	// FindByOrganizationAndMember returns the teams of the organization that userID belongs to.
	FindByOrganizationAndMember(ctx context.Context, organizationID, userID uuid.UUID) ([]*domain.Team, error)
	Update(ctx context.Context, t *domain.Team) error
	Delete(ctx context.Context, organizationID, id uuid.UUID) error

	FindMember(ctx context.Context, organizationID, teamID, userID uuid.UUID) (*domain.Member, error)
	ListMembers(ctx context.Context, organizationID, teamID uuid.UUID) ([]*domain.Member, error)
	AddMember(ctx context.Context, organizationID uuid.UUID, m *domain.Member) error
	UpdateMemberRole(ctx context.Context, organizationID, teamID, userID uuid.UUID, role string) error
	RemoveMember(ctx context.Context, organizationID, teamID, userID uuid.UUID) error
}
