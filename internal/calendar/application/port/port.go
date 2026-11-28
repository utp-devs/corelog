package port

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// Acciones del autorizador de proyecto que usa el calendario.
const ActionViewProject = "view_project"

// Authorizer decide si un actor puede ver un proyecto. Lo implementa el root de composicion.
type Authorizer interface {
	Authorize(ctx context.Context, action string, actorID uuid.UUID, globalRole string, projectID uuid.UUID) error
}

// Event es un hito del calendario del proyecto: inicio o fin de sprint, o fecha limite de una incidencia.
type Event struct {
	Kind     string
	Date     time.Time
	Title    string
	SourceID uuid.UUID
	Link     string
	// ProjectID and ProjectKey are filled by the multi-project reader.
	ProjectID  uuid.UUID
	ProjectKey string
}

// Repository lee los eventos del proyecto dentro de un rango de fechas (ambos extremos incluidos).
type Repository interface {
	Events(ctx context.Context, projectID uuid.UUID, from, to time.Time) ([]Event, error)
}

// Service es el caso de uso del calendario.
type Service interface {
	Events(ctx context.Context, actorID uuid.UUID, globalRole string, projectID uuid.UUID, from, to time.Time) ([]Event, error)
}
