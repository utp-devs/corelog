package usecase

import (
	"context"
	"sort"
	"strings"

	"github.com/google/uuid"

	"github.com/jhonsferg/corelog/internal/organization/application/port"
	"github.com/jhonsferg/corelog/internal/organization/domain"
)

// ListMemberViews joins memberships with the fake user table, oldest first.
func (r *fakeRepo) ListMemberViews(_ context.Context, orgID uuid.UUID) ([]port.MemberView, error) {
	var out []port.MemberView
	for k, m := range r.members {
		if k.org != orgID {
			continue
		}
		u := r.users[k.user]
		out = append(out, port.MemberView{UserID: k.user, Name: u.name, Email: u.email, Role: m.Role, JoinedAt: m.JoinedAt})
	}
	sort.Slice(out, func(i, j int) bool {
		if !out[i].JoinedAt.Equal(out[j].JoinedAt) {
			return out[i].JoinedAt.Before(out[j].JoinedAt)
		}
		return out[i].UserID.String() < out[j].UserID.String()
	})
	return out, nil
}

// FindMemberByEmail matches the email against the fake user table, case-insensitively.
func (r *fakeRepo) FindMemberByEmail(_ context.Context, orgID uuid.UUID, email string) (bool, error) {
	for k := range r.members {
		if k.org == orgID && strings.EqualFold(r.users[k.user].email, email) {
			return true, nil
		}
	}
	return false, nil
}

// CreateInvitation assigns an ID and enforces the partial unique index: one live invitation per
// organization and email.
func (r *fakeRepo) CreateInvitation(_ context.Context, inv *domain.Invitation) error {
	for _, o := range r.invites {
		if o.OrganizationID == inv.OrganizationID && o.Email == inv.Email && o.AcceptedAt == nil && o.RevokedAt == nil {
			return domain.ErrInvitationExists
		}
	}
	inv.ID = uuid.New()
	stored := *inv
	r.invites[inv.ID] = &stored
	return nil
}

func (r *fakeRepo) GetInvitationByID(_ context.Context, orgID, id uuid.UUID) (*domain.Invitation, error) {
	o, ok := r.invites[id]
	if !ok || o.OrganizationID != orgID {
		return nil, domain.ErrInvitationNotFound
	}
	c := *o
	return &c, nil
}

func (r *fakeRepo) GetInvitationByTokenHash(_ context.Context, tokenHash string) (*domain.Invitation, error) {
	for _, o := range r.invites {
		if o.TokenHash == tokenHash {
			c := *o
			return &c, nil
		}
	}
	return nil, domain.ErrInvitationNotFound
}

// ListInvitations returns newest first. With pendingOnly it drops accepted and revoked invitations.
func (r *fakeRepo) ListInvitations(_ context.Context, orgID uuid.UUID, pendingOnly bool) ([]*domain.Invitation, error) {
	var out []*domain.Invitation
	for _, o := range r.invites {
		if o.OrganizationID != orgID {
			continue
		}
		if pendingOnly && (o.AcceptedAt != nil || o.RevokedAt != nil) {
			continue
		}
		c := *o
		out = append(out, &c)
	}
	sort.Slice(out, func(i, j int) bool {
		if !out[i].CreatedAt.Equal(out[j].CreatedAt) {
			return out[i].CreatedAt.After(out[j].CreatedAt)
		}
		return out[i].ID.String() < out[j].ID.String()
	})
	return out, nil
}

func (r *fakeRepo) UpdateInvitation(_ context.Context, inv *domain.Invitation) error {
	if _, ok := r.invites[inv.ID]; !ok {
		return domain.ErrInvitationNotFound
	}
	stored := *inv
	r.invites[inv.ID] = &stored
	return nil
}
