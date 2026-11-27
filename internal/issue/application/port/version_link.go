package port

import (
	"context"

	"github.com/google/uuid"

	"github.com/jhonsferg/corelog/internal/issue/domain"
)

// VersionLinkRepository reads and writes the versions and components linked to issues. The issue module declares
// it as its own port; the postgres adapter implements it.
type VersionLinkRepository interface {
	// ResolveVersions returns the versions among ids that belong to the project.
	ResolveVersions(ctx context.Context, projectID uuid.UUID, ids []uuid.UUID) ([]domain.VersionRef, error)
	// ResolveComponents returns the components among ids that belong to the project.
	ResolveComponents(ctx context.Context, projectID uuid.UUID, ids []uuid.UUID) ([]domain.LinkedRef, error)
	// ComponentDefaults returns the assignee policy of the components among ids, ordered by name.
	ComponentDefaults(ctx context.Context, projectID uuid.UUID, ids []uuid.UUID) ([]domain.ComponentDefault, error)
	// LoadLinks returns the links of every issue in ids with one query per kind.
	LoadLinks(ctx context.Context, issueIDs []uuid.UUID) (map[uuid.UUID]domain.IssueLinks, error)
	// ReplaceIssueVersions sets the versions of one kind (fix or affects) of an issue.
	ReplaceIssueVersions(ctx context.Context, issueID uuid.UUID, kind string, versionIDs []uuid.UUID) error
	// ReplaceIssueComponents sets the components of an issue.
	ReplaceIssueComponents(ctx context.Context, issueID uuid.UUID, componentIDs []uuid.UUID) error
}
