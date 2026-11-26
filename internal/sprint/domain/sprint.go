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
	ErrInvalidSprint       = apperror.WithCode(apperror.KindValidation, "sprint.invalid", "invalid sprint")
	ErrInvalidSprintDates  = apperror.WithCode(apperror.KindValidation, "sprint.invalid_dates", "sprint end date must not be before its start date")
	ErrSprintWithoutDates  = apperror.WithCode(apperror.KindValidation, "sprint.dates_required", "the sprint needs start and end dates")
	ErrSprintNotFound      = apperror.WithCode(apperror.KindNotFound, "sprint.not_found", "sprint not found")
	ErrIssueNotInProject   = apperror.WithCode(apperror.KindNotFound, "sprint.issue_not_in_project", "issue not found in this project")
	ErrSprintNotPlanned    = apperror.WithCode(apperror.KindConflict, "sprint.not_started", "only a planned sprint can be started")
	ErrSprintNotActive     = apperror.WithCode(apperror.KindConflict, "sprint.not_active", "only an active sprint can be closed")
	ErrActiveSprintExists  = apperror.WithCode(apperror.KindConflict, "sprint.already_active", "another sprint is already active in this project")
	ErrSprintClosed        = apperror.WithCode(apperror.KindConflict, "sprint.closed", "a closed sprint cannot change its issues")
	ErrInvalidCloseOption  = apperror.WithCode(apperror.KindValidation, "sprint.invalid_close_option", "invalid close option")
	ErrInvalidTargetSprint = apperror.WithCode(apperror.KindValidation, "sprint.invalid_target", "target sprint must be planned in the same project")
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
