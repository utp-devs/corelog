package port

import (
	"context"
	"time"

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

	// ListMembers returns the members with their account data. Any member except guests may read.
	ListMembers(ctx context.Context, actorID, orgID uuid.UUID) ([]MemberView, error)
	// ChangeMemberRole sets the role of targetUserID. Owner and admin only. Admins may only switch
	// members and guests between those two roles; only owners grant or revoke owner or admin.
	// Changing one's own role returns domain.ErrSelfRoleChange.
	ChangeMemberRole(ctx context.Context, actorID, orgID, targetUserID uuid.UUID, role domain.Role) error
	// RemoveMember removes targetUserID. Owner and admin only. Admins may remove members and guests;
	// owners may remove anyone except the last owner. Removing an owner as a non-owner returns
	// domain.ErrCannotRemoveOwner.
	RemoveMember(ctx context.Context, actorID, orgID, targetUserID uuid.UUID) error
	// CreateInvitations invites several emails in one transaction. Owner and admin only. Admins may only
	// invite members and guests. Returns one result per input, in order, each with its plain token.
	CreateInvitations(ctx context.Context, actorID, orgID uuid.UUID, in []InviteInput) ([]InvitationResult, error)
	// ListInvitations returns the invitations of the organization. Owner and admin only. With pendingOnly
	// it returns only invitations that are still pending at the current instant.
	ListInvitations(ctx context.Context, actorID, orgID uuid.UUID, pendingOnly bool) ([]*domain.Invitation, error)
	// ResendInvitation replaces the token of a pending invitation and restarts its expiry. The old token
	// stops working immediately. Owner and admin only.
	ResendInvitation(ctx context.Context, actorID, orgID, invitationID uuid.UUID) (*InvitationResult, error)
	// RevokeInvitation revokes a pending invitation. Owner and admin only.
	RevokeInvitation(ctx context.Context, actorID, orgID, invitationID uuid.UUID) error
	// GetInvitationPreview describes the invitation behind a plain token to anyone holding it. Unknown
	// tokens return domain.ErrInvitationNotFound, the same error for every unknown token.
	GetInvitationPreview(ctx context.Context, plainToken string) (*InvitationPreview, error)
	// AcceptInvitation adds the actor to the organization. actorEmail must match the invited email
	// (case-insensitive), otherwise domain.ErrEmailMismatch. Returns the new membership.
	AcceptInvitation(ctx context.Context, actorUserID uuid.UUID, actorEmail, plainToken string) (*domain.Member, error)
}

// InviteInput is one invitee of CreateInvitations.
type InviteInput struct {
	Email string
	Role  domain.Role
}

// InvitationResult pairs a stored invitation with its plain token. The plain token is returned once and
// never stored; only its hash is.
type InvitationResult struct {
	Invitation *domain.Invitation
	PlainToken string
}

// InvitationPreview is the public description of an invitation, shown before accepting.
type InvitationPreview struct {
	OrganizationID   uuid.UUID
	OrganizationName string
	Email            string
	Role             domain.Role
	Status           domain.InvitationStatus
	ExpiresAt        time.Time
}
