package port

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/jhonsferg/corelog/internal/organization/domain"
)

// Membership is an organization together with the calling user's role in it.
type Membership struct {
	Organization *domain.Organization
	Role         domain.Role
}

// Repository persists organizations and memberships. Implementations return domain.ErrNotFound and
// domain.ErrMemberNotFound (or an error whose apperror Kind is NotFound) when a row does not exist.
type Repository interface {
	// Create inserts the organization and assigns its ID. A slug clash returns domain.ErrSlugTaken.
	Create(ctx context.Context, org *domain.Organization) error
	// GetByID returns a non-deleted organization. Deleted or missing rows return domain.ErrNotFound.
	GetByID(ctx context.Context, id uuid.UUID) (*domain.Organization, error)
	// GetBySlug returns the organization with this slug, including soft-deleted ones (the slug stays reserved).
	GetBySlug(ctx context.Context, slug string) (*domain.Organization, error)
	// ListByUser returns the non-deleted organizations the user belongs to, with the user's role.
	ListByUser(ctx context.Context, userID uuid.UUID) ([]Membership, error)
	// Update persists the mutable fields (name, description, settings, updated_at).
	Update(ctx context.Context, org *domain.Organization) error
	// SoftDelete sets deleted_at on the organization.
	SoftDelete(ctx context.Context, id uuid.UUID, at time.Time) error
	ListMembers(ctx context.Context, orgID uuid.UUID) ([]*domain.Member, error)
	// GetMember returns domain.ErrMemberNotFound when the user is not a member.
	GetMember(ctx context.Context, orgID, userID uuid.UUID) (*domain.Member, error)
	// AddMember inserts a membership. A duplicate returns domain.ErrMemberExists.
	AddMember(ctx context.Context, m *domain.Member) error
	UpdateMemberRole(ctx context.Context, orgID, userID uuid.UUID, role domain.Role) error
	RemoveMember(ctx context.Context, orgID, userID uuid.UUID) error
	// CountOwners returns how many members of the organization have the owner role.
	CountOwners(ctx context.Context, orgID uuid.UUID) (int, error)
}
