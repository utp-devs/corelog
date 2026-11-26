package usecase

import (
	"context"

	"github.com/google/uuid"

	"github.com/jhonsferg/corelog/internal/organization/application/port"
	"github.com/jhonsferg/corelog/internal/organization/domain"
	"github.com/jhonsferg/corelog/internal/platform/apperror"
	"github.com/jhonsferg/corelog/internal/platform/clock"
	"github.com/jhonsferg/corelog/internal/platform/database"
	"github.com/jhonsferg/corelog/internal/platform/events"
)

// Event types published by this service.
const (
	EventOrgCreated        = "org.created"
	EventOrgUpdated        = "org.updated"
	EventOrgDeleted        = "org.deleted"
	EventMemberAdded       = "member.added"
	EventMemberRoleChanged = "member.role_changed"
	EventMemberRemoved     = "member.removed"
)

// OrganizationService implements port.Service.
type OrganizationService struct {
	repo  port.Repository
	tx    database.TxManager
	pub   events.Publisher
	clock clock.Clock
}

var _ port.Service = (*OrganizationService)(nil)

// NewOrganizationService wires the use cases. All dependencies are required.
func NewOrganizationService(repo port.Repository, tx database.TxManager, pub events.Publisher, clk clock.Clock) *OrganizationService {
	return &OrganizationService{repo: repo, tx: tx, pub: pub, clock: clk}
}

// CreateOrganization creates the organization with default settings and makes the creator its owner.
func (s *OrganizationService) CreateOrganization(ctx context.Context, actorID uuid.UUID, in port.CreateInput) (*domain.Organization, error) {
	now := s.clock.Now()
	org, err := domain.NewOrganization(in.Name, in.Slug, in.Description, actorID, now)
	if err != nil {
		return nil, err
	}
	taken, err := s.repo.GetBySlug(ctx, org.Slug)
	switch {
	case err == nil && taken != nil:
		return nil, domain.ErrSlugTaken
	case err != nil && apperror.KindOf(err) != apperror.KindNotFound:
		return nil, err
	}
	owner, err := domain.NewMember(uuid.Nil, actorID, domain.RoleOwner, now)
	if err != nil {
		return nil, err
	}
	err = s.tx.InTx(ctx, func(ctx context.Context) error {
		if err := s.repo.Create(ctx, org); err != nil {
			return err
		}
		owner.OrganizationID = org.ID
		if err := s.repo.AddMember(ctx, owner); err != nil {
			return err
		}
		if err := s.publish(ctx, org.ID, actorID, EventOrgCreated, "organization", org.ID,
			map[string]any{"slug": org.Slug, "name": org.Name}); err != nil {
			return err
		}
		return s.publish(ctx, org.ID, actorID, EventMemberAdded, "organization_member", actorID,
			map[string]any{"role": string(domain.RoleOwner)})
	})
	if err != nil {
		return nil, err
	}
	return org, nil
}

// GetOrganization returns the organization to any member, guests included.
func (s *OrganizationService) GetOrganization(ctx context.Context, actorID, orgID uuid.UUID) (*domain.Organization, error) {
	if _, err := s.RequireRole(ctx, actorID, orgID, domain.RoleGuest); err != nil {
		return nil, err
	}
	return s.repo.GetByID(ctx, orgID)
}

// UpdateSettings replaces the settings. Owners and admins only.
func (s *OrganizationService) UpdateSettings(ctx context.Context, actorID, orgID uuid.UUID, in port.UpdateInput) (*domain.Organization, error) {
	if _, err := s.RequireRole(ctx, actorID, orgID, domain.RoleAdmin); err != nil {
		return nil, err
	}
	org, err := s.repo.GetByID(ctx, orgID)
	if err != nil {
		return nil, err
	}
	if err := org.ApplySettings(in.Settings, s.clock.Now()); err != nil {
		return nil, err
	}
	err = s.tx.InTx(ctx, func(ctx context.Context) error {
		if err := s.repo.Update(ctx, org); err != nil {
			return err
		}
		return s.publish(ctx, org.ID, actorID, EventOrgUpdated, "organization", org.ID,
			map[string]any{"slug": org.Slug})
	})
	if err != nil {
		return nil, err
	}
	return org, nil
}

// ListMine returns the organizations the actor belongs to.
func (s *OrganizationService) ListMine(ctx context.Context, actorID uuid.UUID) ([]port.Membership, error) {
	return s.repo.ListByUser(ctx, actorID)
}

// DeleteOrganization soft-deletes the organization. Owner only; the typed slug must match.
func (s *OrganizationService) DeleteOrganization(ctx context.Context, actorID, orgID uuid.UUID, confirmSlug string) error {
	if _, err := s.RequireRole(ctx, actorID, orgID, domain.RoleOwner); err != nil {
		return err
	}
	org, err := s.repo.GetByID(ctx, orgID)
	if err != nil {
		return err
	}
	if err := org.ConfirmDeletion(confirmSlug); err != nil {
		return err
	}
	now := s.clock.Now()
	org.MarkDeleted(now)
	return s.tx.InTx(ctx, func(ctx context.Context) error {
		if err := s.repo.SoftDelete(ctx, orgID, now); err != nil {
			return err
		}
		return s.publish(ctx, orgID, actorID, EventOrgDeleted, "organization", orgID,
			map[string]any{"slug": org.Slug})
	})
}

// LeaveOrganization removes the actor from the organization. The last owner cannot leave.
func (s *OrganizationService) LeaveOrganization(ctx context.Context, actorID, orgID uuid.UUID) error {
	role, err := s.RequireRole(ctx, actorID, orgID, domain.RoleGuest)
	if err != nil {
		return err
	}
	if role == domain.RoleOwner {
		owners, err := s.repo.CountOwners(ctx, orgID)
		if err != nil {
			return err
		}
		if err := domain.ValidateLastOwner(owners - 1); err != nil {
			return err
		}
	}
	return s.tx.InTx(ctx, func(ctx context.Context) error {
		if err := s.repo.RemoveMember(ctx, orgID, actorID); err != nil {
			return err
		}
		return s.publish(ctx, orgID, actorID, EventMemberRemoved, "organization_member", actorID,
			map[string]any{"role": string(role), "reason": "left"})
	})
}

// TransferOwnership makes an existing member the owner and demotes the actor to admin, atomically.
func (s *OrganizationService) TransferOwnership(ctx context.Context, actorID, orgID, newOwnerID uuid.UUID) error {
	if _, err := s.RequireRole(ctx, actorID, orgID, domain.RoleOwner); err != nil {
		return err
	}
	if newOwnerID == actorID {
		return domain.ErrTransferSelf
	}
	target, err := s.repo.GetMember(ctx, orgID, newOwnerID)
	if err != nil {
		return err
	}
	if err := domain.CanChangeRole(domain.RoleOwner, target.Role, domain.RoleOwner); err != nil {
		return err
	}
	if err := domain.CanChangeRole(domain.RoleOwner, domain.RoleOwner, domain.RoleAdmin); err != nil {
		return err
	}
	previousTargetRole := target.Role
	return s.tx.InTx(ctx, func(ctx context.Context) error {
		if err := s.repo.UpdateMemberRole(ctx, orgID, newOwnerID, domain.RoleOwner); err != nil {
			return err
		}
		if err := s.repo.UpdateMemberRole(ctx, orgID, actorID, domain.RoleAdmin); err != nil {
			return err
		}
		if err := s.publish(ctx, orgID, actorID, EventMemberRoleChanged, "organization_member", newOwnerID,
			map[string]any{"old_role": string(previousTargetRole), "new_role": string(domain.RoleOwner)}); err != nil {
			return err
		}
		return s.publish(ctx, orgID, actorID, EventMemberRoleChanged, "organization_member", actorID,
			map[string]any{"old_role": string(domain.RoleOwner), "new_role": string(domain.RoleAdmin)})
	})
}

// RequireRole is the organization authorization check. It hides organizations that are deleted or that
// the user does not belong to (domain.ErrNotFound), and denies members below min (domain.ErrForbidden).
func (s *OrganizationService) RequireRole(ctx context.Context, userID, orgID uuid.UUID, min domain.Role) (domain.Role, error) {
	if _, err := s.repo.GetByID(ctx, orgID); err != nil {
		return "", notFoundAs(err, domain.ErrNotFound)
	}
	member, err := s.repo.GetMember(ctx, orgID, userID)
	if err != nil {
		return "", notFoundAs(err, domain.ErrNotFound)
	}
	if member.Role.Rank() < min.Rank() {
		return "", domain.ErrForbidden
	}
	return member.Role, nil
}

// publish records an event with the organization as scope. It runs inside the caller's transaction.
func (s *OrganizationService) publish(ctx context.Context, orgID, actorID uuid.UUID, eventType, entityType string, entityID uuid.UUID, payload map[string]any) error {
	org := orgID
	actor := actorID
	entity := entityID
	return s.pub.Publish(ctx, events.Event{
		OrgID:      &org,
		ActorID:    &actor,
		EntityID:   &entity,
		Type:       eventType,
		EntityType: entityType,
		Payload:    payload,
	})
}

// notFoundAs maps repository not-found errors to replacement and leaves every other error untouched.
func notFoundAs(err, replacement error) error {
	if apperror.KindOf(err) == apperror.KindNotFound {
		return replacement
	}
	return err
}
