package port

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/jhonsferg/corelog/internal/platform/apperror"
	"github.com/jhonsferg/corelog/internal/workflow/domain"
)

// Errors of the workflow administration use cases.
var (
	ErrProjectNotFound     = apperror.WithCode(apperror.KindNotFound, "project.not_found", "proyecto no encontrado")
	ErrVersionRequired     = apperror.WithCode(apperror.KindValidation, "workflow.version_required", "indica la versión del flujo que editas (expected_version)")
	ErrMappingRequired     = apperror.WithCode(apperror.KindValidation, "workflow.status_mapping_required", "hay estados que se eliminan con incidencias; indica a qué estado pasan en status_mapping")
	ErrInvalidMapping      = apperror.WithCode(apperror.KindValidation, "workflow.invalid_status_mapping", "el mapeo de estados no es válido")
	ErrCategoryInUse       = apperror.WithCode(apperror.KindConflict, "workflow.category_in_use", "el cambio de categoría dejaría algún flujo de trabajo inválido")
	ErrReassignInvalid     = apperror.WithCode(apperror.KindValidation, "workflow.reassign_invalid", "el estado de destino no es válido para reasignar las incidencias")
	ErrStatusOrderInvalid  = apperror.WithCode(apperror.KindValidation, "workflow.status_order_invalid", "la nueva ordenación debe contener exactamente los estados de la organización, una sola vez")
	ErrStatusNotInOrg      = apperror.WithCode(apperror.KindValidation, "workflow.status_not_in_org", "el estado no pertenece a la organización")
	ErrStatusRefInvalid    = apperror.WithCode(apperror.KindValidation, "workflow.status_ref_invalid", "el identificador local del estado es obligatorio y no puede repetirse")
	ErrTemplateNotProvided = apperror.WithCode(apperror.KindValidation, "workflow.template_required", "indica la plantilla a aplicar")
)

// ProjectRef is what the administration use cases need to know about a project.
type ProjectRef struct {
	ID             uuid.UUID
	OrganizationID uuid.UUID
	Template       string
}

// MigrationScope limits a move of issues to one project, or to every project of the organization when
// ProjectID is uuid.Nil.
type MigrationScope struct {
	OrganizationID uuid.UUID
	ProjectID      uuid.UUID
}

// MappingRequirement names a removed status that still holds issues and therefore needs a target.
type MappingRequirement struct {
	StatusID   uuid.UUID
	Name       string
	IssueCount int
}

// MoveCount reports how many issues moved from one status to another.
type MoveCount struct {
	From  uuid.UUID
	To    uuid.UUID
	Count int
}

// Authorizer decides project permissions. The workflow module uses the actions "view_project" and
// "manage_workflow" and never inspects the answer beyond the apperror kind.
type Authorizer interface {
	Authorize(ctx context.Context, action string, actorID uuid.UUID, globalRole string, projectID uuid.UUID) error
}

// OrgRoles checks the organization role of a user; min is "guest", "member", "admin" or "owner".
type OrgRoles interface {
	RequireOrgRole(ctx context.Context, userID, orgID uuid.UUID, min string) error
}

// ManagementRepository extends the repository with the queries and writes the administration use cases need.
type ManagementRepository interface {
	Repository
	// ProjectContext returns a live project, or ErrProjectNotFound.
	ProjectContext(ctx context.Context, projectID uuid.UUID) (ProjectRef, error)
	// LockWorkflow locks the workflow row of the project for the rest of the transaction and returns its
	// version. A project without a workflow row returns ErrWorkflowNotFound.
	LockWorkflow(ctx context.Context, projectID uuid.UUID) (int, error)
	// IssueCounts returns the number of issues of the project per status id.
	IssueCounts(ctx context.Context, projectID uuid.UUID) (map[uuid.UUID]int, error)
	// MigrateIssues moves the issues of the scope from one status to the other, keeping the legacy mirror,
	// writing one status history row and one activity per issue. It returns the number of issues moved.
	MigrateIssues(ctx context.Context, scope MigrationScope, from, to OrgStatus, actorID uuid.UUID, at time.Time) (int, error)
	// StatusUsage counts the issues (deleted ones included) and the status history rows that reference a status.
	StatusUsage(ctx context.Context, orgID, statusID uuid.UUID) (issues, history int, err error)
	// WorkflowsUsing returns the workflows of the organization whose definition contains the status.
	WorkflowsUsing(ctx context.Context, orgID, statusID uuid.UUID) ([]Record, error)
	// RewriteStatusHistory points the history rows of one status at another one.
	RewriteStatusHistory(ctx context.Context, from, to uuid.UUID) error
	// SetStatusPositions stores the position of every status of the organization, in the order given.
	SetStatusPositions(ctx context.Context, orgID uuid.UUID, ids []uuid.UUID) error
	// RefreshLegacyStatus recomputes the legacy status of the issues that use the status. It must run in a
	// transaction: it sets the transaction-local flag that keeps status_id unchanged.
	RefreshLegacyStatus(ctx context.Context, statusID uuid.UUID, category domain.Category, at time.Time) error
}

// StatusInput is one status of a requested definition. Ref is the caller's local name used by the
// transitions. ID selects an organization status; without it the status is matched by Name or created.
type StatusInput struct {
	Ref      string
	ID       uuid.UUID
	Name     string
	Category domain.Category
}

// Definition is a requested workflow. Transitions refer to statuses by Ref. Mapping sends the issues of a
// removed status (key) to a status of the new workflow (value).
type Definition struct {
	ExpectedVersion int
	InitialRef      string
	Statuses        []StatusInput
	Transitions     []domain.Transition
	Mapping         map[uuid.UUID]uuid.UUID
}

// ValidationFailure carries the reasons a definition was refused: the code and message of Err, one
// issue per problem and the statuses that need a mapping.
type ValidationFailure struct {
	Err     *apperror.Error
	Issues  []domain.Issue
	Mapping []MappingRequirement
}

func (f *ValidationFailure) Error() string { return f.Err.Error() }

// Unwrap exposes the apperror so KindOf and CodeOf see it.
func (f *ValidationFailure) Unwrap() error { return f.Err }

// StatusView is one status of a stored workflow with its issue count in the project.
type StatusView struct {
	ID         uuid.UUID
	Name       string
	Category   domain.Category
	Position   int
	IssueCount int
}

// WorkflowView is the stored workflow of a project as the API shows it.
type WorkflowView struct {
	Statuses          []StatusView
	InitialStatusID   uuid.UUID
	Transitions       []domain.Transition
	Version           int
	UpdatedAt         time.Time
	IsTemplateDefault bool
}

// SaveResult is a stored workflow plus the warnings of the change.
type SaveResult struct {
	Workflow WorkflowView
	Warnings []domain.Issue
}

// ValidationReport is the outcome of a dry run.
type ValidationReport struct {
	Valid           bool
	Errors          []domain.Issue
	Warnings        []domain.Issue
	MappingRequired []MappingRequirement
}

// MigrationResult reports the moves of an explicit migration.
type MigrationResult struct {
	Total int
	Moves []MoveCount
}

// TemplateView is a template workflow in the request format (refs instead of ids).
type TemplateView struct {
	ID          string
	Name        string
	Description string
	Definition  Definition
}

// StatusPatch changes name and/or category of an organization status; nil fields stay.
type StatusPatch struct {
	Name     *string
	Category *domain.Category
}

// DeleteResult reports what deleting a status did.
type DeleteResult struct {
	ReassignedIssues int
}

// Admin is the in-port of the workflow administration API.
type Admin interface {
	GetWorkflow(ctx context.Context, actorID uuid.UUID, globalRole string, projectID uuid.UUID) (WorkflowView, error)
	ValidateWorkflow(ctx context.Context, actorID uuid.UUID, globalRole string, projectID uuid.UUID, def Definition) (ValidationReport, error)
	ReplaceWorkflow(ctx context.Context, actorID uuid.UUID, globalRole string, projectID uuid.UUID, def Definition) (SaveResult, error)
	ResetWorkflow(ctx context.Context, actorID uuid.UUID, globalRole string, projectID uuid.UUID, template string, expectedVersion int, mapping map[uuid.UUID]uuid.UUID) (SaveResult, error)
	MigrateIssues(ctx context.Context, actorID uuid.UUID, globalRole string, projectID uuid.UUID, mapping map[uuid.UUID]uuid.UUID) (MigrationResult, error)
	ListTemplates(ctx context.Context) []TemplateView
	ListStatuses(ctx context.Context, actorID uuid.UUID, orgID uuid.UUID) ([]OrgStatus, error)
	CreateStatus(ctx context.Context, actorID uuid.UUID, orgID uuid.UUID, name string, category domain.Category) (OrgStatus, error)
	UpdateStatus(ctx context.Context, actorID uuid.UUID, orgID, id uuid.UUID, patch StatusPatch) (OrgStatus, error)
	DeleteStatus(ctx context.Context, actorID uuid.UUID, orgID, id uuid.UUID, reassignTo uuid.UUID) (DeleteResult, error)
	ReorderStatuses(ctx context.Context, actorID uuid.UUID, orgID uuid.UUID, ids []uuid.UUID) ([]OrgStatus, error)
}
