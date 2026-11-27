package usecase

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"

	"github.com/jhonsferg/corelog/internal/organization/application/port"
	"github.com/jhonsferg/corelog/internal/organization/domain"
	"github.com/jhonsferg/corelog/internal/platform/apperror"
)

// Event types for invitations. Member events are declared in organization_service.go.
const (
	EventInvitationCreated  = "invitation.created"
	EventInvitationResent   = "invitation.resent"
	EventInvitationRevoked  = "invitation.revoked"
	EventInvitationAccepted = "invitation.accepted"
)

// maxInvitationBatch limits how many emails one CreateInvitations call may invite.
const maxInvitationBatch = 50

// ListMembers returns the members with their account data. Any member may read; guests may not.
func (s *OrganizationService) ListMembers(ctx context.Context, actorID, orgID uuid.UUID) ([]port.MemberView, error) {
	if _, err := s.RequireRole(ctx, actorID, orgID, domain.RoleMember); err != nil {
		return nil, err
	}
	return s.repo.ListMemberViews(ctx, orgID)
}

// GetMemberProfile returns the profile of a member. Guests may read only their own profile.
func (s *OrganizationService) GetMemberProfile(ctx context.Context, actorID, orgID, targetID uuid.UUID) (*port.MemberProfile, error) {
	actorRole, err := s.RequireRole(ctx, actorID, orgID, domain.RoleGuest)
	if err != nil {
		return nil, err
	}
	if actorRole == domain.RoleGuest && actorID != targetID {
		return nil, domain.ErrForbidden
	}
	return s.repo.GetMemberProfile(ctx, orgID, targetID)
}

// ChangeMemberRole changes the role of another member. See port.Service for the rules.
func (s *OrganizationService) ChangeMemberRole(ctx context.Context, actorID, orgID, targetID uuid.UUID, role domain.Role) error {
	actorRole, err := s.RequireRole(ctx, actorID, orgID, domain.RoleAdmin)
	if err != nil {
		return err
	}
	if actorID == targetID {
		return domain.ErrSelfRoleChange
	}
	if !role.IsValid() {
		return domain.ErrRoleInvalid
	}
	target, err := s.repo.GetMember(ctx, orgID, targetID)
	if err != nil {
		return err
	}
	if err := domain.CanChangeRole(actorRole, target.Role, role); err != nil {
		return err
	}
	if target.Role == role {
		return nil
	}
	if target.Role == domain.RoleOwner {
		if err := s.checkLastOwnerAfterRemoval(ctx, orgID); err != nil {
			return err
		}
	}
	oldRole := target.Role
	return s.tx.InTx(ctx, func(ctx context.Context) error {
		if err := s.repo.UpdateMemberRole(ctx, orgID, targetID, role); err != nil {
			return err
		}
		return s.publish(ctx, orgID, actorID, EventMemberRoleChanged, "organization_member", targetID,
			map[string]any{"old_role": string(oldRole), "new_role": string(role)})
	})
}

// RemoveMember removes another member. See port.Service for the rules.
func (s *OrganizationService) RemoveMember(ctx context.Context, actorID, orgID, targetID uuid.UUID) error {
	actorRole, err := s.RequireRole(ctx, actorID, orgID, domain.RoleAdmin)
	if err != nil {
		return err
	}
	target, err := s.repo.GetMember(ctx, orgID, targetID)
	if err != nil {
		return err
	}
	if target.Role == domain.RoleOwner && actorRole != domain.RoleOwner {
		return domain.ErrCannotRemoveOwner
	}
	if err := domain.CanChangeRole(actorRole, target.Role, target.Role); err != nil {
		return err
	}
	if target.Role == domain.RoleOwner {
		if err := s.checkLastOwnerAfterRemoval(ctx, orgID); err != nil {
			return err
		}
	}
	return s.tx.InTx(ctx, func(ctx context.Context) error {
		if err := s.repo.RemoveMember(ctx, orgID, targetID); err != nil {
			return err
		}
		return s.publish(ctx, orgID, actorID, EventMemberRemoved, "organization_member", targetID,
			map[string]any{"role": string(target.Role), "reason": "removed"})
	})
}

// checkLastOwnerAfterRemoval fails when one owner leaving would leave the organization without owners.
// Callers invoke it only when the affected member is an owner.
func (s *OrganizationService) checkLastOwnerAfterRemoval(ctx context.Context, orgID uuid.UUID) error {
	owners, err := s.repo.CountOwners(ctx, orgID)
	if err != nil {
		return err
	}
	return domain.ValidateLastOwner(owners - 1)
}

// canInvite decides which roles an actor may hand out. Owners may invite admin, member and guest;
// admins may invite member and guest only.
func canInvite(actor, role domain.Role) error {
	if role == domain.RoleOwner || !role.IsValid() {
		return domain.ErrRoleInvalid
	}
	if actor == domain.RoleAdmin && role == domain.RoleAdmin {
		return domain.ErrForbidden
	}
	return nil
}

// CreateInvitations invites several emails atomically. Expired leftovers for an invited email are
// retired first, so they do not block the new invitation.
func (s *OrganizationService) CreateInvitations(ctx context.Context, actorID, orgID uuid.UUID, in []port.InviteInput) ([]port.InvitationResult, error) {
	actorRole, err := s.RequireRole(ctx, actorID, orgID, domain.RoleAdmin)
	if err != nil {
		return nil, err
	}
	if len(in) == 0 || len(in) > maxInvitationBatch {
		return nil, domain.ErrInvitationBatchInvalid
	}
	now := s.clock.Now()

	type prepared struct {
		inv   *domain.Invitation
		plain string
	}
	preps := make([]prepared, 0, len(in))
	seen := make(map[string]struct{}, len(in))
	for _, item := range in {
		if err := canInvite(actorRole, item.Role); err != nil {
			return nil, err
		}
		plain, hash, err := domain.NewToken()
		if err != nil {
			return nil, err
		}
		inv, err := domain.NewInvitation(orgID, item.Email, item.Role, actorID, hash, now)
		if err != nil {
			return nil, err
		}
		if _, dup := seen[inv.Email]; dup {
			return nil, domain.ErrInvitationExists
		}
		seen[inv.Email] = struct{}{}
		isMember, err := s.repo.FindMemberByEmail(ctx, orgID, inv.Email)
		if err != nil {
			return nil, err
		}
		if isMember {
			return nil, domain.ErrAlreadyMember
		}
		preps = append(preps, prepared{inv: inv, plain: plain})
	}

	results := make([]port.InvitationResult, 0, len(preps))
	err = s.tx.InTx(ctx, func(ctx context.Context) error {
		open, err := s.repo.ListInvitations(ctx, orgID, true)
		if err != nil {
			return err
		}
		byEmail := make(map[string]*domain.Invitation, len(open))
		for _, o := range open {
			byEmail[o.Email] = o
		}
		for _, p := range preps {
			if old, ok := byEmail[p.inv.Email]; ok {
				if err := old.Retire(now); err != nil {
					return err
				}
				if old.RevokedAt != nil {
					if err := s.repo.UpdateInvitation(ctx, old); err != nil {
						return err
					}
					if err := s.publish(ctx, orgID, actorID, EventInvitationRevoked, "invitation", old.ID,
						map[string]any{"email": old.Email, "reason": "expired"}); err != nil {
						return err
					}
				}
			}
			if err := s.repo.CreateInvitation(ctx, p.inv); err != nil {
				return err
			}
			if err := s.publish(ctx, orgID, actorID, EventInvitationCreated, "invitation", p.inv.ID,
				map[string]any{"email": p.inv.Email, "role": string(p.inv.Role)}); err != nil {
				return err
			}
			results = append(results, port.InvitationResult{Invitation: p.inv, PlainToken: p.plain})
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return results, nil
}

// ListInvitations returns the invitations of the organization. With pendingOnly, expired ones are
// filtered out as well, using the service clock.
func (s *OrganizationService) ListInvitations(ctx context.Context, actorID, orgID uuid.UUID, pendingOnly bool) ([]*domain.Invitation, error) {
	if _, err := s.RequireRole(ctx, actorID, orgID, domain.RoleAdmin); err != nil {
		return nil, err
	}
	list, err := s.repo.ListInvitations(ctx, orgID, pendingOnly)
	if err != nil {
		return nil, err
	}
	if !pendingOnly {
		return list, nil
	}
	now := s.clock.Now()
	out := make([]*domain.Invitation, 0, len(list))
	for _, inv := range list {
		if inv.Status(now) == domain.InvitationPending {
			out = append(out, inv)
		}
	}
	return out, nil
}

// ResendInvitation issues a new token for a pending invitation; the old token stops working.
func (s *OrganizationService) ResendInvitation(ctx context.Context, actorID, orgID, invitationID uuid.UUID) (*port.InvitationResult, error) {
	if _, err := s.RequireRole(ctx, actorID, orgID, domain.RoleAdmin); err != nil {
		return nil, err
	}
	inv, err := s.repo.GetInvitationByID(ctx, orgID, invitationID)
	if err != nil {
		return nil, err
	}
	plain, hash, err := domain.NewToken()
	if err != nil {
		return nil, err
	}
	if err := inv.Renew(hash, s.clock.Now()); err != nil {
		return nil, err
	}
	err = s.tx.InTx(ctx, func(ctx context.Context) error {
		if err := s.repo.UpdateInvitation(ctx, inv); err != nil {
			return err
		}
		return s.publish(ctx, orgID, actorID, EventInvitationResent, "invitation", inv.ID,
			map[string]any{"email": inv.Email})
	})
	if err != nil {
		return nil, err
	}
	return &port.InvitationResult{Invitation: inv, PlainToken: plain}, nil
}

// RevokeInvitation revokes a pending invitation.
func (s *OrganizationService) RevokeInvitation(ctx context.Context, actorID, orgID, invitationID uuid.UUID) error {
	if _, err := s.RequireRole(ctx, actorID, orgID, domain.RoleAdmin); err != nil {
		return err
	}
	inv, err := s.repo.GetInvitationByID(ctx, orgID, invitationID)
	if err != nil {
		return err
	}
	if err := inv.Revoke(s.clock.Now()); err != nil {
		return err
	}
	return s.tx.InTx(ctx, func(ctx context.Context) error {
		if err := s.repo.UpdateInvitation(ctx, inv); err != nil {
			return err
		}
		return s.publish(ctx, orgID, actorID, EventInvitationRevoked, "invitation", inv.ID,
			map[string]any{"email": inv.Email, "reason": "revoked"})
	})
}

// GetInvitationPreview describes an invitation to whoever holds its token. Every unknown token gets the
// same domain.ErrInvitationNotFound, so the response never reveals whether an email has an account.
func (s *OrganizationService) GetInvitationPreview(ctx context.Context, plainToken string) (*port.InvitationPreview, error) {
	inv, err := s.invitationByToken(ctx, plainToken)
	if err != nil {
		return nil, err
	}
	org, err := s.repo.GetByID(ctx, inv.OrganizationID)
	if err != nil {
		return nil, notFoundAs(err, domain.ErrInvitationNotFound)
	}
	invitedBy, err := s.inviterName(ctx, inv.InvitedBy)
	if err != nil {
		return nil, err
	}
	return &port.InvitationPreview{
		OrganizationID:   org.ID,
		OrganizationName: org.Name,
		HasLogo:          org.HasLogo,
		InvitedBy:        invitedBy,
		Email:            inv.Email,
		Role:             inv.Role,
		Status:           inv.Status(s.clock.Now()),
		ExpiresAt:        inv.ExpiresAt,
	}, nil
}

// inviterName returns the display name of the member who created an invitation, or nil when the id is
// unset or the account is gone.
func (s *OrganizationService) inviterName(ctx context.Context, invitedBy uuid.UUID) (*string, error) {
	if invitedBy == uuid.Nil {
		return nil, nil
	}
	name, err := s.repo.UserDisplayName(ctx, invitedBy)
	if err != nil {
		return nil, fmt.Errorf("organization: inviter name: %w", err)
	}
	if name == "" {
		return nil, nil
	}
	return &name, nil
}

// AcceptInvitation adds the actor as a member with the invited role and marks the invitation accepted,
// in one transaction.
func (s *OrganizationService) AcceptInvitation(ctx context.Context, actorID uuid.UUID, actorEmail, plainToken string) (*domain.Member, error) {
	inv, err := s.invitationByToken(ctx, plainToken)
	if err != nil {
		return nil, err
	}
	now := s.clock.Now()
	if err := inv.RequirePending(now); err != nil {
		return nil, err
	}
	if domain.NormalizeEmail(actorEmail) != inv.Email {
		return nil, domain.ErrEmailMismatch
	}
	if _, err := s.repo.GetByID(ctx, inv.OrganizationID); err != nil {
		return nil, notFoundAs(err, domain.ErrInvitationNotFound)
	}
	existing, err := s.repo.GetMember(ctx, inv.OrganizationID, actorID)
	switch {
	case err == nil && existing != nil:
		return nil, domain.ErrAlreadyMember
	case err != nil && apperror.KindOf(err) != apperror.KindNotFound:
		return nil, err
	}
	member, err := domain.NewMember(inv.OrganizationID, actorID, inv.Role, now)
	if err != nil {
		return nil, err
	}
	if err := inv.Accept(now); err != nil {
		return nil, err
	}
	err = s.tx.InTx(ctx, func(ctx context.Context) error {
		if err := s.repo.UpdateInvitation(ctx, inv); err != nil {
			return err
		}
		if err := s.repo.AddMember(ctx, member); err != nil {
			if errors.Is(err, domain.ErrMemberExists) {
				return domain.ErrAlreadyMember
			}
			return err
		}
		if err := s.publish(ctx, inv.OrganizationID, actorID, EventInvitationAccepted, "invitation", inv.ID,
			map[string]any{"email": inv.Email, "role": string(inv.Role)}); err != nil {
			return err
		}
		return s.publish(ctx, inv.OrganizationID, actorID, EventMemberAdded, "organization_member", actorID,
			map[string]any{"role": string(inv.Role)})
	})
	if err != nil {
		return nil, err
	}
	return member, nil
}

// invitationByToken resolves a plain token. Empty and unknown tokens return domain.ErrInvitationNotFound.
func (s *OrganizationService) invitationByToken(ctx context.Context, plainToken string) (*domain.Invitation, error) {
	if plainToken == "" {
		return nil, domain.ErrInvitationNotFound
	}
	inv, err := s.repo.GetInvitationByTokenHash(ctx, domain.HashToken(plainToken))
	if err != nil {
		return nil, notFoundAs(err, domain.ErrInvitationNotFound)
	}
	return inv, nil
}
