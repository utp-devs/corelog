package postgres

import (
	"context"
	"database/sql"
	"errors"
	"strings"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"

	"github.com/jhonsferg/corelog/internal/platform/apperror"
	"github.com/jhonsferg/corelog/internal/project/application/port"
	"github.com/jhonsferg/corelog/internal/project/domain"
)

type ProjectRepository struct {
	db *sqlx.DB
}

func NewProjectRepository(db *sqlx.DB) *ProjectRepository {
	return &ProjectRepository{db: db}
}

var _ port.Repository = (*ProjectRepository)(nil)

func (r *ProjectRepository) Save(ctx context.Context, p *domain.Project) error {
	row := fromDomain(p)
	const query = `
		INSERT INTO projects (id, key, name, description, type, lead_id, visibility, archived_at, created_at, updated_at)
		VALUES (:id, :key, :name, :description, :type, :lead_id, :visibility, :archived_at, :created_at, :updated_at)`
	if _, err := r.db.NamedExecContext(ctx, query, row); err != nil {
		if strings.Contains(err.Error(), "duplicate key") {
			return domain.ErrProjectKeyTaken
		}
		return apperror.Wrap(apperror.KindInternal, "failed to save project", err)
	}
	return nil
}

func (r *ProjectRepository) FindByID(ctx context.Context, id uuid.UUID) (*domain.Project, error) {
	var row projectRow
	if err := r.db.GetContext(ctx, &row, `SELECT id, key, name, description, type, lead_id, visibility, archived_at, created_at, updated_at FROM projects WHERE id = $1`, id); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, domain.ErrProjectNotFound
		}
		return nil, apperror.Wrap(apperror.KindInternal, "failed to find project", err)
	}
	return row.toDomain(), nil
}

func (r *ProjectRepository) FindByKey(ctx context.Context, key string) (*domain.Project, error) {
	var row projectRow
	if err := r.db.GetContext(ctx, &row, `SELECT id, key, name, description, type, lead_id, visibility, archived_at, created_at, updated_at FROM projects WHERE key = $1`, strings.ToUpper(key)); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, domain.ErrProjectNotFound
		}
		return nil, apperror.Wrap(apperror.KindInternal, "failed to find project", err)
	}
	return row.toDomain(), nil
}

func (r *ProjectRepository) FindAll(ctx context.Context) ([]*domain.Project, error) {
	var rows []projectRow
	if err := r.db.SelectContext(ctx, &rows, `SELECT id, key, name, description, type, lead_id, visibility, archived_at, created_at, updated_at FROM projects ORDER BY name ASC`); err != nil {
		return nil, apperror.Wrap(apperror.KindInternal, "failed to list projects", err)
	}
	projects := make([]*domain.Project, 0, len(rows))
	for _, row := range rows {
		projects = append(projects, row.toDomain())
	}
	return projects, nil
}

func (r *ProjectRepository) Update(ctx context.Context, p *domain.Project) error {
	row := fromDomain(p)
	const query = `
		UPDATE projects
		SET name = :name, description = :description, type = :type, lead_id = :lead_id,
		    visibility = :visibility, archived_at = :archived_at, updated_at = :updated_at
		WHERE id = :id`
	result, err := r.db.NamedExecContext(ctx, query, row)
	if err != nil {
		return apperror.Wrap(apperror.KindInternal, "failed to update project", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return apperror.Wrap(apperror.KindInternal, "failed to update project", err)
	}
	if affected == 0 {
		return domain.ErrProjectNotFound
	}
	return nil
}

func (r *ProjectRepository) Delete(ctx context.Context, id uuid.UUID) error {
	result, err := r.db.ExecContext(ctx, `DELETE FROM projects WHERE id = $1`, id)
	if err != nil {
		return apperror.Wrap(apperror.KindInternal, "failed to delete project", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return apperror.Wrap(apperror.KindInternal, "failed to delete project", err)
	}
	if affected == 0 {
		return domain.ErrProjectNotFound
	}
	return nil
}
