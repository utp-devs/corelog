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
	"github.com/jhonsferg/corelog/internal/user/domain"
)

// touchInterval limita las escrituras de last_seen_at: una peticion no escribe si la sesion se vio hace poco.
const touchInterval = 5 * time.Minute

type sessionRow struct {
	ID         uuid.UUID  `db:"id"`
	UserID     uuid.UUID  `db:"user_id"`
	UserAgent  string     `db:"user_agent"`
	CreatedAt  time.Time  `db:"created_at"`
	LastSeenAt time.Time  `db:"last_seen_at"`
	ExpiresAt  time.Time  `db:"expires_at"`
	RevokedAt  *time.Time `db:"revoked_at"`
}

func (r sessionRow) toDomain() *domain.Session {
	return &domain.Session{
		ID:         r.ID,
		UserID:     r.UserID,
		UserAgent:  r.UserAgent,
		CreatedAt:  r.CreatedAt,
		LastSeenAt: r.LastSeenAt,
		ExpiresAt:  r.ExpiresAt,
		RevokedAt:  r.RevokedAt,
	}
}

type SessionRepository struct {
	db *sqlx.DB
}

func NewSessionRepository(db *sqlx.DB) *SessionRepository {
	return &SessionRepository{db: db}
}

var _ port.SessionRepository = (*SessionRepository)(nil)

func (r *SessionRepository) Create(ctx context.Context, s *domain.Session) error {
	const query = `
		INSERT INTO user_sessions (id, user_id, user_agent, created_at, last_seen_at, expires_at)
		VALUES ($1, $2, $3, $4, $5, $6)`
	if _, err := r.db.ExecContext(ctx, query, s.ID, s.UserID, s.UserAgent, s.CreatedAt, s.LastSeenAt, s.ExpiresAt); err != nil {
		return apperror.Wrap(apperror.KindInternal, "failed to create session", err)
	}
	return nil
}

func (r *SessionRepository) ListActive(ctx context.Context, userID uuid.UUID, now time.Time) ([]*domain.Session, error) {
	const query = `
		SELECT id, user_id, user_agent, created_at, last_seen_at, expires_at, revoked_at
		FROM user_sessions
		WHERE user_id = $1 AND revoked_at IS NULL AND expires_at > $2
		ORDER BY last_seen_at DESC`
	var rows []sessionRow
	if err := r.db.SelectContext(ctx, &rows, query, userID, now); err != nil {
		return nil, apperror.Wrap(apperror.KindInternal, "failed to list sessions", err)
	}
	out := make([]*domain.Session, 0, len(rows))
	for _, row := range rows {
		out = append(out, row.toDomain())
	}
	return out, nil
}

// Revoke cierra una sesion del usuario. Una sesion ajena o inexistente responde no encontrada.
func (r *SessionRepository) Revoke(ctx context.Context, userID, sessionID uuid.UUID, now time.Time) error {
	result, err := r.db.ExecContext(ctx,
		`UPDATE user_sessions SET revoked_at = $3 WHERE id = $2 AND user_id = $1 AND revoked_at IS NULL`,
		userID, sessionID, now)
	if err != nil {
		return apperror.Wrap(apperror.KindInternal, "failed to revoke session", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return apperror.Wrap(apperror.KindInternal, "failed to revoke session", err)
	}
	if affected == 0 {
		return domain.ErrSessionNotFound
	}
	return nil
}

// RevokeExcept cierra todas las sesiones activas del usuario salvo keepID (nil = todas).
func (r *SessionRepository) RevokeExcept(ctx context.Context, userID uuid.UUID, keepID *uuid.UUID, now time.Time) error {
	const query = `
		UPDATE user_sessions SET revoked_at = $3
		WHERE user_id = $1 AND revoked_at IS NULL AND ($2::uuid IS NULL OR id <> $2::uuid)`
	if _, err := database.Executor(ctx, r.db).ExecContext(ctx, query, userID, keepID, now); err != nil {
		return apperror.Wrap(apperror.KindInternal, "failed to revoke sessions", err)
	}
	return nil
}

func (r *SessionRepository) Touch(ctx context.Context, sessionID uuid.UUID, now time.Time) error {
	const query = `
		UPDATE user_sessions SET last_seen_at = $2
		WHERE id = $1 AND last_seen_at < $2 - make_interval(secs => $3)`
	if _, err := r.db.ExecContext(ctx, query, sessionID, now, touchInterval.Seconds()); err != nil {
		return apperror.Wrap(apperror.KindInternal, "failed to touch session", err)
	}
	return nil
}

// Validate se llama en cada peticion autenticada. Un fallo de escritura en Touch no invalida la peticion.
func (r *SessionRepository) Validate(ctx context.Context, sessionID, userID uuid.UUID) error {
	var row struct {
		RevokedAt     *time.Time `db:"revoked_at"`
		ExpiresAt     time.Time  `db:"expires_at"`
		DeactivatedAt *time.Time `db:"deactivated_at"`
	}
	const query = `
		SELECT s.revoked_at, s.expires_at, u.deactivated_at
		FROM user_sessions s
		JOIN users u ON u.id = s.user_id
		WHERE s.id = $1 AND s.user_id = $2`
	if err := r.db.GetContext(ctx, &row, query, sessionID, userID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return apperror.New(apperror.KindUnauthorized, "session is no longer valid")
		}
		return apperror.Wrap(apperror.KindInternal, "failed to validate session", err)
	}

	now := time.Now().UTC()
	if row.RevokedAt != nil || !now.Before(row.ExpiresAt) || row.DeactivatedAt != nil {
		return apperror.New(apperror.KindUnauthorized, "session is no longer valid")
	}

	_ = r.Touch(ctx, sessionID, now)
	return nil
}
