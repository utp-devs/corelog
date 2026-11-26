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
	"github.com/jhonsferg/corelog/internal/platform/database"
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

const insertProjectQuery = `
	INSERT INTO projects (organization_id, key, name, description, type, lead_id, visibility, archived_at, created_at, updated_at)
	VALUES (:organization_id, :key, :name, :description, :type, :lead_id, :visibility, :archived_at, :created_at, :updated_at)
	RETURNING id`

func (r *ProjectRepository) Save(ctx context.Context, p *domain.Project) error {
	id, err := database.InsertReturningID(ctx, r.db, insertProjectQuery, fromDomain(p))
	if err != nil {
		if isUniqueConstraintViolation(err) {
			return domain.ErrProjectKeyTaken
		}
		return apperror.Wrap(apperror.KindInternal, "failed to save project", err)
	}
	p.ID = id
	return nil
}

// CreateWithDefaults persiste el proyecto junto con su miembro lider, el tablero
// por defecto y el workflow por defecto. Los identificadores los genera la BD:
// el proyecto se inserta primero y el resto de filas usan el ID devuelto.
func (r *ProjectRepository) CreateWithDefaults(ctx context.Context, p *domain.Project, lead port.Member) error {
	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return apperror.Wrap(apperror.KindInternal, "failed to begin project creation", err)
	}
	defer func() { _ = tx.Rollback() }()

	projectID, err := database.InsertReturningID(ctx, tx, insertProjectQuery, fromDomain(p))
	if err != nil {
		if isUniqueConstraintViolation(err) {
			return domain.ErrProjectKeyTaken
		}
		return apperror.Wrap(apperror.KindInternal, "failed to save project", err)
	}
	p.ID = projectID
	lead.ProjectID = projectID

	if _, err := tx.ExecContext(ctx, `
		INSERT INTO project_members (project_id, user_id, role, created_at)
		VALUES ($1, $2, $3, $4)`, lead.ProjectID, lead.UserID, lead.Role, lead.CreatedAt); err != nil {
		if isForeignKeyViolation(err) {
			return domain.ErrUserNotFound
		}
		return apperror.Wrap(apperror.KindInternal, "failed to add project lead", err)
	}

	defaultBoard, err := boarddomain.NewDefaultBoard(p.ID, boarddomain.Type(p.Type))
	if err != nil {
		return err
	}
	boardID, err := database.InsertReturningID(ctx, tx, `
		INSERT INTO boards (project_id, name, type, config, is_default, created_at, updated_at)
		VALUES (:project_id, :name, :type, CAST(:config AS jsonb), :is_default, :created_at, :updated_at)
		RETURNING id`, boardRowArgs(defaultBoard))
	if err != nil {
		return apperror.Wrap(apperror.KindInternal, "failed to create default project board", err)
	}
	defaultBoard.ID = boardID

	for i := range defaultBoard.Columns {
		column := &defaultBoard.Columns[i]
		column.BoardID = boardID
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO board_columns (board_id, name, status, position, wip_limit, created_at, updated_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7)`,
			column.BoardID, column.Name, column.Status, column.Position,
			column.WIPLimit, column.CreatedAt, column.UpdatedAt); err != nil {
			return apperror.Wrap(apperror.KindInternal, "failed to create default board column", err)
		}
	}

	defaultWorkflowID, err := database.InsertReturningID(ctx, tx, `
		INSERT INTO workflows (project_id, name, is_default, created_at, updated_at)
		VALUES (:project_id, 'Default', true, now(), now())
		RETURNING id`, map[string]interface{}{"project_id": p.ID})
	if err != nil {
		return apperror.Wrap(apperror.KindInternal, "failed to create default project workflow", err)
	}
	for _, transition := range [][2]string{
		{"open", "in_progress"}, {"open", "closed"},
		{"in_progress", "resolved"}, {"in_progress", "closed"},
		{"resolved", "closed"}, {"resolved", "in_progress"},
	} {
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO workflow_transitions (workflow_id, from_status, to_status, allowed_roles)
			VALUES ($1, $2, $3, 'admin,agent,requester')`, defaultWorkflowID, transition[0], transition[1]); err != nil {
			return apperror.Wrap(apperror.KindInternal, "failed to create default project workflow transition", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return apperror.Wrap(apperror.KindInternal, "failed to commit project creation", err)
	}
	return nil
}

// isUniqueConstraintViolation reports SQLSTATE 23505. On projects the only unique constraints are
// the primary key and uq_projects_organization_key, so a violation means the key is taken.
func isUniqueConstraintViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}

// isForeignKeyViolation reports SQLSTATE 23503.
func isForeignKeyViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23503"
}

func (r *ProjectRepository) FindByID(ctx context.Context, id uuid.UUID) (*domain.Project, error) {
	var row projectRow
	if err := r.db.GetContext(ctx, &row, `SELECT `+projectColumns+` FROM projects WHERE id = $1`, id); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, domain.ErrProjectNotFound
		}
		return nil, apperror.Wrap(apperror.KindInternal, "failed to find project", err)
	}
	return row.toDomain(), nil
}

// FindByKey busca una clave dentro de una organizacion; las claves son unicas por organizacion.
func (r *ProjectRepository) FindByKey(ctx context.Context, orgID uuid.UUID, key string) (*domain.Project, error) {
	var row projectRow
	if err := r.db.GetContext(ctx, &row, `
		SELECT `+projectColumns+` FROM projects
		WHERE organization_id = $1 AND key = $2`, orgID, strings.ToUpper(strings.TrimSpace(key))); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, domain.ErrProjectNotFound
		}
		return nil, apperror.Wrap(apperror.KindInternal, "failed to find project", err)
	}
	return row.toDomain(), nil
}

// FindByOrganization lista los proyectos de la organizacion que el alcance permite ver.
// Los proyectos donde el usuario es miembro explicito siempre se incluyen.
func (r *ProjectRepository) FindByOrganization(ctx context.Context, orgID uuid.UUID, scope port.VisibilityScope) ([]*domain.Project, error) {
	var rows []projectRow
	if err := r.db.SelectContext(ctx, &rows, `
		SELECT `+projectColumns+` FROM projects
		WHERE organization_id = $1
		  AND ($2::boolean
		       OR id IN (SELECT pm.project_id FROM project_members pm WHERE pm.user_id = $3)
		       OR ($4::boolean AND visibility = $5))
		ORDER BY name ASC`,
		orgID, scope.All, scope.UserID, scope.Open, storedOpen); err != nil {
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
		WHERE id = :id AND organization_id = :organization_id`
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

func (r *ProjectRepository) Delete(ctx context.Context, orgID, id uuid.UUID) error {
	result, err := r.db.ExecContext(ctx, `DELETE FROM projects WHERE id = $1 AND organization_id = $2`, id, orgID)
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

// boardRowArgs arma los parametros nombrados del INSERT de boards.
func boardRowArgs(b *boarddomain.Board) map[string]interface{} {
	return map[string]interface{}{
		"project_id": b.ProjectID,
		"name":       b.Name,
		"type":       b.Type,
		"config":     string(b.Config),
		"is_default": b.IsDefault,
		"created_at": b.CreatedAt,
		"updated_at": b.UpdatedAt,
	}
}
