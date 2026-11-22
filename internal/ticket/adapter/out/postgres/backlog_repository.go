package postgres

import (
	"context"

	"github.com/google/uuid"

	"github.com/jhonsferg/corelog/internal/platform/apperror"
	"github.com/jhonsferg/corelog/internal/ticket/application/port"
	"github.com/jhonsferg/corelog/internal/ticket/domain"
)

var _ port.BacklogRepository = (*TicketRepository)(nil)

// ListBacklog devuelve las incidencias del proyecto ordenadas por rank y, a igualdad, por id.
func (r *TicketRepository) ListBacklog(ctx context.Context, projectID uuid.UUID) ([]*domain.Ticket, error) {
	var rows []ticketRow
	if err := r.db.SelectContext(ctx, &rows, `
		SELECT id, project_id, title, description, status, priority, issue_type, parent_id,
		       due_date, story_points, rank, closed_at, requester_id, assignee_id, created_at, updated_at
		FROM issues
		WHERE project_id = $1
		ORDER BY rank ASC, id ASC`,
		projectID,
	); err != nil {
		return nil, apperror.Wrap(apperror.KindInternal, "failed to list backlog", err)
	}
	items := make([]*domain.Ticket, 0, len(rows))
	for _, row := range rows {
		items = append(items, row.toDomain())
	}
	return items, nil
}

// SaveRanks guarda el rank de varias incidencias del proyecto en una sola transaccion.
func (r *TicketRepository) SaveRanks(ctx context.Context, projectID uuid.UUID, ranks map[uuid.UUID]string) error {
	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return apperror.Wrap(apperror.KindInternal, "failed to save backlog order", err)
	}
	defer func() { _ = tx.Rollback() }()
	for id, rank := range ranks {
		if _, err := tx.ExecContext(ctx, `UPDATE issues SET rank = $3, updated_at = now() WHERE id = $1 AND project_id = $2`, id, projectID, rank); err != nil {
			return apperror.Wrap(apperror.KindInternal, "failed to save backlog order", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return apperror.Wrap(apperror.KindInternal, "failed to save backlog order", err)
	}
	return nil
}
