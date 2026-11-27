package port

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"

	"github.com/jhonsferg/corelog/internal/issue/domain"
)

// Field is one optional field of a partial update. Set is false when the client did not send the field.
// When Set is true and Value is nil the client sent null, which clears nullable fields.
type Field[T any] struct {
	Set   bool
	Value *T
}

// Some builds a field that is set to v.
func Some[T any](v T) Field[T] { return Field[T]{Set: true, Value: &v} }

// Null builds a field that is set to null.
func Null[T any]() Field[T] { return Field[T]{Set: true} }

// PatchInput is a partial update of an issue. Only fields with Set change.
type PatchInput struct {
	// ExpectedUpdatedAt, when set, must equal the stored updated_at or the patch fails with issue.stale.
	ExpectedUpdatedAt *time.Time
	// HideForbidden makes a caller who cannot see the issue get not found instead of forbidden.
	HideForbidden bool

	Summary                  Field[string]
	Description              Field[string]
	IssueTypeID              Field[uuid.UUID]
	IssueType                Field[domain.IssueType]
	PriorityID               Field[uuid.UUID]
	Priority                 Field[domain.Priority]
	ParentID                 Field[uuid.UUID]
	AssigneeID               Field[uuid.UUID]
	StoryPoints              Field[float64]
	OriginalEstimateSeconds  Field[int64]
	RemainingEstimateSeconds Field[int64]
	StartDate                Field[time.Time]
	DueDate                  Field[time.Time]
	Flagged                  Field[bool]
	// Labels replaces the labels of the issue by name; unknown names are created. Nil Value clears them.
	Labels Field[[]string]
	// Custom is the raw "custom" object. Nil when absent.
	Custom map[string]json.RawMessage
}

// PatchResult is the issue after a patch with the changes that were applied. Changes is empty when the patch
// changed nothing; nothing is written in that case.
type PatchResult struct {
	Issue   *domain.Issue
	Changes []domain.Change
	BatchID uuid.UUID
}

// Transition is a status the issue can move to.
type Transition struct {
	ID   string
	Name string
	// To is the legacy status that mirrors the target status category.
	To domain.Status
	// ToStatusID, ToStatusName and ToCategory describe the workflow status the transition leads to.
	ToStatusID   uuid.UUID
	ToStatusName string
	ToCategory   string
	// RequiredFields, RequiresComment, RequiresResolution and RequiresAssignee tell what the client must ask for.
	RequiredFields     []string
	RequiresComment    bool
	RequiresResolution bool
	RequiresAssignee   bool
	NeedsScreen        bool
}

// IssueView is the full view model of one issue for the caller.
type IssueView struct {
	Issue       *domain.Issue
	Permissions []string
	Transitions []Transition
	Counts      domain.IssueCounts
	Breadcrumb  domain.Breadcrumb
	// ViewerHasVoted tells whether the caller has voted for the issue.
	ViewerHasVoted bool
}

// TransitionProvider lists the transitions the caller may run on an issue. The issue service implements it with
// the workflow engine.
type TransitionProvider interface {
	AvailableTransitions(ctx context.Context, issue *domain.Issue, actorID uuid.UUID, globalRole string) ([]Transition, error)
}

// IssuePermissionChecker answers per-permission questions about an issue. holders name the special holders
// the caller matches on the issue ("reporter", "assignee").
type IssuePermissionChecker interface {
	Can(ctx context.Context, actorID uuid.UUID, globalRole string, projectID uuid.UUID, permission string, holders ...string) (bool, error)
	// Effective lists the permission names the caller holds on the project without special holders.
	Effective(ctx context.Context, actorID uuid.UUID, globalRole string, projectID uuid.UUID) ([]string, error)
}

// ViewReader loads the counts and the breadcrumb of an issue with a fixed number of statements.
type ViewReader interface {
	ViewExtras(ctx context.Context, issueID uuid.UUID) (domain.IssueCounts, domain.Breadcrumb, error)
}

// HierarchyReader reads the hierarchy levels of the live children of an issue.
type HierarchyReader interface {
	ChildLevels(ctx context.Context, parentID uuid.UUID) ([]int, error)
}

// IssueLifecycle covers soft delete, restore, row locking and rank allocation.
type IssueLifecycle interface {
	SoftDelete(ctx context.Context, id uuid.UUID, at time.Time) error
	Restore(ctx context.Context, id uuid.UUID, at time.Time) error
	FindDeleted(ctx context.Context, id uuid.UUID) (*domain.Issue, error)
	// LockForUpdate takes the row lock of a live issue until the transaction ends.
	LockForUpdate(ctx context.Context, id uuid.UUID) error
	// NextRank returns a rank that sorts after every issue of the project.
	NextRank(ctx context.Context, projectID uuid.UUID) (string, error)
}

// ProjectDefaults reads the issue defaults of a project.
type ProjectDefaults interface {
	// DefaultAssigneePolicy returns "unassigned" or "lead" and the id of the project lead.
	DefaultAssigneePolicy(ctx context.Context, projectID uuid.UUID) (policy string, leadID *uuid.UUID, err error)
}

// RequiredFieldReader lists the system fields a project requires for an issue type.
type RequiredFieldReader interface {
	RequiredSystemFields(ctx context.Context, projectID, issueTypeID uuid.UUID) ([]string, error)
}

// ActivityBatchWriter writes the activities of one patch together.
type ActivityBatchWriter interface {
	SaveActivities(ctx context.Context, activities []*domain.Activity) error
}
