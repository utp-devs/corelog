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
// solo cuenta si la organizacion no esta eliminada. El rol de proyecto directo y el mejor rol de
// equipo solo cuentan si el usuario es miembro vivo de la organizacion del proyecto: un miembro
// retirado no conserva acceso. El mejor rol de equipo ordena lead, contributor, viewer.
const accessQuery = `
SELECT p.visibility,
       p.archived_at IS NOT NULL,
       om.role,
       CASE WHEN om.user_id IS NULL THEN NULL ELSE pm.role END,
       CASE WHEN om.user_id IS NULL THEN NULL ELSE (
           SELECT ptr.role FROM project_team_roles ptr
           JOIN team_members tm ON tm.team_id = ptr.team_id
           WHERE ptr.project_id = p.id AND tm.user_id = $2
           ORDER BY CASE ptr.role WHEN 'lead' THEN 0 WHEN 'contributor' THEN 1 ELSE 2 END
           LIMIT 1) END
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
		archived   bool
		orgRole    sql.NullString
		projRole   sql.NullString
		teamRole   sql.NullString
	)
	err := r.db.QueryRowContext(ctx, accessQuery, projectID, userID).Scan(&visibility, &archived, &orgRole, &projRole, &teamRole)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.Access{}, nil
	}
	if err != nil {
		return domain.Access{}, apperror.Wrap(apperror.KindInternal, "failed to look up project access", err)
	}
	access := domain.Access{OrgRole: domain.OrgRoleNone, Visibility: domain.VisibilityOpen}
	access.Archived = archived
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
	if teamRole.Valid {
		role := domain.ProjectRole(teamRole.String)
		access.TeamRole = &role
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
