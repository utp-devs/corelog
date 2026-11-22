package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jmoiron/sqlx"

	"github.com/jhonsferg/corelog/internal/platform/apperror"
	"github.com/jhonsferg/corelog/internal/ticket/application/port"
	"github.com/jhonsferg/corelog/internal/ticket/domain"
)

const pgForeignKeyViolation = "23503"

type TicketRepository struct {
	db *sqlx.DB
}

func NewTicketRepository(db *sqlx.DB) *TicketRepository {
	return &TicketRepository{db: db}
}

var _ port.Repository = (*TicketRepository)(nil)
var _ port.AssigneeWatcherRepository = (*TicketRepository)(nil)

func (r *TicketRepository) Save(ctx context.Context, t *domain.Ticket) error {
	row := fromDomain(t)

	if _, err := r.db.ExecContext(ctx, `
		INSERT INTO projects (
			id, key, name, description, type, lead_id, visibility, archived_at, created_at, updated_at
		) VALUES (
			'00000000-0000-4000-8000-000000000001',
			'CORE',
			'CoreLog',
			'Proyecto por defecto',
			'kanban',
			$1,
			'team',
			NULL,
			$2,
			$2
		) ON CONFLICT (id) DO NOTHING`,
		t.RequesterID,
		t.CreatedAt,
	); err != nil {
		return apperror.Wrap(apperror.KindInternal, "failed to ensure default project", err)
	}

	const query = `
		INSERT INTO issues (
			id, project_id, title, description, status, priority, issue_type, parent_id,
			due_date, story_points, rank, closed_at, requester_id, assignee_id, created_at, updated_at
		)
		VALUES (
			:id, '00000000-0000-4000-8000-000000000001', :title, :description, :status, :priority,
			:issue_type, :parent_id, :due_date, :story_points, :rank, :closed_at,
			:requester_id, :assignee_id, :created_at, :updated_at
		)`

	if _, err := r.db.NamedExecContext(ctx, query, row); err != nil {
		if isForeignKeyViolation(err) {
			return domain.ErrRequesterNotFound
		}
		return apperror.Wrap(apperror.KindInternal, "failed to save ticket", err)
	}
	return nil
}

func (r *TicketRepository) FindByID(ctx context.Context, id uuid.UUID) (*domain.Ticket, error) {
	var row ticketRow

	const query = `
		SELECT id, project_id, title, description, status, priority, issue_type, parent_id,
		       due_date, story_points, rank, closed_at, requester_id, assignee_id, created_at, updated_at
		FROM issues WHERE id = $1`

	if err := r.db.GetContext(ctx, &row, query, id); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, domain.ErrTicketNotFound
		}
		return nil, apperror.Wrap(apperror.KindInternal, "failed to find ticket", err)
	}

	return row.toDomain(), nil
}

func (r *TicketRepository) FindAll(ctx context.Context, filter port.ListFilter) ([]*domain.Ticket, error) {
	query := `
		SELECT id, project_id, title, description, status, priority, issue_type, parent_id,
		       due_date, story_points, rank, closed_at, requester_id, assignee_id, created_at, updated_at
		FROM issues WHERE 1 = 1`
	args := []any{}
	if len(filter.ProjectIDs) > 0 {
		placeholders := make([]string, 0, len(filter.ProjectIDs))
		for _, projectID := range filter.ProjectIDs {
			args = append(args, projectID)
			placeholders = append(placeholders, fmt.Sprintf("$%d", len(args)))
		}
		query += " AND project_id IN (" + strings.Join(placeholders, ", ") + ")"
	}

	if filter.Status != "" {
		args = append(args, filter.Status)
		query += fmt.Sprintf(" AND status = $%d", len(args))
	}
	if filter.Priority != "" {
		args = append(args, filter.Priority)
		query += fmt.Sprintf(" AND priority = $%d", len(args))
	}
	if filter.Query != "" {
		args = append(args, "%"+filter.Query+"%")
		query += fmt.Sprintf(" AND (title ILIKE $%d OR description ILIKE $%d)", len(args), len(args))
	}
	if filter.IssueType != "" {
		args = append(args, filter.IssueType)
		query += fmt.Sprintf(" AND issue_type = $%d", len(args))
	}
	if filter.LabelID != uuid.Nil {
		args = append(args, filter.LabelID)
		query += fmt.Sprintf(" AND EXISTS (SELECT 1 FROM issue_labels il WHERE il.issue_id = issues.id AND il.label_id = $%d)", len(args))
	}
	if filter.AssigneeID != nil {
		args = append(args, *filter.AssigneeID)
		query += fmt.Sprintf(" AND assignee_id = $%d", len(args))
	}
	if filter.DueFrom != nil {
		args = append(args, *filter.DueFrom)
		query += fmt.Sprintf(" AND due_date >= $%d", len(args))
	}
	if filter.DueTo != nil {
		args = append(args, *filter.DueTo)
		query += fmt.Sprintf(" AND due_date <= $%d", len(args))
	}

	query += " ORDER BY rank ASC, created_at DESC"

	args = append(args, filter.PageSize)
	query += fmt.Sprintf(" LIMIT $%d", len(args))

	args = append(args, (filter.Page-1)*filter.PageSize)
	query += fmt.Sprintf(" OFFSET $%d", len(args))

	var rows []ticketRow
	if err := r.db.SelectContext(ctx, &rows, query, args...); err != nil {
		return nil, apperror.Wrap(apperror.KindInternal, "failed to list tickets", err)
	}

	tickets := make([]*domain.Ticket, 0, len(rows))
	for _, row := range rows {
		tickets = append(tickets, row.toDomain())
	}
	return tickets, nil
}

func (r *TicketRepository) Update(ctx context.Context, t *domain.Ticket) error {
	row := fromDomain(t)

	const query = `
		UPDATE issues
		SET title = :title, description = :description, status = :status, priority = :priority,
		    issue_type = :issue_type, parent_id = :parent_id, due_date = :due_date,
		    story_points = :story_points, rank = :rank, closed_at = :closed_at,
		    assignee_id = :assignee_id, updated_at = :updated_at
		WHERE id = :id`

	result, err := r.db.NamedExecContext(ctx, query, row)
	if err != nil {
		if isForeignKeyViolation(err) {
			return domain.ErrAssigneeNotFound
		}
		return apperror.Wrap(apperror.KindInternal, "failed to update ticket", err)
	}

	affected, err := result.RowsAffected()
	if err != nil {
		return apperror.Wrap(apperror.KindInternal, "failed to update ticket", err)
	}
	if affected == 0 {
		return domain.ErrTicketNotFound
	}

	return nil
}

func (r *TicketRepository) UpdateAndWatchAssignee(ctx context.Context, t *domain.Ticket, assigneeID uuid.UUID) error {
	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return apperror.Wrap(apperror.KindInternal, "failed to begin assignment transaction", err)
	}
	defer func() { _ = tx.Rollback() }()
	row := fromDomain(t)
	const query = `
		UPDATE issues
		SET title = :title, description = :description, status = :status, priority = :priority,
		    issue_type = :issue_type, parent_id = :parent_id, due_date = :due_date,
		    story_points = :story_points, rank = :rank, closed_at = :closed_at,
		    assignee_id = :assignee_id, updated_at = :updated_at
		WHERE id = :id`
	result, err := sqlx.NamedExecContext(ctx, tx, query, row)
	if err != nil {
		if isForeignKeyViolation(err) {
			return domain.ErrAssigneeNotFound
		}
		return apperror.Wrap(apperror.KindInternal, "failed to assign ticket", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return apperror.Wrap(apperror.KindInternal, "failed to assign ticket", err)
	}
	if affected == 0 {
		return domain.ErrTicketNotFound
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO watchers(issue_id, user_id) VALUES($1, $2)
		ON CONFLICT(issue_id, user_id) DO NOTHING`, t.ID, assigneeID); err != nil {
		if isForeignKeyViolation(err) {
			return domain.ErrAssigneeNotFound
		}
		return apperror.Wrap(apperror.KindInternal, "failed to follow issue after assignment", err)
	}
	if err := tx.Commit(); err != nil {
		return apperror.Wrap(apperror.KindInternal, "failed to commit assignment", err)
	}
	return nil
}

func isForeignKeyViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == pgForeignKeyViolation
}
