package usecase

import (
	"context"

	"github.com/google/uuid"

	"github.com/jhonsferg/corelog/internal/authorization/application/port"
	"github.com/jhonsferg/corelog/internal/authorization/domain"
	"github.com/jhonsferg/corelog/internal/platform/apperror"
)

// Authorizer decide si un usuario puede ejecutar una accion sobre un proyecto. La decision usa el
// esquema de permisos del proyecto (el guardado o, si no hay, el por defecto) aplicado al contexto
// efectivo del usuario. Es el unico punto donde se traduce accion a permiso.
//
// Proyectos archivados: solo se permiten las operaciones de lectura y las de la lista de acciones
// permitidas en archivados (ver domain.ArchivedAllowed). Cualquier otra escritura, aunque el usuario
// tenga el permiso, responde domain.ErrProjectArchived (409).
type Authorizer struct {
	roles   port.RoleLookup
	access  port.AccessLookup // nil when the lookup only knows explicit project roles
	schemes port.SchemeRepository
}

// NewAuthorizer usa el contexto completo cuando el puerto lo implementa (AccessLookup) y, si no,
// solo la membresia de proyecto (FindRole), sin reglas de organizacion, visibilidad ni archivado.
// Sin esquemas conectados, todos los proyectos usan el esquema por defecto.
func NewAuthorizer(roles port.RoleLookup) *Authorizer {
	access, _ := roles.(port.AccessLookup)
	return &Authorizer{roles: roles, access: access}
}

// WithSchemes conecta el repositorio de esquemas editados por proyecto.
func (a *Authorizer) WithSchemes(schemes port.SchemeRepository) *Authorizer {
	a.schemes = schemes
	return a
}

// Authorize devuelve nil si la accion esta permitida, un error forbidden si el usuario no tiene el
// permiso, o domain.ErrProjectArchived si lo tiene pero el proyecto esta archivado y la accion es
// una escritura. globalRole es el rol de plataforma autenticado: "admin" o "user". Cualquier otro
// valor se trata como "user".
func (a *Authorizer) Authorize(ctx context.Context, action port.Action, actorID uuid.UUID, globalRole string, projectID uuid.UUID) error {
	permission, ok := domain.PermissionForAction(string(action))
	if !ok {
		return forbidden()
	}
	return a.requireWith(ctx, string(action), actorID, globalRole, projectID, permission)
}

// Require es como Authorize para quien conoce el permiso y no una accion con nombre. Las escrituras
// sobre un proyecto archivado responden domain.ErrProjectArchived, salvo las de lectura.
func (a *Authorizer) Require(ctx context.Context, actorID uuid.UUID, globalRole string, projectID uuid.UUID, permission domain.Permission, holders ...domain.Holder) error {
	return a.requireWith(ctx, "", actorID, globalRole, projectID, permission, holders...)
}

// Can decide si el usuario tiene el permiso sobre el proyecto. holders son los titulares especiales
// del objeto que el llamador conoce (reporter, assignee, watcher); los roles y org_admin se derivan
// del contexto del usuario. Una escritura sobre un proyecto archivado devuelve false sin error.
func (a *Authorizer) Can(ctx context.Context, actorID uuid.UUID, globalRole string, projectID uuid.UUID, permission domain.Permission, holders ...domain.Holder) (bool, error) {
	access, err := a.lookup(ctx, actorID, globalRole, projectID)
	if err != nil {
		return false, err
	}
	scheme, err := a.scheme(ctx, projectID)
	if err != nil {
		return false, err
	}
	specials := append(domain.SpecialHolders(access), holders...)
	allowed := domain.Allowed(scheme, permission, domain.EffectiveRole(access), specials...)
	return allowed && !domain.ArchiveBlocks(access, "", permission), nil
}

// Effective devuelve el rol efectivo del usuario en el proyecto (nil si no tiene) y los permisos del
// catalogo que obtiene sin titulares especiales, en orden de catalogo. En un proyecto archivado solo
// aparecen los permisos de lectura y los permitidos en archivados.
func (a *Authorizer) Effective(ctx context.Context, actorID uuid.UUID, globalRole string, projectID uuid.UUID) (*domain.ProjectRole, []domain.Permission, error) {
	access, err := a.lookup(ctx, actorID, globalRole, projectID)
	if err != nil {
		return nil, nil, err
	}
	scheme, err := a.scheme(ctx, projectID)
	if err != nil {
		return nil, nil, err
	}
	role := domain.EffectiveRole(access)
	specials := domain.SpecialHolders(access)
	granted := []domain.Permission{}
	for _, p := range domain.Permissions {
		if domain.Allowed(scheme, p, role, specials...) && !domain.ArchiveBlocks(access, "", p) {
			granted = append(granted, p)
		}
	}
	return role, granted, nil
}

// requireWith evaluates the decision and maps the outcome to the error contract: forbidden when the
// permission is missing, ErrProjectArchived when it is held but the archive rules block it.
func (a *Authorizer) requireWith(ctx context.Context, action string, actorID uuid.UUID, globalRole string, projectID uuid.UUID, permission domain.Permission, holders ...domain.Holder) error {
	access, err := a.lookup(ctx, actorID, globalRole, projectID)
	if err != nil {
		return err
	}
	scheme, err := a.scheme(ctx, projectID)
	if err != nil {
		return err
	}
	specials := append(domain.SpecialHolders(access), holders...)
	if !domain.Allowed(scheme, permission, domain.EffectiveRole(access), specials...) {
		return forbidden()
	}
	if domain.ArchiveBlocks(access, action, permission) {
		return domain.ErrProjectArchived
	}
	return nil
}

// scheme devuelve el esquema guardado del proyecto o el esquema por defecto si no tiene filas.
func (a *Authorizer) scheme(ctx context.Context, projectID uuid.UUID) (domain.Scheme, error) {
	if a.schemes == nil {
		return domain.DefaultScheme(), nil
	}
	stored, err := a.schemes.Load(ctx, projectID)
	if err != nil {
		return domain.Scheme{}, err
	}
	if stored == nil {
		return domain.DefaultScheme(), nil
	}
	return *stored, nil
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

func forbidden() error {
	return apperror.New(apperror.KindForbidden, "forbidden project operation")
}
