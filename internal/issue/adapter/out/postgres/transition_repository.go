package postgres

import (
	"context"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"

	"github.com/jhonsferg/corelog/internal/issue/application/port"
	"github.com/jhonsferg/corelog/internal/platform/apperror"
	"github.com/jhonsferg/corelog/internal/platform/database"
)

// TransitionRepository writes the status part of workflow transitions.
type TransitionRepository struct {
	db *sqlx.DB
}

func NewTransitionRepository(db *sqlx.DB) *TransitionRepository {
	return &TransitionRepository{db: db}
}

var _ port.TransitionRepository = (*TransitionRepository)(nil)

// OpenSubtasks counts the live children of the issue that are not in a done status.
func (r *TransitionRepository) OpenSubtasks(ctx context.Context, issueID uuid.UUID) (int, error) {
	var count int
	if err := sqlx.GetContext(ctx, database.Executor(ctx, r.db), &count, `
		SELECT count(*)
		FROM issues c JOIN statuses st ON st.id = c.status_id
		WHERE c.parent_id = $1 AND c.deleted_at IS NULL AND st.category <> 'done'`, issueID); err != nil {
		return 0, apperror.Wrap(apperror.KindInternal, "failed to count open subtasks", err)
	}
	return count, nil
}

// StatusInfo returns the name and category of a status.
func (r *TransitionRepository) StatusInfo(ctx context.Context, statusID uuid.UUID) (string, string, error) {
	var row struct {
		Name     string `db:"name"`
		Category string `db:"category"`
	}
	if err := sqlx.GetContext(ctx, database.Executor(ctx, r.db), &row, `SELECT name, category FROM statuses WHERE id = $1`, statusID); err != nil {
		return "", "", apperror.Wrap(apperror.KindInternal, "failed to read status", err)
	}
	return row.Name, row.Category, nil
}

// ApplyStatus moves the issue to the target status, keeps the legacy status mirror, applies the resolution
// change and appends the status history row. It reports false when the issue is no longer in FromStatusID.
func (r *TransitionRepository) ApplyStatus(ctx context.Context, w port.TransitionWrite) (bool, error) {
	exec := database.Executor(ctx, r.db)
	result, err := exec.ExecContext(ctx, `
		UPDATE issues
		SET status_id = $3, status = $4::varchar,
		    closed_at = CASE WHEN $4::text = 'closed' THEN COALESCE(closed_at, $5::timestamptz) ELSE NULL END,
		    resolution_id = CASE $6::text WHEN 'set' THEN $7::uuid WHEN 'clear' THEN NULL ELSE resolution_id END,
		    resolved_at = CASE
		        WHEN $6::text = 'set' THEN $5::timestamptz
		        WHEN $6::text = 'clear' THEN NULL
		        WHEN $8::boolean THEN COALESCE(resolved_at, $5::timestamptz)
		        ELSE NULL END,
		    updated_at = $5::timestamptz
		WHERE id = $1 AND status_id = $2 AND deleted_at IS NULL`,
		w.IssueID, w.FromStatusID, w.ToStatusID, string(w.Legacy), w.At, w.ResolutionAction, w.ResolutionID, w.ToDone)
	if err != nil {
		return false, apperror.Wrap(apperror.KindInternal, "failed to apply transition", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return false, apperror.Wrap(apperror.KindInternal, "failed to apply transition", err)
	}
	if affected == 0 {
		return false, nil
	}
	if _, err := exec.ExecContext(ctx, `
		INSERT INTO issue_status_history (issue_id, from_status_id, to_status_id, changed_by, changed_at, transition_id)
		VALUES ($1, $2, $3, $4, $5, $6)`,
		w.IssueID, w.FromStatusID, w.ToStatusID, w.ActorID, w.At, w.TransitionID); err != nil {
		return false, apperror.Wrap(apperror.KindInternal, "failed to write status history", err)
	}
	return true, nil
}
