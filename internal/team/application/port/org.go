package port

import (
	"context"

	"github.com/google/uuid"
)

// OrgDirectory is the narrow view of the organization module that teams need.
// The organization module (or its postgres adapter) implements it; cmd/api wires it.
type OrgDirectory interface {
	// MemberRole returns the organization role of userID in orgID (owner|admin|member|guest).
	// found is false when the user is not a member of the organization.
	MemberRole(ctx context.Context, orgID, userID uuid.UUID) (role string, found bool, err error)
	// TeamCreationPolicy returns the org setting who_can_create_teams: "members" or "admins".
	TeamCreationPolicy(ctx context.Context, orgID uuid.UUID) (string, error)
}
