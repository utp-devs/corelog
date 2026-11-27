// Package postgres persists workflows and organization statuses.
package postgres

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jmoiron/sqlx"

	"github.com/jhonsferg/corelog/internal/platform/apperror"
	"github.com/jhonsferg/corelog/internal/platform/database"
	"github.com/jhonsferg/corelog/internal/workflow/application/port"
	"github.com/jhonsferg/corelog/internal/workflow/domain"
)

// Repository stores workflows (one JSON definition per project) and the status catalog of organizations.
type Repository struct {
	db *sqlx.DB
}

func NewRepository(db *sqlx.DB) *Repository { return &Repository{db: db} }

var _ port.Repository = (*Repository)(nil)

func (r *Repository) exec(ctx context.Context) sqlx.ExtContext { return database.Executor(ctx, r.db) }

// FindWorkflow returns the workflow of the project. A live project that has none yet (it was written before
// its first issue) gets the workflow of its template first.
func (r *Repository) FindWorkflow(ctx context.Context, projectID uuid.UUID) (port.Record, error) {
	record, err := r.findWorkflow(ctx, projectID)
	if !errors.Is(err, sql.ErrNoRows) {
		return record, err
	}
	if _, perr := r.exec(ctx).ExecContext(ctx, `SELECT corelog_ensure_workflow($1)`, projectID); perr != nil {
		return port.Record{}, apperror.Wrap(apperror.KindInternal, "failed to provision workflow", perr)
	}
	record, err = r.findWorkflow(ctx, projectID)
	if errors.Is(err, sql.ErrNoRows) {
		return port.Record{}, port.ErrWorkflowNotFound
	}
	return record, err
}

func (r *Repository) findWorkflow(ctx context.Context, projectID uuid.UUID) (port.Record, error) {
	var row struct {
		Definition []byte    `db:"definition"`
		Version    int       `db:"version"`
		UpdatedAt  time.Time `db:"updated_at"`
	}
	err := sqlx.GetContext(ctx, r.exec(ctx), &row, `SELECT definition, version, updated_at FROM workflows WHERE project_id = $1`, projectID)
	if errors.Is(err, sql.ErrNoRows) {
		return port.Record{}, err
	}
	if err != nil {
		return port.Record{}, apperror.Wrap(apperror.KindInternal, "failed to load workflow", err)
	}
	w, err := domain.Decode(row.Definition)
	if err != nil {
		return port.Record{}, apperror.Wrap(apperror.KindInternal, "stored workflow is unreadable", err)
	}
	return port.Record{ProjectID: projectID, Workflow: w, Version: row.Version, UpdatedAt: row.UpdatedAt}, nil
}

// SaveWorkflow replaces the stored definition when the version matches.
func (r *Repository) SaveWorkflow(ctx context.Context, projectID uuid.UUID, w domain.Workflow, expectedVersion int, at time.Time) (int, error) {
	raw, err := domain.Encode(w)
	if err != nil {
		return 0, err
	}
	var version int
	err = sqlx.GetContext(ctx, r.exec(ctx), &version, `
		UPDATE workflows SET definition = $2::jsonb, version = version + 1, updated_at = $3
		WHERE project_id = $1 AND version = $4
		RETURNING version`, projectID, string(raw), at, expectedVersion)
	if errors.Is(err, sql.ErrNoRows) {
		var exists bool
		if gerr := sqlx.GetContext(ctx, r.exec(ctx), &exists, `SELECT EXISTS (SELECT 1 FROM workflows WHERE project_id = $1)`, projectID); gerr != nil {
			return 0, apperror.Wrap(apperror.KindInternal, "failed to save workflow", gerr)
		}
		if !exists {
			return 0, port.ErrWorkflowNotFound
		}
		return 0, port.ErrVersionConflict
	}
	if err != nil {
		return 0, apperror.Wrap(apperror.KindInternal, "failed to save workflow", err)
	}
	return version, nil
}

type statusRow struct {
	ID             uuid.UUID `db:"id"`
	OrganizationID uuid.UUID `db:"organization_id"`
	Name           string    `db:"name"`
	Category       string    `db:"category"`
	Position       int       `db:"position"`
}

func (s statusRow) toPort() port.OrgStatus {
	return port.OrgStatus{ID: s.ID, OrganizationID: s.OrganizationID, Name: s.Name, Category: domain.Category(s.Category), Position: s.Position}
}

// ListStatuses lists the statuses of the organization in catalog order.
func (r *Repository) ListStatuses(ctx context.Context, orgID uuid.UUID) ([]port.OrgStatus, error) {
	var rows []statusRow
	if err := sqlx.SelectContext(ctx, r.exec(ctx), &rows, `
		SELECT id, organization_id, name, category, position FROM statuses
		WHERE organization_id = $1 ORDER BY position, lower(name), id`, orgID); err != nil {
		return nil, apperror.Wrap(apperror.KindInternal, "failed to list statuses", err)
	}
	out := make([]port.OrgStatus, 0, len(rows))
	for _, row := range rows {
		out = append(out, row.toPort())
	}
	return out, nil
}

// CreateStatus appends a status at the end of the catalog.
func (r *Repository) CreateStatus(ctx context.Context, status port.OrgStatus) (port.OrgStatus, error) {
	var row statusRow
	err := sqlx.GetContext(ctx, r.exec(ctx), &row, `
		INSERT INTO statuses (organization_id, name, category, position)
		VALUES ($1, $2, $3, COALESCE((SELECT max(position) + 1 FROM statuses WHERE organization_id = $1), 0))
		RETURNING id, organization_id, name, category, position`,
		status.OrganizationID, status.Name, string(status.Category))
	if err != nil {
		if isCode(err, "23505") {
			return port.OrgStatus{}, port.ErrStatusNameTaken
		}
		return port.OrgStatus{}, apperror.Wrap(apperror.KindInternal, "failed to create status", err)
	}
	return row.toPort(), nil
}

// UpdateStatus changes the name, category and position of a status.
func (r *Repository) UpdateStatus(ctx context.Context, status port.OrgStatus) error {
	result, err := r.exec(ctx).ExecContext(ctx, `
		UPDATE statuses SET name = $3, category = $4, position = $5
		WHERE id = $1 AND organization_id = $2`,
		status.ID, status.OrganizationID, status.Name, string(status.Category), status.Position)
	if err != nil {
		if isCode(err, "23505") {
			return port.ErrStatusNameTaken
		}
		return apperror.Wrap(apperror.KindInternal, "failed to update status", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return apperror.Wrap(apperror.KindInternal, "failed to update status", err)
	}
	if affected == 0 {
		return port.ErrStatusNotFound
	}
	return nil
}

// DeleteStatus removes a status nothing uses.
func (r *Repository) DeleteStatus(ctx context.Context, orgID, id uuid.UUID) error {
	var inWorkflow bool
	if err := sqlx.GetContext(ctx, r.exec(ctx), &inWorkflow, `
		SELECT EXISTS (
			SELECT 1 FROM workflows w JOIN projects p ON p.id = w.project_id
			WHERE p.organization_id = $1
			  AND w.definition -> 'statuses' @> jsonb_build_array(jsonb_build_object('id', $2::text)))`, orgID, id.String()); err != nil {
		return apperror.Wrap(apperror.KindInternal, "failed to check status usage", err)
	}
	if inWorkflow {
		return port.ErrStatusInUse
	}
	result, err := r.exec(ctx).ExecContext(ctx, `DELETE FROM statuses WHERE id = $1 AND organization_id = $2`, id, orgID)
	if err != nil {
		if isCode(err, "23503") {
			return port.ErrStatusInUse
		}
		return apperror.Wrap(apperror.KindInternal, "failed to delete status", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return apperror.Wrap(apperror.KindInternal, "failed to delete status", err)
	}
	if affected == 0 {
		return port.ErrStatusNotFound
	}
	return nil
}

// ColumnStatusIDs returns the statuses bound to a board column.
func (r *Repository) ColumnStatusIDs(ctx context.Context, columnID uuid.UUID) ([]uuid.UUID, error) {
	var ids []uuid.UUID
	if err := sqlx.SelectContext(ctx, r.exec(ctx), &ids, `SELECT status_id FROM board_column_statuses WHERE column_id = $1 ORDER BY status_id`, columnID); err != nil {
		return nil, apperror.Wrap(apperror.KindInternal, "failed to read column statuses", err)
	}
	return ids, nil
}

func isCode(err error, code string) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == code
}
