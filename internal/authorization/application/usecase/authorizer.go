package usecase

import (
	"context"

	"github.com/google/uuid"

	"github.com/jhonsferg/corelog/internal/authorization/application/port"
	"github.com/jhonsferg/corelog/internal/authorization/domain"
	"github.com/jhonsferg/corelog/internal/platform/apperror"
)

// Authorizer decide si un usuario puede ejecutar una accion sobre un proyecto, aplicando la
// matriz de permisos del dominio sobre el contexto efectivo (domain.EffectiveRole). Es el unico
// punto donde se traduce accion a regla.
type Authorizer struct {
	roles  port.RoleLookup
	access port.AccessLookup // nil when the lookup only knows explicit project roles
	policy domain.Policy
}

// NewAuthorizer usa el contexto completo cuando el puerto lo implementa (AccessLookup) y, si no,
// solo la membresia de proyecto (FindRole), sin reglas de organizacion ni visibilidad.
func NewAuthorizer(roles port.RoleLookup) *Authorizer {
	access, _ := roles.(port.AccessLookup)
	return &Authorizer{roles: roles, access: access}
}

// Authorize devuelve nil si la accion esta permitida y un error de tipo forbidden si no.
// globalRole es el rol de plataforma autenticado: "admin" o "user". Cualquier otro valor se trata
// como "user".
func (a *Authorizer) Authorize(ctx context.Context, action port.Action, actorID uuid.UUID, globalRole string, projectID uuid.UUID) error {
	access, err := a.lookup(ctx, actorID, globalRole, projectID)
	if err != nil {
		return err
	}
	if !a.allowed(action, access) {
		return apperror.New(apperror.KindForbidden, "forbidden project operation")
	}
	return nil
}

func (a *Authorizer) lookup(ctx context.Context, actorID uuid.UUID, globalRole string, projectID uuid.UUID) (domain.Access, error) {
	platformRole := domain.PlatformRoleUser
	if globalRole == domain.PlatformRoleAdmin {
		platformRole = domain.PlatformRoleAdmin
	}
	if a.access != nil {
		found, err := a.access.FindAccess(ctx, projectID, actorID)
		if err != nil {
			return domain.Access{}, err
		}
		found.PlatformRole = platformRole
		return found, nil
	}
	role, err := a.roles.FindRole(ctx, projectID, actorID)
	if err != nil {
		return domain.Access{}, err
	}
	return domain.Access{PlatformRole: platformRole, OrgRole: domain.OrgRoleNone, ProjectRole: role}, nil
}

func (a *Authorizer) allowed(action port.Action, access domain.Access) bool {
	switch action {
	case port.ActionViewProject:
		return a.policy.CanViewProject(access)
	case port.ActionEditProject:
		return a.policy.CanEditProject(access)
	case port.ActionDeleteProject:
		return a.policy.CanDeleteProject(access)
	case port.ActionManageMembers:
		return a.policy.CanManageProjectMembers(access)
	case port.ActionCreateIssue:
		return a.policy.CanCreateIssue(access)
	case port.ActionEditIssue:
		return a.policy.CanEditIssue(access)
	case port.ActionComment:
		return a.policy.CanComment(access)
	case port.ActionWatchIssue:
		return a.policy.CanWatchIssue(access)
	case port.ActionMoveCards:
		return a.policy.CanMoveCards(access)
	case port.ActionManageBoard:
		return a.policy.CanManageBoard(access)
	case port.ActionManageWorkflow:
		return a.policy.CanManageWorkflow(access)
	case port.ActionManageSprints:
		return a.policy.CanManageSprints(access)
	default:
		return false
	}
}
