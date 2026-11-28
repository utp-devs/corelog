package postgres

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"

	"github.com/jhonsferg/corelog/internal/notification/domain"
	"github.com/jhonsferg/corelog/internal/platform/apperror"
)

// FindByID loads one notification of the user. A missing or foreign row answers ErrNotificationNotFound.
func (r *NotificationRepository) FindByID(ctx context.Context, userID, id uuid.UUID) (*domain.Notification, error) {
	var row notificationRow
	err := r.db.GetContext(ctx, &row, `
		SELECT id, user_id, kind, message, link, read_at, created_at, data
		FROM notifications WHERE id = $1 AND user_id = $2`, id, userID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, domain.ErrNotificationNotFound
	}
	if err != nil {
		return nil, apperror.Wrap(apperror.KindInternal, "failed to load notification", err)
	}
	return row.toDomain(), nil
}

// ListAfter returns, oldest first, up to limit notifications of the user created after the one with id
// afterID. Login alerts of the session excludeSession are skipped (that session is the one that logged in).
// An unknown afterID returns nothing.
func (r *NotificationRepository) ListAfter(ctx context.Context, userID, afterID, excludeSession uuid.UUID, limit int) ([]*domain.Notification, error) {
	var rows []notificationRow
	err := r.db.SelectContext(ctx, &rows, `
		SELECT n.id, n.user_id, n.kind, n.message, n.link, n.read_at, n.created_at, n.data
		FROM notifications n
		JOIN notifications ref ON ref.id = $2 AND ref.user_id = $1
		WHERE n.user_id = $1 AND (n.created_at, n.id) > (ref.created_at, ref.id)
		  AND NOT (n.kind = $3 AND n.data ->> 'session_id' = $4)
		ORDER BY n.created_at, n.id
		LIMIT $5`, userID, afterID, domain.KindNewLogin, excludeSession.String(), limit)
	if err != nil {
		return nil, apperror.Wrap(apperror.KindInternal, "failed to list notifications", err)
	}
	items := make([]*domain.Notification, 0, len(rows))
	for _, row := range rows {
		items = append(items, row.toDomain())
	}
	return items, nil
}

// FingerprintRepository remembers which devices logged in to each account.
type FingerprintRepository struct {
	db *sqlx.DB
}

func NewFingerprintRepository(db *sqlx.DB) *FingerprintRepository {
	return &FingerprintRepository{db: db}
}

// Touch records a login of the device and reports when it was last seen before (nil when never) and whether
// the account had any device on record.
func (r *FingerprintRepository) Touch(ctx context.Context, userID uuid.UUID, fingerprint string, now time.Time) (lastSeen *time.Time, hasHistory bool, err error) {
	var row struct {
		PrevSeen   *time.Time `db:"prev_seen"`
		HasHistory bool       `db:"has_history"`
	}
	const query = `
		WITH prev AS (SELECT last_seen_at FROM login_fingerprints WHERE user_id = $1 AND fingerprint = $2),
		known AS (SELECT EXISTS (SELECT 1 FROM login_fingerprints WHERE user_id = $1) AS has),
		up AS (
			INSERT INTO login_fingerprints (user_id, fingerprint, first_seen_at, last_seen_at)
			VALUES ($1, $2, $3, $3)
			ON CONFLICT (user_id, fingerprint) DO UPDATE SET last_seen_at = EXCLUDED.last_seen_at
			RETURNING 1)
		SELECT (SELECT last_seen_at FROM prev) AS prev_seen, (SELECT has FROM known) AS has_history FROM up`
	if err := r.db.GetContext(ctx, &row, query, userID, fingerprint, now); err != nil {
		return nil, false, apperror.Wrap(apperror.KindInternal, "failed to record login device", err)
	}
	return row.PrevSeen, row.HasHistory, nil
}
