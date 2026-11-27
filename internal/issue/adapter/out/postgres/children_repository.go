package postgres

import (
	"context"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"

	"github.com/jhonsferg/corelog/internal/issue/application/port"
	"github.com/jhonsferg/corelog/internal/issue/domain"
	"github.com/jhonsferg/corelog/internal/platform/apperror"
	"github.com/jhonsferg/corelog/internal/platform/database"
)

var _ port.ChildReader = (*IssueRepository)(nil)

// ListChildren returns the live direct children of an issue ordered by rank.
func (r *IssueRepository) ListChildren(ctx context.Context, parentID uuid.UUID) ([]*domain.Issue, error) {
	var rows []issueRow
	if err := sqlx.SelectContext(ctx, database.Executor(ctx, r.db), &rows, `
		SELECT `+issueColumns+`
		FROM issues
		WHERE parent_id = $1 AND deleted_at IS NULL
		  AND EXISTS (SELECT 1 FROM projects p WHERE p.id = issues.project_id AND p.deleted_at IS NULL)
		ORDER BY rank COLLATE "C", key_number`, parentID); err != nil {
		return nil, apperror.Wrap(apperror.KindInternal, "failed to list children", err)
	}
	out := make([]*domain.Issue, 0, len(rows))
	for i := range rows {
		out = append(out, rows[i].toDomain())
	}
	return out, nil
}
