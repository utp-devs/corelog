package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"

	"github.com/jhonsferg/corelog/internal/user/domain"
)

// AvatarRepository reads and writes users.avatar_key. It satisfies the narrow AvatarRepository port
// declared by the avatar HTTP handler.
type AvatarRepository struct {
	db *sqlx.DB
}

// NewAvatarRepository returns an avatar repository backed by db.
func NewAvatarRepository(db *sqlx.DB) *AvatarRepository {
	return &AvatarRepository{db: db}
}

// SetAvatarKey stores key (nil clears the avatar) and returns the previous key, if any. The row is
// locked while it is read and written, so concurrent uploads cannot lose a file reference.
func (r *AvatarRepository) SetAvatarKey(ctx context.Context, userID uuid.UUID, key *string) (*string, error) {
	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin avatar update: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	var current sql.NullString
	err = tx.GetContext(ctx, &current, `SELECT avatar_key FROM users WHERE id = $1 FOR UPDATE`, userID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, domain.ErrUserNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("lock user for avatar: %w", err)
	}
	if _, err := tx.ExecContext(ctx,
		`UPDATE users SET avatar_key = $2, updated_at = now() WHERE id = $1`, userID, key); err != nil {
		return nil, fmt.Errorf("update avatar key: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit avatar update: %w", err)
	}
	if !current.Valid {
		return nil, nil
	}
	old := current.String
	return &old, nil
}

// GetAvatarKey returns the avatar key of the user, or nil when the user has no avatar or does not exist.
func (r *AvatarRepository) GetAvatarKey(ctx context.Context, userID uuid.UUID) (*string, error) {
	var current sql.NullString
	err := r.db.GetContext(ctx, &current, `SELECT avatar_key FROM users WHERE id = $1`, userID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read avatar key: %w", err)
	}
	if !current.Valid {
		return nil, nil
	}
	key := current.String
	return &key, nil
}
