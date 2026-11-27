package port

import (
	"context"

	"github.com/google/uuid"

	"github.com/jhonsferg/corelog/internal/authorization/domain"
)

// Action es una operacion sensible sobre un proyecto o sus incidencias.
type Action string

const (
	ActionViewProject    Action = "view_project"
	ActionEditProject    Action = "edit_project"
	ActionDeleteProject  Action = "delete_project"
	ActionManageMembers  Action = "manage_members"
	ActionCreateIssue    Action = "create_issue"
	ActionEditIssue      Action = "edit_issue"
	ActionComment        Action = "comment"
	ActionWatchIssue     Action = "watch_issue"
	ActionMoveCards      Action = "move_cards"
	ActionManageBoard    Action = "manage_board"
	ActionManageWorkflow Action = "manage_workflow"
	ActionManageSprints  Action = "manage_sprint"
	ActionArchiveProject Action = "archive_project"
	ActionRestoreProject Action = "restore_project"
)

// Actions lista todas las acciones de la matriz de permisos.
var Actions = []Action{
	ActionViewProject,
	ActionEditProject,
	ActionDeleteProject,
	ActionManageMembers,
	ActionCreateIssue,
	ActionEditIssue,
	ActionComment,
	ActionWatchIssue,
	ActionMoveCards,
	ActionManageBoard,
	ActionManageWorkflow,
	ActionManageSprints,
	ActionArchiveProject,
	ActionRestoreProject,
}

// RoleLookup es el puerto de salida que devuelve el rol de proyecto explicito de un usuario.
// Devuelve nil si el usuario no es miembro del proyecto. Se conserva para los adaptadores que
// solo conocen la membresia de proyecto; el caso de uso prefiere AccessLookup cuando existe.
type RoleLookup interface {
	FindRole(ctx context.Context, projectID, userID uuid.UUID) (*domain.ProjectRole, error)
}

// AccessLookup devuelve el contexto efectivo del usuario sobre un proyecto: membresia de proyecto,
// rol en la organizacion propietaria (organizacion eliminada = sin rol) y visibilidad del proyecto.
// Un proyecto inexistente devuelve un Access vacio sin error.
type AccessLookup interface {
	FindAccess(ctx context.Context, projectID, userID uuid.UUID) (domain.Access, error)
}

// SchemeRepository almacena los esquemas de permisos editados por proyecto. Un proyecto sin filas
// usa el esquema por defecto: Load devuelve nil en ese caso.
type SchemeRepository interface {
	Load(ctx context.Context, projectID uuid.UUID) (*domain.Scheme, error)
	Replace(ctx context.Context, projectID uuid.UUID, scheme domain.Scheme) error
	Reset(ctx context.Context, projectID uuid.UUID) error
}
