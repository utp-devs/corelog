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

// CanDeleteProject: solo el administrador global puede eliminar un proyecto.
func (Policy) CanDeleteProject(globalRole string, _ *ProjectRole) bool {
	return globalRole == "admin"
}

// CanWatchIssue: cualquier miembro del proyecto puede seguir una incidencia.
func (Policy) CanWatchIssue(globalRole string, projectRole *ProjectRole) bool {
	if globalRole == "admin" {
		return true
	}
	return projectRole != nil
}

// CanMoveCards: reordenar tarjetas y backlog exige poder editar incidencias.
func (Policy) CanMoveCards(globalRole string, projectRole *ProjectRole) bool {
	if globalRole == "admin" {
		return true
	}
	return projectRole != nil && (*projectRole == ProjectRoleLead || *projectRole == ProjectRoleContributor)
}

// CanManageBoard: gestionar tableros y columnas es exclusivo del lider del proyecto.
func (Policy) CanManageBoard(globalRole string, projectRole *ProjectRole) bool {
	if globalRole == "admin" {
		return true
	}
	return projectRole != nil && *projectRole == ProjectRoleLead
}

// CanManageWorkflow: gestionar el flujo de estados del proyecto es exclusivo del lider del proyecto.
func (Policy) CanManageWorkflow(globalRole string, projectRole *ProjectRole) bool {
	if globalRole == "admin" {
		return true
	}
	return projectRole != nil && *projectRole == ProjectRoleLead
}

// CanManageSprints: planificar, iniciar y cerrar sprints es exclusivo del lider del proyecto.
func (Policy) CanManageSprints(globalRole string, projectRole *ProjectRole) bool {
	if globalRole == "admin" {
		return true
	}
	return projectRole != nil && *projectRole == ProjectRoleLead
}
