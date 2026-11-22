package port

import (
	"context"

	"github.com/google/uuid"
	"github.com/jhonsferg/corelog/internal/board/domain"
)

type Repository interface {
	FindDefaultByProject(ctx context.Context, projectID uuid.UUID) (*domain.Board, error)
	UpdateWIPLimit(ctx context.Context, columnID uuid.UUID, limit *int) error
}

// ActionManageBoard es la accion del autorizador para gestionar tableros y columnas.
const ActionManageBoard = "manage_board"

// ActionMoveCards es la accion del autorizador para mover tarjetas entre columnas.
const ActionMoveCards = "move_cards"

// Authorizer decide si un actor puede ejecutar una accion sobre un proyecto (matriz de permisos).
type Authorizer interface {
	Authorize(ctx context.Context, action string, actorID uuid.UUID, globalRole string, projectID uuid.UUID) error
}

// IssueMover cambia el estado de una incidencia del proyecto. Lo implementa el modulo ticket
// mediante un adaptador en el root de composicion, asi el tablero no importa tipos de ticket.
type IssueMover interface {
	MoveIssue(ctx context.Context, actorID uuid.UUID, globalRole string, projectID, issueID uuid.UUID, status string) error
}
