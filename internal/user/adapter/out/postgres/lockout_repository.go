package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/jmoiron/sqlx"

	"github.com/jhonsferg/corelog/internal/platform/database"
	"github.com/jhonsferg/corelog/internal/user/application/port"
	"github.com/jhonsferg/corelog/internal/user/domain"
)

// LockoutRepository persists the failed login counters in account_lockouts so a restart does not forgive them.
type LockoutRepository struct {
	db *sqlx.DB
}

var _ port.LockoutStore = (*LockoutRepository)(nil)

// NewLockoutRepository returns the repository.
func NewLockoutRepository(db *sqlx.DB) *LockoutRepository { return &LockoutRepository{db: db} }

type lockoutRow struct {
	Failures       int        `db:"failures"`
	LockCount      int        `db:"lock_count"`
	FirstFailureAt *time.Time `db:"first_failure_at"`
	LockedUntil    *time.Time `db:"locked_until"`
}

// Get returns the state of the key; an unknown key has the zero state.
func (r *LockoutRepository) Get(ctx context.Context, key string) (domain.LockoutState, error) {
	var row lockoutRow
	err := r.db.GetContext(ctx, &row,
		`SELECT failures, lock_count, first_failure_at, locked_until FROM account_lockouts WHERE key_hash = $1`, key)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.LockoutState{}, nil
	}
	if err != nil {
		return domain.LockoutState{}, fmt.Errorf("lockout: get: %w", err)
	}
	return domain.LockoutState{
		Failures: row.Failures, LockCount: row.LockCount, FirstFailureAt: row.FirstFailureAt, LockedUntil: row.LockedUntil,
	}, nil
}

// Save upserts the state.
func (r *LockoutRepository) Save(ctx context.Context, key string, s domain.LockoutState, now time.Time) error {
	_, err := r.db.ExecContext(ctx, `
INSERT INTO account_lockouts (key_hash, failures, lock_count, first_failure_at, locked_until, updated_at)
VALUES ($1, $2, $3, $4, $5, $6)
ON CONFLICT (key_hash) DO UPDATE SET failures = EXCLUDED.failures, lock_count = EXCLUDED.lock_count,
    first_failure_at = EXCLUDED.first_failure_at, locked_until = EXCLUDED.locked_until, updated_at = EXCLUDED.updated_at`,
		key, s.Failures, s.LockCount, s.FirstFailureAt, s.LockedUntil, now)
	if err != nil {
		return fmt.Errorf("lockout: save: %w", err)
	}
	return nil
}

// Delete clears the counter of the key (successful login, reset or unlock).
func (r *LockoutRepository) Delete(ctx context.Context, key string) error {
	if _, err := database.Executor(ctx, r.db).ExecContext(ctx, `DELETE FROM account_lockouts WHERE key_hash = $1`, key); err != nil {
		return fmt.Errorf("lockout: delete: %w", err)
	}
	return nil
}

// PurgeStale removes counters that are not locked and have not been touched since the instant.
func (r *LockoutRepository) PurgeStale(ctx context.Context, before time.Time) (int64, error) {
	res, err := r.db.ExecContext(ctx,
		`DELETE FROM account_lockouts WHERE updated_at < $1 AND (locked_until IS NULL OR locked_until < $1)`, before)
	if err != nil {
		return 0, fmt.Errorf("lockout: purge: %w", err)
	}
	return res.RowsAffected()
}
