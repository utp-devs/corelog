package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jmoiron/sqlx"

	"github.com/jhonsferg/corelog/internal/platform/apperror"
	"github.com/jhonsferg/corelog/internal/platform/database"
	"github.com/jhonsferg/corelog/internal/sprint/application/port"
	"github.com/jhonsferg/corelog/internal/sprint/domain"
)

const pgUniqueViolation = "23505"

// SprintRepository persiste sprints, su historial de alcance y la asignacion de incidencias
// (issues.sprint_id) en PostgreSQL. Usa la transaccion del contexto cuando la hay.
type SprintRepository struct {
	db *sqlx.DB
}

func NewSprintRepository(db *sqlx.DB) *SprintRepository {
	return &SprintRepository{db: db}
}

var _ port.Repository = (*SprintRepository)(nil)

func (r *SprintRepository) ext(ctx context.Context) sqlx.ExtContext {
	return database.Executor(ctx, r.db)
}

func internal(msg string, err error) error {
	return apperror.Wrap(apperror.KindInternal, msg, err)
}

// idStrings convierte ids en texto para pasarlos como uuid[] a PostgreSQL.
func idStrings(ids []uuid.UUID) []string {
	out := make([]string, len(ids))
	for i, id := range ids {
		out[i] = id.String()
	}
	return out
}

type sprintRow struct {
	ID              uuid.UUID  `db:"id"`
	ProjectID       uuid.UUID  `db:"project_id"`
	BoardID         *uuid.UUID `db:"board_id"`
	Name            string     `db:"name"`
	Goal            string     `db:"goal"`
	Status          string     `db:"status"`
	StartDate       *time.Time `db:"start_date"`
	EndDate         *time.Time `db:"end_date"`
	StartedAt       *time.Time `db:"started_at"`
	CompletedAt     *time.Time `db:"completed_at"`
	ClosedAt        *time.Time `db:"closed_at"`
	CommittedPoints *float64   `db:"committed_points"`
	CompletedPoints *float64   `db:"completed_points"`
	CreatedAt       time.Time  `db:"created_at"`
	UpdatedAt       time.Time  `db:"updated_at"`
}

func (r sprintRow) toDomain() *domain.Sprint {
	return &domain.Sprint{
		ID: r.ID, ProjectID: r.ProjectID, BoardID: r.BoardID, Name: r.Name, Goal: r.Goal, Status: domain.Status(r.Status),
		StartDate: r.StartDate, EndDate: r.EndDate, StartedAt: r.StartedAt, CompletedAt: r.CompletedAt, ClosedAt: r.ClosedAt,
		CommittedPoints: r.CommittedPoints, CompletedPoints: r.CompletedPoints, CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt,
	}
}

func fromDomain(s *domain.Sprint) sprintRow {
	return sprintRow{
		ID: s.ID, ProjectID: s.ProjectID, BoardID: s.BoardID, Name: s.Name, Goal: s.Goal, Status: string(s.Status),
		StartDate: s.StartDate, EndDate: s.EndDate, StartedAt: s.StartedAt, CompletedAt: s.CompletedAt, ClosedAt: s.ClosedAt,
		CommittedPoints: s.CommittedPoints, CompletedPoints: s.CompletedPoints, CreatedAt: s.CreatedAt, UpdatedAt: s.UpdatedAt,
	}
}

const sprintColumns = `id, project_id, board_id, name, goal, status, start_date, end_date, started_at, completed_at, closed_at,
	committed_points, completed_points, created_at, updated_at`

func (r *SprintRepository) Save(ctx context.Context, sprint *domain.Sprint) error {
	const query = `
		INSERT INTO sprints (project_id, board_id, name, goal, status, start_date, end_date, started_at, completed_at, closed_at,
		                     committed_points, completed_points, created_at, updated_at)
		VALUES (:project_id, :board_id, :name, :goal, :status, :start_date, :end_date, :started_at, :completed_at, :closed_at,
		        :committed_points, :completed_points, :created_at, :updated_at)
		RETURNING id`
	id, err := database.InsertReturningID(ctx, r.ext(ctx), query, fromDomain(sprint))
	if err != nil {
		return internal("failed to save sprint", err)
	}
	sprint.ID = id
	return nil
}

func (r *SprintRepository) find(ctx context.Context, id uuid.UUID, suffix string) (*domain.Sprint, error) {
	var row sprintRow
	err := sqlx.GetContext(ctx, r.ext(ctx), &row, `SELECT `+sprintColumns+` FROM sprints WHERE id = $1`+suffix, id)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, domain.ErrSprintNotFound
	}
	if err != nil {
		return nil, internal("failed to find sprint", err)
	}
	return row.toDomain(), nil
}

func (r *SprintRepository) FindByID(ctx context.Context, id uuid.UUID) (*domain.Sprint, error) {
	return r.find(ctx, id, "")
}

func (r *SprintRepository) FindByIDForUpdate(ctx context.Context, id uuid.UUID) (*domain.Sprint, error) {
	return r.find(ctx, id, " FOR UPDATE")
}

func (r *SprintRepository) ListByProject(ctx context.Context, projectID uuid.UUID) ([]*domain.Sprint, error) {
	var rows []sprintRow
	if err := sqlx.SelectContext(ctx, r.ext(ctx), &rows, `
		SELECT `+sprintColumns+` FROM sprints
		WHERE project_id = $1
		ORDER BY start_date ASC NULLS LAST, created_at ASC, id ASC`, projectID); err != nil {
		return nil, internal("failed to list sprints", err)
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
		SET name = :name, goal = :goal, status = :status, start_date = :start_date, end_date = :end_date,
		    started_at = :started_at, completed_at = :completed_at, closed_at = :closed_at,
		    committed_points = :committed_points, completed_points = :completed_points, updated_at = :updated_at
		WHERE id = :id`
	result, err := sqlx.NamedExecContext(ctx, r.ext(ctx), query, fromDomain(sprint))
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == pgUniqueViolation && strings.Contains(pgErr.ConstraintName, "one_active") {
			return domain.ErrActiveSprintExists
		}
		return internal("failed to update sprint", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return internal("failed to update sprint", err)
	}
	if affected == 0 {
		return domain.ErrSprintNotFound
	}
	return nil
}

func (r *SprintRepository) Delete(ctx context.Context, id uuid.UUID) error {
	if _, err := r.ext(ctx).ExecContext(ctx, `UPDATE issues SET sprint_id = NULL, updated_at = now() WHERE sprint_id = $1`, id); err != nil {
		return internal("failed to release sprint issues", err)
	}
	result, err := r.ext(ctx).ExecContext(ctx, `DELETE FROM sprints WHERE id = $1`, id)
	if err != nil {
		return internal("failed to delete sprint", err)
	}
	if n, _ := result.RowsAffected(); n == 0 {
		return domain.ErrSprintNotFound
	}
	return nil
}

func (r *SprintRepository) HasActive(ctx context.Context, projectID, exceptID uuid.UUID) (bool, error) {
	var exists bool
	if err := sqlx.GetContext(ctx, r.ext(ctx), &exists, `
		SELECT EXISTS (SELECT 1 FROM sprints WHERE project_id = $1 AND status = 'active' AND id <> $2)`,
		projectID, exceptID,
	); err != nil {
		return false, internal("failed to check active sprint", err)
	}
	return exists, nil
}

const liveProject = `EXISTS (SELECT 1 FROM projects p WHERE p.id = issues.project_id AND p.deleted_at IS NULL)`

func (r *SprintRepository) SetIssueSprint(ctx context.Context, projectID, issueID uuid.UUID, sprintID *uuid.UUID) error {
	result, err := r.ext(ctx).ExecContext(ctx,
		`UPDATE issues SET sprint_id = $3, updated_at = now() WHERE id = $1 AND project_id = $2 AND deleted_at IS NULL AND `+liveProject,
		issueID, projectID, sprintID)
	if err != nil {
		return internal("failed to assign issue to sprint", err)
	}
	return requireAffected(result)
}

func (r *SprintRepository) RemoveIssueFromSprint(ctx context.Context, projectID, sprintID, issueID uuid.UUID) error {
	result, err := r.ext(ctx).ExecContext(ctx,
		`UPDATE issues SET sprint_id = NULL, updated_at = now() WHERE id = $1 AND project_id = $2 AND sprint_id = $3 AND deleted_at IS NULL AND `+liveProject,
		issueID, projectID, sprintID)
	if err != nil {
		return internal("failed to remove issue from sprint", err)
	}
	return requireAffected(result)
}

func (r *SprintRepository) IssueProjects(ctx context.Context, ids []uuid.UUID) (map[uuid.UUID]uuid.UUID, error) {
	var rows []struct {
		ID        uuid.UUID `db:"id"`
		ProjectID uuid.UUID `db:"project_id"`
	}
	if err := sqlx.SelectContext(ctx, r.ext(ctx), &rows,
		`SELECT id, project_id FROM issues WHERE id = ANY($1::uuid[]) AND deleted_at IS NULL AND `+liveProject, idStrings(ids)); err != nil {
		return nil, internal("failed to resolve issue projects", err)
	}
	out := make(map[uuid.UUID]uuid.UUID, len(rows))
	for _, row := range rows {
		out[row.ID] = row.ProjectID
	}
	return out, nil
}

func (r *SprintRepository) LockIssues(ctx context.Context, projectID uuid.UUID, ids []uuid.UUID) ([]port.IssueState, error) {
	var rows []struct {
		ID       uuid.UUID  `db:"id"`
		SprintID *uuid.UUID `db:"sprint_id"`
		Points   *float64   `db:"story_points"`
		Category string     `db:"category"`
	}
	if err := sqlx.SelectContext(ctx, r.ext(ctx), &rows, `
		SELECT id, sprint_id, story_points, corelog_issue_category(status_id, status) AS category
		FROM issues
		WHERE id = ANY($1::uuid[]) AND project_id = $2 AND deleted_at IS NULL AND `+liveProject+`
		ORDER BY id FOR UPDATE`, idStrings(ids), projectID); err != nil {
		return nil, internal("failed to lock issues", err)
	}
	out := make([]port.IssueState, 0, len(rows))
	for _, row := range rows {
		out = append(out, port.IssueState{ID: row.ID, SprintID: row.SprintID, Points: row.Points, Category: row.Category})
	}
	return out, nil
}

func (r *SprintRepository) LockMembers(ctx context.Context, sprintID uuid.UUID) ([]domain.Member, error) {
	var rows []struct {
		ID       uuid.UUID `db:"id"`
		Points   *float64  `db:"story_points"`
		Category string    `db:"category"`
	}
	if err := sqlx.SelectContext(ctx, r.ext(ctx), &rows, `
		SELECT id, story_points, corelog_issue_category(status_id, status) AS category
		FROM issues
		WHERE sprint_id = $1 AND deleted_at IS NULL AND `+liveProject+`
		ORDER BY rank, id FOR UPDATE`, sprintID); err != nil {
		return nil, internal("failed to lock sprint issues", err)
	}
	out := make([]domain.Member, 0, len(rows))
	for _, row := range rows {
		out = append(out, domain.Member{IssueID: row.ID, Points: row.Points, Category: row.Category})
	}
	return out, nil
}

func (r *SprintRepository) PlaceIssues(ctx context.Context, projectID uuid.UUID, sprintID *uuid.UUID, ranks map[uuid.UUID]string) error {
	for id, rank := range ranks {
		result, err := r.ext(ctx).ExecContext(ctx,
			`UPDATE issues SET sprint_id = $3, rank = $4, updated_at = now() WHERE id = $1 AND project_id = $2 AND deleted_at IS NULL AND `+liveProject,
			id, projectID, sprintID, rank)
		if err != nil {
			return internal("failed to place issue", err)
		}
		if err := requireAffected(result); err != nil {
			return err
		}
	}
	return nil
}

func (r *SprintRepository) MoveIssues(ctx context.Context, ids []uuid.UUID, sprintID *uuid.UUID) error {
	if _, err := r.ext(ctx).ExecContext(ctx,
		`UPDATE issues SET sprint_id = $2, updated_at = now() WHERE id = ANY($1::uuid[]) AND deleted_at IS NULL`,
		idStrings(ids), sprintID); err != nil {
		return internal("failed to move issues", err)
	}
	return nil
}

// sectionFilter es el criterio de pertenencia de una incidencia a una seccion del backlog: las epicas no
// aparecen y el backlog sin sprint no muestra las terminadas.
const (
	notEpic            = `i.issue_type <> 'epic'`
	backlogOnlyPending = `corelog_issue_category(i.status_id, i.status) <> 'done'`
)

func (r *SprintRepository) SectionRanks(ctx context.Context, projectID uuid.UUID, sprintID *uuid.UUID, exclude []uuid.UUID) ([]domain.RankedID, error) {
	query := `SELECT i.id, i.rank FROM issues i
		WHERE i.project_id = $1 AND i.deleted_at IS NULL AND ` + notEpic + ` AND NOT (i.id = ANY($2::uuid[]))
		  AND EXISTS (SELECT 1 FROM projects p WHERE p.id = i.project_id AND p.deleted_at IS NULL)`
	args := []any{projectID, idStrings(exclude)}
	if sprintID == nil {
		query += ` AND i.sprint_id IS NULL AND ` + backlogOnlyPending
	} else {
		query += ` AND i.sprint_id = $3`
		args = append(args, *sprintID)
	}
	query += ` ORDER BY i.rank, i.id`
	var rows []struct {
		ID   uuid.UUID `db:"id"`
		Rank string    `db:"rank"`
	}
	if err := sqlx.SelectContext(ctx, r.ext(ctx), &rows, query, args...); err != nil {
		return nil, internal("failed to read section ranks", err)
	}
	out := make([]domain.RankedID, 0, len(rows))
	for _, row := range rows {
		out = append(out, domain.RankedID{ID: row.ID, Rank: row.Rank})
	}
	return out, nil
}

const eventChunk = 400

func (r *SprintRepository) AddEvents(ctx context.Context, events []domain.Event) error {
	for start := 0; start < len(events); start += eventChunk {
		end := start + eventChunk
		if end > len(events) {
			end = len(events)
		}
		var b strings.Builder
		b.WriteString(`INSERT INTO sprint_issue_events (sprint_id, issue_id, event, points_at_time, status_category_at_time, happened_at, actor_id, after_start) VALUES `)
		args := make([]any, 0, (end-start)*8)
		for i, e := range events[start:end] {
			if i > 0 {
				b.WriteString(", ")
			}
			n := len(args)
			fmt.Fprintf(&b, "($%d, $%d, $%d, $%d, NULLIF($%d, ''), $%d, $%d, $%d)", n+1, n+2, n+3, n+4, n+5, n+6, n+7, n+8)
			args = append(args, e.SprintID, e.IssueID, string(e.Kind), e.Points, e.Category, e.HappenedAt, e.ActorID, e.AfterStart)
		}
		if _, err := r.ext(ctx).ExecContext(ctx, b.String(), args...); err != nil {
			return internal("failed to record sprint events", err)
		}
	}
	return nil
}

func (r *SprintRepository) Events(ctx context.Context, sprintID uuid.UUID) ([]domain.Event, error) {
	var rows []struct {
		ID         int64      `db:"id"`
		SprintID   uuid.UUID  `db:"sprint_id"`
		IssueID    uuid.UUID  `db:"issue_id"`
		Event      string     `db:"event"`
		Points     *float64   `db:"points_at_time"`
		Category   string     `db:"category"`
		HappenedAt time.Time  `db:"happened_at"`
		ActorID    *uuid.UUID `db:"actor_id"`
		AfterStart bool       `db:"after_start"`
		Key        string     `db:"key"`
		Summary    string     `db:"summary"`
	}
	if err := sqlx.SelectContext(ctx, r.ext(ctx), &rows, `
		SELECT e.id, e.sprint_id, e.issue_id, e.event, e.points_at_time, COALESCE(e.status_category_at_time, '') AS category,
		       e.happened_at, e.actor_id, e.after_start, p.key || '-' || i.key_number AS key, i.summary
		FROM sprint_issue_events e
		JOIN issues i ON i.id = e.issue_id AND i.deleted_at IS NULL
		JOIN projects p ON p.id = i.project_id AND p.deleted_at IS NULL
		WHERE e.sprint_id = $1
		ORDER BY e.happened_at, e.id`, sprintID); err != nil {
		return nil, internal("failed to read sprint events", err)
	}
	out := make([]domain.Event, 0, len(rows))
	for _, row := range rows {
		out = append(out, domain.Event{
			ID: row.ID, SprintID: row.SprintID, IssueID: row.IssueID, Kind: domain.EventKind(row.Event), Points: row.Points,
			Category: row.Category, HappenedAt: row.HappenedAt, ActorID: row.ActorID, AfterStart: row.AfterStart,
			Key: row.Key, Summary: row.Summary,
		})
	}
	return out, nil
}

func (r *SprintRepository) LiveCategories(ctx context.Context, issueIDs []uuid.UUID) (map[uuid.UUID]string, error) {
	out := map[uuid.UUID]string{}
	if len(issueIDs) == 0 {
		return out, nil
	}
	var rows []struct {
		ID       uuid.UUID `db:"id"`
		Category string    `db:"category"`
	}
	if err := sqlx.SelectContext(ctx, r.ext(ctx), &rows, `
		SELECT id, corelog_issue_category(status_id, status) AS category FROM issues
		WHERE id = ANY($1::uuid[]) AND deleted_at IS NULL AND `+liveProject, idStrings(issueIDs)); err != nil {
		return nil, internal("failed to read issue categories", err)
	}
	for _, row := range rows {
		out[row.ID] = row.Category
	}
	return out, nil
}

func (r *SprintRepository) DoneTimelines(ctx context.Context, issueIDs []uuid.UUID) (map[uuid.UUID]domain.DoneTimeline, error) {
	out := map[uuid.UUID]domain.DoneTimeline{}
	if len(issueIDs) == 0 {
		return out, nil
	}
	var changes []struct {
		IssueID uuid.UUID `db:"issue_id"`
		At      time.Time `db:"changed_at"`
		Done    bool      `db:"done"`
	}
	if err := sqlx.SelectContext(ctx, r.ext(ctx), &changes, `
		SELECT h.issue_id, h.changed_at, (s.category = 'done') AS done
		FROM issue_status_history h JOIN statuses s ON s.id = h.to_status_id
		WHERE h.issue_id = ANY($1::uuid[])
		ORDER BY h.issue_id, h.changed_at, h.id`, idStrings(issueIDs)); err != nil {
		return nil, internal("failed to read status history", err)
	}
	for _, c := range changes {
		t := out[c.IssueID]
		t.Changes = append(t.Changes, domain.StatusChange{At: c.At, Done: c.Done})
		out[c.IssueID] = t
	}
	var closed []struct {
		ID     uuid.UUID  `db:"id"`
		DoneAt *time.Time `db:"done_at"`
	}
	if err := sqlx.SelectContext(ctx, r.ext(ctx), &closed, `
		SELECT id, COALESCE(resolved_at, closed_at, updated_at) AS done_at FROM issues
		WHERE id = ANY($1::uuid[]) AND corelog_issue_category(status_id, status) = 'done'`, idStrings(issueIDs)); err != nil {
		return nil, internal("failed to read closing dates", err)
	}
	for _, c := range closed {
		t := out[c.ID]
		t.DoneAt = c.DoneAt
		out[c.ID] = t
	}
	return out, nil
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
	if err := sqlx.SelectContext(ctx, r.ext(ctx), &rows, `
		SELECT id, summary AS title, status, priority, story_points, closed_at
		FROM issues
		WHERE sprint_id = $1 AND deleted_at IS NULL AND `+liveProject+`
		ORDER BY rank ASC, id ASC`, sprintID); err != nil {
		return nil, internal("failed to list sprint issues", err)
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

type velocityRow struct {
	ID     uuid.UUID `db:"id"`
	Name   string    `db:"name"`
	Points float64   `db:"points"`
}

// Velocity devuelve los puntos completados de los ultimos sprints cerrados, del mas reciente al mas antiguo.
// Usa los puntos fijados al completar el sprint y, para los sprints anteriores a ese registro, los de las
// incidencias cerradas que aun tiene.
func (r *SprintRepository) Velocity(ctx context.Context, projectID uuid.UUID, limit int) ([]port.VelocityPoint, error) {
	var rows []velocityRow
	if err := sqlx.SelectContext(ctx, r.ext(ctx), &rows, `
		SELECT s.id, s.name,
		       COALESCE(s.completed_points, SUM(i.story_points) FILTER (WHERE i.status = 'closed'), 0) AS points
		FROM sprints s
		JOIN projects p ON p.id = s.project_id AND p.deleted_at IS NULL
		LEFT JOIN issues i ON i.sprint_id = s.id AND i.deleted_at IS NULL
		WHERE s.project_id = $1 AND s.status = 'closed'
		GROUP BY s.id, s.name, s.closed_at, s.completed_points
		ORDER BY s.closed_at DESC NULLS LAST
		LIMIT $2`,
		projectID, limit,
	); err != nil {
		return nil, internal("failed to compute velocity", err)
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
		return internal("failed to update issue", err)
	}
	if affected == 0 {
		return domain.ErrIssueNotInProject
	}
	return nil
}
