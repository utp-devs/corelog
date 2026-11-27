package postgres

import (
	"context"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"

	"github.com/jhonsferg/corelog/internal/platform/apperror"
	"github.com/jhonsferg/corelog/internal/platform/database"
)

// ProjectLabelIDs returns the names of the given labels that belong to the project.
func (r *OperationsRepository) ProjectLabelIDs(ctx context.Context, projectID uuid.UUID, labelIDs []uuid.UUID) (map[uuid.UUID]string, error) {
	out := make(map[uuid.UUID]string, len(labelIDs))
	if len(labelIDs) == 0 {
		return out, nil
	}
	list, args := idList(labelIDs, 1)
	args = append([]any{projectID}, args...)
	var rows []struct {
		ID   uuid.UUID `db:"id"`
		Name string    `db:"name"`
	}
	if err := sqlx.SelectContext(ctx, database.Executor(ctx, r.db), &rows,
		"SELECT id, name FROM labels WHERE project_id = $1 AND id IN ("+list+")", args...); err != nil {
		return nil, apperror.Wrap(apperror.KindInternal, "failed to read labels", err)
	}
	for _, row := range rows {
		out[row.ID] = row.Name
	}
	return out, nil
}

// IssueLabelNames returns the sorted names of the labels of an issue.
func (r *OperationsRepository) IssueLabelNames(ctx context.Context, issueID uuid.UUID) ([]string, error) {
	var names []string
	if err := sqlx.SelectContext(ctx, database.Executor(ctx, r.db), &names, `
		SELECT l.name FROM issue_labels il JOIN labels l ON l.id = il.label_id
		WHERE il.issue_id = $1 ORDER BY l.name`, issueID); err != nil {
		return nil, apperror.Wrap(apperror.KindInternal, "failed to read issue labels", err)
	}
	return names, nil
}

// AddIssueLabels attaches the labels to the issue. Labels already attached are left as they are.
func (r *OperationsRepository) AddIssueLabels(ctx context.Context, issueID uuid.UUID, labelIDs []uuid.UUID) error {
	exec := database.Executor(ctx, r.db)
	for _, labelID := range labelIDs {
		if _, err := exec.ExecContext(ctx, `
			INSERT INTO issue_labels (issue_id, label_id) VALUES ($1, $2)
			ON CONFLICT (issue_id, label_id) DO NOTHING`, issueID, labelID); err != nil {
			return apperror.Wrap(apperror.KindInternal, "failed to add labels", err)
		}
	}
	return nil
}

// RemoveIssueLabels detaches the labels from the issue. Labels that are not attached are ignored.
func (r *OperationsRepository) RemoveIssueLabels(ctx context.Context, issueID uuid.UUID, labelIDs []uuid.UUID) error {
	list, args := idList(labelIDs, 1)
	args = append([]any{issueID}, args...)
	if _, err := database.Executor(ctx, r.db).ExecContext(ctx,
		"DELETE FROM issue_labels WHERE issue_id = $1 AND label_id IN ("+list+")", args...); err != nil {
		return apperror.Wrap(apperror.KindInternal, "failed to remove labels", err)
	}
	return nil
}
