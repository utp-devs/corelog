package postgres

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"

	"github.com/jhonsferg/corelog/internal/platform/apperror"
	"github.com/jhonsferg/corelog/internal/platform/database"
	"github.com/jhonsferg/corelog/internal/sprint/application/port"
	"github.com/jhonsferg/corelog/internal/sprint/domain"
)

// SprintRepository persiste sprints y la asignacion de incidencias (issues.sprint_id) en PostgreSQL.
type SprintRepository struct {
	db *sqlx.DB
}

func NewSprintRepository(db *sqlx.DB) *SprintRepository {
	return &SprintRepository{db: db}
}

var _ port.Repository = (*SprintRepository)(nil)

type sprintRow struct {
	ID        uuid.UUID  `db:"id"`
	ProjectID uuid.UUID  `db:"project_id"`
	Name      string     `db:"name"`
	Goal      string     `db:"goal"`
	Status    string     `db:"status"`
	StartDate *time.Time `db:"start_date"`
	EndDate   *time.Time `db:"end_date"`
	ClosedAt  *time.Time `db:"closed_at"`
	CreatedAt time.Time  `db:"created_at"`
	UpdatedAt time.Time  `db:"updated_at"`
}

func (r sprintRow) toDomain() *domain.Sprint {
	return &domain.Sprint{
		ID: r.ID, ProjectID: r.ProjectID, Name: r.Name, Goal: r.Goal, Status: domain.Status(r.Status),
		StartDate: r.StartDate, EndDate: r.EndDate, ClosedAt: r.ClosedAt, CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt,
	}
}

func fromDomain(s *domain.Sprint) sprintRow {
	return sprintRow{
		ID: s.ID, ProjectID: s.ProjectID, Name: s.Name, Goal: s.Goal, Status: string(s.Status),
		StartDate: s.StartDate, EndDate: s.EndDate, ClosedAt: s.ClosedAt, CreatedAt: s.CreatedAt, UpdatedAt: s.UpdatedAt,
	}
}

const sprintColumns = `id, project_id, name, goal, status, start_date, end_date, closed_at, created_at, updated_at`

func (r *SprintRepository) Save(ctx context.Context, sprint *domain.Sprint) error {
	const query = `
		INSERT INTO sprints (project_id, name, goal, status, start_date, end_date, closed_at, created_at, updated_at)
		VALUES (:project_id, :name, :goal, :status, :start_date, :end_date, :closed_at, :created_at, :updated_at)
		RETURNING id`
	id, err := database.InsertReturningID(ctx, r.db, query, fromDomain(sprint))
	if err != nil {
		return apperror.Wrap(apperror.KindInternal, "failed to save sprint", err)
	}
	sprint.ID = id
	return nil
}

func (r *SprintRepository) FindByID(ctx context.Context, id uuid.UUID) (*domain.Sprint, error) {
	var row sprintRow
	err := r.db.GetContext(ctx, &row, `SELECT `+sprintColumns+` FROM sprints WHERE id = $1`, id)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, domain.ErrSprintNotFound
	}
	if err != nil {
		return nil, apperror.Wrap(apperror.KindInternal, "failed to find sprint", err)
	}
	return row.toDomain(), nil
}

func (r *SprintRepository) ListByProject(ctx context.Context, projectID uuid.UUID) ([]*domain.Sprint, error) {
	var rows []sprintRow
	if err := r.db.SelectContext(ctx, &rows, `
		SELECT `+sprintColumns+` FROM sprints
		WHERE project_id = $1
		ORDER BY start_date ASC NULLS LAST, created_at ASC, id ASC`, projectID); err != nil {
		return nil, apperror.Wrap(apperror.KindInternal, "failed to list sprints", err)
	}
	sprints := make([]*domain.Sprint, 0, len(rows))
	for _, row := range rows {
		sprints = append(sprints, row.toDomain())
	}
	return sprints, nil
}

func (r *SprintRepository) Update(ctx context.Context, sprint *domain.Sprint) error {
	const query = `
		UPDATE sprints
		SET name = :name, goal = :goal, status = :status, start_date = :start_date,
		    end_date = :end_date, closed_at = :closed_at, updated_at = :updated_at
		WHERE id = :id`
	result, err := r.db.NamedExecContext(ctx, query, fromDomain(sprint))
	if err != nil {
		return apperror.Wrap(apperror.KindInternal, "failed to update sprint", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return apperror.Wrap(apperror.KindInternal, "failed to update sprint", err)
	}
	if affected == 0 {
		return domain.ErrSprintNotFound
	}
	return nil
}

func (r *SprintRepository) HasActive(ctx context.Context, projectID, exceptID uuid.UUID) (bool, error) {
	var exists bool
	if err := r.db.GetContext(ctx, &exists, `
		SELECT EXISTS (SELECT 1 FROM sprints WHERE project_id = $1 AND status = 'active' AND id <> $2)`,
		projectID, exceptID,
	); err != nil {
		return false, apperror.Wrap(apperror.KindInternal, "failed to check active sprint", err)
	}
	return exists, nil
}

func (r *SprintRepository) SetIssueSprint(ctx context.Context, projectID, issueID uuid.UUID, sprintID *uuid.UUID) error {
	result, err := r.db.ExecContext(ctx,
		`UPDATE issues SET sprint_id = $3, updated_at = now() WHERE id = $1 AND project_id = $2 AND EXISTS (SELECT 1 FROM projects p WHERE p.id = issues.project_id AND p.deleted_at IS NULL)`,
		issueID, projectID, sprintID,
	)
	if err != nil {
		return apperror.Wrap(apperror.KindInternal, "failed to assign issue to sprint", err)
	}
	return requireAffected(result)
}

func (r *SprintRepository) RemoveIssueFromSprint(ctx context.Context, projectID, sprintID, issueID uuid.UUID) error {
	result, err := r.db.ExecContext(ctx,
		`UPDATE issues SET sprint_id = NULL, updated_at = now() WHERE id = $1 AND project_id = $2 AND sprint_id = $3 AND EXISTS (SELECT 1 FROM projects p WHERE p.id = issues.project_id AND p.deleted_at IS NULL)`,
		issueID, projectID, sprintID,
	)
	if err != nil {
		return apperror.Wrap(apperror.KindInternal, "failed to remove issue from sprint", err)
	}
	return requireAffected(result)
}

type issueRefRow struct {
	ID          uuid.UUID  `db:"id"`
	Title       string     `db:"title"`
	Status      string     `db:"status"`
	Priority    string     `db:"priority"`
	StoryPoints *float64   `db:"story_points"`
	ClosedAt    *time.Time `db:"closed_at"`
}

func (r *SprintRepository) ListIssues(ctx context.Context, sprintID uuid.UUID) ([]port.IssueRef, error) {
	var rows []issueRefRow
	if err := r.db.SelectContext(ctx, &rows, `
		SELECT id, summary AS title, status, priority, story_points, closed_at
		FROM issues
		WHERE sprint_id = $1
		  AND EXISTS (SELECT 1 FROM projects p WHERE p.id = issues.project_id AND p.deleted_at IS NULL)
		ORDER BY rank ASC, id ASC`, sprintID); err != nil {
		return nil, apperror.Wrap(apperror.KindInternal, "failed to list sprint issues", err)
	}
	refs := make([]port.IssueRef, 0, len(rows))
	for _, row := range rows {
		refs = append(refs, port.IssueRef{
			ID: row.ID, Title: row.Title, Status: row.Status, Priority: row.Priority,
			StoryPoints: row.StoryPoints, ClosedAt: row.ClosedAt,
		})
	}
	return refs, nil
}

// MoveUnfinished mueve las incidencias no cerradas del sprint. Con target nil quedan sin sprint (backlog).
func (r *SprintRepository) MoveUnfinished(ctx context.Context, sprintID uuid.UUID, target *uuid.UUID) error {
	if _, err := r.db.ExecContext(ctx, `
		UPDATE issues SET sprint_id = $2, updated_at = now()
		WHERE sprint_id = $1 AND status <> 'closed'
		  AND EXISTS (SELECT 1 FROM projects p WHERE p.id = issues.project_id AND p.deleted_at IS NULL)`,
		sprintID, target,
	); err != nil {
		return apperror.Wrap(apperror.KindInternal, "failed to move unfinished issues", err)
	}
	return nil
}

type velocityRow struct {
	ID     uuid.UUID `db:"id"`
	Name   string    `db:"name"`
	Points float64   `db:"points"`
}

// Velocity devuelve los puntos completados de los ultimos sprints cerrados, del mas reciente al mas antiguo.
func (r *SprintRepository) Velocity(ctx context.Context, projectID uuid.UUID, limit int) ([]port.VelocityPoint, error) {
	var rows []velocityRow
	if err := r.db.SelectContext(ctx, &rows, `
		SELECT s.id, s.name, COALESCE(SUM(i.story_points) FILTER (WHERE i.status = 'closed'), 0) AS points
		FROM sprints s
		JOIN projects p ON p.id = s.project_id AND p.deleted_at IS NULL
		LEFT JOIN issues i ON i.sprint_id = s.id
		WHERE s.project_id = $1 AND s.status = 'closed'
		GROUP BY s.id, s.name, s.closed_at
		ORDER BY s.closed_at DESC NULLS LAST
		LIMIT $2`,
		projectID, limit,
	); err != nil {
		return nil, apperror.Wrap(apperror.KindInternal, "failed to compute velocity", err)
	}
	points := make([]port.VelocityPoint, 0, len(rows))
	for _, row := range rows {
		points = append(points, port.VelocityPoint{SprintID: row.ID, Name: row.Name, Points: row.Points})
	}
	return points, nil
}

// requireAffected convierte "ninguna fila cambiada" en incidencia no encontrada en el proyecto.
func requireAffected(result sql.Result) error {
	affected, err := result.RowsAffected()
	if err != nil {
		return apperror.Wrap(apperror.KindInternal, "failed to update issue", err)
	}
	if affected == 0 {
		return domain.ErrIssueNotInProject
	}
	return nil
}
