package postgres

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"

	"github.com/jhonsferg/corelog/internal/notification/application/port"
	"github.com/jhonsferg/corelog/internal/notification/domain"
	"github.com/jhonsferg/corelog/internal/platform/database"
)

// DeliveryRepository keeps the dedupe claims and the digest items.
type DeliveryRepository struct {
	db *sqlx.DB
}

// NewDeliveryRepository returns the repository.
func NewDeliveryRepository(db *sqlx.DB) *DeliveryRepository { return &DeliveryRepository{db: db} }

var _ port.DeliveryStore = (*DeliveryRepository)(nil)

func (r *DeliveryRepository) ext(ctx context.Context) sqlx.ExtContext {
	return database.Executor(ctx, r.db)
}

func (r *DeliveryRepository) Claim(ctx context.Context, key string, userID uuid.UUID, now time.Time) (bool, error) {
	res, err := r.ext(ctx).ExecContext(ctx, `
		INSERT INTO notification_deliveries (dedup_key, user_id, created_at) VALUES ($1, $2, $3) ON CONFLICT DO NOTHING`, key, userID, now)
	if err != nil {
		return false, internal("no se pudo registrar la entrega", err)
	}
	n, _ := res.RowsAffected()
	return n > 0, nil
}

func (r *DeliveryRepository) AddDigestItem(ctx context.Context, item port.DigestItem) error {
	if _, err := r.ext(ctx).ExecContext(ctx, `
		INSERT INTO notification_digest_items (user_id, event, issue_key, summary, actor_name, link, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)`,
		item.UserID, string(item.Event), item.IssueKey, item.Summary, domain.Truncate(item.ActorName, 120), domain.Truncate(item.Link, 300), item.CreatedAt); err != nil {
		return internal("no se pudo guardar el aviso del resumen", err)
	}
	return nil
}

func (r *DeliveryRepository) UsersWithDigest(ctx context.Context, limit int) ([]uuid.UUID, error) {
	var ids []uuid.UUID
	if err := sqlx.SelectContext(ctx, r.ext(ctx), &ids, `
		SELECT user_id FROM notification_digest_items WHERE digested_at IS NULL GROUP BY user_id ORDER BY min(id) LIMIT $1`, limit); err != nil {
		return nil, internal("no se pudieron leer los resumenes pendientes", err)
	}
	return ids, nil
}

func (r *DeliveryRepository) DigestItems(ctx context.Context, userID uuid.UUID, limit int) ([]port.DigestItem, error) {
	var rows []struct {
		ID        int64     `db:"id"`
		Event     string    `db:"event"`
		IssueKey  string    `db:"issue_key"`
		Summary   string    `db:"summary"`
		ActorName string    `db:"actor_name"`
		Link      string    `db:"link"`
		CreatedAt time.Time `db:"created_at"`
	}
	if err := sqlx.SelectContext(ctx, r.ext(ctx), &rows, `
		SELECT id, event, issue_key, summary, actor_name, link, created_at FROM notification_digest_items
		WHERE user_id = $1 AND digested_at IS NULL ORDER BY id LIMIT $2`, userID, limit); err != nil {
		return nil, internal("no se pudieron leer los avisos del resumen", err)
	}
	out := make([]port.DigestItem, 0, len(rows))
	for _, row := range rows {
		out = append(out, port.DigestItem{ID: row.ID, UserID: userID, Event: domain.Event(row.Event), IssueKey: row.IssueKey,
			Summary: row.Summary, ActorName: row.ActorName, Link: row.Link, CreatedAt: row.CreatedAt})
	}
	return out, nil
}

func (r *DeliveryRepository) MarkDigested(ctx context.Context, ids []int64, at time.Time) error {
	for start := 0; start < len(ids); start += 500 {
		end := min(start+500, len(ids))
		query, args, err := sqlx.In(`UPDATE notification_digest_items SET digested_at = ? WHERE id IN (?)`, at, ids[start:end])
		if err != nil {
			return internal("no se pudo cerrar el resumen", err)
		}
		if _, err := r.ext(ctx).ExecContext(ctx, sqlx.Rebind(sqlx.DOLLAR, query), args...); err != nil {
			return internal("no se pudo cerrar el resumen", err)
		}
	}
	return nil
}

func (r *DeliveryRepository) PurgeDeliveries(ctx context.Context, before time.Time) (int64, error) {
	res, err := r.ext(ctx).ExecContext(ctx, `DELETE FROM notification_deliveries WHERE created_at < $1`, before)
	if err != nil {
		return 0, internal("no se pudieron limpiar las entregas", err)
	}
	n, _ := res.RowsAffected()
	return n, nil
}
