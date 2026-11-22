package postgres

import (
	"context"
	"database/sql"
	"errors"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jmoiron/sqlx"

	boarddomain "github.com/jhonsferg/corelog/internal/board/domain"
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
var _ port.ProjectCreationRepository = (*ProjectRepository)(nil)

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

func (r *ProjectRepository) CreateWithDefaults(ctx context.Context, p *domain.Project, lead port.Member) error {
	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return apperror.Wrap(apperror.KindInternal, "failed to begin project creation", err)
	}
	defer func() { _ = tx.Rollback() }()
	row := fromDomain(p)
	const projectQuery = `
		INSERT INTO projects (id, key, name, description, type, lead_id, visibility, archived_at, created_at, updated_at)
		VALUES (:id, :key, :name, :description, :type, :lead_id, :visibility, :archived_at, :created_at, :updated_at)`
	if _, err := tx.NamedExecContext(ctx, projectQuery, row); err != nil {
		if isUniqueConstraintViolation(err) {
			return domain.ErrProjectKeyTaken
		}
		return apperror.Wrap(apperror.KindInternal, "failed to save project", err)
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO project_members (project_id, user_id, role, created_at)
		VALUES ($1, $2, $3, $4)`, lead.ProjectID, lead.UserID, lead.Role, lead.CreatedAt); err != nil {
		return apperror.Wrap(apperror.KindInternal, "failed to add project lead", err)
	}
	defaultBoard, err := boarddomain.NewDefaultBoard(p.ID, boarddomain.Type(p.Type))
	if err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO boards (id, project_id, name, type, config, is_default, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5::jsonb, $6, $7, $8)`,
		defaultBoard.ID, defaultBoard.ProjectID, defaultBoard.Name, defaultBoard.Type,
		string(defaultBoard.Config), defaultBoard.IsDefault, defaultBoard.CreatedAt, defaultBoard.UpdatedAt); err != nil {
		return apperror.Wrap(apperror.KindInternal, "failed to create default project board", err)
	}
	for _, column := range defaultBoard.Columns {
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO board_columns (id, board_id, name, status, position, wip_limit, created_at, updated_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
			column.ID, column.BoardID, column.Name, column.Status, column.Position,
			column.WIPLimit, column.CreatedAt, column.UpdatedAt); err != nil {
			return apperror.Wrap(apperror.KindInternal, "failed to create default board column", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return apperror.Wrap(apperror.KindInternal, "failed to commit project creation", err)
	}
	return nil
}

func isUniqueConstraintViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
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
