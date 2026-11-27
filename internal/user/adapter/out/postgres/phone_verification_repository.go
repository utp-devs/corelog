package postgres

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jmoiron/sqlx"

	"github.com/jhonsferg/corelog/internal/platform/apperror"
	"github.com/jhonsferg/corelog/internal/platform/database"
	"github.com/jhonsferg/corelog/internal/user/application/port"
	"github.com/jhonsferg/corelog/internal/user/domain"
)

const (
	uniqueViolationCode = "23505"
	verifiedPhoneIndex  = "uq_users_phone_e164_verified"
)

// isPhoneTaken reports whether err is the violation of the verified-phone unique index.
func isPhoneTaken(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == uniqueViolationCode && pgErr.ConstraintName == verifiedPhoneIndex
}

// PhoneVerificationRepository stores SMS verification codes (hashed) in user_phone_verifications.
type PhoneVerificationRepository struct {
	db *sqlx.DB
}

func NewPhoneVerificationRepository(db *sqlx.DB) *PhoneVerificationRepository {
	return &PhoneVerificationRepository{db: db}
}

var _ port.PhoneVerificationRepository = (*PhoneVerificationRepository)(nil)

func (r *PhoneVerificationRepository) CreateWithinLimit(ctx context.Context, v *domain.PhoneVerification, since time.Time, max int) error {
	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return apperror.Wrap(apperror.KindInternal, "failed to store verification", err)
	}
	defer func() { _ = tx.Rollback() }()

	// Serialize the requests of one user so the count and the insert cannot interleave.
	if _, err := tx.ExecContext(ctx, `SELECT 1 FROM users WHERE id = $1 FOR UPDATE`, v.UserID); err != nil {
		return apperror.Wrap(apperror.KindInternal, "failed to store verification", err)
	}
	var sent int
	if err := tx.GetContext(ctx, &sent,
		`SELECT count(*) FROM user_phone_verifications WHERE user_id = $1 AND created_at > $2`, v.UserID, since); err != nil {
		return apperror.Wrap(apperror.KindInternal, "failed to store verification", err)
	}
	if sent >= max {
		return domain.ErrPhoneSendLimit
	}
	if err := tx.GetContext(ctx, &v.ID, `
		INSERT INTO user_phone_verifications (user_id, phone_e164, code_hash, expires_at, created_at)
		VALUES ($1, $2, $3, $4, $5) RETURNING id`,
		v.UserID, v.E164, v.CodeHash, v.ExpiresAt, v.CreatedAt); err != nil {
		return apperror.Wrap(apperror.KindInternal, "failed to store verification", err)
	}
	if err := tx.Commit(); err != nil {
		return apperror.Wrap(apperror.KindInternal, "failed to store verification", err)
	}
	return nil
}

func (r *PhoneVerificationRepository) FindLatestOpen(ctx context.Context, userID uuid.UUID, e164 string) (*domain.PhoneVerification, error) {
	var v domain.PhoneVerification
	err := r.db.QueryRowxContext(ctx, `
		SELECT id, user_id, phone_e164, code_hash, attempts, expires_at, consumed_at, created_at
		FROM user_phone_verifications
		WHERE user_id = $1 AND phone_e164 = $2 AND consumed_at IS NULL
		ORDER BY created_at DESC LIMIT 1`, userID, e164).
		Scan(&v.ID, &v.UserID, &v.E164, &v.CodeHash, &v.Attempts, &v.ExpiresAt, &v.ConsumedAt, &v.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, domain.ErrPhoneCodeExpired
	}
	if err != nil {
		return nil, apperror.Wrap(apperror.KindInternal, "failed to load verification", err)
	}
	return &v, nil
}

func (r *PhoneVerificationRepository) RegisterAttempt(ctx context.Context, id uuid.UUID) (int, error) {
	var attempts int
	err := r.db.GetContext(ctx, &attempts,
		`UPDATE user_phone_verifications SET attempts = attempts + 1 WHERE id = $1 AND consumed_at IS NULL RETURNING attempts`, id)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, domain.ErrPhoneCodeExpired
	}
	if err != nil {
		return 0, apperror.Wrap(apperror.KindInternal, "failed to register attempt", err)
	}
	return attempts, nil
}

func (r *PhoneVerificationRepository) Consume(ctx context.Context, userID, _ uuid.UUID, at time.Time) error {
	// Every open code of the user dies with the successful one (id is the code that matched).
	if _, err := database.Executor(ctx, r.db).ExecContext(ctx,
		`UPDATE user_phone_verifications SET consumed_at = $2 WHERE user_id = $1 AND consumed_at IS NULL`,
		userID, at); err != nil {
		return apperror.Wrap(apperror.KindInternal, "failed to consume verification", err)
	}
	return nil
}
