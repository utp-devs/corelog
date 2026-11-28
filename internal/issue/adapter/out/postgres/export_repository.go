package postgres

import (
	"context"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"

	"github.com/jhonsferg/corelog/internal/platform/apperror"
	"github.com/jhonsferg/corelog/internal/platform/database"
)

// LabelNamesByIssue returns the sorted label names of each of the issues in one statement.
func (r *OperationsRepository) LabelNamesByIssue(ctx context.Context, issueIDs []uuid.UUID) (map[uuid.UUID][]string, error) {
	out := make(map[uuid.UUID][]string, len(issueIDs))
	if len(issueIDs) == 0 {
		return out, nil
	}
	list, args := idList(issueIDs, 0)
	var rows []struct {
		IssueID uuid.UUID `db:"issue_id"`
		Name    string    `db:"name"`
	}
	if err := sqlx.SelectContext(ctx, database.Executor(ctx, r.db), &rows, `
		SELECT il.issue_id, l.name
		FROM issue_labels il JOIN labels l ON l.id = il.label_id AND l.deleted_at IS NULL
		WHERE il.issue_id IN (`+list+`) AND l.deleted_at IS NULL
		ORDER BY il.issue_id, l.name`, args...); err != nil {
		return nil, apperror.Wrap(apperror.KindInternal, "failed to read labels of the export", err)
	}
	for _, row := range rows {
		out[row.IssueID] = append(out[row.IssueID], row.Name)
	}
	return out, nil
}
