package domain

// Policy maps the effective project role of a caller to the project permissions. Every check is
// made on the effective role from EffectiveRole, so org owner/admin are lead everywhere and
// platform admin only reads.
type Policy struct{}

// hasAny reports whether the effective role is one of the allowed roles.
func hasAny(role *ProjectRole, allowed ...ProjectRole) bool {
	if role == nil {
		return false
	}
	for _, r := range allowed {
		if *role == r {
			return true
		}
	}
	return false
}

func (Policy) CanViewProject(a Access) bool {
	return hasAny(EffectiveRole(a), ProjectRoleLead, ProjectRoleContributor, ProjectRoleViewer)
}

func (Policy) CanEditProject(a Access) bool {
	return hasAny(EffectiveRole(a), ProjectRoleLead)
}

// CanDeleteProject: the project lead or an organization owner/admin (both are lead through
// EffectiveRole). Platform admin support access is not enough. Callers must still confirm the
// project name.
func (Policy) CanDeleteProject(a Access) bool {
	return a.OrgRole == OrgRoleOwner || a.OrgRole == OrgRoleAdmin || hasAny(EffectiveRole(a), ProjectRoleLead)
}

func (Policy) CanManageProjectMembers(a Access) bool {
	return hasAny(EffectiveRole(a), ProjectRoleLead)
}

func (Policy) CanCreateIssue(a Access) bool {
	return hasAny(EffectiveRole(a), ProjectRoleLead, ProjectRoleContributor)
}

func (Policy) CanEditIssue(a Access) bool {
	return hasAny(EffectiveRole(a), ProjectRoleLead, ProjectRoleContributor)
}

func (Policy) CanComment(a Access) bool {
	return hasAny(EffectiveRole(a), ProjectRoleLead, ProjectRoleContributor)
}

// CanWatchIssue: any project reader may follow an issue.
func (Policy) CanWatchIssue(a Access) bool {
	return hasAny(EffectiveRole(a), ProjectRoleLead, ProjectRoleContributor, ProjectRoleViewer)
}

// CanMoveCards: reordering cards and the backlog requires being able to edit issues.
func (Policy) CanMoveCards(a Access) bool {
	return hasAny(EffectiveRole(a), ProjectRoleLead, ProjectRoleContributor)
}

// CanManageBoard: managing boards and columns is lead-only.
func (Policy) CanManageBoard(a Access) bool {
	return hasAny(EffectiveRole(a), ProjectRoleLead)
}

// CanManageWorkflow: managing the project status flow is lead-only.
func (Policy) CanManageWorkflow(a Access) bool {
	return hasAny(EffectiveRole(a), ProjectRoleLead)
}

// CanManageSprints: planning, starting and closing sprints is lead-only.
func (Policy) CanManageSprints(a Access) bool {
	return hasAny(EffectiveRole(a), ProjectRoleLead)
}
