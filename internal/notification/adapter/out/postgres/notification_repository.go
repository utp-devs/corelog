package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"sync"
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
	Data      []byte     `db:"data"`
}

func (r notificationRow) toDomain() *domain.Notification {
	n := &domain.Notification{ID: r.ID, UserID: r.UserID, Kind: r.Kind, Message: r.Message, Link: r.Link, ReadAt: r.ReadAt, CreatedAt: r.CreatedAt}
	if len(r.Data) > 0 {
		var data map[string]any
		if json.Unmarshal(r.Data, &data) == nil && len(data) > 0 {
			n.Data = data
		}
	}
	return n
}

// SaveHook runs after a notification row is inserted, inside the transaction of the insert when there is one.
// It is the single choke point that pushes new notifications to the live streams.
type SaveHook func(ctx context.Context, n *domain.Notification)

var saveHooks sync.Map // *sqlx.DB -> SaveHook

// SetSaveHook registers the hook for every repository built over db (several are built per process: the
// service, the scheme engine and the jobs). It is called once at startup; nil removes it.
func SetSaveHook(db *sqlx.DB, h SaveHook) {
	if h == nil {
		saveHooks.Delete(db)
		return
	}
	saveHooks.Store(db, h)
}

func (r *NotificationRepository) Save(ctx context.Context, n *domain.Notification) error {
	const query = `
		INSERT INTO notifications (user_id, kind, message, link, created_at, data)
		VALUES (:user_id, :kind, :message, :link, :created_at, CAST(:data AS jsonb))
		RETURNING id`
	data := []byte("{}")
	if len(n.Data) > 0 {
		encoded, err := json.Marshal(n.Data)
		if err != nil {
			return apperror.Wrap(apperror.KindInternal, "failed to encode notification data", err)
		}
		data = encoded
	}
	row := notificationRow{UserID: n.UserID, Kind: n.Kind, Message: n.Message, Link: n.Link, CreatedAt: n.CreatedAt, Data: data}
	id, err := database.InsertReturningID(ctx, database.Executor(ctx, r.db), query, row)
	if err != nil {
		return apperror.Wrap(apperror.KindInternal, "failed to save notification", err)
	}
	n.ID = id
	if hook, ok := saveHooks.Load(r.db); ok {
		hook.(SaveHook)(ctx, n)
	}
	return nil
}

func (r *NotificationRepository) ListByUser(ctx context.Context, userID uuid.UUID, limit int) ([]*domain.Notification, error) {
	var rows []notificationRow
	if err := r.db.SelectContext(ctx, &rows, `
		SELECT id, user_id, kind, message, link, read_at, created_at, data
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
	var row struct {
		InApp            bool   `db:"in_app"`
		EmailMode        string `db:"email_mode"`
		NotifyOwnActions bool   `db:"notify_own_actions"`
	}
	err := sqlx.GetContext(ctx, database.Executor(ctx, r.db), &row, `SELECT in_app, email_mode, notify_own_actions FROM notification_preferences WHERE user_id = $1`, userID)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.DefaultPreferences(), nil
	}
	if err != nil {
		return domain.Preferences{}, apperror.Wrap(apperror.KindInternal, "failed to load notification preferences", err)
	}
	return domain.Preferences{InApp: row.InApp, Email: domain.EmailMode(row.EmailMode), NotifyOwnActions: row.NotifyOwnActions}, nil
}

func (r *NotificationRepository) SavePreferences(ctx context.Context, userID uuid.UUID, prefs domain.Preferences) error {
	mode := prefs.Email
	if mode == "" {
		mode = domain.EmailImmediate
	}
	const query = `
		INSERT INTO notification_preferences (user_id, in_app, email_mode, notify_own_actions, updated_at)
		VALUES ($1, $2, $3, $4, now())
		ON CONFLICT (user_id) DO UPDATE SET in_app = EXCLUDED.in_app, email_mode = EXCLUDED.email_mode,
			notify_own_actions = EXCLUDED.notify_own_actions, updated_at = now()`
	if _, err := database.Executor(ctx, r.db).ExecContext(ctx, query, userID, prefs.InApp, string(mode), prefs.NotifyOwnActions); err != nil {
		return apperror.Wrap(apperror.KindInternal, "failed to save notification preferences", err)
	}
	return nil
}
