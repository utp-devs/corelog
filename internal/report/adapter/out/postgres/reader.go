// Package postgres reads the history tables the reports are computed from. Every query is parameterized,
// project scoped and bounded by a row cap; the number of statements per report does not depend on the data.
package postgres

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"

	"github.com/jhonsferg/corelog/internal/platform/apperror"
	"github.com/jhonsferg/corelog/internal/report/application/port"
	"github.com/jhonsferg/corelog/internal/report/domain"
)

// Reader implements port.Reader over PostgreSQL.
type Reader struct {
	db sqlx.ExtContext
}

var _ port.Reader = (*Reader)(nil)

// NewReader builds the reader. It accepts a *sqlx.DB or any sqlx.ExtContext (tests count statements).
func NewReader(db sqlx.ExtContext) *Reader { return &Reader{db: db} }

func internalErr(msg string, err error) error {
	return apperror.Wrap(apperror.KindInternal, msg, err)
}

func idStrings(ids *[]uuid.UUID) (bool, []string) {
	if ids == nil {
		return false, []string{}
	}
	out := make([]string, len(*ids))
	for i, id := range *ids {
		out[i] = id.String()
	}
	return true, out
}

// Project returns the project with the time zone of its organization.
func (r *Reader) Project(ctx context.Context, projectID uuid.UUID) (domain.ProjectInfo, error) {
	var row struct {
		ID       uuid.UUID `db:"id"`
		Key      string    `db:"key"`
		Name     string    `db:"name"`
		Timezone string    `db:"timezone"`
	}
	err := sqlx.GetContext(ctx, r.db, &row, `
		SELECT p.id, p.key, p.name, COALESCE(NULLIF(o.settings->>'timezone', ''), 'UTC') AS timezone
		FROM projects p
		JOIN organizations o ON o.id = p.organization_id
		WHERE p.id = $1 AND p.deleted_at IS NULL`, projectID)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.ProjectInfo{}, apperror.WithCode(apperror.KindNotFound, "project.not_found", "El proyecto no existe")
	}
	if err != nil {
		return domain.ProjectInfo{}, internalErr("failed to read the project", err)
	}
	return domain.ProjectInfo{ID: row.ID, Key: row.Key, Name: row.Name, Timezone: row.Timezone}, nil
}

type sprintRow struct {
	ID          uuid.UUID  `db:"id"`
	Name        string     `db:"name"`
	Status      string     `db:"status"`
	StartDate   *time.Time `db:"start_date"`
	EndDate     *time.Time `db:"end_date"`
	StartedAt   *time.Time `db:"started_at"`
	CompletedAt *time.Time `db:"completed_at"`
	Committed   *float64   `db:"committed_points"`
	Completed   *float64   `db:"completed_points"`
}

const sprintColumns = `s.id, s.name, s.status, s.start_date, s.end_date, s.started_at, s.completed_at, s.committed_points, s.completed_points`

func (s sprintRow) toDomain() domain.SprintInfo {
	return domain.SprintInfo{ID: s.ID, Name: s.Name, Status: s.Status, StartDate: s.StartDate, EndDate: s.EndDate,
		StartedAt: s.StartedAt, CompletedAt: s.CompletedAt, Committed: s.Committed, Completed: s.Completed}
}

// Sprint returns the sprint of the project; without id the active sprint, else the latest started one.
func (r *Reader) Sprint(ctx context.Context, projectID uuid.UUID, id *uuid.UUID) (domain.SprintInfo, error) {
	var row sprintRow
	err := sqlx.GetContext(ctx, r.db, &row, `
		SELECT `+sprintColumns+`
		FROM sprints s
		WHERE s.project_id = $1 AND ($2::uuid IS NULL OR s.id = $2::uuid)
		ORDER BY (s.status = 'active') DESC, COALESCE(s.started_at, s.created_at) DESC, s.id
		LIMIT 1`, projectID, id)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.SprintInfo{}, domain.ErrSprintNotFound
	}
	if err != nil {
		return domain.SprintInfo{}, internalErr("failed to read the sprint", err)
	}
	return row.toDomain(), nil
}

// RecentSprints returns the latest closed sprints, the most recent first.
func (r *Reader) RecentSprints(ctx context.Context, projectID uuid.UUID, limit int) ([]domain.SprintInfo, error) {
	var rows []sprintRow
	if err := sqlx.SelectContext(ctx, r.db, &rows, `
		SELECT `+sprintColumns+`
		FROM sprints s
		WHERE s.project_id = $1 AND s.status = 'closed'
		ORDER BY s.completed_at DESC NULLS LAST, s.id
		LIMIT $2`, projectID, limit); err != nil {
		return nil, internalErr("failed to read the sprints", err)
	}
	out := make([]domain.SprintInfo, 0, len(rows))
	for _, row := range rows {
		out = append(out, row.toDomain())
	}
	return out, nil
}

// ScopeEvents returns the scope history of a sprint ordered by time.
func (r *Reader) ScopeEvents(ctx context.Context, sprintID uuid.UUID) ([]domain.ScopeEvent, error) {
	var rows []struct {
		IssueID    uuid.UUID `db:"issue_id"`
		Kind       string    `db:"event"`
		Points     *float64  `db:"points_at_time"`
		At         time.Time `db:"happened_at"`
		AfterStart bool      `db:"after_start"`
	}
	if err := sqlx.SelectContext(ctx, r.db, &rows, `
		SELECT e.issue_id, e.event, e.points_at_time, e.happened_at, e.after_start
		FROM sprint_issue_events e
		WHERE e.sprint_id = $1
		ORDER BY e.happened_at, e.id
		LIMIT $2`, sprintID, port.MaxScopeEventRows+1); err != nil {
		return nil, internalErr("failed to read the sprint scope history", err)
	}
	if len(rows) > port.MaxScopeEventRows {
		return nil, domain.ErrTooManyRows
	}
	out := make([]domain.ScopeEvent, 0, len(rows))
	for _, row := range rows {
		out = append(out, domain.ScopeEvent{IssueID: row.IssueID, Kind: row.Kind, Points: row.Points, At: row.At, AfterStart: row.AfterStart})
	}
	return out, nil
}

// Version returns a version of the project.
func (r *Reader) Version(ctx context.Context, projectID, versionID uuid.UUID) (domain.VersionInfo, error) {
	var row struct {
		ID          uuid.UUID  `db:"id"`
		Name        string     `db:"name"`
		Description string     `db:"description"`
		Status      string     `db:"status"`
		StartDate   *time.Time `db:"start_date"`
		ReleaseDate *time.Time `db:"release_date"`
	}
	err := sqlx.GetContext(ctx, r.db, &row, `
		SELECT v.id, v.name, v.description, v.status, v.start_date, v.release_date
		FROM versions v WHERE v.id = $1 AND v.project_id = $2`, versionID, projectID)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.VersionInfo{}, domain.ErrVersionNotFound
	}
	if err != nil {
		return domain.VersionInfo{}, internalErr("failed to read the version", err)
	}
	return domain.VersionInfo{ID: row.ID, Name: row.Name, Description: row.Description, Status: row.Status,
		StartDate: row.StartDate, ReleaseDate: row.ReleaseDate}, nil
}

// Issues returns the live issues of the project that match the query, oldest first.
func (r *Reader) Issues(ctx context.Context, q port.IssueQuery) ([]domain.IssueRow, error) {
	restrict, ids := idStrings(q.OnlyIDs)
	var rows []struct {
		ID           uuid.UUID  `db:"id"`
		Key          string     `db:"key"`
		Summary      string     `db:"summary"`
		TypeName     string     `db:"type_name"`
		PriorityName string     `db:"priority_name"`
		AssigneeID   *uuid.UUID `db:"assignee_id"`
		AssigneeName string     `db:"assignee_name"`
		StatusName   string     `db:"status_name"`
		Category     string     `db:"category"`
		CreatedAt    time.Time  `db:"created_at"`
		ResolvedAt   *time.Time `db:"resolved"`
		Points       *float64   `db:"story_points"`
		OriginalEst  *int64     `db:"original_estimate_seconds"`
		RemainingEst *int64     `db:"remaining_estimate_seconds"`
	}
	err := sqlx.SelectContext(ctx, r.db, &rows, `
		WITH base AS (
			SELECT i.id, p.key || '-' || i.key_number AS key, i.summary, it.name AS type_name, pr.name AS priority_name,
			       i.assignee_id, COALESCE(u.display_name, u.name, '') AS assignee_name, COALESCE(st.name, '') AS status_name,
			       corelog_issue_category(i.status_id, i.status) AS category, i.created_at,
			       CASE WHEN corelog_issue_category(i.status_id, i.status) = 'done'
			            THEN COALESCE(i.resolved_at, i.closed_at, i.updated_at) END AS resolved,
			       i.story_points, i.original_estimate_seconds, i.remaining_estimate_seconds
			FROM issues i
			JOIN projects p ON p.id = i.project_id AND p.deleted_at IS NULL
			JOIN issue_types it ON it.id = i.issue_type_id
			JOIN priorities pr ON pr.id = i.priority_id
			LEFT JOIN statuses st ON st.id = i.status_id
			LEFT JOIN users u ON u.id = i.assignee_id
			WHERE i.project_id = $1 AND i.deleted_at IS NULL
			  AND (NOT $2::boolean OR i.id = ANY($3::uuid[]))
			  AND ($4::uuid IS NULL OR i.sprint_id = $4::uuid)
			  AND ($5::uuid IS NULL OR EXISTS (
			        SELECT 1 FROM issue_versions iv WHERE iv.issue_id = i.id AND iv.version_id = $5::uuid AND iv.kind = 'fix'))
			  AND ($6::timestamptz IS NULL OR i.created_at < $6::timestamptz)
		)
		SELECT id, key, summary, type_name, priority_name, assignee_id, assignee_name, status_name, category,
		       created_at, resolved, story_points, original_estimate_seconds, remaining_estimate_seconds
		FROM base
		WHERE (NOT $7::boolean OR category <> 'done')
		  AND ($8::timestamptz IS NULL OR created_at >= $8::timestamptz OR resolved >= $8::timestamptz)
		ORDER BY created_at, id
		LIMIT $9`,
		q.ProjectID, restrict, ids, q.SprintID, q.VersionID, q.CreatedBefore, q.OpenOnly, q.TouchedSince, port.MaxIssues+1)
	if err != nil {
		return nil, internalErr("failed to read the issues", err)
	}
	if len(rows) > port.MaxIssues {
		return nil, domain.ErrTooManyRows
	}
	out := make([]domain.IssueRow, 0, len(rows))
	for _, row := range rows {
		out = append(out, domain.IssueRow{ID: row.ID, Key: row.Key, Summary: row.Summary, TypeName: row.TypeName,
			PriorityName: row.PriorityName, AssigneeID: row.AssigneeID, AssigneeName: row.AssigneeName, StatusName: row.StatusName,
			Category: row.Category, CreatedAt: row.CreatedAt, ResolvedAt: row.ResolvedAt, Points: row.Points,
			OriginalEst: row.OriginalEst, RemainingEst: row.RemainingEst})
	}
	return out, nil
}

// StatusHistory returns the transitions of the live issues of the project made before the instant.
func (r *Reader) StatusHistory(ctx context.Context, q port.HistoryQuery) ([]domain.StatusChange, error) {
	restrict, ids := idStrings(q.OnlyIDs)
	var rows []struct {
		IssueID  uuid.UUID `db:"issue_id"`
		At       time.Time `db:"changed_at"`
		Category string    `db:"category"`
	}
	if err := sqlx.SelectContext(ctx, r.db, &rows, `
		SELECT h.issue_id, h.changed_at, s.category
		FROM issue_status_history h
		JOIN statuses s ON s.id = h.to_status_id
		JOIN issues i ON i.id = h.issue_id AND i.deleted_at IS NULL
		WHERE i.project_id = $1 AND h.changed_at < $2
		  AND (NOT $3::boolean OR h.issue_id = ANY($4::uuid[]))
		ORDER BY h.issue_id, h.changed_at, h.id
		LIMIT $5`, q.ProjectID, q.Before, restrict, ids, port.MaxHistoryRows+1); err != nil {
		return nil, internalErr("failed to read the status history", err)
	}
	if len(rows) > port.MaxHistoryRows {
		return nil, domain.ErrTooManyRows
	}
	out := make([]domain.StatusChange, 0, len(rows))
	for _, row := range rows {
		out = append(out, domain.StatusChange{IssueID: row.IssueID, At: row.At, Category: row.Category})
	}
	return out, nil
}

// Worklogs returns the live worklogs of the project started inside [Lo, Hi).
func (r *Reader) Worklogs(ctx context.Context, q port.WorklogQuery) ([]domain.WorklogRow, error) {
	restrict, ids := idStrings(q.OnlyIDs)
	var rows []struct {
		IssueID      uuid.UUID `db:"issue_id"`
		Key          string    `db:"key"`
		Summary      string    `db:"summary"`
		AuthorID     uuid.UUID `db:"author_id"`
		AuthorName   string    `db:"author_name"`
		StartedAt    time.Time `db:"started_at"`
		Seconds      int64     `db:"time_spent_seconds"`
		OriginalEst  *int64    `db:"original_estimate_seconds"`
		RemainingEst *int64    `db:"remaining_estimate_seconds"`
	}
	if err := sqlx.SelectContext(ctx, r.db, &rows, `
		SELECT w.issue_id, p.key || '-' || i.key_number AS key, i.summary, w.author_id,
		       COALESCE(u.display_name, u.name, '') AS author_name, w.started_at, w.time_spent_seconds,
		       i.original_estimate_seconds, i.remaining_estimate_seconds
		FROM worklogs w
		JOIN issues i ON i.id = w.issue_id AND i.deleted_at IS NULL
		JOIN projects p ON p.id = i.project_id AND p.deleted_at IS NULL
		JOIN users u ON u.id = w.author_id
		WHERE i.project_id = $1 AND w.deleted_at IS NULL AND w.started_at >= $2 AND w.started_at < $3
		  AND (NOT $4::boolean OR w.issue_id = ANY($5::uuid[]))
		  AND ($6::uuid IS NULL OR i.sprint_id = $6::uuid)
		ORDER BY w.started_at, w.id
		LIMIT $7`, q.ProjectID, q.Lo, q.Hi, restrict, ids, q.SprintID, port.MaxWorklogRows+1); err != nil {
		return nil, internalErr("failed to read the worklogs", err)
	}
	if len(rows) > port.MaxWorklogRows {
		return nil, domain.ErrTooManyRows
	}
	out := make([]domain.WorklogRow, 0, len(rows))
	for _, row := range rows {
		out = append(out, domain.WorklogRow{IssueID: row.IssueID, IssueKey: row.Key, Summary: row.Summary, AuthorID: row.AuthorID,
			AuthorName: row.AuthorName, StartedAt: row.StartedAt, Seconds: row.Seconds, OriginalEst: row.OriginalEst, RemainingEst: row.RemainingEst})
	}
	return out, nil
}
