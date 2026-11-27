package postgres

import (
	"context"

	"github.com/google/uuid"

	"github.com/jhonsferg/corelog/internal/issue/application/port"
	"github.com/jhonsferg/corelog/internal/issue/domain"
	"github.com/jhonsferg/corelog/internal/platform/apperror"
)

var _ port.BacklogRepository = (*IssueRepository)(nil)

// ListBacklog devuelve las incidencias del proyecto ordenadas por rank y, a igualdad, por id.
func (r *IssueRepository) ListBacklog(ctx context.Context, projectID uuid.UUID) ([]*domain.Issue, error) {
	var rows []issueRow
	if err := r.db.SelectContext(ctx, &rows, `
		SELECT `+issueColumns+`
		FROM issues
		WHERE project_id = $1
		  AND EXISTS (SELECT 1 FROM projects p WHERE p.id = issues.project_id AND p.deleted_at IS NULL)
		ORDER BY rank ASC, id ASC`,
		projectID,
	); err != nil {
		return nil, apperror.Wrap(apperror.KindInternal, "failed to list backlog", err)
	}
	items := make([]*domain.Issue, 0, len(rows))
	for _, row := range rows {
		items = append(items, row.toDomain())
	}
	return items, nil
}

// SaveRanks guarda el rank de varias incidencias del proyecto en una sola transaccion.
func (r *IssueRepository) SaveRanks(ctx context.Context, projectID uuid.UUID, ranks map[uuid.UUID]string) error {
	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return apperror.Wrap(apperror.KindInternal, "failed to save backlog order", err)
	}
	defer func() { _ = tx.Rollback() }()
	for id, rank := range ranks {
		if _, err := tx.ExecContext(ctx, `UPDATE issues SET rank = $3, updated_at = now() WHERE id = $1 AND project_id = $2 AND EXISTS (SELECT 1 FROM projects p WHERE p.id = issues.project_id AND p.deleted_at IS NULL)`, id, projectID, rank); err != nil {
			return apperror.Wrap(apperror.KindInternal, "failed to save backlog order", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return apperror.Wrap(apperror.KindInternal, "failed to save backlog order", err)
	}
	return nil
}
