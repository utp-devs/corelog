package port

import (
	"context"

	"github.com/google/uuid"

	"github.com/jhonsferg/corelog/internal/issue/domain"
)

// MoveInput is the destination of a move. IssueTypeID, when set, is the type of the moved issue in the target
// project (it applies to the moved issue only, not to its children). Mapping resolves the other values.
type MoveInput struct {
	ProjectID   uuid.UUID
	IssueTypeID *uuid.UUID
	Mapping     domain.MoveMapping
}

// MovedIssue is one issue that a move re-keyed.
type MovedIssue struct {
	ID      uuid.UUID
	FromKey string
	ToKey   string
}

// MoveResult is the moved issue with its warnings and the issues that moved with it.
type MoveResult struct {
	Issue    *domain.Issue
	Warnings []string
	Moved    []MovedIssue
}

// RelocateResult is the new key of a relocated issue and what the target project did not keep.
type RelocateResult struct {
	KeyNumber         int
	ProjectKey        string
	DroppedLabels     int
	DroppedComponents int
}

// MoveRepository relocates issues between projects and resolves the keys they had before.
type MoveRepository interface {
	// Relocate moves the issue to the target project under the next number of its counter. The previous key
	// (source project and number) is kept as an alias. Labels and components of the source project are removed.
	Relocate(ctx context.Context, issueID, sourceProjectID uuid.UUID, sourceNumber int, targetProjectID uuid.UUID) (RelocateResult, error)
	// FindAliasIssues returns the live issues that had the key projectKey-number, in the organization when orgID is
	// not Nil. Visibility is applied by the caller.
	FindAliasIssues(ctx context.Context, projectKey string, number int, orgID uuid.UUID) ([]uuid.UUID, error)
}

// OperationsRepository is the adapter that implements every out-port of the bulk, clone, move and export
// operations on one database.
type OperationsRepository interface {
	MoveRepository
	CloneRepository
	BulkLabelRepository
	ExportLabelReader
}
