package port

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/jhonsferg/corelog/internal/sprint/domain"
)

// Acciones del autorizador de proyecto que usa el modulo sprint (nombres de la matriz de permisos).
const (
	ActionViewProject   = "view_project"
	ActionManageSprints = "manage_sprint"
)

// Authorizer decide si un actor puede ejecutar una accion sobre un proyecto. Lo implementa el root.
type Authorizer interface {
	Authorize(ctx context.Context, action string, actorID uuid.UUID, globalRole string, projectID uuid.UUID) error
}

// Repository persiste sprints y la asignacion de incidencias a sprints.
type Repository interface {
	Save(ctx context.Context, sprint *domain.Sprint) error
	FindByID(ctx context.Context, id uuid.UUID) (*domain.Sprint, error)
	ListByProject(ctx context.Context, projectID uuid.UUID) ([]*domain.Sprint, error)
	Update(ctx context.Context, sprint *domain.Sprint) error
	HasActive(ctx context.Context, projectID, exceptID uuid.UUID) (bool, error)
	// SetIssueSprint asigna la incidencia del proyecto al sprint indicado, o la quita del sprint si es nil.
	SetIssueSprint(ctx context.Context, projectID, issueID uuid.UUID, sprintID *uuid.UUID) error
	RemoveIssueFromSprint(ctx context.Context, projectID, sprintID, issueID uuid.UUID) error
	ListIssues(ctx context.Context, sprintID uuid.UUID) ([]IssueRef, error)
	// MoveUnfinished mueve las incidencias no cerradas del sprint al destino (nil = backlog).
	MoveUnfinished(ctx context.Context, sprintID uuid.UUID, target *uuid.UUID) error
	Velocity(ctx context.Context, projectID uuid.UUID, limit int) ([]VelocityPoint, error)
}

// IssueRef es la vista de una incidencia dentro de un sprint, con lo necesario para el burndown.
type IssueRef struct {
	ID          uuid.UUID
	Title       string
	Status      string
	Priority    string
	StoryPoints *float64
	ClosedAt    *time.Time
}

// VelocityPoint son los puntos completados de un sprint cerrado.
type VelocityPoint struct {
	SprintID uuid.UUID
	Name     string
	Points   float64
}
