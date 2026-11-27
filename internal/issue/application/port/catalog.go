package port

import (
	"context"

	"github.com/google/uuid"

	"github.com/jhonsferg/corelog/internal/issue/domain"
)

// CatalogSelector identifies a catalog item by id, by legacy key or as the organization default.
type CatalogSelector struct {
	ID        uuid.UUID
	LegacyKey string
	Default   bool
}

// CatalogStore is the out-port of the catalogs of an organization and of the project issue-type settings.
type CatalogStore interface {
	// LockOrganization serialises catalog mutations of one organization until the transaction ends.
	LockOrganization(ctx context.Context, orgID uuid.UUID) error
	ListItems(ctx context.Context, orgID uuid.UUID, kind domain.CatalogKind) ([]*domain.CatalogItem, error)
	FindItem(ctx context.Context, orgID uuid.UUID, kind domain.CatalogKind, id uuid.UUID) (*domain.CatalogItem, error)
	InsertItem(ctx context.Context, item *domain.CatalogItem) error
	UpdateItem(ctx context.Context, item *domain.CatalogItem) error
	DeleteItem(ctx context.Context, item *domain.CatalogItem) error
	// SetDefaultItem makes id the only default item of its kind in the organization.
	SetDefaultItem(ctx context.Context, orgID uuid.UUID, kind domain.CatalogKind, id uuid.UUID) error
	// SetPositions stores the position of each id (its index in ids).
	SetPositions(ctx context.Context, orgID uuid.UUID, kind domain.CatalogKind, ids []uuid.UUID) error
	// MoveIssues reassigns every issue that uses from to target and returns how many issues moved.
	MoveIssues(ctx context.Context, from, target *domain.CatalogItem) (int64, error)
	// ProjectOrganization returns the organization that owns the project. Missing projects return a not found error.
	ProjectOrganization(ctx context.Context, projectID uuid.UUID) (uuid.UUID, error)
	// ProjectIssueTypes returns the issue types configured for the project in position order. The flag is
	// false when the project has no configuration, in which case every issue type of the organization applies.
	ProjectIssueTypes(ctx context.Context, projectID uuid.UUID) ([]*domain.CatalogItem, bool, error)
	ReplaceProjectIssueTypes(ctx context.Context, projectID uuid.UUID, issueTypeIDs []uuid.UUID) error
	// SeedDefaults inserts the default catalogs of an organization. Existing items are left untouched.
	SeedDefaults(ctx context.Context, orgID uuid.UUID) error
}

// IssueCatalog is the narrow read port the issue use cases need to resolve catalog references.
type IssueCatalog interface {
	// Select resolves a catalog item of the project's organization. Unknown items return domain.ErrCatalogNotFound.
	Select(ctx context.Context, projectID uuid.UUID, kind domain.CatalogKind, sel CatalogSelector) (*domain.CatalogItem, error)
	IssueTypeEnabled(ctx context.Context, projectID, issueTypeID uuid.UUID) (bool, error)
}

// OrgRoleChecker checks the role of a user in an organization. It returns the organization's typed errors
// (not found for outsiders, forbidden below min).
type OrgRoleChecker interface {
	RequireOrgRole(ctx context.Context, userID, orgID uuid.UUID, min string) error
}

// CatalogInput is the body of a catalog item creation.
type CatalogInput struct {
	Name           string
	Description    string
	Icon           string
	Color          string
	HierarchyLevel int
	IsDefault      bool
}

// CatalogPatch is a partial change of a catalog item. Nil fields keep their value.
type CatalogPatch struct {
	Name        *string
	Description *string
	Icon        *string
	Color       *string
	IsDefault   *bool
}

// CatalogService is the in-port that manages catalogs and project issue types.
type CatalogService interface {
	ListCatalog(ctx context.Context, actorID, orgID uuid.UUID, kind domain.CatalogKind) ([]*domain.CatalogItem, error)
	CreateCatalogItem(ctx context.Context, actorID, orgID uuid.UUID, kind domain.CatalogKind, in CatalogInput) (*domain.CatalogItem, error)
	UpdateCatalogItem(ctx context.Context, actorID, orgID uuid.UUID, kind domain.CatalogKind, id uuid.UUID, patch CatalogPatch) (*domain.CatalogItem, error)
	// DeleteCatalogItem removes an item. When issues use it, reassignTo must name a compatible item, which
	// receives those issues in the same transaction.
	DeleteCatalogItem(ctx context.Context, actorID, orgID uuid.UUID, kind domain.CatalogKind, id uuid.UUID, reassignTo *uuid.UUID) error
	ReorderCatalog(ctx context.Context, actorID, orgID uuid.UUID, kind domain.CatalogKind, ids []uuid.UUID) ([]*domain.CatalogItem, error)
	ProjectIssueTypes(ctx context.Context, actorID uuid.UUID, globalRole string, projectID uuid.UUID) ([]*domain.CatalogItem, bool, error)
	ReplaceProjectIssueTypes(ctx context.Context, actorID uuid.UUID, globalRole string, projectID uuid.UUID, issueTypeIDs []uuid.UUID) ([]*domain.CatalogItem, error)
}
