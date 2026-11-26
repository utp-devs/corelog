package port

import (
	"context"

	"github.com/google/uuid"
)

// OrgDirectory is the narrow read model of organization membership and policy that the project
// module needs. The composition root adapts the organization module to it.
type OrgDirectory interface {
	// MemberRole returns the user's org role (owner|admin|member|guest). found is false when the
	// user does not belong to the organization.
	MemberRole(ctx context.Context, orgID, userID uuid.UUID) (role string, found bool, err error)
	// ProjectCreationPolicy returns who_can_create_projects (members|admins) and the
	// default_project_visibility (open|private) of the organization.
	ProjectCreationPolicy(ctx context.Context, orgID uuid.UUID) (policy string, defaultVisibility string, err error)
}
