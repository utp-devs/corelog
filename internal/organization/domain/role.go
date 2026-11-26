package domain

// Role is an organization role (organization_members.role).
type Role string

const (
	RoleOwner  Role = "owner"
	RoleAdmin  Role = "admin"
	RoleMember Role = "member"
	RoleGuest  Role = "guest"
)

// IsValid reports whether r is one of the four organization roles.
func (r Role) IsValid() bool {
	return r.Rank() > 0
}

// Rank orders roles by power: owner > admin > member > guest. Unknown roles rank 0.
func (r Role) Rank() int {
	switch r {
	case RoleOwner:
		return 4
	case RoleAdmin:
		return 3
	case RoleMember:
		return 2
	case RoleGuest:
		return 1
	default:
		return 0
	}
}

// CanChangeRole decides whether an actor with role actor may change a member whose current role is
// target to newRole. Only an owner may grant or revoke owner or admin. An admin may manage members
// and guests only. Everybody else may not change roles. The rule against changing one's own role is
// enforced by the caller, because this function only sees roles.
func CanChangeRole(actor, target, newRole Role) error {
	if !actor.IsValid() || !target.IsValid() || !newRole.IsValid() {
		return ErrRoleInvalid
	}
	switch actor {
	case RoleOwner:
		return nil
	case RoleAdmin:
		if target == RoleOwner || target == RoleAdmin || newRole == RoleOwner || newRole == RoleAdmin {
			return ErrForbidden
		}
		return nil
	default:
		return ErrForbidden
	}
}

// ValidateLastOwner must receive the number of owners that remain after a change. Zero is rejected.
func ValidateLastOwner(owners int) error {
	if owners < 1 {
		return ErrLastOwner
	}
	return nil
}
