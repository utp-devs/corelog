package port

import (
	"context"

	"github.com/google/uuid"

	"github.com/jhonsferg/corelog/internal/issue/domain"
)

// BulkInput is one bulk request. Only the fields of its action are read.
type BulkInput struct {
	// Refs are issue UUIDs or keys, already checked for count and blanks.
	Refs   []string
	Action domain.BulkAction
	// Assignee is the assignee of an assign action. A set field with a nil value unassigns.
	Assignee Field[uuid.UUID]
	// PriorityID is the catalog priority of a set_priority action.
	PriorityID uuid.UUID
	// LabelsAdd and LabelsRemove are the label changes of a set_labels action.
	LabelsAdd    []uuid.UUID
	LabelsRemove []uuid.UUID
	// Fields is the partial update of a set_field action. It goes through the single-issue patch rules.
	Fields PatchInput
	// Status is the target status of a transition action.
	Status domain.Status
	// Move is the destination of a move action.
	Move MoveInput
}

// BulkFailure reports one issue the bulk request could not change. Ref is the reference the caller sent.
type BulkFailure struct {
	Ref     string
	Code    string
	Message string
}

// BulkResult lists the issues that changed and the ones that did not. Each issue is one transaction.
type BulkResult struct {
	Succeeded []*domain.Issue
	Failed    []BulkFailure
}

// BulkLabelRepository reads and writes the labels of issues for the label actions and the clone copies.
type BulkLabelRepository interface {
	// ProjectLabelIDs returns the names of the given labels that belong to the project. Other ids are absent.
	ProjectLabelIDs(ctx context.Context, projectID uuid.UUID, labelIDs []uuid.UUID) (map[uuid.UUID]string, error)
	// IssueLabelNames returns the sorted names of the labels of an issue.
	IssueLabelNames(ctx context.Context, issueID uuid.UUID) ([]string, error)
	AddIssueLabels(ctx context.Context, issueID uuid.UUID, labelIDs []uuid.UUID) error
	RemoveIssueLabels(ctx context.Context, issueID uuid.UUID, labelIDs []uuid.UUID) error
}

// IssueOperations is the in-port of the bulk, clone, move, convert and export operations.
type IssueOperations interface {
	// BulkChange applies one action to every referenced issue. Each issue is checked and written on its own; the
	// result lists the successes and the per-issue failures. Only a malformed request is an error.
	BulkChange(ctx context.Context, actorID uuid.UUID, globalRole string, in BulkInput) (BulkResult, error)
	// CloneIssue copies an issue into a project the caller can create issues in.
	CloneIssue(ctx context.Context, sourceID, actorID uuid.UUID, globalRole string, in CloneInput) (*domain.Issue, error)
	// MoveIssue moves an issue and its children to another project under new keys.
	MoveIssue(ctx context.Context, id, actorID uuid.UUID, globalRole string, in MoveInput) (*MoveResult, error)
	// ConvertIssue changes the issue type through the hierarchy rules of the patch.
	ConvertIssue(ctx context.Context, id, actorID uuid.UUID, globalRole string, in ConvertInput) (*PatchResult, error)
	// ExportIssues streams the issues the caller can browse that match the filter, in batches. start runs once
	// before the first batch with whether the export was truncated at the row limit.
	ExportIssues(ctx context.Context, actorID uuid.UUID, globalRole string, filter ListFilter,
		start func(truncated bool) error, emit func(ExportBatch) error) error
	// ExportFileName is the download name of an export started now.
	ExportFileName() string
}
