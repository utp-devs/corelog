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
	ActionMoveCards     = "move_cards"
)

// Authorizer decide si un actor puede ejecutar una accion sobre un proyecto. Lo implementa el root.
type Authorizer interface {
	Authorize(ctx context.Context, action string, actorID uuid.UUID, globalRole string, projectID uuid.UUID) error
}

// IssueState es el estado de una incidencia viva del proyecto, bloqueada para la transaccion en curso.
type IssueState struct {
	ID       uuid.UUID
	SprintID *uuid.UUID
	Points   *float64
	Category string
}

// Repository persiste sprints, su historial de alcance y la asignacion de incidencias (issues.sprint_id).
// Todos los metodos participan en la transaccion del contexto si la hay.
type Repository interface {
	Save(ctx context.Context, sprint *domain.Sprint) error
	FindByID(ctx context.Context, id uuid.UUID) (*domain.Sprint, error)
	// FindByIDForUpdate bloquea la fila del sprint hasta el final de la transaccion.
	FindByIDForUpdate(ctx context.Context, id uuid.UUID) (*domain.Sprint, error)
	ListByProject(ctx context.Context, projectID uuid.UUID) ([]*domain.Sprint, error)
	Update(ctx context.Context, sprint *domain.Sprint) error
	Delete(ctx context.Context, id uuid.UUID) error
	HasActive(ctx context.Context, projectID, exceptID uuid.UUID) (bool, error)

	// SetIssueSprint asigna la incidencia del proyecto al sprint, o la quita si es nil.
	SetIssueSprint(ctx context.Context, projectID, issueID uuid.UUID, sprintID *uuid.UUID) error
	RemoveIssueFromSprint(ctx context.Context, projectID, sprintID, issueID uuid.UUID) error
	// IssueProjects devuelve el proyecto de cada incidencia viva indicada.
	IssueProjects(ctx context.Context, ids []uuid.UUID) (map[uuid.UUID]uuid.UUID, error)
	// LockIssues carga y bloquea las incidencias vivas del proyecto indicadas.
	LockIssues(ctx context.Context, projectID uuid.UUID, ids []uuid.UUID) ([]IssueState, error)
	// LockMembers carga y bloquea las incidencias vivas del sprint.
	LockMembers(ctx context.Context, sprintID uuid.UUID) ([]domain.Member, error)
	// PlaceIssues fija sprint y rank de cada incidencia.
	PlaceIssues(ctx context.Context, projectID uuid.UUID, sprintID *uuid.UUID, ranks map[uuid.UUID]string) error
	// MoveIssues cambia el sprint de las incidencias sin tocar su rank.
	MoveIssues(ctx context.Context, ids []uuid.UUID, sprintID *uuid.UUID) error
	// SectionRanks lista, por rank, las incidencias de una seccion (sprint o backlog) del proyecto, sin las excluidas.
	SectionRanks(ctx context.Context, projectID uuid.UUID, sprintID *uuid.UUID, exclude []uuid.UUID) ([]domain.RankedID, error)

	AddEvents(ctx context.Context, events []domain.Event) error
	Events(ctx context.Context, sprintID uuid.UUID) ([]domain.Event, error)
	// LiveCategories da la categoria actual de las incidencias vivas (no borradas) indicadas.
	LiveCategories(ctx context.Context, issueIDs []uuid.UUID) (map[uuid.UUID]string, error)
	DoneTimelines(ctx context.Context, issueIDs []uuid.UUID) (map[uuid.UUID]domain.DoneTimeline, error)

	ListIssues(ctx context.Context, sprintID uuid.UUID) ([]IssueRef, error)
	Velocity(ctx context.Context, projectID uuid.UUID, limit int) ([]VelocityPoint, error)
	Backlog(ctx context.Context, projectID uuid.UUID, filter BacklogFilter) (*BacklogData, error)
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

// BacklogFilter son los filtros de la lectura del backlog. Version se acepta pero aun no filtra.
type BacklogFilter struct {
	EpicID  *uuid.UUID
	Version string
	Query   string
}

// BacklogLabel es una etiqueta de una incidencia del backlog.
type BacklogLabel struct {
	ID    uuid.UUID
	Name  string
	Color string
}

// BacklogIssue es una fila del backlog.
type BacklogIssue struct {
	ID                      uuid.UUID
	Key                     string
	Summary                 string
	IssueType               string
	IssueTypeName           string
	IssueTypeIcon           string
	IssueTypeColor          string
	Status                  string
	StatusName              string
	StatusCategory          string
	Priority                string
	PriorityName            string
	PriorityIcon            string
	PriorityColor           string
	AssigneeID              *uuid.UUID
	AssigneeName            string
	StoryPoints             *float64
	OriginalEstimateSeconds *int64
	Rank                    string
	EpicID                  *uuid.UUID
	EpicKey                 string
	EpicSummary             string
	Flagged                 bool
	SprintID                *uuid.UUID
	Labels                  []BacklogLabel
}

// BacklogEpic resume una epica con el progreso de sus incidencias hijas.
type BacklogEpic struct {
	ID         uuid.UUID
	Key        string
	Summary    string
	Status     string
	Total      int
	Done       int
	Points     float64
	DonePoints float64
}

// BacklogData es lo que lee el repositorio: sprints abiertos, incidencias ordenadas por rank y epicas.
type BacklogData struct {
	Sprints   []*domain.Sprint
	Issues    []BacklogIssue
	Epics     []BacklogEpic
	Truncated bool
}
