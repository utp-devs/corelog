package postgres

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"

	"github.com/jhonsferg/corelog/internal/notification/application/port"
	"github.com/jhonsferg/corelog/internal/notification/domain"
	"github.com/jhonsferg/corelog/internal/platform/apperror"
	"github.com/jhonsferg/corelog/internal/platform/database"
)

// NotificationRepository guarda notificaciones en PostgreSQL. Toda lectura y marca filtra por usuario.
type NotificationRepository struct {
	db *sqlx.DB
}

func NewNotificationRepository(db *sqlx.DB) *NotificationRepository {
	return &NotificationRepository{db: db}
}

var _ port.Repository = (*NotificationRepository)(nil)

type notificationRow struct {
	ID        uuid.UUID  `db:"id"`
	UserID    uuid.UUID  `db:"user_id"`
	Kind      string     `db:"kind"`
	Message   string     `db:"message"`
	Link      string     `db:"link"`
	ReadAt    *time.Time `db:"read_at"`
	CreatedAt time.Time  `db:"created_at"`
}

func (r notificationRow) toDomain() *domain.Notification {
	return &domain.Notification{ID: r.ID, UserID: r.UserID, Kind: r.Kind, Message: r.Message, Link: r.Link, ReadAt: r.ReadAt, CreatedAt: r.CreatedAt}
}

func (r *NotificationRepository) Save(ctx context.Context, n *domain.Notification) error {
	const query = `
		INSERT INTO notifications (user_id, kind, message, link, created_at)
		VALUES (:user_id, :kind, :message, :link, :created_at)
		RETURNING id`
	row := notificationRow{UserID: n.UserID, Kind: n.Kind, Message: n.Message, Link: n.Link, CreatedAt: n.CreatedAt}
	id, err := database.InsertReturningID(ctx, r.db, query, row)
	if err != nil {
		return apperror.Wrap(apperror.KindInternal, "failed to save notification", err)
	}
	n.ID = id
	return nil
}

func (r *NotificationRepository) ListByUser(ctx context.Context, userID uuid.UUID, limit int) ([]*domain.Notification, error) {
	var rows []notificationRow
	if err := r.db.SelectContext(ctx, &rows, `
		SELECT id, user_id, kind, message, link, read_at, created_at
		FROM notifications WHERE user_id = $1
		ORDER BY created_at DESC, id DESC LIMIT $2`, userID, limit); err != nil {
		return nil, apperror.Wrap(apperror.KindInternal, "failed to list notifications", err)
	}
	items := make([]*domain.Notification, 0, len(rows))
	for _, row := range rows {
		items = append(items, row.toDomain())
	}
	return items, nil
}

func (r *NotificationRepository) UnreadCount(ctx context.Context, userID uuid.UUID) (int, error) {
	var count int
	if err := r.db.GetContext(ctx, &count, `SELECT COUNT(1) FROM notifications WHERE user_id = $1 AND read_at IS NULL`, userID); err != nil {
		return 0, apperror.Wrap(apperror.KindInternal, "failed to count notifications", err)
	}
	return count, nil
}

func (r *NotificationRepository) MarkRead(ctx context.Context, userID, id uuid.UUID) error {
	result, err := r.db.ExecContext(ctx, `
		UPDATE notifications SET read_at = now()
		WHERE id = $1 AND user_id = $2 AND read_at IS NULL`, id, userID)
	if err != nil {
		return apperror.Wrap(apperror.KindInternal, "failed to mark notification", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return apperror.Wrap(apperror.KindInternal, "failed to mark notification", err)
	}
	if affected == 0 {
		// Puede estar ya leida; distinguimos "no es suya o no existe" de "ya estaba leida".
		var exists bool
		if err := r.db.GetContext(ctx, &exists, `SELECT EXISTS (SELECT 1 FROM notifications WHERE id = $1 AND user_id = $2)`, id, userID); err != nil {
			return apperror.Wrap(apperror.KindInternal, "failed to mark notification", err)
		}
		if !exists {
			return domain.ErrNotificationNotFound
		}
	}
	return nil
}

func (r *NotificationRepository) MarkAllRead(ctx context.Context, userID uuid.UUID) error {
	if _, err := r.db.ExecContext(ctx, `UPDATE notifications SET read_at = now() WHERE user_id = $1 AND read_at IS NULL`, userID); err != nil {
		return apperror.Wrap(apperror.KindInternal, "failed to mark notifications", err)
	}
	return nil
}

// GetPreferences devuelve los ajustes del usuario; sin fila aplican los valores por defecto.
func (r *NotificationRepository) GetPreferences(ctx context.Context, userID uuid.UUID) (domain.Preferences, error) {
	var inApp bool
	err := r.db.GetContext(ctx, &inApp, `SELECT in_app FROM notification_preferences WHERE user_id = $1`, userID)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.DefaultPreferences(), nil
	}
	if err != nil {
		return domain.Preferences{}, apperror.Wrap(apperror.KindInternal, "failed to load notification preferences", err)
	}
	return domain.Preferences{InApp: inApp}, nil
}

func (r *NotificationRepository) SavePreferences(ctx context.Context, userID uuid.UUID, prefs domain.Preferences) error {
	const query = `
		INSERT INTO notification_preferences (user_id, in_app, updated_at)
		VALUES ($1, $2, now())
		ON CONFLICT (user_id) DO UPDATE SET in_app = EXCLUDED.in_app, updated_at = now()`
	if _, err := r.db.ExecContext(ctx, query, userID, prefs.InApp); err != nil {
		return apperror.Wrap(apperror.KindInternal, "failed to save notification preferences", err)
	}
	return nil
}
