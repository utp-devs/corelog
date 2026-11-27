package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jhonsferg/corelog/internal/issue/application/port"
	"github.com/jhonsferg/corelog/internal/issue/domain"
	"github.com/jhonsferg/corelog/internal/platform/apperror"
	"github.com/jmoiron/sqlx"
)

type SavedFilterRepository struct{ db *sqlx.DB }

func NewSavedFilterRepository(db *sqlx.DB) *SavedFilterRepository {
	return &SavedFilterRepository{db: db}
}

var _ port.SavedFilterRepository = (*SavedFilterRepository)(nil)

type savedFilterRow struct {
	ID        uuid.UUID `db:"id"`
	OwnerID   uuid.UUID `db:"owner_id"`
	Name      string    `db:"name"`
	Filters   []byte    `db:"filters"`
	CreatedAt time.Time `db:"created_at"`
	UpdatedAt time.Time `db:"updated_at"`
}

func (r *SavedFilterRepository) SaveSavedFilter(ctx context.Context, filter *domain.SavedFilter) error {
	criteria, err := json.Marshal(filter.Criteria)
	if err != nil {
		return apperror.Wrap(apperror.KindInternal, "failed to encode saved filter", err)
	}
	err = r.db.QueryRowContext(ctx, `
		INSERT INTO saved_filters(owner_id, name, filters, created_at, updated_at)
		VALUES($1, $2, $3::jsonb, $4, $5)
		RETURNING id`, filter.OwnerID, filter.Name, string(criteria), filter.CreatedAt, filter.UpdatedAt).Scan(&filter.ID)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return apperror.New(apperror.KindConflict, "a saved filter with this name already exists")
		}
		return apperror.Wrap(apperror.KindInternal, "failed to save filter", err)
	}
	return nil
}

func (r *SavedFilterRepository) ListSavedFilters(ctx context.Context, ownerID uuid.UUID) ([]*domain.SavedFilter, error) {
	var rows []savedFilterRow
	if err := r.db.SelectContext(ctx, &rows, `
		SELECT id, owner_id, name, filters, created_at, updated_at
		FROM saved_filters WHERE owner_id=$1 ORDER BY name, id`, ownerID); err != nil {
		return nil, apperror.Wrap(apperror.KindInternal, "failed to list saved filters", err)
	}
	out := make([]*domain.SavedFilter, 0, len(rows))
	for _, row := range rows {
		var criteria domain.SavedFilterCriteria
		if err := json.Unmarshal(row.Filters, &criteria); err != nil {
			return nil, apperror.Wrap(apperror.KindInternal, "failed to decode saved filter", err)
		}
		out = append(out, &domain.SavedFilter{ID: row.ID, OwnerID: row.OwnerID, Name: row.Name, Criteria: criteria, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt})
	}
	return out, nil
}

func (r *SavedFilterRepository) DeleteSavedFilter(ctx context.Context, ownerID, filterID uuid.UUID) error {
	result, err := r.db.ExecContext(ctx, `DELETE FROM saved_filters WHERE owner_id=$1 AND id=$2`, ownerID, filterID)
	if err != nil {
		return apperror.Wrap(apperror.KindInternal, "failed to delete saved filter", err)
	}
	count, err := result.RowsAffected()
	if err != nil {
		return apperror.Wrap(apperror.KindInternal, "failed to delete saved filter", err)
	}
	if count == 0 {
		return apperror.New(apperror.KindNotFound, "saved filter not found")
	}
	return nil
}
