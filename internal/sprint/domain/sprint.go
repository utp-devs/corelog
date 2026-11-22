package domain

import (
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"

	"github.com/jhonsferg/corelog/internal/platform/apperror"
)

// Status es el estado de un sprint: planificado, activo o cerrado.
type Status string

const (
	StatusPlanned Status = "planned"
	StatusActive  Status = "active"
	StatusClosed  Status = "closed"
)

var (
	ErrInvalidSprint       = apperror.New(apperror.KindValidation, "invalid sprint")
	ErrInvalidSprintDates  = apperror.New(apperror.KindValidation, "sprint end date must not be before its start date")
	ErrSprintWithoutDates  = apperror.New(apperror.KindValidation, "the sprint needs start and end dates")
	ErrSprintNotFound      = apperror.New(apperror.KindNotFound, "sprint not found")
	ErrIssueNotInProject   = apperror.New(apperror.KindNotFound, "issue not found in this project")
	ErrSprintNotPlanned    = apperror.New(apperror.KindConflict, "only a planned sprint can be started")
	ErrSprintNotActive     = apperror.New(apperror.KindConflict, "only an active sprint can be closed")
	ErrActiveSprintExists  = apperror.New(apperror.KindConflict, "another sprint is already active in this project")
	ErrSprintClosed        = apperror.New(apperror.KindConflict, "a closed sprint cannot change its issues")
	ErrInvalidCloseOption  = apperror.New(apperror.KindValidation, "invalid close option")
	ErrInvalidTargetSprint = apperror.New(apperror.KindValidation, "target sprint must be planned in the same project")
)

// Sprint es una iteracion de trabajo de un proyecto con fechas opcionales.
type Sprint struct {
	ID        uuid.UUID
	ProjectID uuid.UUID
	Name      string
	Goal      string
	Status    Status
	StartDate *time.Time
	EndDate   *time.Time
	ClosedAt  *time.Time
	CreatedAt time.Time
	UpdatedAt time.Time
}

// NewSprint valida los datos de un sprint nuevo. Nace planificado; el id lo asigna la base de datos.
func NewSprint(projectID uuid.UUID, name, goal string, start, end *time.Time) (*Sprint, error) {
	name = strings.TrimSpace(name)
	goal = strings.TrimSpace(goal)
	if projectID == uuid.Nil || name == "" || utf8.RuneCountInString(name) > 80 || utf8.RuneCountInString(goal) > 500 {
		return nil, ErrInvalidSprint
	}
	if start != nil && end != nil && end.Before(*start) {
		return nil, ErrInvalidSprintDates
	}
	now := time.Now().UTC()
	return &Sprint{
		ProjectID: projectID, Name: name, Goal: goal, Status: StatusPlanned,
		StartDate: start, EndDate: end, CreatedAt: now, UpdatedAt: now,
	}, nil
}

// Start activa un sprint planificado.
func (s *Sprint) Start() error {
	if s.Status != StatusPlanned {
		return ErrSprintNotPlanned
	}
	s.Status = StatusActive
	s.UpdatedAt = time.Now().UTC()
	return nil
}

// Close cierra un sprint activo y registra la hora de cierre.
func (s *Sprint) Close() error {
	if s.Status != StatusActive {
		return ErrSprintNotActive
	}
	now := time.Now().UTC()
	s.Status = StatusClosed
	s.ClosedAt = &now
	s.UpdatedAt = now
	return nil
}
