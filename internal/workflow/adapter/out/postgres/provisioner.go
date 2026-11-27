package postgres

import (
	"context"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"

	"github.com/jhonsferg/corelog/internal/platform/apperror"
)

// Provisioner gives a new project the workflow and statuses of its template. It runs inside the transaction
// that creates the project, through the database function the migration also uses, so the template data lives
// in one place.
type Provisioner struct{}

func NewProvisioner() Provisioner { return Provisioner{} }

// ProvisionWorkflow creates the organization statuses the template needs, stores the workflow of the project and
// binds its board columns to the statuses. The template is the one stored in the project row.
func (Provisioner) ProvisionWorkflow(ctx context.Context, tx sqlx.ExtContext, projectID uuid.UUID, _ string) error {
	if _, err := tx.ExecContext(ctx, `SELECT corelog_ensure_workflow($1)`, projectID); err != nil {
		return apperror.Wrap(apperror.KindInternal, "failed to provision workflow", err)
	}
	return nil
}
