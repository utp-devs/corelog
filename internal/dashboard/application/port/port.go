package port

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// SprintRef es un sprint activo de uno de los proyectos del usuario.
type SprintRef struct {
	ID         uuid.UUID
	Name       string
	ProjectKey string
	EndDate    *time.Time
}

// IssueRef es una incidencia abierta asignada al usuario.
type IssueRef struct {
	ID         uuid.UUID
	Title      string
	Status     string
	Priority   string
	ProjectKey string
	DueDate    *time.Time
}

// Summary es el resumen personal: trabajo asignado, retrasos, proyectos y sprints activos.
type Summary struct {
	AssignedOpen     int
	AssignedByStatus map[string]int
	Overdue          int
	ProjectCount     int
	ActiveSprints    []SprintRef
	RecentIssues     []IssueRef
}

// Reader lee el resumen del usuario. Solo ve datos del propio usuario y de sus proyectos.
type Reader interface {
	Summary(ctx context.Context, userID uuid.UUID, now time.Time) (Summary, error)
}

// Service es el caso de uso del dashboard personal.
type Service interface {
	Me(ctx context.Context, userID uuid.UUID) (Summary, error)
}
