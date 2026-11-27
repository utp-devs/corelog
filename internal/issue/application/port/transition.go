package port

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/jhonsferg/corelog/internal/issue/domain"
)

// TransitionActor describes who runs a transition, as the workflow engine needs it. Role is the project role
// (lead, contributor or viewer); a platform administrator counts as lead.
type TransitionActor struct {
	UserID      uuid.UUID
	Role        string
	LeadID      uuid.UUID
	Permissions []string
}

// TransitionIssue is the read-only view of an issue the workflow engine evaluates. A nil uuid means "none".
type TransitionIssue struct {
	StatusID     uuid.UUID
	AssigneeID   uuid.UUID
	ReporterID   uuid.UUID
	ResolutionID uuid.UUID
	// OpenSubtasks counts the live children that are not in a done status.
	OpenSubtasks int
	// Fields holds the values of standard and custom fields by workflow field name.
	Fields map[string]string
}

// TransitionOption is a transition the caller may run from the current status of an issue.
type TransitionOption struct {
	ID                 string
	Name               string
	ToStatusID         uuid.UUID
	ToStatusName       string
	ToCategory         string
	RequiredFields     []string
	RequiresComment    bool
	RequiresResolution bool
	RequiresAssignee   bool
}

// NeedsScreen reports whether the client must ask for input before running the transition.
func (o TransitionOption) NeedsScreen() bool {
	return len(o.RequiredFields) > 0 || o.RequiresComment || o.RequiresResolution || o.RequiresAssignee
}

// TransitionInput is what the caller supplies when running a transition.
type TransitionInput struct {
	Fields       map[string]string
	Comment      string
	ResolutionID uuid.UUID
	AssigneeID   uuid.UUID
}

// Resolution actions of a transition plan.
const (
	PlanResolutionNone  = ""
	PlanResolutionSet   = "set"
	PlanResolutionClear = "clear"
)

// PlannedField is one field write of a transition plan.
type PlannedField struct {
	Field string
	Value string
}

// TransitionPlan is what the workflow engine decided: the changes to apply in the transaction of the transition.
type TransitionPlan struct {
	TransitionID   string
	TransitionName string
	FromStatusID   uuid.UUID
	ToStatusID     uuid.UUID
	ToStatusName   string
	ToCategory     string
	// ResolutionAction is one of the PlanResolution constants. UseDefaultDone asks for the organization's Done
	// resolution instead of ResolutionID.
	ResolutionAction string
	ResolutionID     uuid.UUID
	UseDefaultDone   bool
	AssigneeSet      bool
	AssigneeID       uuid.UUID
	Fields           []PlannedField
	// Comment is the comment the user typed; Comments are automatic comments added by post-functions.
	Comment  string
	Comments []string
	Events   []string
}

// WorkflowEngine loads the workflow of a project and evaluates transitions against it. The workflow module
// implements it; the composition root adapts its types to these.
type WorkflowEngine interface {
	// Available lists, in workflow order, the transitions the actor may run from the current status.
	Available(ctx context.Context, projectID uuid.UUID, issue TransitionIssue, actor TransitionActor) ([]TransitionOption, error)
	// Plan checks the transition and the input and returns the planned changes. It fails with
	// workflow.transition_not_allowed or workflow.validation_failed.
	Plan(ctx context.Context, projectID uuid.UUID, issue TransitionIssue, actor TransitionActor, transitionID string, input TransitionInput) (TransitionPlan, error)
	// ColumnStatusIDs returns the statuses a board column shows.
	ColumnStatusIDs(ctx context.Context, columnID uuid.UUID) ([]uuid.UUID, error)
}

// TransitionWrite is the status part of a transition. Legacy is the legacy status that mirrors the target
// category; ToDone tells that the target is in the done category.
type TransitionWrite struct {
	IssueID          uuid.UUID
	FromStatusID     uuid.UUID
	ToStatusID       uuid.UUID
	TransitionID     string
	ActorID          uuid.UUID
	Legacy           domain.Status
	ToDone           bool
	ResolutionAction string
	ResolutionID     *uuid.UUID
	At               time.Time
}

// TransitionRepository is the persistence the transition use case needs besides the issue repository.
type TransitionRepository interface {
	// OpenSubtasks counts the live children of the issue that are not in a done status.
	OpenSubtasks(ctx context.Context, issueID uuid.UUID) (int, error)
	// ApplyStatus writes the new status (and its legacy mirror), the resolution and the status history row.
	// It returns false when the issue no longer has FromStatusID (a concurrent change won).
	ApplyStatus(ctx context.Context, w TransitionWrite) (bool, error)
	// StatusInfo returns name and category of a status.
	StatusInfo(ctx context.Context, statusID uuid.UUID) (name, category string, err error)
}

// TransitionRequest is one transition to run. Issue is the issue id.
type TransitionRequest struct {
	IssueID      uuid.UUID
	ActorID      uuid.UUID
	GlobalRole   string
	TransitionID string
	Input        TransitionInput
	// ExpectedStatusID, when set, must be the current status of the issue or the transition fails with
	// issue.status_conflict.
	ExpectedStatusID uuid.UUID
}

// TransitionResult is the issue after a transition with the plan that was applied.
type TransitionResult struct {
	Issue *domain.Issue
	Plan  TransitionPlan
}

// TransitionService is the in-port of the transition use cases for the HTTP adapter and the board.
type TransitionService interface {
	// ListTransitions returns the transitions the caller may run on the issue.
	ListTransitions(ctx context.Context, issueID, actorID uuid.UUID, globalRole string) ([]TransitionOption, error)
	// TransitionIssue runs a transition in one transaction.
	TransitionIssue(ctx context.Context, in TransitionRequest) (*TransitionResult, error)
	// MoveIssueToColumn runs the transition that leads the issue to a board column.
	MoveIssueToColumn(ctx context.Context, actorID uuid.UUID, globalRole string, projectID, issueID, columnID uuid.UUID) error
}
