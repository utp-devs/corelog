package domain

// ProjectRole is the role of a user inside one project (project_members.role).
type ProjectRole string

const (
	ProjectRoleLead        ProjectRole = "lead"
	ProjectRoleContributor ProjectRole = "contributor"
	ProjectRoleViewer      ProjectRole = "viewer"
)

// PlatformRole is the global role of an account (users.role): admin or user.
const (
	PlatformRoleAdmin = "admin"
	PlatformRoleUser  = "user"
)

// OrgRole is the role of a user in the organization that owns a project (organization_members.role).
// OrgRoleNone means the user is not a live member of that organization.
type OrgRole string

const (
	OrgRoleOwner  OrgRole = "owner"
	OrgRoleAdmin  OrgRole = "admin"
	OrgRoleMember OrgRole = "member"
	OrgRoleGuest  OrgRole = "guest"
	OrgRoleNone   OrgRole = "none"
)

// Visibility of a project. The legacy value 'team' is read as open.
type Visibility string

const (
	VisibilityOpen    Visibility = "open"
	VisibilityPrivate Visibility = "private"
)

// Access is the effective context of one caller for one project. It is what the policy decides on.
type Access struct {
	PlatformRole string
	OrgRole      OrgRole
	// ProjectRole is the explicit project membership, nil when the caller has none.
	ProjectRole *ProjectRole
	Visibility  Visibility
}

// EffectiveRole resolves the project role used by every permission check.
// The first rule that applies wins:
//  1. org owner/admin: lead on every project of the organization, private ones included.
//  2. explicit project membership.
//  3. org member (not guest) on an open project: viewer.
//  4. platform admin: viewer (support access). It never grants manage rights. Support access is
//     audited in a later change.
//
// Anything else is nil (no access).
func EffectiveRole(a Access) *ProjectRole {
	switch a.OrgRole {
	case OrgRoleOwner, OrgRoleAdmin:
		return roleOf(ProjectRoleLead)
	}
	if a.ProjectRole != nil {
		return a.ProjectRole
	}
	if a.OrgRole == OrgRoleMember && a.Visibility == VisibilityOpen {
		return roleOf(ProjectRoleViewer)
	}
	if a.PlatformRole == PlatformRoleAdmin {
		return roleOf(ProjectRoleViewer)
	}
	return nil
}

func roleOf(r ProjectRole) *ProjectRole {
	return &r
}
