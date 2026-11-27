package domain

import (
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
)

// EventKind es el tipo de un evento del historial de alcance de un sprint.
type EventKind string

const (
	EventAdded         EventKind = "added"
	EventRemoved       EventKind = "removed"
	EventCompleted     EventKind = "completed"
	EventIncomplete    EventKind = "incomplete"
	EventMovedOut      EventKind = "moved_out"
	EventPointsChanged EventKind = "points_changed"
)

// Categorias de estado de una incidencia (las del workflow).
const (
	CategoryTodo       = "todo"
	CategoryInProgress = "in_progress"
	CategoryDone       = "done"
)

// Event es una fila del historial de alcance: que paso con una incidencia en el sprint y con cuantos puntos.
// Key y Summary se rellenan al leer, para el informe.
type Event struct {
	ID         int64
	SprintID   uuid.UUID
	IssueID    uuid.UUID
	Kind       EventKind
	Points     *float64
	Category   string
	HappenedAt time.Time
	ActorID    *uuid.UUID
	AfterStart bool
	Key        string
	Summary    string
}

// Member es una incidencia viva del sprint con sus puntos y su categoria de estado actuales.
type Member struct {
	IssueID  uuid.UUID
	Points   *float64
	Category string
}

// Snapshot genera los eventos "added" del alcance comprometido al iniciar el sprint.
func Snapshot(sprintID uuid.UUID, actor *uuid.UUID, at time.Time, members []Member) ([]Event, float64) {
	events := make([]Event, 0, len(members))
	committed := 0.0
	for _, m := range members {
		committed += pointsOf(m.Points)
		events = append(events, Event{
			SprintID: sprintID, IssueID: m.IssueID, Kind: EventAdded, Points: m.Points,
			Category: m.Category, HappenedAt: at, ActorID: actor, AfterStart: false,
		})
	}
	return events, committed
}

// Completion es el resultado de completar un sprint: eventos, puntos completados e incidencias a mover.
type Completion struct {
	Events           []Event
	CompletedPoints  float64
	IncompletePoints float64
	Completed        int
	Incomplete       []uuid.UUID
}

// PlanCompletion clasifica las incidencias del sprint por su categoria de estado: las "done" se completan
// y el resto queda incompleto y debe moverse al destino.
func PlanCompletion(sprintID uuid.UUID, actor *uuid.UUID, at time.Time, members []Member) Completion {
	var c Completion
	for _, m := range members {
		kind := EventIncomplete
		if m.Category == CategoryDone {
			kind = EventCompleted
			c.CompletedPoints += pointsOf(m.Points)
			c.Completed++
		} else {
			c.IncompletePoints += pointsOf(m.Points)
			c.Incomplete = append(c.Incomplete, m.IssueID)
		}
		c.Events = append(c.Events, Event{
			SprintID: sprintID, IssueID: m.IssueID, Kind: kind, Points: m.Points,
			Category: m.Category, HappenedAt: at, ActorID: actor, AfterStart: true,
		})
	}
	return c
}

// MoveEvents genera los eventos de alcance al mover una incidencia entre contenedores. Solo los sprints
// activos tienen historial de cambios: los planificados aun no tienen alcance comprometido. fromActive y
// toActive son el sprint de origen y de destino cuando estan activos (nil en otro caso); leaving es el
// evento de salida (removed si va al backlog, moved_out si va a otro sprint).
func MoveEvents(issueID uuid.UUID, points *float64, category string, actor *uuid.UUID, at time.Time,
	fromActive, toActive *uuid.UUID, leaving EventKind) []Event {
	var events []Event
	if fromActive != nil {
		events = append(events, Event{SprintID: *fromActive, IssueID: issueID, Kind: leaving, Points: points,
			Category: category, HappenedAt: at, ActorID: actor, AfterStart: true})
	}
	if toActive != nil {
		events = append(events, Event{SprintID: *toActive, IssueID: issueID, Kind: EventAdded, Points: points,
			Category: category, HappenedAt: at, ActorID: actor, AfterStart: true})
	}
	return events
}

// DestinationKind es el destino de las incidencias no terminadas al completar un sprint.
type DestinationKind string

const (
	DestinationBacklog DestinationKind = "backlog"
	DestinationSprint  DestinationKind = "sprint"
	DestinationNew     DestinationKind = "new"
)

// Destination es el destino ya interpretado: backlog, un sprint planificado o un sprint nuevo.
type Destination struct {
	Kind     DestinationKind
	SprintID uuid.UUID
}

// ParseDestination interpreta "backlog", "new" o "sprint:<uuid>". Vacio equivale a backlog.
func ParseDestination(raw string) (Destination, error) {
	raw = strings.TrimSpace(raw)
	switch {
	case raw == "" || raw == string(DestinationBacklog):
		return Destination{Kind: DestinationBacklog}, nil
	case raw == string(DestinationNew):
		return Destination{Kind: DestinationNew}, nil
	case strings.HasPrefix(raw, "sprint:"):
		id, err := uuid.Parse(strings.TrimPrefix(raw, "sprint:"))
		if err != nil || id == uuid.Nil {
			return Destination{}, ErrInvalidCloseOption
		}
		return Destination{Kind: DestinationSprint, SprintID: id}, nil
	}
	return Destination{}, ErrInvalidCloseOption
}

// NextSprintName propone el nombre del sprint siguiente: incrementa el numero final ("Sprint 12" pasa a
// "Sprint 13") o añade " 2" si el nombre no termina en numero.
func NextSprintName(name string) string {
	name = strings.TrimSpace(name)
	end := len(name)
	start := end
	for start > 0 && name[start-1] >= '0' && name[start-1] <= '9' {
		start--
	}
	next := name + " 2"
	if start < end {
		if n, err := strconv.Atoi(name[start:end]); err == nil {
			next = name[:start] + strconv.Itoa(n+1)
		}
	}
	if runes := []rune(next); len(runes) > 80 {
		next = string(runes[:80])
	}
	return next
}

func pointsOf(p *float64) float64 {
	if p == nil {
		return 0
	}
	return *p
}
