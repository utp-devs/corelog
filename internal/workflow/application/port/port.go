// Package port declares what the workflow use cases need from the outside world.
package port

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/jhonsferg/corelog/internal/workflow/domain"
)

// Record is a stored workflow with its optimistic-locking version.
type Record struct {
	ProjectID uuid.UUID
	Workflow  domain.Workflow
	Version   int
	UpdatedAt time.Time
}

// OrgStatus is one entry of the status catalog of an organization.
type OrgStatus struct {
	ID             uuid.UUID
	OrganizationID uuid.UUID
	Name           string
	Category       domain.Category
	Position       int
}

// Repository is the persistence of workflows and organization statuses.
type Repository interface {
	// FindWorkflow returns the workflow of the project, or workflow.not_found.
	FindWorkflow(ctx context.Context, projectID uuid.UUID) (Record, error)
	// SaveWorkflow replaces the workflow of the project when its stored version is expectedVersion and
	// returns the new version. A different stored version fails with workflow.version_conflict.
	SaveWorkflow(ctx context.Context, projectID uuid.UUID, w domain.Workflow, expectedVersion int, at time.Time) (int, error)
	ListStatuses(ctx context.Context, orgID uuid.UUID) ([]OrgStatus, error)
	// CreateStatus appends a status to the catalog. A name already taken (ignoring case) fails with
	// workflow.status_name_taken.
	CreateStatus(ctx context.Context, status OrgStatus) (OrgStatus, error)
	UpdateStatus(ctx context.Context, status OrgStatus) error
	// DeleteStatus removes a status that no issue or workflow uses; otherwise it fails with
	// workflow.status_in_use.
	DeleteStatus(ctx context.Context, orgID, id uuid.UUID) error
	// ColumnStatusIDs returns the statuses a board column shows.
	ColumnStatusIDs(ctx context.Context, columnID uuid.UUID) ([]uuid.UUID, error)
}
