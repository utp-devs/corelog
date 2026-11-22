package postgres

import (
	"context"
	"database/sql"
	"errors"
	"strings"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"

	"github.com/jhonsferg/corelog/internal/platform/apperror"
	"github.com/jhonsferg/corelog/internal/ticket/application/port"
	"github.com/jhonsferg/corelog/internal/ticket/domain"
)

type WorkflowRepository struct {
	db *sqlx.DB
}

func NewWorkflowRepository(db *sqlx.DB) *WorkflowRepository {
	return &WorkflowRepository{db: db}
}

var _ port.WorkflowRepository = (*WorkflowRepository)(nil)

func (r *WorkflowRepository) CanTransition(ctx context.Context, projectID uuid.UUID, fromStatus, toStatus domain.Status, actorRole string) (bool, error) {
	if actorRole == "" {
		return false, nil
	}
	var allowedRoles string
	err := r.db.GetContext(ctx, &allowedRoles, `
		SELECT wt.allowed_roles
		FROM workflows w
		JOIN workflow_transitions wt ON wt.workflow_id = w.id
		WHERE w.project_id = $1
		  AND w.is_default = true
		  AND wt.from_status = $2
		  AND wt.to_status = $3
		ORDER BY w.created_at ASC
		LIMIT 1`,
		projectID,
		string(fromStatus),
		string(toStatus),
	)
	if err != nil {
		if strings.Contains(err.Error(), "sql: no rows") {
			return false, nil
		}
		return false, apperror.Wrap(apperror.KindInternal, "failed to evaluate workflow transition", err)
	}

	for _, role := range strings.Split(allowedRoles, ",") {
		if strings.TrimSpace(role) == actorRole {
			return true, nil
		}
	}
	return false, nil
}

type workflowRow struct {
	ID        uuid.UUID `db:"id"`
	ProjectID uuid.UUID `db:"project_id"`
	Name      string    `db:"name"`
}

type transitionRow struct {
	From  string `db:"from_status"`
	To    string `db:"to_status"`
	Roles string `db:"allowed_roles"`
}

// DefaultWorkflow devuelve el flujo por defecto del proyecto con sus transiciones, o nil si no existe.
func (r *WorkflowRepository) DefaultWorkflow(ctx context.Context, projectID uuid.UUID) (*port.Workflow, error) {
	var row workflowRow
	err := r.db.GetContext(ctx, &row, `
		SELECT id, project_id, name
		FROM workflows
		WHERE project_id = $1 AND is_default = true
		ORDER BY created_at ASC
		LIMIT 1`,
		projectID,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, apperror.Wrap(apperror.KindInternal, "failed to load workflow", err)
	}

	var rows []transitionRow
	if err := r.db.SelectContext(ctx, &rows, `
		SELECT from_status, to_status, allowed_roles
		FROM workflow_transitions
		WHERE workflow_id = $1
		ORDER BY from_status, to_status`,
		row.ID,
	); err != nil {
		return nil, apperror.Wrap(apperror.KindInternal, "failed to load workflow transitions", err)
	}

	workflow := &port.Workflow{ID: row.ID, ProjectID: row.ProjectID, Name: row.Name, Transitions: make([]port.WorkflowTransition, 0, len(rows))}
	for _, item := range rows {
		roles := []string{}
		for _, role := range strings.Split(item.Roles, ",") {
			if role = strings.TrimSpace(role); role != "" {
				roles = append(roles, role)
			}
		}
		workflow.Transitions = append(workflow.Transitions, port.WorkflowTransition{
			From:  domain.Status(item.From),
			To:    domain.Status(item.To),
			Roles: roles,
		})
	}
	return workflow, nil
}
