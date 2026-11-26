package postgres

import (
	"context"
	"database/sql"
	"errors"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"

	"github.com/jhonsferg/corelog/internal/authorization/application/port"
	"github.com/jhonsferg/corelog/internal/authorization/domain"
	"github.com/jhonsferg/corelog/internal/platform/apperror"
)

// RoleLookup resuelve el contexto de autorizacion de un usuario sobre un proyecto desde
// projects, organization_members, organizations y project_members.
type RoleLookup struct {
	db *sqlx.DB
}

func NewRoleLookup(db *sqlx.DB) *RoleLookup {
	return &RoleLookup{db: db}
}

var (
	_ port.RoleLookup   = (*RoleLookup)(nil)
	_ port.AccessLookup = (*RoleLookup)(nil)
)

// accessQuery devuelve una fila por proyecto (cero filas si no existe). La membresia de organizacion
// solo cuenta si la organizacion no esta eliminada. El rol de proyecto solo cuenta si el usuario es
// miembro vivo de la organizacion del proyecto: un miembro retirado no conserva acceso.
const accessQuery = `
SELECT p.visibility,
       om.role,
       CASE WHEN om.user_id IS NULL THEN NULL ELSE pm.role END
FROM projects p
LEFT JOIN organizations o ON o.id = p.organization_id AND o.deleted_at IS NULL
LEFT JOIN organization_members om ON om.organization_id = o.id AND om.user_id = $2
LEFT JOIN project_members pm ON pm.project_id = p.id AND pm.user_id = $2
WHERE p.id = $1`

// FindAccess devuelve el contexto efectivo del usuario sobre el proyecto. Un proyecto inexistente
// devuelve un Access vacio (sin acceso) y ningun error.
func (r *RoleLookup) FindAccess(ctx context.Context, projectID, userID uuid.UUID) (domain.Access, error) {
	var (
		visibility sql.NullString
		orgRole    sql.NullString
		projRole   sql.NullString
	)
	err := r.db.QueryRowContext(ctx, accessQuery, projectID, userID).Scan(&visibility, &orgRole, &projRole)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.Access{}, nil
	}
	if err != nil {
		return domain.Access{}, apperror.Wrap(apperror.KindInternal, "failed to look up project access", err)
	}
	access := domain.Access{OrgRole: domain.OrgRoleNone, Visibility: domain.VisibilityOpen}
	if visibility.String == string(domain.VisibilityPrivate) {
		access.Visibility = domain.VisibilityPrivate
	}
	if orgRole.Valid {
		access.OrgRole = domain.OrgRole(orgRole.String)
	}
	if projRole.Valid {
		role := domain.ProjectRole(projRole.String)
		access.ProjectRole = &role
	}
	return access, nil
}

// FindRole devuelve solo la membresia explicita del usuario en el proyecto, o nil si no es miembro.
// Lo usan los adaptadores que no conocen la organizacion; el caso de uso prefiere FindAccess.
func (r *RoleLookup) FindRole(ctx context.Context, projectID, userID uuid.UUID) (*domain.ProjectRole, error) {
	var role string
	err := r.db.GetContext(ctx, &role,
		`SELECT role FROM project_members WHERE project_id = $1 AND user_id = $2`,
		projectID, userID,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, apperror.Wrap(apperror.KindInternal, "failed to look up project role", err)
	}
	value := domain.ProjectRole(role)
	return &value, nil
}
