package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"

	"github.com/jhonsferg/corelog/internal/platform/database"
	"github.com/jhonsferg/corelog/internal/user/application/port"
)

// TokenStore keeps single use tokens (hashed) in one of the two token tables.
type TokenStore struct {
	db    *sqlx.DB
	table string
}

var _ port.TokenStore = (*TokenStore)(nil)

// NewPasswordResetTokens stores password reset tokens.
func NewPasswordResetTokens(db *sqlx.DB) *TokenStore {
	return &TokenStore{db: db, table: "password_reset_tokens"}
}

// NewEmailVerificationTokens stores email verification tokens.
func NewEmailVerificationTokens(db *sqlx.DB) *TokenStore {
	return &TokenStore{db: db, table: "email_verification_tokens"}
}

// Replace discards the unused tokens of the user and stores the new one in one transaction. The table name
// comes from the two constructors above, never from input.
func (s *TokenStore) Replace(ctx context.Context, userID uuid.UUID, hash []byte, expiresAt, now time.Time) error {
	run := func(ctx context.Context, exec sqlx.ExtContext) error {
		if _, err := exec.ExecContext(ctx, "DELETE FROM "+s.table+" WHERE user_id = $1 AND used_at IS NULL", userID); err != nil {
			return fmt.Errorf("discard old tokens: %w", err)
		}
		if _, err := exec.ExecContext(ctx,
			"INSERT INTO "+s.table+" (user_id, token_hash, expires_at, created_at) VALUES ($1, $2, $3, $4)",
			userID, hash, expiresAt, now); err != nil {
			return fmt.Errorf("insert token: %w", err)
		}
		return nil
	}
	if tx, ok := database.TxFromContext(ctx); ok {
		return run(ctx, tx)
	}
	tx, err := s.db.BeginTxx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin token tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	if err := run(ctx, tx); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit token tx: %w", err)
	}
	return nil
}

// Find returns the owner of an unused, unexpired token without consuming it.
func (s *TokenStore) Find(ctx context.Context, hash []byte, now time.Time) (uuid.UUID, bool, error) {
	var userID uuid.UUID
	err := sqlx.GetContext(ctx, database.Executor(ctx, s.db), &userID,
		"SELECT user_id FROM "+s.table+" WHERE token_hash = $1 AND used_at IS NULL AND expires_at > $2", hash, now)
	if errors.Is(err, sql.ErrNoRows) {
		return uuid.Nil, false, nil
	}
	if err != nil {
		return uuid.Nil, false, fmt.Errorf("find token: %w", err)
	}
	return userID, true, nil
}

// Consume atomically marks an unused, unexpired token as used.
func (s *TokenStore) Consume(ctx context.Context, hash []byte, now time.Time) (uuid.UUID, bool, error) {
	var userID uuid.UUID
	err := sqlx.GetContext(ctx, database.Executor(ctx, s.db), &userID,
		"UPDATE "+s.table+" SET used_at = $2 WHERE token_hash = $1 AND used_at IS NULL AND expires_at > $2 RETURNING user_id",
		hash, now)
	if errors.Is(err, sql.ErrNoRows) {
		return uuid.Nil, false, nil
	}
	if err != nil {
		return uuid.Nil, false, fmt.Errorf("consume token: %w", err)
	}
	return userID, true, nil
}

// PurgeExpired deletes tokens that expired or were used before the instant.
func (s *TokenStore) PurgeExpired(ctx context.Context, before time.Time) (int64, error) {
	res, err := s.db.ExecContext(ctx,
		"DELETE FROM "+s.table+" WHERE expires_at < $1 OR (used_at IS NOT NULL AND used_at < $1)", before)
	if err != nil {
		return 0, fmt.Errorf("purge tokens: %w", err)
	}
	return res.RowsAffected()
}
