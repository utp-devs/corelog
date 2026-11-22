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
}

// RoleLookup es el puerto de salida que devuelve el rol de un usuario en un proyecto.
// Devuelve nil si el usuario no es miembro del proyecto.
type RoleLookup interface {
	FindRole(ctx context.Context, projectID, userID uuid.UUID) (*domain.ProjectRole, error)
}
