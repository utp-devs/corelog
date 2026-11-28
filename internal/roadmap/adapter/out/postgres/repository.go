// Package postgres reads the roadmap with a fixed number of queries per request.
package postgres

import (
	"context"
	"database/sql"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"

	"github.com/jhonsferg/corelog/internal/platform/apperror"
	"github.com/jhonsferg/corelog/internal/roadmap/application/port"
	"github.com/jhonsferg/corelog/internal/roadmap/domain"
)

type querier interface {
	SelectContext(ctx context.Context, dest any, query string, args ...any) error
	BeginTxx(ctx context.Context, opts *sql.TxOptions) (*sqlx.Tx, error)
}

// Repository implements port.Repository.
type Repository struct {
	db querier
}

var _ port.Repository = (*Repository)(nil)

// NewRepository builds the repository.
func NewRepository(db *sqlx.DB) *Repository { return &Repository{db: db} }

func internal(msg string, err error) error { return apperror.Wrap(apperror.KindInternal, msg, err) }

type epicRow struct {
	ID          uuid.UUID  `db:"id"`
	Key         string     `db:"key"`
	Title       string     `db:"title"`
	Category    string     `db:"category"`
	Assignee    *string    `db:"assignee"`
	Start       *time.Time `db:"start_date"`
	Due         *time.Time `db:"due_date"`
	Total       int        `db:"total"`
	Todo        int        `db:"todo"`
	InProgress  int        `db:"in_progress"`
	Done        int        `db:"done"`
	PointsTotal float64    `db:"points_total"`
	PointsDone  float64    `db:"points_done"`
	ChildStart  *time.Time `db:"child_start"`
	ChildDue    *time.Time `db:"child_due"`
}

// Epics lists the epics (issues of the epic-level type) with the aggregates of their direct children in one
// statement: the number of queries does not depend on the number of epics.
func (r *Repository) Epics(ctx context.Context, projectID uuid.UUID) ([]domain.EpicRow, error) {
	var rows []epicRow
	err := r.db.SelectContext(ctx, &rows, `
		SELECT e.id, p.key || '-' || e.key_number AS key, e.summary AS title,
		       corelog_issue_category(e.status_id, e.status) AS category, au.display_name AS assignee,
		       e.start_date, e.due_date,
		       COALESCE(agg.total, 0) AS total, COALESCE(agg.todo, 0) AS todo, COALESCE(agg.in_progress, 0) AS in_progress,
		       COALESCE(agg.done, 0) AS done, COALESCE(agg.points_total, 0)::float8 AS points_total, COALESCE(agg.points_done, 0)::float8 AS points_done,
		       agg.child_start, agg.child_due
		FROM issues e
		JOIN projects p ON p.id = e.project_id AND p.deleted_at IS NULL
		JOIN issue_types t ON t.id = e.issue_type_id AND t.hierarchy_level = 1
		LEFT JOIN users au ON au.id = e.assignee_id
		LEFT JOIN roadmap_epic_ranks rk ON rk.project_id = e.project_id AND rk.issue_id = e.id
		LEFT JOIN LATERAL (
			SELECT count(*) AS total,
			       count(*) FILTER (WHERE c.cat = 'todo') AS todo,
			       count(*) FILTER (WHERE c.cat = 'in_progress') AS in_progress,
			       count(*) FILTER (WHERE c.cat = 'done') AS done,
			       COALESCE(sum(c.story_points), 0) AS points_total,
			       COALESCE(sum(c.story_points) FILTER (WHERE c.cat = 'done'), 0) AS points_done,
			       min(COALESCE(c.start_date, c.due_date)) AS child_start,
			       max(COALESCE(c.due_date, c.start_date)) AS child_due
			FROM (SELECT ch.story_points, ch.start_date, ch.due_date, corelog_issue_category(ch.status_id, ch.status) AS cat
			      FROM issues ch WHERE ch.parent_id = e.id AND ch.deleted_at IS NULL) c
		) agg ON TRUE
		WHERE e.project_id = $1 AND e.deleted_at IS NULL
		ORDER BY rk.position NULLS LAST, e.rank, e.created_at, e.id`, projectID)
	if err != nil {
		return nil, internal("failed to read the epics", err)
	}
	out := make([]domain.EpicRow, len(rows))
	for i, row := range rows {
		assignee := ""
		if row.Assignee != nil {
			assignee = *row.Assignee
		}
		out[i] = domain.EpicRow{
			ID: row.ID, Key: row.Key, Title: row.Title, StatusCategory: row.Category, Assignee: assignee, Start: utcDate(row.Start), Due: utcDate(row.Due),
			Total: row.Total, Todo: row.Todo, InProgress: row.InProgress, Done: row.Done, PointsTotal: row.PointsTotal, PointsDone: row.PointsDone,
			ChildStart: utcDate(row.ChildStart), ChildDue: utcDate(row.ChildDue),
		}
	}
	return out, nil
}

func utcDate(t *time.Time) *time.Time {
	if t == nil {
		return nil
	}
	d := time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
	return &d
}

type periodRow struct {
	ID          uuid.UUID  `db:"id"`
	Name        string     `db:"name"`
	Status      string     `db:"status"`
	Start       *time.Time `db:"start_date"`
	End         *time.Time `db:"end_date"`
	Total       int        `db:"total"`
	Todo        int        `db:"todo"`
	InProgress  int        `db:"in_progress"`
	Done        int        `db:"done"`
	PointsTotal float64    `db:"points_total"`
	PointsDone  float64    `db:"points_done"`
}

func periods(kind string, rows []periodRow) []domain.PeriodRow {
	out := make([]domain.PeriodRow, len(rows))
	for i, row := range rows {
		out[i] = domain.PeriodRow{
			ID: row.ID, Kind: kind, Name: row.Name, Status: row.Status, Start: utcDate(row.Start), End: utcDate(row.End),
			Total: row.Total, Todo: row.Todo, InProgress: row.InProgress, Done: row.Done, PointsTotal: row.PointsTotal, PointsDone: row.PointsDone,
		}
	}
	return out
}

const progressColumns = `
		       count(i.id) AS total,
		       count(i.id) FILTER (WHERE i.cat = 'todo') AS todo,
		       count(i.id) FILTER (WHERE i.cat = 'in_progress') AS in_progress,
		       count(i.id) FILTER (WHERE i.cat = 'done') AS done,
		       COALESCE(sum(i.story_points), 0)::float8 AS points_total,
		       COALESCE(sum(i.story_points) FILTER (WHERE i.cat = 'done'), 0)::float8 AS points_done`

// Versions lists the live versions of the project with the progress of their fix issues.
func (r *Repository) Versions(ctx context.Context, projectID uuid.UUID) ([]domain.PeriodRow, error) {
	var rows []periodRow
	err := r.db.SelectContext(ctx, &rows, `
		SELECT v.id, v.name, v.status, v.start_date, v.release_date AS end_date,`+progressColumns+`
		FROM versions v
		JOIN projects p ON p.id = v.project_id AND p.deleted_at IS NULL
		LEFT JOIN (SELECT iv.version_id, x.id, x.story_points, corelog_issue_category(x.status_id, x.status) AS cat
		           FROM issue_versions iv JOIN issues x ON x.id = iv.issue_id AND x.deleted_at IS NULL
		           WHERE iv.kind = 'fix') i ON i.version_id = v.id
		WHERE v.project_id = $1 AND v.archived_at IS NULL
		GROUP BY v.id
		ORDER BY COALESCE(v.release_date, v.start_date) NULLS LAST, lower(v.name), v.id`, projectID)
	if err != nil {
		return nil, internal("failed to read the versions", err)
	}
	return periods("version", rows), nil
}

// Sprints lists the sprints of the project with the progress of their issues.
func (r *Repository) Sprints(ctx context.Context, projectID uuid.UUID) ([]domain.PeriodRow, error) {
	var rows []periodRow
	err := r.db.SelectContext(ctx, &rows, `
		SELECT s.id, s.name, s.status, s.start_date, s.end_date,`+progressColumns+`
		FROM sprints s
		JOIN projects p ON p.id = s.project_id AND p.deleted_at IS NULL
		LEFT JOIN (SELECT x.sprint_id, x.id, x.story_points, corelog_issue_category(x.status_id, x.status) AS cat
		           FROM issues x WHERE x.deleted_at IS NULL AND x.sprint_id IS NOT NULL) i ON i.sprint_id = s.id
		WHERE s.project_id = $1
		GROUP BY s.id
		ORDER BY COALESCE(s.start_date, s.end_date) NULLS LAST, s.created_at, s.id`, projectID)
	if err != nil {
		return nil, internal("failed to read the sprints", err)
	}
	return periods("sprint", rows), nil
}

type depRow struct {
	From uuid.UUID `db:"source_issue_id"`
	To   uuid.UUID `db:"target_issue_id"`
}

// Dependencies lists the blocks links whose two ends are epics of the project.
func (r *Repository) Dependencies(ctx context.Context, projectID uuid.UUID) ([]domain.Dependency, error) {
	var rows []depRow
	err := r.db.SelectContext(ctx, &rows, `
		SELECT l.source_issue_id, l.target_issue_id
		FROM issue_links l
		JOIN issues s ON s.id = l.source_issue_id AND s.project_id = $1 AND s.deleted_at IS NULL
		JOIN issue_types st ON st.id = s.issue_type_id AND st.hierarchy_level = 1
		JOIN issues t ON t.id = l.target_issue_id AND t.project_id = $1 AND t.deleted_at IS NULL
		JOIN issue_types tt ON tt.id = t.issue_type_id AND tt.hierarchy_level = 1
		WHERE l.kind = 'blocks'
		ORDER BY l.created_at, l.id`, projectID)
	if err != nil {
		return nil, internal("failed to read the dependencies", err)
	}
	out := make([]domain.Dependency, len(rows))
	for i, row := range rows {
		out[i] = domain.Dependency{From: row.From, To: row.To, Kind: "blocks"}
	}
	return out, nil
}

// SaveRank replaces the manual order of the epics of the project. The ids must all be epics of the project.
func (r *Repository) SaveRank(ctx context.Context, projectID uuid.UUID, epicIDs []uuid.UUID) error {
	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return internal("failed to begin the rank transaction", err)
	}
	defer func() { _ = tx.Rollback() }()
	parts := make([]string, len(epicIDs))
	for i, id := range epicIDs {
		parts[i] = id.String()
	}
	ids := "{" + strings.Join(parts, ",") + "}"
	var valid int
	if err := tx.GetContext(ctx, &valid, `
		SELECT count(*) FROM issues e JOIN issue_types t ON t.id = e.issue_type_id AND t.hierarchy_level = 1
		WHERE e.project_id = $1 AND e.deleted_at IS NULL AND e.id = ANY($2::uuid[])`, projectID, ids); err != nil {
		return internal("failed to validate the epics", err)
	}
	if valid != len(epicIDs) {
		return apperror.WithCode(apperror.KindValidation, domain.CodeInvalidRank, "el orden incluye incidencias que no son épicas de este proyecto")
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM roadmap_epic_ranks WHERE project_id = $1`, projectID); err != nil {
		return internal("failed to clear the epic order", err)
	}
	if len(epicIDs) > 0 {
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO roadmap_epic_ranks (project_id, issue_id, position)
			SELECT $1, u.id, u.ord - 1 FROM unnest($2::uuid[]) WITH ORDINALITY AS u(id, ord)`, projectID, ids); err != nil {
			return internal("failed to store the epic order", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return internal("failed to commit the epic order", err)
	}
	return nil
}
