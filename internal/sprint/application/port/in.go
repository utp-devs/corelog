package port

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/jhonsferg/corelog/internal/sprint/domain"
)

// Opciones heredadas del cierre (campo unfinished) que siguen aceptandose.
const (
	UnfinishedToBacklog = "backlog"
	UnfinishedToSprint  = "sprint"
)

// CompleteInput indica donde van las incidencias no terminadas: Destination es "backlog", "new" o
// "sprint:<id>". Los campos Unfinished y TargetSprintID son la forma antigua y se traducen a Destination.
type CompleteInput struct {
	Destination    string
	Unfinished     string
	TargetSprintID *uuid.UUID
	NewSprintName  string
}

// CloseInput se conserva como alias de la entrada de completado.
type CloseInput = CompleteInput

// StartInput son los datos que se confirman al iniciar. Force permite iniciar un sprint sin incidencias.
type StartInput struct {
	Name      *string
	Goal      *string
	StartDate *time.Time
	EndDate   *time.Time
	Force     bool
}

// BurndownPoint es un dia del burndown: trabajo restante real frente a la linea ideal.
type BurndownPoint struct {
	Date      time.Time
	Ideal     float64
	Remaining float64
}

// CompleteResult es el sprint cerrado y lo ocurrido con sus incidencias.
type CompleteResult struct {
	Sprint          *domain.Sprint
	NextSprint      *domain.Sprint
	CompletedIssues int
	MovedIssues     int
	CompletedPoints float64
	CommittedPoints float64
}

// SprintReport es el informe de un sprint mas la velocidad reciente del proyecto.
type SprintReport struct {
	*domain.Report
	Velocity []VelocityPoint
}

// RankInput mueve un conjunto de incidencias a una seccion. TargetSprintID nil significa backlog.
type RankInput struct {
	ProjectID      *uuid.UUID
	IssueIDs       []uuid.UUID
	TargetSprintID *uuid.UUID
	BeforeIssueID  *uuid.UUID
	AfterIssueID   *uuid.UUID
}

// BacklogSection es un contenedor del backlog: sprint activo, sprint planificado o el backlog sin sprint.
type BacklogSection struct {
	Kind   string // active, planned, backlog
	Sprint *domain.Sprint
	Issues []BacklogIssue
	Totals SectionTotals
}

// CategoryTotals acumula incidencias, puntos y estimacion de una categoria de estado.
type CategoryTotals struct {
	Issues          int
	Points          float64
	EstimateSeconds int64
}

// SectionTotals son los totales de una seccion, con desglose por categoria de estado.
type SectionTotals struct {
	CategoryTotals
	ByCategory map[string]CategoryTotals
}

// Backlog es la respuesta de la lectura del backlog de un proyecto.
type Backlog struct {
	ProjectID       uuid.UUID
	Sections        []BacklogSection
	Epics           []BacklogEpic
	CanManageSprint bool
	CanMoveCards    bool
	Truncated       bool
}

// Service es el caso de uso de sprints y backlog. Los permisos se deciden aqui con el autorizador.
type Service interface {
	Create(ctx context.Context, actorID uuid.UUID, globalRole string, projectID uuid.UUID, name, goal string, start, end *time.Time) (*domain.Sprint, error)
	List(ctx context.Context, actorID uuid.UUID, globalRole string, projectID uuid.UUID) ([]*domain.Sprint, error)
	Get(ctx context.Context, actorID uuid.UUID, globalRole string, projectID, sprintID uuid.UUID) (*domain.Sprint, error)
	Update(ctx context.Context, actorID uuid.UUID, globalRole string, projectID, sprintID uuid.UUID, patch domain.Patch) (*domain.Sprint, error)
	Delete(ctx context.Context, actorID uuid.UUID, globalRole string, projectID, sprintID uuid.UUID) error
	Start(ctx context.Context, actorID uuid.UUID, globalRole string, projectID, sprintID uuid.UUID, input StartInput) (*domain.Sprint, error)
	Complete(ctx context.Context, actorID uuid.UUID, globalRole string, projectID, sprintID uuid.UUID, input CompleteInput) (*CompleteResult, error)
	Report(ctx context.Context, actorID uuid.UUID, globalRole string, projectID, sprintID uuid.UUID) (*SprintReport, error)
	// ResolveSprintProject devuelve el proyecto de un sprint, para las rutas que no traen project_id.
	ResolveSprintProject(ctx context.Context, sprintID uuid.UUID) (uuid.UUID, error)
	ListIssues(ctx context.Context, actorID uuid.UUID, globalRole string, projectID, sprintID uuid.UUID) ([]IssueRef, error)
	AssignIssue(ctx context.Context, actorID uuid.UUID, globalRole string, projectID, sprintID, issueID uuid.UUID) error
	RemoveIssue(ctx context.Context, actorID uuid.UUID, globalRole string, projectID, sprintID, issueID uuid.UUID) error
	Burndown(ctx context.Context, actorID uuid.UUID, globalRole string, projectID, sprintID uuid.UUID) ([]BurndownPoint, error)
	Velocity(ctx context.Context, actorID uuid.UUID, globalRole string, projectID uuid.UUID, limit int) ([]VelocityPoint, error)

	Backlog(ctx context.Context, actorID uuid.UUID, globalRole string, projectID uuid.UUID, filter BacklogFilter) (*Backlog, error)
	// Rank mueve incidencias a una seccion y devuelve el proyecto afectado.
	Rank(ctx context.Context, actorID uuid.UUID, globalRole string, input RankInput) (uuid.UUID, error)
}
