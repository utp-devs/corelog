package postgres

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"

	"github.com/jhonsferg/corelog/internal/platform/database"
	"github.com/jhonsferg/corelog/internal/user/application/port"
)

// MembershipDetacher implements port.MembershipDetacher on the organization, team and project tables of
// the shared database. It holds plain SQL only and imports no other hexagon.
type MembershipDetacher struct {
	db *sqlx.DB
}

var _ port.MembershipDetacher = (*MembershipDetacher)(nil)

// NewMembershipDetacher returns a detacher backed by db. Statements join the transaction in the context.
func NewMembershipDetacher(db *sqlx.DB) *MembershipDetacher {
	return &MembershipDetacher{db: db}
}

// SoleOwnedOrganizations locks the live organizations the user owns, then counts those where the user is
// the only owner. The lock is a separate statement so the count runs on a fresh snapshot after it.
func (d *MembershipDetacher) SoleOwnedOrganizations(ctx context.Context, userID uuid.UUID) (int, error) {
	ex := database.Executor(ctx, d.db)

	var locked []uuid.UUID
	if err := sqlx.SelectContext(ctx, ex, &locked, `
		SELECT o.id
		FROM organizations o
		JOIN organization_members m ON m.organization_id = o.id AND m.user_id = $1 AND m.role = 'owner'
		WHERE o.deleted_at IS NULL
		FOR UPDATE OF o`, userID); err != nil {
		return 0, fmt.Errorf("user: lock owned organizations: %w", err)
	}
	if len(locked) == 0 {
		return 0, nil
	}

	var n int
	if err := sqlx.GetContext(ctx, ex, &n, `
		SELECT COUNT(*)
		FROM organization_members m
		JOIN organizations o ON o.id = m.organization_id AND o.deleted_at IS NULL
		WHERE m.user_id = $1 AND m.role = 'owner'
		  AND NOT EXISTS (
		      SELECT 1 FROM organization_members other
		      WHERE other.organization_id = m.organization_id AND other.role = 'owner' AND other.user_id <> $1
		  )`, userID); err != nil {
		return 0, fmt.Errorf("user: count sole-owned organizations: %w", err)
	}
	return n, nil
}

// DetachMemberships removes the user from every organization, team and project membership.
func (d *MembershipDetacher) DetachMemberships(ctx context.Context, userID uuid.UUID) error {
	ex := database.Executor(ctx, d.db)
	for _, table := range []string{"organization_members", "team_members", "project_members"} {
		// table is a constant from the list above, never caller input.
		if _, err := ex.ExecContext(ctx, `DELETE FROM `+table+` WHERE user_id = $1`, userID); err != nil {
			return fmt.Errorf("user: detach from %s: %w", table, err)
		}
	}
	return nil
}
