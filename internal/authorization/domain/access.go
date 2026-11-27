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
	// TeamRole is the best grant of a project team the caller belongs to, nil when there is none.
	TeamRole   *ProjectRole
	Visibility Visibility
	// Archived is true when the project is archived: every write except the archive-allowed set is
	// blocked, while reads keep working.
	Archived bool
}

// rank orders project roles: lead is the strongest. Unknown values rank last.
func rank(r ProjectRole) int {
	switch r {
	case ProjectRoleLead:
		return 0
	case ProjectRoleContributor:
		return 1
	case ProjectRoleViewer:
		return 2
	default:
		return 3
	}
}

// BestRole returns the stronger of two optional project roles. A nil role means no role.
func BestRole(a, b *ProjectRole) *ProjectRole {
	switch {
	case a == nil:
		return b
	case b == nil:
		return a
	case rank(*b) < rank(*a):
		return b
	default:
		return a
	}
}

// EffectiveRole resolves the project role used by every permission check.
// The first rule that applies wins:
//  1. org owner/admin: lead on every project of the organization, private ones included.
//  2. the best of the explicit project membership and every team grant the caller belongs to.
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
	if direct := BestRole(a.ProjectRole, a.TeamRole); direct != nil {
		return direct
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
