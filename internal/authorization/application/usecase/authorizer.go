package usecase

import (
	"context"

	"github.com/google/uuid"

	"github.com/jhonsferg/corelog/internal/authorization/application/port"
	"github.com/jhonsferg/corelog/internal/authorization/domain"
	"github.com/jhonsferg/corelog/internal/platform/apperror"
)

// Authorizer decide si un usuario puede ejecutar una accion sobre un proyecto, aplicando la
// matriz de permisos del dominio. Es el unico punto donde se traduce accion a regla.
type Authorizer struct {
	roles  port.RoleLookup
	policy domain.Policy
}

func NewAuthorizer(roles port.RoleLookup) *Authorizer {
	return &Authorizer{roles: roles}
}

// Authorize devuelve nil si la accion esta permitida y un error de tipo forbidden si no.
// El administrador global no necesita rol de proyecto, asi que no se consulta la membresia.
func (a *Authorizer) Authorize(ctx context.Context, action port.Action, actorID uuid.UUID, globalRole string, projectID uuid.UUID) error {
	var role *domain.ProjectRole
	if globalRole != "admin" {
		found, err := a.roles.FindRole(ctx, projectID, actorID)
		if err != nil {
			return err
		}
		role = found
	}
	if !a.allowed(action, globalRole, role) {
		return apperror.New(apperror.KindForbidden, "forbidden project operation")
	}
	return nil
}

func (a *Authorizer) allowed(action port.Action, globalRole string, role *domain.ProjectRole) bool {
	switch action {
	case port.ActionViewProject:
		return a.policy.CanViewProject(globalRole, role)
	case port.ActionEditProject:
		return a.policy.CanEditProject(globalRole, role)
	case port.ActionDeleteProject:
		return a.policy.CanDeleteProject(globalRole, role)
	case port.ActionManageMembers:
		return a.policy.CanManageProjectMembers(globalRole, role)
	case port.ActionCreateIssue:
		return a.policy.CanCreateIssue(globalRole, role)
	case port.ActionEditIssue:
		return a.policy.CanEditIssue(globalRole, role)
	case port.ActionComment:
		return a.policy.CanComment(globalRole, role)
	case port.ActionWatchIssue:
		return a.policy.CanWatchIssue(globalRole, role)
	case port.ActionMoveCards:
		return a.policy.CanMoveCards(globalRole, role)
	case port.ActionManageBoard:
		return a.policy.CanManageBoard(globalRole, role)
	case port.ActionManageWorkflow:
		return a.policy.CanManageWorkflow(globalRole, role)
	case port.ActionManageSprints:
		return a.policy.CanManageSprints(globalRole, role)
	default:
		return false
	}
}
