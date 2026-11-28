package port

import (
	"context"
	"io"

	"github.com/google/uuid"
)

// OrgAccess answers whether a user may read the audit log of an organization. Implementations return a
// not-found error for users who are not members and a forbidden error for members below admin.
type OrgAccess interface {
	RequireAdmin(ctx context.Context, userID, orgID uuid.UUID) error
}

// Service is the read side of the audit log.
type Service interface {
	// ListPlatform lists entries of every organization (platform administrators only; the caller checks the role).
	ListPlatform(ctx context.Context, f ListFilter) (ListPage, error)
	// ListOrganization lists the entries of one organization for its owners and admins.
	ListOrganization(ctx context.Context, actorID, orgID uuid.UUID, f ListFilter) (ListPage, error)
	// ExportPlatform and ExportOrganization write the matching entries as CSV.
	ExportPlatform(ctx context.Context, f ListFilter, w io.Writer) error
	ExportOrganization(ctx context.Context, actorID, orgID uuid.UUID, f ListFilter, w io.Writer) error
}
