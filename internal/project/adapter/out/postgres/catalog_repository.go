package postgres

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"

	"github.com/jhonsferg/corelog/internal/platform/apperror"
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

type componentRow struct {
	ID          uuid.UUID `db:"id"`
	ProjectID   uuid.UUID `db:"project_id"`
	Name        string    `db:"name"`
	Description string    `db:"description"`
	CreatedAt   time.Time `db:"created_at"`
	UpdatedAt   time.Time `db:"updated_at"`
}

type CatalogRepository struct {
	db *sqlx.DB
}

func NewCatalogRepository(db *sqlx.DB) *CatalogRepository {
	return &CatalogRepository{db: db}
}

var _ port.CatalogRepository = (*CatalogRepository)(nil)

func (r *CatalogRepository) ListLabels(ctx context.Context, projectID uuid.UUID) ([]domain.Label, error) {
	var rows []labelRow
	if err := r.db.SelectContext(ctx, &rows, `
		SELECT id, project_id, name, color, created_at, updated_at
		FROM labels WHERE project_id = $1 ORDER BY name ASC`, projectID); err != nil {
		return nil, apperror.Wrap(apperror.KindInternal, "failed to list labels", err)
	}
	labels := make([]domain.Label, 0, len(rows))
	for _, row := range rows {
		labels = append(labels, domain.Label{
			ID: row.ID, ProjectID: row.ProjectID, Name: row.Name, Color: row.Color,
			CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
		})
	}
	return labels, nil
}

func (r *CatalogRepository) CreateLabel(ctx context.Context, label *domain.Label) error {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO labels (id, project_id, name, color, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (project_id, name) DO NOTHING`,
		label.ID, label.ProjectID, label.Name, label.Color, label.CreatedAt, label.UpdatedAt)
	if err != nil {
		return apperror.Wrap(apperror.KindInternal, "failed to create label", err)
	}
	return nil
}

func (r *CatalogRepository) DeleteLabel(ctx context.Context, projectID, labelID uuid.UUID) error {
	result, err := r.db.ExecContext(ctx, `DELETE FROM labels WHERE project_id = $1 AND id = $2`, projectID, labelID)
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

func (r *CatalogRepository) ListComponents(ctx context.Context, projectID uuid.UUID) ([]domain.Component, error) {
	var rows []componentRow
	if err := r.db.SelectContext(ctx, &rows, `
		SELECT id, project_id, name, description, created_at, updated_at
		FROM components WHERE project_id = $1 ORDER BY name ASC`, projectID); err != nil {
		return nil, apperror.Wrap(apperror.KindInternal, "failed to list components", err)
	}
	components := make([]domain.Component, 0, len(rows))
	for _, row := range rows {
		components = append(components, domain.Component{
			ID: row.ID, ProjectID: row.ProjectID, Name: row.Name, Description: row.Description,
			CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
		})
	}
	return components, nil
}

func (r *CatalogRepository) CreateComponent(ctx context.Context, component *domain.Component) error {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO components (id, project_id, name, description, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (project_id, name) DO NOTHING`,
		component.ID, component.ProjectID, component.Name, component.Description,
		component.CreatedAt, component.UpdatedAt)
	if err != nil {
		return apperror.Wrap(apperror.KindInternal, "failed to create component", err)
	}
	return nil
}

func (r *CatalogRepository) DeleteComponent(ctx context.Context, projectID, componentID uuid.UUID) error {
	result, err := r.db.ExecContext(ctx, `DELETE FROM components WHERE project_id = $1 AND id = $2`, projectID, componentID)
	if err != nil {
		return apperror.Wrap(apperror.KindInternal, "failed to delete component", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return apperror.Wrap(apperror.KindInternal, "failed to delete component", err)
	}
	if affected == 0 {
		return apperror.New(apperror.KindNotFound, "component not found")
	}
	return nil
}
