package postgres

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"

	"github.com/jhonsferg/corelog/internal/platform/apperror"
	"github.com/jhonsferg/corelog/internal/platform/database"
	"github.com/jhonsferg/corelog/internal/user/application/port"
)

// TwoFactorRepository stores TOTP setups, recovery codes and login challenges.
type TwoFactorRepository struct {
	db *sqlx.DB
}

func NewTwoFactorRepository(db *sqlx.DB) *TwoFactorRepository {
	return &TwoFactorRepository{db: db}
}

var _ port.TwoFactorRepository = (*TwoFactorRepository)(nil)

type twoFactorSetupRow struct {
	UserID          uuid.UUID  `db:"user_id"`
	SecretEncrypted []byte     `db:"secret_encrypted"`
	EnabledAt       *time.Time `db:"enabled_at"`
	LastUsedStep    *int64     `db:"last_used_step"`
}

type recoveryCodeRow struct {
	ID       uuid.UUID `db:"id"`
	CodeHash []byte    `db:"code_hash"`
}

type challengeRow struct {
	ID             uuid.UUID  `db:"id"`
	UserID         uuid.UUID  `db:"user_id"`
	ExpiresAt      time.Time  `db:"expires_at"`
	FailedAttempts int        `db:"failed_attempts"`
	ConsumedAt     *time.Time `db:"consumed_at"`
}

func (r *TwoFactorRepository) FindSetup(ctx context.Context, userID uuid.UUID) (*port.TwoFactorSetup, error) {
	const query = `
		SELECT user_id, secret_encrypted, enabled_at, last_used_step
		FROM user_two_factor
		WHERE user_id = $1`
	var row twoFactorSetupRow
	err := database.Executor(ctx, r.db).QueryRowxContext(ctx, query, userID).StructScan(&row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, apperror.Wrap(apperror.KindInternal, "failed to load two-factor setup", err)
	}
	return &port.TwoFactorSetup{
		UserID:          row.UserID,
		SecretEncrypted: row.SecretEncrypted,
		EnabledAt:       row.EnabledAt,
		LastUsedStep:    row.LastUsedStep,
	}, nil
}

// SaveSetup replaces a pending secret. The WHERE clause on the conflict branch refuses to touch an enabled
// setup, so a concurrent enrollment cannot overwrite a working second factor.
func (r *TwoFactorRepository) SaveSetup(ctx context.Context, userID uuid.UUID, secretEncrypted []byte) (bool, error) {
	const query = `
		INSERT INTO user_two_factor (user_id, secret_encrypted, enabled_at, last_used_step, created_at)
		VALUES ($1, $2, NULL, NULL, now())
		ON CONFLICT (user_id) DO UPDATE
		SET secret_encrypted = EXCLUDED.secret_encrypted,
		    enabled_at = NULL,
		    last_used_step = NULL,
		    created_at = now()
		WHERE user_two_factor.enabled_at IS NULL`
	res, err := database.Executor(ctx, r.db).ExecContext(ctx, query, userID, secretEncrypted)
	if err != nil {
		return false, apperror.Wrap(apperror.KindInternal, "failed to save two-factor setup", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return false, apperror.Wrap(apperror.KindInternal, "failed to save two-factor setup", err)
	}
	return n == 1, nil
}

func (r *TwoFactorRepository) MarkEnabled(ctx context.Context, userID uuid.UUID, at time.Time, step int64) (bool, error) {
	const query = `
		UPDATE user_two_factor
		SET enabled_at = $2, last_used_step = $3
		WHERE user_id = $1 AND enabled_at IS NULL`
	res, err := database.Executor(ctx, r.db).ExecContext(ctx, query, userID, at, step)
	if err != nil {
		return false, apperror.Wrap(apperror.KindInternal, "failed to enable two-factor", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return false, apperror.Wrap(apperror.KindInternal, "failed to enable two-factor", err)
	}
	return n == 1, nil
}

func (r *TwoFactorRepository) AdvanceLastUsedStep(ctx context.Context, userID uuid.UUID, step int64) (bool, error) {
	const query = `
		UPDATE user_two_factor
		SET last_used_step = $2
		WHERE user_id = $1 AND (last_used_step IS NULL OR last_used_step < $2)`
	res, err := database.Executor(ctx, r.db).ExecContext(ctx, query, userID, step)
	if err != nil {
		return false, apperror.Wrap(apperror.KindInternal, "failed to record two-factor step", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return false, apperror.Wrap(apperror.KindInternal, "failed to record two-factor step", err)
	}
	return n == 1, nil
}

func (r *TwoFactorRepository) ListUnusedRecoveryCodes(ctx context.Context, userID uuid.UUID) ([]port.RecoveryCodeRecord, error) {
	const query = `
		SELECT id, code_hash
		FROM user_recovery_codes
		WHERE user_id = $1 AND used_at IS NULL
		ORDER BY created_at, id`
	var rows []recoveryCodeRow
	if err := sqlx.SelectContext(ctx, database.Executor(ctx, r.db), &rows, query, userID); err != nil {
		return nil, apperror.Wrap(apperror.KindInternal, "failed to load recovery codes", err)
	}
	out := make([]port.RecoveryCodeRecord, 0, len(rows))
	for _, row := range rows {
		out = append(out, port.RecoveryCodeRecord{ID: row.ID, CodeHash: row.CodeHash})
	}
	return out, nil
}

func (r *TwoFactorRepository) CountUnusedRecoveryCodes(ctx context.Context, userID uuid.UUID) (int, error) {
	const query = `SELECT count(*) FROM user_recovery_codes WHERE user_id = $1 AND used_at IS NULL`
	var n int
	if err := database.Executor(ctx, r.db).QueryRowxContext(ctx, query, userID).Scan(&n); err != nil {
		return 0, apperror.Wrap(apperror.KindInternal, "failed to count recovery codes", err)
	}
	return n, nil
}

func (r *TwoFactorRepository) MarkRecoveryCodeUsed(ctx context.Context, userID, codeID uuid.UUID, at time.Time) (bool, error) {
	const query = `
		UPDATE user_recovery_codes
		SET used_at = $3
		WHERE id = $2 AND user_id = $1 AND used_at IS NULL`
	res, err := database.Executor(ctx, r.db).ExecContext(ctx, query, userID, codeID, at)
	if err != nil {
		return false, apperror.Wrap(apperror.KindInternal, "failed to consume recovery code", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return false, apperror.Wrap(apperror.KindInternal, "failed to consume recovery code", err)
	}
	return n == 1, nil
}

func (r *TwoFactorRepository) ReplaceRecoveryCodes(ctx context.Context, userID uuid.UUID, hashes [][]byte) error {
	ex := database.Executor(ctx, r.db)
	if _, err := ex.ExecContext(ctx, `DELETE FROM user_recovery_codes WHERE user_id = $1`, userID); err != nil {
		return apperror.Wrap(apperror.KindInternal, "failed to replace recovery codes", err)
	}
	const insert = `INSERT INTO user_recovery_codes (user_id, code_hash) VALUES ($1, $2)`
	for _, h := range hashes {
		if _, err := ex.ExecContext(ctx, insert, userID, h); err != nil {
			return apperror.Wrap(apperror.KindInternal, "failed to store recovery code", err)
		}
	}
	return nil
}

func (r *TwoFactorRepository) Delete(ctx context.Context, userID uuid.UUID) error {
	ex := database.Executor(ctx, r.db)
	if _, err := ex.ExecContext(ctx, `DELETE FROM user_recovery_codes WHERE user_id = $1`, userID); err != nil {
		return apperror.Wrap(apperror.KindInternal, "failed to remove recovery codes", err)
	}
	if _, err := ex.ExecContext(ctx, `DELETE FROM user_two_factor WHERE user_id = $1`, userID); err != nil {
		return apperror.Wrap(apperror.KindInternal, "failed to remove two-factor setup", err)
	}
	return nil
}

func (r *TwoFactorRepository) CreateChallenge(ctx context.Context, userID uuid.UUID, expiresAt time.Time) (uuid.UUID, error) {
	const query = `
		INSERT INTO user_two_factor_challenges (user_id, expires_at)
		VALUES ($1, $2)
		RETURNING id`
	var id uuid.UUID
	if err := database.Executor(ctx, r.db).QueryRowxContext(ctx, query, userID, expiresAt).Scan(&id); err != nil {
		return uuid.Nil, apperror.Wrap(apperror.KindInternal, "failed to create login challenge", err)
	}
	return id, nil
}

func (r *TwoFactorRepository) FindChallenge(ctx context.Context, challengeID uuid.UUID) (*port.LoginChallenge, error) {
	const query = `
		SELECT id, user_id, expires_at, failed_attempts, consumed_at
		FROM user_two_factor_challenges
		WHERE id = $1`
	var row challengeRow
	err := database.Executor(ctx, r.db).QueryRowxContext(ctx, query, challengeID).StructScan(&row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, apperror.Wrap(apperror.KindInternal, "failed to load login challenge", err)
	}
	return &port.LoginChallenge{
		ID:             row.ID,
		UserID:         row.UserID,
		ExpiresAt:      row.ExpiresAt,
		FailedAttempts: row.FailedAttempts,
		ConsumedAt:     row.ConsumedAt,
	}, nil
}

// RecordFailedChallengeAttempt increments the counter atomically and consumes the challenge when the limit
// is reached, so concurrent wrong codes cannot exceed the limit.
func (r *TwoFactorRepository) RecordFailedChallengeAttempt(ctx context.Context, challengeID uuid.UUID, maxAttempts int, at time.Time) error {
	const query = `
		UPDATE user_two_factor_challenges
		SET failed_attempts = failed_attempts + 1,
		    consumed_at = CASE WHEN failed_attempts + 1 >= $2 THEN $3 ELSE consumed_at END
		WHERE id = $1 AND consumed_at IS NULL`
	if _, err := database.Executor(ctx, r.db).ExecContext(ctx, query, challengeID, maxAttempts, at); err != nil {
		return apperror.Wrap(apperror.KindInternal, "failed to record login attempt", err)
	}
	return nil
}

func (r *TwoFactorRepository) ConsumeChallenge(ctx context.Context, challengeID uuid.UUID, at time.Time, maxAttempts int) (bool, error) {
	const query = `
		UPDATE user_two_factor_challenges
		SET consumed_at = $2
		WHERE id = $1 AND consumed_at IS NULL AND expires_at > $2 AND failed_attempts < $3`
	res, err := database.Executor(ctx, r.db).ExecContext(ctx, query, challengeID, at, maxAttempts)
	if err != nil {
		return false, apperror.Wrap(apperror.KindInternal, "failed to consume login challenge", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return false, apperror.Wrap(apperror.KindInternal, "failed to consume login challenge", err)
	}
	return n == 1, nil
}
