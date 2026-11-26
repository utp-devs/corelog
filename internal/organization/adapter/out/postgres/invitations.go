package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jmoiron/sqlx"

	"github.com/jhonsferg/corelog/internal/organization/application/port"
	"github.com/jhonsferg/corelog/internal/organization/domain"
)

// pendingInvitationIndex is the partial unique index that allows one live invitation per email.
const pendingInvitationIndex = "uq_organization_invitations_pending"

// invitationRow is the storage shape of organization_invitations. It is private to the adapter.
type invitationRow struct {
	ID             uuid.UUID     `db:"id"`
	OrganizationID uuid.UUID     `db:"organization_id"`
	Email          string        `db:"email"`
	Role           string        `db:"role"`
	TokenHash      string        `db:"token_hash"`
	InvitedBy      uuid.NullUUID `db:"invited_by"`
	ExpiresAt      time.Time     `db:"expires_at"`
	AcceptedAt     *time.Time    `db:"accepted_at"`
	RevokedAt      *time.Time    `db:"revoked_at"`
	CreatedAt      time.Time     `db:"created_at"`
}

// memberViewRow is the join of organization_members and users used by ListMemberViews.
type memberViewRow struct {
	UserID   uuid.UUID `db:"user_id"`
	Name     string    `db:"name"`
	Email    string    `db:"email"`
	Role     string    `db:"role"`
	JoinedAt time.Time `db:"joined_at"`
}

// invitationColumns lists the explicit columns of organization_invitations. The alias i is required.
const invitationColumns = `i.id, i.organization_id, i.email, i.role, i.token_hash, i.invited_by,
i.expires_at, i.accepted_at, i.revoked_at, i.created_at`

// invitationJoin keeps invitations of deleted organizations out of every read.
const invitationJoin = `FROM organization_invitations i
JOIN organizations o ON o.id = i.organization_id AND o.deleted_at IS NULL`

func (row invitationRow) toDomain() *domain.Invitation {
	inv := &domain.Invitation{
		ID:             row.ID,
		OrganizationID: row.OrganizationID,
		Email:          row.Email,
		Role:           domain.Role(row.Role),
		TokenHash:      row.TokenHash,
		ExpiresAt:      row.ExpiresAt.UTC(),
		CreatedAt:      row.CreatedAt.UTC(),
	}
	if row.InvitedBy.Valid {
		inv.InvitedBy = row.InvitedBy.UUID
	}
	if row.AcceptedAt != nil {
		at := row.AcceptedAt.UTC()
		inv.AcceptedAt = &at
	}
	if row.RevokedAt != nil {
		at := row.RevokedAt.UTC()
		inv.RevokedAt = &at
	}
	return inv
}

// pendingConflict reports whether err is the pending-invitation unique index violation.
func pendingConflict(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == uniqueViolation && pgErr.ConstraintName == pendingInvitationIndex
}

// CreateInvitation inserts the invitation and assigns its ID. A live invitation for the same email in
// the same organization returns domain.ErrInvitationExists.
func (r *Repository) CreateInvitation(ctx context.Context, inv *domain.Invitation) error {
	var id uuid.UUID
	err := sqlx.GetContext(ctx, r.exec(ctx), &id, `
INSERT INTO organization_invitations (organization_id, email, role, token_hash, invited_by, expires_at, created_at)
VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING id`,
		inv.OrganizationID, inv.Email, string(inv.Role), inv.TokenHash, nullableUUID(inv.InvitedBy),
		inv.ExpiresAt.UTC(), inv.CreatedAt.UTC(),
	)
	if pendingConflict(err) {
		return domain.ErrInvitationExists
	}
	if err != nil {
		return fmt.Errorf("organization: create invitation: %w", err)
	}
	inv.ID = id
	return nil
}

// GetInvitationByID returns the invitation only when it belongs to orgID.
func (r *Repository) GetInvitationByID(ctx context.Context, orgID, id uuid.UUID) (*domain.Invitation, error) {
	var row invitationRow
	err := sqlx.GetContext(ctx, r.exec(ctx), &row, `
SELECT `+invitationColumns+`
`+invitationJoin+`
WHERE i.id = $1 AND i.organization_id = $2`, id, orgID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, domain.ErrInvitationNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("organization: get invitation: %w", err)
	}
	return row.toDomain(), nil
}

// GetInvitationByTokenHash looks an invitation up by the hash of its token.
func (r *Repository) GetInvitationByTokenHash(ctx context.Context, tokenHash string) (*domain.Invitation, error) {
	var row invitationRow
	err := sqlx.GetContext(ctx, r.exec(ctx), &row, `
SELECT `+invitationColumns+`
`+invitationJoin+`
WHERE i.token_hash = $1`, tokenHash)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, domain.ErrInvitationNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("organization: get invitation by token: %w", err)
	}
	return row.toDomain(), nil
}

// ListInvitations returns the invitations of a live organization, newest first. With pendingOnly the
// accepted and revoked ones are excluded.
func (r *Repository) ListInvitations(ctx context.Context, orgID uuid.UUID, pendingOnly bool) ([]*domain.Invitation, error) {
	var rows []invitationRow
	err := sqlx.SelectContext(ctx, r.exec(ctx), &rows, `
SELECT `+invitationColumns+`
`+invitationJoin+`
WHERE i.organization_id = $1
  AND (NOT $2::boolean OR (i.accepted_at IS NULL AND i.revoked_at IS NULL))
ORDER BY i.created_at DESC, i.id DESC`, orgID, pendingOnly)
	if err != nil {
		return nil, fmt.Errorf("organization: list invitations: %w", err)
	}
	out := make([]*domain.Invitation, 0, len(rows))
	for _, row := range rows {
		out = append(out, row.toDomain())
	}
	return out, nil
}

// UpdateInvitation persists the mutable state of an invitation: token hash, expiry, accepted and revoked
// timestamps. An unknown invitation in the organization returns domain.ErrInvitationNotFound.
func (r *Repository) UpdateInvitation(ctx context.Context, inv *domain.Invitation) error {
	res, err := r.exec(ctx).ExecContext(ctx, `
UPDATE organization_invitations
SET token_hash = $3, expires_at = $4, accepted_at = $5, revoked_at = $6
WHERE id = $1 AND organization_id = $2`,
		inv.ID, inv.OrganizationID, inv.TokenHash, inv.ExpiresAt.UTC(),
		timePtrUTC(inv.AcceptedAt), timePtrUTC(inv.RevokedAt))
	return rowsOrNotFound(res, err, "update invitation", domain.ErrInvitationNotFound)
}

// timePtrUTC converts an optional instant to UTC, keeping nil as SQL NULL.
func timePtrUTC(t *time.Time) *time.Time {
	if t == nil {
		return nil
	}
	at := t.UTC()
	return &at
}

// ListMemberViews returns the members of a live organization with the account name and email.
// A single statement joins users, so there is no per-member query.
func (r *Repository) ListMemberViews(ctx context.Context, orgID uuid.UUID) ([]port.MemberView, error) {
	var rows []memberViewRow
	err := sqlx.SelectContext(ctx, r.exec(ctx), &rows, `
SELECT m.user_id, COALESCE(u.display_name, u.name) AS name, u.email, m.role, m.joined_at
FROM organization_members m
JOIN organizations o ON o.id = m.organization_id AND o.deleted_at IS NULL
JOIN users u ON u.id = m.user_id
WHERE m.organization_id = $1
ORDER BY m.joined_at, m.user_id`, orgID)
	if err != nil {
		return nil, fmt.Errorf("organization: list member views: %w", err)
	}
	out := make([]port.MemberView, 0, len(rows))
	for _, row := range rows {
		out = append(out, port.MemberView{
			UserID:   row.UserID,
			Name:     row.Name,
			Email:    row.Email,
			Role:     domain.Role(row.Role),
			JoinedAt: row.JoinedAt.UTC(),
		})
	}
	return out, nil
}

// FindMemberByEmail reports whether a member of the organization uses this email (case-insensitive).
func (r *Repository) FindMemberByEmail(ctx context.Context, orgID uuid.UUID, email string) (bool, error) {
	var found bool
	err := sqlx.GetContext(ctx, r.exec(ctx), &found, `
SELECT EXISTS (
    SELECT 1
    FROM organization_members m
    JOIN users u ON u.id = m.user_id
    WHERE m.organization_id = $1 AND lower(u.email) = lower($2)
)`, orgID, email)
	if err != nil {
		return false, fmt.Errorf("organization: find member by email: %w", err)
	}
	return found, nil
}
