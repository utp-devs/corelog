package postgres

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"

	"github.com/jhonsferg/corelog/internal/platform/apperror"
	"github.com/jhonsferg/corelog/internal/workflow/application/port"
	"github.com/jhonsferg/corelog/internal/workflow/domain"
)

var _ port.ManagementRepository = (*Repository)(nil)

// ProjectContext returns the organization and template of a live project.
func (r *Repository) ProjectContext(ctx context.Context, projectID uuid.UUID) (port.ProjectRef, error) {
	var row struct {
		ID             uuid.UUID `db:"id"`
		OrganizationID uuid.UUID `db:"organization_id"`
		Template       string    `db:"template"`
	}
	err := sqlx.GetContext(ctx, r.exec(ctx), &row, `
		SELECT id, organization_id, template FROM projects WHERE id = $1 AND deleted_at IS NULL`, projectID)
	if errors.Is(err, sql.ErrNoRows) {
		return port.ProjectRef{}, port.ErrProjectNotFound
	}
	if err != nil {
		return port.ProjectRef{}, apperror.Wrap(apperror.KindInternal, "failed to load project", err)
	}
	return port.ProjectRef{ID: row.ID, OrganizationID: row.OrganizationID, Template: row.Template}, nil
}

// LockWorkflow locks the workflow row of the project until the transaction ends.
func (r *Repository) LockWorkflow(ctx context.Context, projectID uuid.UUID) (int, error) {
	var version int
	err := sqlx.GetContext(ctx, r.exec(ctx), &version, `SELECT version FROM workflows WHERE project_id = $1 FOR UPDATE`, projectID)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, port.ErrWorkflowNotFound
	}
	if err != nil {
		return 0, apperror.Wrap(apperror.KindInternal, "failed to lock workflow", err)
	}
	return version, nil
}

// IssueCounts counts the issues of the project per status. Soft-deleted issues are included: they still hold
// their status and must move with it.
func (r *Repository) IssueCounts(ctx context.Context, projectID uuid.UUID) (map[uuid.UUID]int, error) {
	var rows []struct {
		StatusID uuid.UUID `db:"status_id"`
		Count    int       `db:"count"`
	}
	if err := sqlx.SelectContext(ctx, r.exec(ctx), &rows, `
		SELECT status_id, count(*) AS count FROM issues WHERE project_id = $1 GROUP BY status_id`, projectID); err != nil {
		return nil, apperror.Wrap(apperror.KindInternal, "failed to count issues", err)
	}
	out := make(map[uuid.UUID]int, len(rows))
	for _, row := range rows {
		out[row.StatusID] = row.Count
	}
	return out, nil
}

// MigrateIssues moves the issues of the scope from one status to another in one statement. Every moved issue
// gets a status history row and a "status_changed" activity. A nil project id in the scope means every project
// of the organization. The legacy status follows the new category through the trigger on issues.
func (r *Repository) MigrateIssues(ctx context.Context, scope port.MigrationScope, from, to port.OrgStatus, actorID uuid.UUID, at time.Time) (int, error) {
	var projectID any
	if scope.ProjectID != uuid.Nil {
		projectID = scope.ProjectID
	}
	var count int
	err := sqlx.GetContext(ctx, r.exec(ctx), &count, `
		WITH moved AS (
			UPDATE issues AS i
			SET status_id = $4::uuid,
			    closed_at = CASE WHEN corelog_legacy_status($5::text) = 'closed' THEN COALESCE(i.closed_at, $6::timestamptz) ELSE NULL END,
			    updated_at = $6::timestamptz
			WHERE i.status_id = $3::uuid
			  AND i.project_id IN (
			      SELECT p.id FROM projects p
			      WHERE p.organization_id = $1::uuid AND ($2::uuid IS NULL OR p.id = $2::uuid))
			RETURNING i.id
		), history AS (
			INSERT INTO issue_status_history (issue_id, from_status_id, to_status_id, changed_by, changed_at, transition_id)
			SELECT id, $3::uuid, $4::uuid, $7::uuid, $6::timestamptz, NULL FROM moved
			RETURNING 1
		), activity AS (
			INSERT INTO activities (id, issue_id, actor_id, action, field_name, old_value, new_value, created_at)
			SELECT gen_random_uuid(), id, $7::uuid, 'status_changed', 'status', $8::text, $9::text, $6::timestamptz FROM moved
			RETURNING 1
		)
		SELECT count(*) FROM moved`,
		scope.OrganizationID, projectID, from.ID, to.ID, string(to.Category), at, actorID, from.Name, to.Name)
	if err != nil {
		return 0, apperror.Wrap(apperror.KindInternal, "failed to move issues", err)
	}
	return count, nil
}

// StatusUsage counts the issues and the status history rows that reference the status.
func (r *Repository) StatusUsage(ctx context.Context, orgID, statusID uuid.UUID) (int, int, error) {
	var row struct {
		Issues  int `db:"issues"`
		History int `db:"history"`
	}
	err := sqlx.GetContext(ctx, r.exec(ctx), &row, `
		SELECT
			(SELECT count(*) FROM issues i JOIN projects p ON p.id = i.project_id
			 WHERE i.status_id = $2 AND p.organization_id = $1) AS issues,
			(SELECT count(*) FROM issue_status_history h
			 WHERE h.to_status_id = $2 OR h.from_status_id = $2) AS history`, orgID, statusID)
	if err != nil {
		return 0, 0, apperror.Wrap(apperror.KindInternal, "failed to count status usage", err)
	}
	return row.Issues, row.History, nil
}

// WorkflowsUsing returns the workflows of the organization whose definition contains the status.
func (r *Repository) WorkflowsUsing(ctx context.Context, orgID, statusID uuid.UUID) ([]port.Record, error) {
	var rows []struct {
		ProjectID  uuid.UUID `db:"project_id"`
		Definition []byte    `db:"definition"`
		Version    int       `db:"version"`
		UpdatedAt  time.Time `db:"updated_at"`
	}
	if err := sqlx.SelectContext(ctx, r.exec(ctx), &rows, `
		SELECT w.project_id, w.definition, w.version, w.updated_at
		FROM workflows w JOIN projects p ON p.id = w.project_id
		WHERE p.organization_id = $1
		  AND w.definition -> 'statuses' @> jsonb_build_array(jsonb_build_object('id', $2::text))
		ORDER BY w.project_id`, orgID, statusID.String()); err != nil {
		return nil, apperror.Wrap(apperror.KindInternal, "failed to load workflows", err)
	}
	out := make([]port.Record, 0, len(rows))
	for _, row := range rows {
		w, err := domain.Decode(row.Definition)
		if err != nil {
			return nil, apperror.Wrap(apperror.KindInternal, "stored workflow is unreadable", err)
		}
		out = append(out, port.Record{ProjectID: row.ProjectID, Workflow: w, Version: row.Version, UpdatedAt: row.UpdatedAt})
	}
	return out, nil
}

// RewriteStatusHistory points the history rows of one status at another status, so deleting the first one keeps
// the history.
func (r *Repository) RewriteStatusHistory(ctx context.Context, from, to uuid.UUID) error {
	exec := r.exec(ctx)
	if _, err := exec.ExecContext(ctx, `UPDATE issue_status_history SET to_status_id = $2 WHERE to_status_id = $1`, from, to); err != nil {
		return apperror.Wrap(apperror.KindInternal, "failed to rewrite status history", err)
	}
	if _, err := exec.ExecContext(ctx, `UPDATE issue_status_history SET from_status_id = $2 WHERE from_status_id = $1`, from, to); err != nil {
		return apperror.Wrap(apperror.KindInternal, "failed to rewrite status history", err)
	}
	return nil
}

// SetStatusPositions stores the position of each status in the order given.
func (r *Repository) SetStatusPositions(ctx context.Context, orgID uuid.UUID, ids []uuid.UUID) error {
	exec := r.exec(ctx)
	for position, id := range ids {
		if _, err := exec.ExecContext(ctx, `UPDATE statuses SET position = $3 WHERE id = $1 AND organization_id = $2`, id, orgID, position); err != nil {
			return apperror.Wrap(apperror.KindInternal, "failed to reorder statuses", err)
		}
	}
	return nil
}

// RefreshLegacyStatus recomputes the legacy status of the issues of a status after its category changed. The
// transaction-local flag tells the sync trigger to keep status_id; the flag is cleared again afterwards.
func (r *Repository) RefreshLegacyStatus(ctx context.Context, statusID uuid.UUID, category domain.Category, at time.Time) error {
	exec := r.exec(ctx)
	if _, err := exec.ExecContext(ctx, `SELECT set_config('corelog.keep_status_id', 'on', true)`); err != nil {
		return apperror.Wrap(apperror.KindInternal, "failed to refresh issues", err)
	}
	defer func() {
		_, _ = exec.ExecContext(ctx, `SELECT set_config('corelog.keep_status_id', '', true)`)
	}()
	if _, err := exec.ExecContext(ctx, `
		UPDATE issues
		SET status = corelog_legacy_status($2::text),
		    closed_at = CASE WHEN corelog_legacy_status($2::text) = 'closed' THEN COALESCE(closed_at, $3::timestamptz) ELSE NULL END
		WHERE status_id = $1 AND status <> corelog_legacy_status($2::text)`, statusID, string(category), at); err != nil {
		return apperror.Wrap(apperror.KindInternal, "failed to refresh issues", err)
	}
	return nil
}
