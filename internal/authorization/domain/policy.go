package domain

type ProjectRole string

const (
	ProjectRoleLead        ProjectRole = "lead"
	ProjectRoleContributor ProjectRole = "contributor"
	ProjectRoleViewer      ProjectRole = "viewer"
)

type Policy struct{}

func (Policy) CanViewProject(globalRole string, projectRole *ProjectRole) bool {
	if globalRole == "admin" {
		return true
	}
	return projectRole != nil
}

func (Policy) CanEditProject(globalRole string, projectRole *ProjectRole) bool {
	if globalRole == "admin" {
		return true
	}
	return projectRole != nil && *projectRole == ProjectRoleLead
}

func (Policy) CanManageProjectMembers(globalRole string, projectRole *ProjectRole) bool {
	if globalRole == "admin" {
		return true
	}
	return projectRole != nil && *projectRole == ProjectRoleLead
}

func (Policy) CanCreateIssue(globalRole string, projectRole *ProjectRole) bool {
	if globalRole == "admin" {
		return true
	}
	return projectRole != nil && (*projectRole == ProjectRoleLead || *projectRole == ProjectRoleContributor)
}

func (Policy) CanEditIssue(globalRole string, projectRole *ProjectRole) bool {
	if globalRole == "admin" {
		return true
	}
	return projectRole != nil && (*projectRole == ProjectRoleLead || *projectRole == ProjectRoleContributor)
}

func (Policy) CanComment(globalRole string, projectRole *ProjectRole) bool {
	if globalRole == "admin" {
		return true
	}
	return projectRole != nil && (*projectRole == ProjectRoleLead || *projectRole == ProjectRoleContributor)
}
