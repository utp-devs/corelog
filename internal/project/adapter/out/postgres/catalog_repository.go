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
	"github.com/jhonsferg/corelog/internal/project/application/port"
	"github.com/jhonsferg/corelog/internal/project/domain"
)

type labelRow struct {
	ID        uuid.UUID `db:"id"`
	ProjectID uuid.UUID `db:"project_id"`
	Name      string    `db:"name"`
	Color     string    `db:"color"`
	CreatedAt time.Time `db:"created_at"`
	UpdatedAt time.Time `db:"updated_at"`
}

type CatalogRepository struct {
	db *sqlx.DB
}

func NewCatalogRepository(db *sqlx.DB) *CatalogRepository {
	return &CatalogRepository{db: db}
}

var _ port.CatalogRepository = (*CatalogRepository)(nil)

type labelUsageRow struct {
	labelRow
	UsageCount int `db:"usage_count"`
}

// ListLabels returns the labels of the project with the number of live issues that carry each one.
func (r *CatalogRepository) ListLabels(ctx context.Context, projectID uuid.UUID) ([]domain.Label, error) {
	var rows []labelUsageRow
	if err := r.db.SelectContext(ctx, &rows, `
		SELECT l.id, l.project_id, l.name, l.color, l.created_at, l.updated_at,
		       COUNT(i.id) AS usage_count
		FROM labels l
		LEFT JOIN issue_labels il ON il.label_id = l.id
		LEFT JOIN issues i ON i.id = il.issue_id AND i.deleted_at IS NULL
		WHERE l.project_id = $1 AND l.deleted_at IS NULL
		GROUP BY l.id
		ORDER BY l.name ASC`, projectID); err != nil {
		return nil, apperror.Wrap(apperror.KindInternal, "failed to list labels", err)
	}
	labels := make([]domain.Label, 0, len(rows))
	for _, row := range rows {
		labels = append(labels, domain.Label{
			ID: row.ID, ProjectID: row.ProjectID, Name: row.Name, Color: row.Color,
			UsageCount: row.UsageCount, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
		})
	}
	return labels, nil
}

// FindLabel returns one label of the project or a not found error.
func (r *CatalogRepository) FindLabel(ctx context.Context, projectID, labelID uuid.UUID) (*domain.Label, error) {
	var row labelRow
	if err := r.db.GetContext(ctx, &row, `
		SELECT id, project_id, name, color, created_at, updated_at
		FROM labels WHERE project_id = $1 AND id = $2 AND deleted_at IS NULL`, projectID, labelID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, apperror.New(apperror.KindNotFound, "label not found")
		}
		return nil, apperror.Wrap(apperror.KindInternal, "failed to find label", err)
	}
	return &domain.Label{
		ID: row.ID, ProjectID: row.ProjectID, Name: row.Name, Color: row.Color,
		CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
	}, nil
}

// UpdateLabel saves the name and color of a label of the project. The database sets updated_at and the
// saved timestamp is copied back into the label.
func (r *CatalogRepository) UpdateLabel(ctx context.Context, label *domain.Label) error {
	err := r.db.QueryRowContext(ctx, `
		UPDATE labels SET name = $1, color = $2, updated_at = now()
		WHERE project_id = $3 AND id = $4 AND deleted_at IS NULL
		RETURNING updated_at`,
		label.Name, label.Color, label.ProjectID, label.ID).Scan(&label.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return apperror.New(apperror.KindNotFound, "label not found")
	}
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return apperror.New(apperror.KindConflict, "a label with this name already exists")
		}
		return apperror.Wrap(apperror.KindInternal, "failed to update label", err)
	}
	return nil
}

func (r *CatalogRepository) CreateLabel(ctx context.Context, label *domain.Label) error {
	id, err := database.InsertReturningID(ctx, r.db, `
		INSERT INTO labels (project_id, name, color, created_at, updated_at)
		VALUES (:project_id, :name, :color, :created_at, :updated_at)
		ON CONFLICT (project_id, name) WHERE deleted_at IS NULL DO NOTHING
		RETURNING id`, map[string]interface{}{
		"project_id": label.ProjectID, "name": label.Name, "color": label.Color,
		"created_at": label.CreatedAt, "updated_at": label.UpdatedAt,
	})
	if errors.Is(err, sql.ErrNoRows) {
		return apperror.New(apperror.KindConflict, "a label with this name already exists")
	}
	if err != nil {
		return apperror.Wrap(apperror.KindInternal, "failed to create label", err)
	}
	label.ID = id
	return nil
}

func (r *CatalogRepository) DeleteLabel(ctx context.Context, projectID, labelID uuid.UUID) error {
	result, err := r.db.ExecContext(ctx, `UPDATE labels SET deleted_at = now() WHERE project_id = $1 AND id = $2 AND deleted_at IS NULL`, projectID, labelID)
	if err != nil {
		return apperror.Wrap(apperror.KindInternal, "failed to delete label", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return apperror.Wrap(apperror.KindInternal, "failed to delete label", err)
	}
	if affected == 0 {
		return apperror.New(apperror.KindNotFound, "label not found")
	}
	return nil
}
