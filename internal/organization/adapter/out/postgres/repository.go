// Package postgres implements the organization repository on PostgreSQL. Every method runs through
// database.Executor, so it joins the transaction opened by the use case when there is one.
package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jmoiron/sqlx"

	"github.com/jhonsferg/corelog/internal/organization/application/port"
	"github.com/jhonsferg/corelog/internal/organization/domain"
	"github.com/jhonsferg/corelog/internal/platform/database"
)

const uniqueViolation = "23505"

// Repository stores organizations, members and memberships in PostgreSQL.
type Repository struct {
	db *sqlx.DB
}

var _ port.Repository = (*Repository)(nil)

// NewRepository returns a repository backed by db.
func NewRepository(db *sqlx.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) exec(ctx context.Context) sqlx.ExtContext {
	return database.Executor(ctx, r.db)
}

// orgRow is the storage shape of an organization. It is private to the adapter.
type orgRow struct {
	ID          uuid.UUID     `db:"id"`
	Slug        string        `db:"slug"`
	Name        string        `db:"name"`
	Description string        `db:"description"`
	Settings    []byte        `db:"settings"`
	CreatedBy   uuid.NullUUID `db:"created_by"`
	CreatedAt   time.Time     `db:"created_at"`
	UpdatedAt   time.Time     `db:"updated_at"`
	DeletedAt   *time.Time    `db:"deleted_at"`
}

// membershipRow is an organization row joined with the caller's role.
type membershipRow struct {
	orgRow
	Role string `db:"role"`
}

// memberRow is the storage shape of organization_members.
type memberRow struct {
	OrganizationID uuid.UUID `db:"organization_id"`
	UserID         uuid.UUID `db:"user_id"`
	Role           string    `db:"role"`
	JoinedAt       time.Time `db:"joined_at"`
}

// settingsJSON is the JSONB shape of organizations.settings. The domain has no json tags.
type settingsJSON struct {
	WhoCanCreateProjects     string  `json:"who_can_create_projects"`
	WhoCanCreateTeams        string  `json:"who_can_create_teams"`
	DefaultProjectVisibility string  `json:"default_project_visibility"`
	Require2FA               bool    `json:"require_2fa"`
	MaxUploadBytes           int64   `json:"max_upload_bytes"`
	HoursPerDay              float64 `json:"hours_per_day"`
	DaysPerWeek              float64 `json:"days_per_week"`
	Timezone                 string  `json:"timezone"`
}

const orgColumns = `o.id, o.slug, o.name, o.description, o.settings, o.created_by, o.created_at, o.updated_at, o.deleted_at`

const memberColumns = `m.organization_id, m.user_id, m.role, m.joined_at`

func encodeSettings(s domain.Settings) ([]byte, error) {
	raw, err := json.Marshal(settingsJSON{
		WhoCanCreateProjects:     s.WhoCanCreateProjects,
		WhoCanCreateTeams:        s.WhoCanCreateTeams,
		DefaultProjectVisibility: s.DefaultProjectVisibility,
		Require2FA:               s.Require2FA,
		MaxUploadBytes:           s.MaxUploadBytes,
		HoursPerDay:              s.HoursPerDay,
		DaysPerWeek:              s.DaysPerWeek,
		Timezone:                 s.Timezone,
	})
	if err != nil {
		return nil, fmt.Errorf("encode settings: %w", err)
	}
	return raw, nil
}

func decodeSettings(raw []byte) (domain.Settings, error) {
	var j settingsJSON
	if err := json.Unmarshal(raw, &j); err != nil {
		return domain.Settings{}, fmt.Errorf("decode settings: %w", err)
	}
	return domain.Settings{
		WhoCanCreateProjects:     j.WhoCanCreateProjects,
		WhoCanCreateTeams:        j.WhoCanCreateTeams,
		DefaultProjectVisibility: j.DefaultProjectVisibility,
		Require2FA:               j.Require2FA,
		MaxUploadBytes:           j.MaxUploadBytes,
		HoursPerDay:              j.HoursPerDay,
		DaysPerWeek:              j.DaysPerWeek,
		Timezone:                 j.Timezone,
	}, nil
}

func (row orgRow) toDomain() (*domain.Organization, error) {
	settings, err := decodeSettings(row.Settings)
	if err != nil {
		return nil, err
	}
	org := &domain.Organization{
		ID:          row.ID,
		Slug:        row.Slug,
		Name:        row.Name,
		Description: row.Description,
		Settings:    settings,
		CreatedAt:   row.CreatedAt.UTC(),
		UpdatedAt:   row.UpdatedAt.UTC(),
	}
	if row.CreatedBy.Valid {
		org.CreatedBy = row.CreatedBy.UUID
	}
	if row.DeletedAt != nil {
		at := row.DeletedAt.UTC()
		org.DeletedAt = &at
	}
	return org, nil
}

func (row memberRow) toDomain() *domain.Member {
	return &domain.Member{
		OrganizationID: row.OrganizationID,
		UserID:         row.UserID,
		Role:           domain.Role(row.Role),
		JoinedAt:       row.JoinedAt.UTC(),
	}
}

func nullableUUID(id uuid.UUID) any {
	if id == uuid.Nil {
		return nil
	}
	return id
}

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == uniqueViolation
}

// slugTaken reports whether any organization, deleted or not, already uses slug. The unique index
// only covers live rows, so the reservation of deleted slugs is checked here.
func (r *Repository) slugTaken(ctx context.Context, exec sqlx.ExtContext, slug string) (bool, error) {
	var taken bool
	err := sqlx.GetContext(ctx, exec, &taken,
		`SELECT EXISTS (SELECT 1 FROM organizations WHERE lower(slug) = lower($1))`, slug)
	if err != nil {
		return false, fmt.Errorf("organization: check slug: %w", err)
	}
	return taken, nil
}

// Create inserts the organization and assigns its ID. A slug already in use (live or soft-deleted,
// case-insensitive) returns domain.ErrSlugTaken.
func (r *Repository) Create(ctx context.Context, org *domain.Organization) error {
	exec := r.exec(ctx)
	taken, err := r.slugTaken(ctx, exec, org.Slug)
	if err != nil {
		return err
	}
	if taken {
		return domain.ErrSlugTaken
	}
	settings, err := encodeSettings(org.Settings)
	if err != nil {
		return fmt.Errorf("organization: create: %w", err)
	}
	var id uuid.UUID
	err = sqlx.GetContext(ctx, exec, &id, `
INSERT INTO organizations (slug, name, description, settings, created_by, created_at, updated_at)
VALUES ($1, $2, $3, $4::jsonb, $5, $6, $7)
RETURNING id`,
		org.Slug, org.Name, org.Description, string(settings), nullableUUID(org.CreatedBy),
		org.CreatedAt, org.UpdatedAt,
	)
	if isUniqueViolation(err) {
		return domain.ErrSlugTaken
	}
	if err != nil {
		return fmt.Errorf("organization: create: %w", err)
	}
	org.ID = id
	return nil
}

// GetByID returns a non-deleted organization.
func (r *Repository) GetByID(ctx context.Context, id uuid.UUID) (*domain.Organization, error) {
	var row orgRow
	err := sqlx.GetContext(ctx, r.exec(ctx), &row,
		`SELECT `+orgColumns+` FROM organizations o WHERE o.id = $1 AND o.deleted_at IS NULL`, id)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, domain.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("organization: get by id: %w", err)
	}
	return row.toDomain()
}

// GetBySlug returns the organization with the slug, case-insensitively. Soft-deleted organizations are
// included so the slug stays reserved; a live organization is preferred when both exist.
func (r *Repository) GetBySlug(ctx context.Context, slug string) (*domain.Organization, error) {
	var row orgRow
	err := sqlx.GetContext(ctx, r.exec(ctx), &row, `
SELECT `+orgColumns+`
FROM organizations o
WHERE lower(o.slug) = lower($1)
ORDER BY (o.deleted_at IS NULL) DESC, o.created_at DESC
LIMIT 1`, slug)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, domain.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("organization: get by slug: %w", err)
	}
	return row.toDomain()
}

// ListByUser returns the non-deleted organizations of the user with the user's role, ordered by name.
func (r *Repository) ListByUser(ctx context.Context, userID uuid.UUID) ([]port.Membership, error) {
	var rows []membershipRow
	err := sqlx.SelectContext(ctx, r.exec(ctx), &rows, `
SELECT `+orgColumns+`, m.role
FROM organization_members m
JOIN organizations o ON o.id = m.organization_id
WHERE m.user_id = $1 AND o.deleted_at IS NULL
ORDER BY o.name, o.id`, userID)
	if err != nil {
		return nil, fmt.Errorf("organization: list by user: %w", err)
	}
	out := make([]port.Membership, 0, len(rows))
	for _, row := range rows {
		org, err := row.orgRow.toDomain()
		if err != nil {
			return nil, err
		}
		out = append(out, port.Membership{Organization: org, Role: domain.Role(row.Role)})
	}
	return out, nil
}

// Update persists name, description, settings and updated_at of a non-deleted organization.
func (r *Repository) Update(ctx context.Context, org *domain.Organization) error {
	settings, err := encodeSettings(org.Settings)
	if err != nil {
		return fmt.Errorf("organization: update: %w", err)
	}
	res, err := r.exec(ctx).ExecContext(ctx, `
UPDATE organizations
SET name = $2, description = $3, settings = $4::jsonb, updated_at = $5
WHERE id = $1 AND deleted_at IS NULL`,
		org.ID, org.Name, org.Description, string(settings), org.UpdatedAt)
	return rowsOrNotFound(res, err, "update", domain.ErrNotFound)
}

// SoftDelete sets deleted_at on a non-deleted organization. The slug stays reserved.
func (r *Repository) SoftDelete(ctx context.Context, id uuid.UUID, at time.Time) error {
	res, err := r.exec(ctx).ExecContext(ctx, `
UPDATE organizations
SET deleted_at = $2, updated_at = $2
WHERE id = $1 AND deleted_at IS NULL`, id, at.UTC())
	return rowsOrNotFound(res, err, "soft delete", domain.ErrNotFound)
}

// ListMembers returns the members of a non-deleted organization, oldest first.
func (r *Repository) ListMembers(ctx context.Context, orgID uuid.UUID) ([]*domain.Member, error) {
	var rows []memberRow
	err := sqlx.SelectContext(ctx, r.exec(ctx), &rows, `
SELECT `+memberColumns+`
FROM organization_members m
JOIN organizations o ON o.id = m.organization_id
WHERE m.organization_id = $1 AND o.deleted_at IS NULL
ORDER BY m.joined_at, m.user_id`, orgID)
	if err != nil {
		return nil, fmt.Errorf("organization: list members: %w", err)
	}
	out := make([]*domain.Member, 0, len(rows))
	for _, row := range rows {
		out = append(out, row.toDomain())
	}
	return out, nil
}

// GetMember returns the membership of a user in a non-deleted organization.
func (r *Repository) GetMember(ctx context.Context, orgID, userID uuid.UUID) (*domain.Member, error) {
	var row memberRow
	err := sqlx.GetContext(ctx, r.exec(ctx), &row, `
SELECT `+memberColumns+`
FROM organization_members m
JOIN organizations o ON o.id = m.organization_id
WHERE m.organization_id = $1 AND m.user_id = $2 AND o.deleted_at IS NULL`, orgID, userID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, domain.ErrMemberNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("organization: get member: %w", err)
	}
	return row.toDomain(), nil
}

// AddMember inserts a membership. A duplicate returns domain.ErrMemberExists.
func (r *Repository) AddMember(ctx context.Context, m *domain.Member) error {
	_, err := r.exec(ctx).ExecContext(ctx, `
INSERT INTO organization_members (organization_id, user_id, role, joined_at)
VALUES ($1, $2, $3, $4)`,
		m.OrganizationID, m.UserID, string(m.Role), m.JoinedAt.UTC())
	if isUniqueViolation(err) {
		return domain.ErrMemberExists
	}
	if err != nil {
		return fmt.Errorf("organization: add member: %w", err)
	}
	return nil
}

// UpdateMemberRole changes the role of an existing member.
func (r *Repository) UpdateMemberRole(ctx context.Context, orgID, userID uuid.UUID, role domain.Role) error {
	res, err := r.exec(ctx).ExecContext(ctx, `
UPDATE organization_members SET role = $3
WHERE organization_id = $1 AND user_id = $2`, orgID, userID, string(role))
	return rowsOrNotFound(res, err, "update member role", domain.ErrMemberNotFound)
}

// RemoveMember deletes a membership.
func (r *Repository) RemoveMember(ctx context.Context, orgID, userID uuid.UUID) error {
	res, err := r.exec(ctx).ExecContext(ctx, `
DELETE FROM organization_members
WHERE organization_id = $1 AND user_id = $2`, orgID, userID)
	return rowsOrNotFound(res, err, "remove member", domain.ErrMemberNotFound)
}

// CountOwners counts the owners of a non-deleted organization.
func (r *Repository) CountOwners(ctx context.Context, orgID uuid.UUID) (int, error) {
	var n int
	err := sqlx.GetContext(ctx, r.exec(ctx), &n, `
SELECT COUNT(*)
FROM organization_members m
JOIN organizations o ON o.id = m.organization_id
WHERE m.organization_id = $1 AND m.role = $2 AND o.deleted_at IS NULL`, orgID, string(domain.RoleOwner))
	if err != nil {
		return 0, fmt.Errorf("organization: count owners: %w", err)
	}
	return n, nil
}

// CountOwnedBy counts the non-deleted organizations in which the user has the owner role.
func (r *Repository) CountOwnedBy(ctx context.Context, userID uuid.UUID) (int, error) {
	var n int
	err := sqlx.GetContext(ctx, r.exec(ctx), &n, `
SELECT COUNT(*)
FROM organization_members m
JOIN organizations o ON o.id = m.organization_id
WHERE m.user_id = $1 AND m.role = $2 AND o.deleted_at IS NULL`, userID, string(domain.RoleOwner))
	if err != nil {
		return 0, fmt.Errorf("organization: count owned: %w", err)
	}
	return n, nil
}

// rowsOrNotFound turns a write error or an empty result into the typed error of the operation.
func rowsOrNotFound(res sql.Result, err error, op string, notFound error) error {
	if err != nil {
		return fmt.Errorf("organization: %s: %w", op, err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("organization: %s: rows affected: %w", op, err)
	}
	if n == 0 {
		return notFound
	}
	return nil
}

// GetByIDForUpdate returns a non-deleted organization and locks its row until the enclosing transaction
// ends. Callers must run it inside a transaction, otherwise the lock is released immediately.
func (r *Repository) GetByIDForUpdate(ctx context.Context, id uuid.UUID) (*domain.Organization, error) {
	var row orgRow
	err := sqlx.GetContext(ctx, r.exec(ctx), &row,
		`SELECT `+orgColumns+` FROM organizations o WHERE o.id = $1 AND o.deleted_at IS NULL FOR UPDATE`, id)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, domain.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("organization: get by id for update: %w", err)
	}
	return row.toDomain()
}

// LockOwnerCreation takes a transaction-scoped advisory lock keyed by the user. It is released when the
// transaction commits or rolls back.
func (r *Repository) LockOwnerCreation(ctx context.Context, userID uuid.UUID) error {
	if _, err := r.exec(ctx).ExecContext(ctx,
		`SELECT pg_advisory_xact_lock(hashtext($1))`, "org_create:"+userID.String()); err != nil {
		return fmt.Errorf("organization: lock owner creation: %w", err)
	}
	return nil
}
