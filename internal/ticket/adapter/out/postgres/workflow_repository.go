package postgres

import (
	"context"
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
