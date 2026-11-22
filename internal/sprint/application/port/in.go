package port

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/jhonsferg/corelog/internal/sprint/domain"
)

// Opciones para las incidencias no cerradas al cerrar un sprint.
const (
	UnfinishedToBacklog = "backlog"
	UnfinishedToSprint  = "sprint"
)

// CloseInput indica que hacer con las incidencias no cerradas. TargetSprintID solo aplica a "sprint".
type CloseInput struct {
	Unfinished     string
	TargetSprintID *uuid.UUID
}

// BurndownPoint es un dia del burndown: trabajo restante real frente a la linea ideal.
type BurndownPoint struct {
	Date      time.Time
	Ideal     float64
	Remaining float64
}

// Service es el caso de uso de sprints. Todas las operaciones reciben el proyecto para comprobar permisos.
type Service interface {
	Create(ctx context.Context, actorID uuid.UUID, globalRole string, projectID uuid.UUID, name, goal string, start, end *time.Time) (*domain.Sprint, error)
	List(ctx context.Context, actorID uuid.UUID, globalRole string, projectID uuid.UUID) ([]*domain.Sprint, error)
	Start(ctx context.Context, actorID uuid.UUID, globalRole string, projectID, sprintID uuid.UUID) (*domain.Sprint, error)
	Close(ctx context.Context, actorID uuid.UUID, globalRole string, projectID, sprintID uuid.UUID, input CloseInput) (*domain.Sprint, error)
	ListIssues(ctx context.Context, actorID uuid.UUID, globalRole string, projectID, sprintID uuid.UUID) ([]IssueRef, error)
	AssignIssue(ctx context.Context, actorID uuid.UUID, globalRole string, projectID, sprintID, issueID uuid.UUID) error
	RemoveIssue(ctx context.Context, actorID uuid.UUID, globalRole string, projectID, sprintID, issueID uuid.UUID) error
	Burndown(ctx context.Context, actorID uuid.UUID, globalRole string, projectID, sprintID uuid.UUID) ([]BurndownPoint, error)
	Velocity(ctx context.Context, actorID uuid.UUID, globalRole string, projectID uuid.UUID, limit int) ([]VelocityPoint, error)
}
