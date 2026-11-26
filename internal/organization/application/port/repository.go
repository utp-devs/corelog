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
	// GetByIDForUpdate is GetByID that also locks the row (SELECT ... FOR UPDATE) until the transaction ends.
	// It must run inside a transaction.
	GetByIDForUpdate(ctx context.Context, id uuid.UUID) (*domain.Organization, error)
	// LockOwnerCreation takes a transaction-scoped advisory lock for the user, so that concurrent organization
	// creations by the same user are serialised and the owned-organization limit cannot be exceeded. It must
	// run inside a transaction; the lock is released at commit or rollback.
	LockOwnerCreation(ctx context.Context, userID uuid.UUID) error
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
	// CountOwnedBy returns how many non-deleted organizations the user owns (owner role). It enforces
	// the per-user organization limit (domain.MaxOwnedOrganizations).
	CountOwnedBy(ctx context.Context, userID uuid.UUID) (int, error)
	// ListMemberViews returns the members of a non-deleted organization joined with their user's name
	// and email, oldest first. It is the read model for the members screen.
	ListMemberViews(ctx context.Context, orgID uuid.UUID) ([]MemberView, error)
	// GetMemberProfile returns the profile of one member of a non-deleted organization. A user who is not
	// a member returns domain.ErrMemberNotFound.
	GetMemberProfile(ctx context.Context, orgID, userID uuid.UUID) (*MemberProfile, error)
	// FindMemberByEmail reports whether a member of the organization has this email (case-insensitive).
	FindMemberByEmail(ctx context.Context, orgID uuid.UUID, email string) (bool, error)

	// CreateInvitation inserts the invitation and assigns its ID. A live (not accepted, not revoked)
	// invitation for the same organization and email returns domain.ErrInvitationExists, expired or not.
	CreateInvitation(ctx context.Context, inv *domain.Invitation) error
	// GetInvitationByID returns the invitation only when it belongs to orgID; otherwise
	// domain.ErrInvitationNotFound.
	GetInvitationByID(ctx context.Context, orgID, id uuid.UUID) (*domain.Invitation, error)
	// GetInvitationByTokenHash returns the invitation with this token hash, or domain.ErrInvitationNotFound.
	GetInvitationByTokenHash(ctx context.Context, tokenHash string) (*domain.Invitation, error)
	// ListInvitations returns the invitations of an organization, newest first. With pendingOnly it
	// excludes accepted and revoked ones; expiry is left to the caller, which knows the clock.
	ListInvitations(ctx context.Context, orgID uuid.UUID, pendingOnly bool) ([]*domain.Invitation, error)
	// UpdateInvitation persists token_hash, expires_at, accepted_at and revoked_at of an invitation.
	UpdateInvitation(ctx context.Context, inv *domain.Invitation) error
}

// MemberView is a membership with the member's account data, as shown to other members. HasAvatar reports
// whether the account has an avatar; the storage key never leaves the adapter.
type MemberView struct {
	UserID      uuid.UUID
	Name        string
	DisplayName string
	Email       string
	JobTitle    string
	Department  string
	HasAvatar   bool
	Role        domain.Role
	JoinedAt    time.Time
}

// MemberProfile is the member view plus the account timezone, shown on the person screen.
type MemberProfile struct {
	MemberView
	Timezone string
}
