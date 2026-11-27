// Package postgres implements the worklog repository on top of PostgreSQL. The module owns the worklogs
// table; it also reads issues and projects and updates the time columns of issues (estimates and the
// time spent aggregate) through the statements below, which are the only writes to those columns.
package postgres

import (
	"context"
	"database/sql"
	"errors"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"

	"github.com/jhonsferg/corelog/internal/platform/apperror"
	"github.com/jhonsferg/corelog/internal/platform/database"
	"github.com/jhonsferg/corelog/internal/worklog/application/port"
	"github.com/jhonsferg/corelog/internal/worklog/domain"
)

// Repository is the PostgreSQL adapter of port.Repository.
type Repository struct {
	db *sqlx.DB
}

var _ port.Repository = (*Repository)(nil)

// NewRepository returns a repository bound to db. Statements run inside the transaction of the context
// when there is one.
func NewRepository(db *sqlx.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) ext(ctx context.Context) sqlx.ExtContext {
	return database.Executor(ctx, r.db)
}

func internal(msg string, err error) error {
	return apperror.Wrap(apperror.KindInternal, msg, err)
}

// selectWorklog is the shared projection of worklogs with their issue, project and author.
const selectWorklog = `
	SELECT w.id, w.issue_id, p.key || '-' || i.key_number AS issue_key, i.summary AS issue_title,
	       i.project_id, p.organization_id, w.author_id, u.display_name AS author_name,
	       w.started_at, w.time_spent_seconds, w.description, w.remaining_mode,
	       w.remaining_adjust_seconds, w.created_at, w.updated_at, w.deleted_at
	FROM worklogs w
	JOIN issues i ON i.id = w.issue_id
	JOIN projects p ON p.id = i.project_id
	JOIN users u ON u.id = w.author_id`

type worklogRow struct {
	ID                     uuid.UUID  `db:"id"`
	IssueID                uuid.UUID  `db:"issue_id"`
	IssueKey               string     `db:"issue_key"`
	IssueTitle             string     `db:"issue_title"`
	ProjectID              uuid.UUID  `db:"project_id"`
	OrganizationID         uuid.UUID  `db:"organization_id"`
	AuthorID               uuid.UUID  `db:"author_id"`
	AuthorName             string     `db:"author_name"`
	StartedAt              time.Time  `db:"started_at"`
	TimeSpentSeconds       int64      `db:"time_spent_seconds"`
	Description            string     `db:"description"`
	RemainingMode          string     `db:"remaining_mode"`
	RemainingAdjustSeconds int64      `db:"remaining_adjust_seconds"`
	CreatedAt              time.Time  `db:"created_at"`
	UpdatedAt              time.Time  `db:"updated_at"`
	DeletedAt              *time.Time `db:"deleted_at"`
}

func (row worklogRow) toDomain() domain.Worklog {
	return domain.Worklog{
		ID: row.ID, IssueID: row.IssueID, IssueKey: row.IssueKey, IssueTitle: row.IssueTitle,
		ProjectID: row.ProjectID, OrganizationID: row.OrganizationID, AuthorID: row.AuthorID, AuthorName: row.AuthorName,
		StartedAt: row.StartedAt.UTC(), TimeSpentSeconds: row.TimeSpentSeconds, Description: row.Description,
		RemainingMode: domain.RemainingMode(row.RemainingMode), RemainingAdjustSeconds: row.RemainingAdjustSeconds,
		CreatedAt: row.CreatedAt.UTC(), UpdatedAt: row.UpdatedAt.UTC(), DeletedAt: row.DeletedAt,
	}
}

type issueTimeRow struct {
	IssueID        uuid.UUID `db:"issue_id"`
	IssueKey       string    `db:"issue_key"`
	IssueTitle     string    `db:"issue_title"`
	ProjectID      uuid.UUID `db:"project_id"`
	OrganizationID uuid.UUID `db:"organization_id"`
	Original       *int64    `db:"original_estimate_seconds"`
	Remaining      *int64    `db:"remaining_estimate_seconds"`
	Spent          int64     `db:"time_spent_seconds"`
}

// IssueTime reads the time data of a live issue. With lock it takes the issue row lock until the end of
// the transaction, which serializes every change of estimates and worklogs on that issue.
func (r *Repository) IssueTime(ctx context.Context, issueID uuid.UUID, lock bool) (port.IssueTime, error) {
	q := `
		SELECT i.id AS issue_id, p.key || '-' || i.key_number AS issue_key, i.summary AS issue_title,
		       i.project_id, p.organization_id, i.original_estimate_seconds, i.remaining_estimate_seconds,
		       i.time_spent_seconds
		FROM issues i
		JOIN projects p ON p.id = i.project_id
		WHERE i.id = $1 AND i.deleted_at IS NULL AND p.deleted_at IS NULL`
	if lock {
		q += " FOR UPDATE OF i"
	}
	var row issueTimeRow
	if err := sqlx.GetContext(ctx, r.ext(ctx), &row, q, issueID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return port.IssueTime{}, domain.ErrIssueNotFound
		}
		return port.IssueTime{}, internal("failed to load issue time", err)
	}
	return port.IssueTime{
		IssueID: row.IssueID, IssueKey: row.IssueKey, IssueTitle: row.IssueTitle,
		ProjectID: row.ProjectID, OrganizationID: row.OrganizationID,
		Original: row.Original, Remaining: row.Remaining, Spent: row.Spent,
	}, nil
}

// Worklog returns a live worklog with its issue and project data.
func (r *Repository) Worklog(ctx context.Context, id uuid.UUID, lock bool) (domain.Worklog, error) {
	q := selectWorklog + `
		WHERE w.id = $1 AND w.deleted_at IS NULL AND i.deleted_at IS NULL AND p.deleted_at IS NULL`
	if lock {
		q += " FOR UPDATE OF w"
	}
	var row worklogRow
	if err := sqlx.GetContext(ctx, r.ext(ctx), &row, q, id); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return domain.Worklog{}, domain.ErrWorklogNotFound
		}
		return domain.Worklog{}, internal("failed to load worklog", err)
	}
	return row.toDomain(), nil
}

// ListIssueWorklogs returns the live worklogs of an issue, newest start first.
func (r *Repository) ListIssueWorklogs(ctx context.Context, issueID uuid.UUID) ([]domain.Worklog, error) {
	var rows []worklogRow
	q := selectWorklog + `
		WHERE w.issue_id = $1 AND w.deleted_at IS NULL
		ORDER BY w.started_at DESC, w.id`
	if err := sqlx.SelectContext(ctx, r.ext(ctx), &rows, q, issueID); err != nil {
		return nil, internal("failed to list worklogs", err)
	}
	out := make([]domain.Worklog, 0, len(rows))
	for _, row := range rows {
		out = append(out, row.toDomain())
	}
	return out, nil
}

// InsertWorklog stores a new worklog; the database assigns the id and the timestamps.
func (r *Repository) InsertWorklog(ctx context.Context, w *domain.Worklog) error {
	err := r.ext(ctx).QueryRowxContext(ctx, `
		INSERT INTO worklogs (issue_id, author_id, started_at, time_spent_seconds, description, remaining_mode, remaining_adjust_seconds)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id, created_at, updated_at`,
		w.IssueID, w.AuthorID, w.StartedAt, w.TimeSpentSeconds, w.Description, string(w.RemainingMode), w.RemainingAdjustSeconds,
	).Scan(&w.ID, &w.CreatedAt, &w.UpdatedAt)
	if err != nil {
		return internal("failed to insert worklog", err)
	}
	w.CreatedAt = w.CreatedAt.UTC()
	w.UpdatedAt = w.UpdatedAt.UTC()
	return nil
}

// UpdateWorklog stores the editable fields and the remaining adjust of a live worklog.
func (r *Repository) UpdateWorklog(ctx context.Context, w *domain.Worklog) error {
	err := r.ext(ctx).QueryRowxContext(ctx, `
		UPDATE worklogs
		SET started_at = $2, time_spent_seconds = $3, description = $4, remaining_adjust_seconds = $5, updated_at = now()
		WHERE id = $1 AND deleted_at IS NULL
		RETURNING updated_at`,
		w.ID, w.StartedAt, w.TimeSpentSeconds, w.Description, w.RemainingAdjustSeconds,
	).Scan(&w.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.ErrWorklogNotFound
	}
	if err != nil {
		return internal("failed to update worklog", err)
	}
	w.UpdatedAt = w.UpdatedAt.UTC()
	return nil
}

// SoftDeleteWorklog marks a live worklog as deleted.
func (r *Repository) SoftDeleteWorklog(ctx context.Context, id uuid.UUID, at time.Time) error {
	res, err := r.ext(ctx).ExecContext(ctx, `
		UPDATE worklogs SET deleted_at = $2, updated_at = $2 WHERE id = $1 AND deleted_at IS NULL`, id, at)
	if err != nil {
		return internal("failed to delete worklog", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return domain.ErrWorklogNotFound
	}
	return nil
}

// RecomputeSpent sets issues.time_spent_seconds from the live worklogs of the issue.
func (r *Repository) RecomputeSpent(ctx context.Context, issueID uuid.UUID) error {
	_, err := r.ext(ctx).ExecContext(ctx, `
		UPDATE issues
		SET time_spent_seconds = (SELECT COALESCE(SUM(w.time_spent_seconds), 0) FROM worklogs w
		                          WHERE w.issue_id = $1 AND w.deleted_at IS NULL)
		WHERE id = $1`, issueID)
	if err != nil {
		return internal("failed to recompute time spent", err)
	}
	return nil
}

// SetRemaining sets issues.remaining_estimate_seconds (nil clears it).
func (r *Repository) SetRemaining(ctx context.Context, issueID uuid.UUID, value *int64) error {
	_, err := r.ext(ctx).ExecContext(ctx, `UPDATE issues SET remaining_estimate_seconds = $2 WHERE id = $1`, issueID, value)
	if err != nil {
		return internal("failed to set remaining estimate", err)
	}
	return nil
}

// SetOriginal sets issues.original_estimate_seconds (nil clears it).
func (r *Repository) SetOriginal(ctx context.Context, issueID uuid.UUID, value *int64) error {
	_, err := r.ext(ctx).ExecContext(ctx, `UPDATE issues SET original_estimate_seconds = $2 WHERE id = $1`, issueID, value)
	if err != nil {
		return internal("failed to set original estimate", err)
	}
	return nil
}

// Window returns the live worklogs whose start falls in [Lo, Hi), optionally by author or project.
func (r *Repository) Window(ctx context.Context, w port.Window) ([]domain.Entry, error) {
	var author, project any
	if w.AuthorID != nil {
		author = *w.AuthorID
	}
	if w.ProjectID != nil {
		project = *w.ProjectID
	}
	var rows []worklogRow
	q := selectWorklog + `
		WHERE w.deleted_at IS NULL AND i.deleted_at IS NULL AND p.deleted_at IS NULL
		  AND w.started_at >= $1 AND w.started_at < $2
		  AND ($3::uuid IS NULL OR w.author_id = $3::uuid)
		  AND ($4::uuid IS NULL OR i.project_id = $4::uuid)
		ORDER BY w.started_at, w.id`
	if err := sqlx.SelectContext(ctx, r.ext(ctx), &rows, q, w.Lo, w.Hi, author, project); err != nil {
		return nil, internal("failed to list worklogs in range", err)
	}
	out := make([]domain.Entry, 0, len(rows))
	for _, row := range rows {
		out = append(out, domain.Entry{
			ID: row.ID, IssueID: row.IssueID, IssueKey: row.IssueKey, IssueTitle: row.IssueTitle,
			ProjectID: row.ProjectID, OrganizationID: row.OrganizationID, AuthorID: row.AuthorID, AuthorName: row.AuthorName,
			StartedAt: row.StartedAt.UTC(), Seconds: row.TimeSpentSeconds, Description: row.Description,
		})
	}
	return out, nil
}

type orgTimeRow struct {
	ID       uuid.UUID `db:"id"`
	Timezone string    `db:"timezone"`
	Hours    *string   `db:"hours"`
	Days     *string   `db:"days"`
}

// OrgTimes reads the working-time settings of the organizations. Missing or invalid values fall back to the
// defaults (8 hours a day, 5 days a week, UTC).
func (r *Repository) OrgTimes(ctx context.Context, orgIDs []uuid.UUID) (map[uuid.UUID]domain.OrgTime, error) {
	ids := make([]string, 0, len(orgIDs))
	for _, id := range orgIDs {
		ids = append(ids, id.String())
	}
	var rows []orgTimeRow
	err := sqlx.SelectContext(ctx, r.ext(ctx), &rows, `
		SELECT id, COALESCE(NULLIF(settings->>'timezone', ''), 'UTC') AS timezone,
		       settings->>'hours_per_day' AS hours, settings->>'days_per_week' AS days
		FROM organizations
		WHERE id = ANY($1::uuid[])`, ids)
	if err != nil {
		return nil, internal("failed to load organization settings", err)
	}
	out := make(map[uuid.UUID]domain.OrgTime, len(rows))
	for _, row := range rows {
		loc, err := time.LoadLocation(row.Timezone)
		if err != nil {
			loc = time.UTC
		}
		cfg := domain.DefaultConfig()
		if h, ok := parseFloat(row.Hours); ok {
			cfg.HoursPerDay = h
		}
		if d, ok := parseFloat(row.Days); ok {
			cfg.DaysPerWeek = d
		}
		if !cfg.Valid() {
			cfg = domain.DefaultConfig()
		}
		out[row.ID] = domain.OrgTime{Location: loc, Config: cfg}
	}
	return out, nil
}

func parseFloat(v *string) (float64, bool) {
	if v == nil {
		return 0, false
	}
	f, err := strconv.ParseFloat(*v, 64)
	return f, err == nil
}

// AddActivities stores activity rows in the issue history, the table the activity feed reads.
func (r *Repository) AddActivities(ctx context.Context, rows []port.Activity) error {
	ext := r.ext(ctx)
	for _, a := range rows {
		_, err := ext.ExecContext(ctx, `
			INSERT INTO activities (id, issue_id, actor_id, action, field_name, old_value, new_value, created_at, batch_id)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`,
			uuid.New(), a.IssueID, a.ActorID, a.Action, a.Field, a.OldValue, a.NewValue, a.At, a.BatchID)
		if err != nil {
			return internal("failed to record activity", err)
		}
	}
	return nil
}
