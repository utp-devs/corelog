package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"

	"github.com/jhonsferg/corelog/internal/organization/domain"
)

// LogoRepository reads and writes organizations.logo_key. It satisfies the narrow LogoRepository port
// declared by the logo HTTP handler. Live (not soft-deleted) organizations only.
type LogoRepository struct {
	db *sqlx.DB
}

// NewLogoRepository returns a logo repository backed by db.
func NewLogoRepository(db *sqlx.DB) *LogoRepository {
	return &LogoRepository{db: db}
}

// SetLogoKey stores key (nil clears the logo) and returns the previous key, if any. The row is locked
// while it is read and written, so concurrent uploads cannot lose a file reference.
func (r *LogoRepository) SetLogoKey(ctx context.Context, orgID uuid.UUID, key *string) (*string, error) {
	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin logo update: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	var current sql.NullString
	err = tx.GetContext(ctx, &current,
		`SELECT logo_key FROM organizations WHERE id = $1 AND deleted_at IS NULL FOR UPDATE`, orgID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, domain.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("lock organization for logo: %w", err)
	}
	if _, err := tx.ExecContext(ctx,
		`UPDATE organizations SET logo_key = $2, updated_at = now() WHERE id = $1`, orgID, key); err != nil {
		return nil, fmt.Errorf("update logo key: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit logo update: %w", err)
	}
	if !current.Valid {
		return nil, nil
	}
	old := current.String
	return &old, nil
}

// GetLogoKey returns the logo key of the organization, or nil when it has no logo or does not exist.
func (r *LogoRepository) GetLogoKey(ctx context.Context, orgID uuid.UUID) (*string, error) {
	var current sql.NullString
	err := r.db.GetContext(ctx, &current,
		`SELECT logo_key FROM organizations WHERE id = $1 AND deleted_at IS NULL`, orgID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read logo key: %w", err)
	}
	if !current.Valid {
		return nil, nil
	}
	key := current.String
	return &key, nil
}
