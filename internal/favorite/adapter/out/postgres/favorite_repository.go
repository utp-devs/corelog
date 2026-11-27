package postgres

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"

	"github.com/jhonsferg/corelog/internal/favorite/application/port"
	"github.com/jhonsferg/corelog/internal/favorite/domain"
	"github.com/jhonsferg/corelog/internal/platform/apperror"
	"github.com/jhonsferg/corelog/internal/platform/database"
)

// Repository stores favorites and recent items in PostgreSQL (tables favorites and recent_items).
type Repository struct {
	db *sqlx.DB
}

var _ port.Repository = (*Repository)(nil)

func NewRepository(db *sqlx.DB) *Repository {
	return &Repository{db: db}
}

// entryRow is the joined display data of one entity. ViewedAt is only set for recents.
type entryRow struct {
	EntityID  uuid.UUID  `db:"entity_id"`
	Name      string     `db:"name"`
	Key       string     `db:"key"`
	Color     string     `db:"color"`
	CreatedAt time.Time  `db:"created_at"`
	ViewedAt  *time.Time `db:"viewed_at"`
}

// favoriteProjectsQuery joins starred projects with their display data. Deleted projects are skipped.
const favoriteProjectsQuery = `
	SELECT p.id AS entity_id, p.name, p.key, COALESCE(p.color, '') AS color,
	       f.created_at, NULL::timestamptz AS viewed_at
	FROM favorites f
	JOIN projects p ON p.id = f.entity_id
	WHERE f.user_id = $1 AND f.entity_type = $2 AND p.deleted_at IS NULL
	ORDER BY f.created_at DESC, p.id ASC`

// recentProjectsQuery joins recent views with project display data. Deleted projects are skipped.
const recentProjectsQuery = `
	SELECT p.id AS entity_id, p.name, p.key, COALESCE(p.color, '') AS color,
	       p.created_at, r.viewed_at
	FROM recent_items r
	JOIN projects p ON p.id = r.entity_id
	WHERE r.user_id = $1 AND r.entity_type = $2 AND p.deleted_at IS NULL
	ORDER BY r.viewed_at DESC, p.id ASC
	LIMIT $3`

// supportsDisplay reports whether the entity type has a display query. Other types resolve to no rows.
func supportsDisplay(entityType domain.EntityType) bool {
	return entityType == domain.EntityProject
}

func (r *Repository) AddFavorite(ctx context.Context, favorite domain.Favorite) error {
	if _, err := database.Executor(ctx, r.db).ExecContext(ctx, `
		INSERT INTO favorites (user_id, entity_type, entity_id, created_at)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (user_id, entity_type, entity_id) DO NOTHING`,
		favorite.UserID, string(favorite.Type), favorite.EntityID, favorite.CreatedAt,
	); err != nil {
		return apperror.Wrap(apperror.KindInternal, "failed to save favorite", err)
	}
	return nil
}

func (r *Repository) RemoveFavorite(ctx context.Context, userID uuid.UUID, entityType domain.EntityType, entityID uuid.UUID) error {
	if _, err := database.Executor(ctx, r.db).ExecContext(ctx, `
		DELETE FROM favorites WHERE user_id = $1 AND entity_type = $2 AND entity_id = $3`,
		userID, string(entityType), entityID,
	); err != nil {
		return apperror.Wrap(apperror.KindInternal, "failed to remove favorite", err)
	}
	return nil
}

func (r *Repository) ListFavorites(ctx context.Context, userID uuid.UUID, entityType domain.EntityType) ([]port.Entry, error) {
	if !supportsDisplay(entityType) {
		return []port.Entry{}, nil
	}
	var rows []entryRow
	if err := sqlx.SelectContext(ctx, database.Executor(ctx, r.db), &rows, favoriteProjectsQuery, userID, string(entityType)); err != nil {
		return nil, apperror.Wrap(apperror.KindInternal, "failed to list favorites", err)
	}
	return toEntries(entityType, rows), nil
}

// TouchRecent upserts the view time and deletes the entries beyond the retention window, in one transaction.
// The window keeps the most recent views; ties are broken by entity id so the result is deterministic.
func (r *Repository) TouchRecent(ctx context.Context, item domain.RecentItem) error {
	return r.inTx(ctx, func(q sqlx.ExtContext) error {
		if _, err := q.ExecContext(ctx, `
			INSERT INTO recent_items (user_id, entity_type, entity_id, viewed_at)
			VALUES ($1, $2, $3, $4)
			ON CONFLICT (user_id, entity_type, entity_id) DO UPDATE SET viewed_at = EXCLUDED.viewed_at`,
			item.UserID, string(item.Type), item.EntityID, item.ViewedAt,
		); err != nil {
			return apperror.Wrap(apperror.KindInternal, "failed to record recent item", err)
		}
		if _, err := q.ExecContext(ctx, `
			DELETE FROM recent_items
			WHERE user_id = $1 AND entity_type = $2
			  AND entity_id NOT IN (
			      SELECT entity_id FROM recent_items
			      WHERE user_id = $1 AND entity_type = $2
			      ORDER BY viewed_at DESC, entity_id ASC
			      LIMIT $3)`,
			item.UserID, string(item.Type), domain.RecentRetention,
		); err != nil {
			return apperror.Wrap(apperror.KindInternal, "failed to prune recent items", err)
		}
		return nil
	})
}

func (r *Repository) ListRecent(ctx context.Context, userID uuid.UUID, entityType domain.EntityType, limit int) ([]port.Entry, error) {
	if !supportsDisplay(entityType) {
		return []port.Entry{}, nil
	}
	var rows []entryRow
	if err := sqlx.SelectContext(ctx, database.Executor(ctx, r.db), &rows, recentProjectsQuery, userID, string(entityType), limit); err != nil {
		return nil, apperror.Wrap(apperror.KindInternal, "failed to list recent items", err)
	}
	return toEntries(entityType, rows), nil
}

// inTx runs fn inside the transaction bound to ctx, or inside a new one when there is none.
func (r *Repository) inTx(ctx context.Context, fn func(q sqlx.ExtContext) error) error {
	if tx, ok := database.TxFromContext(ctx); ok {
		return fn(tx)
	}
	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return apperror.Wrap(apperror.KindInternal, "failed to start transaction", err)
	}
	if err := fn(tx); err != nil {
		_ = tx.Rollback()
		return err
	}
	if err := tx.Commit(); err != nil {
		return apperror.Wrap(apperror.KindInternal, "failed to commit recent item", err)
	}
	return nil
}

func toEntries(entityType domain.EntityType, rows []entryRow) []port.Entry {
	entries := make([]port.Entry, 0, len(rows))
	for _, row := range rows {
		entries = append(entries, port.Entry{
			Type: entityType, ID: row.EntityID, Name: row.Name, Key: row.Key, Color: row.Color,
			CreatedAt: row.CreatedAt, ViewedAt: row.ViewedAt,
		})
	}
	return entries
}
