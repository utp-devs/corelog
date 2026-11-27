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

// defaultSprintDays es la duracion que se asume al iniciar un sprint sin fecha de fin.
const defaultSprintDays = 14

var (
	ErrInvalidSprint       = apperror.WithCode(apperror.KindValidation, "sprint.invalid", "El sprint no es valido: el nombre es obligatorio (80 caracteres como maximo) y el objetivo admite hasta 500")
	ErrInvalidSprintDates  = apperror.WithCode(apperror.KindValidation, "sprint.invalid_dates", "La fecha de fin del sprint no puede ser anterior a la de inicio")
	ErrSprintWithoutDates  = apperror.WithCode(apperror.KindValidation, "sprint.dates_required", "El sprint necesita fecha de inicio y de fin")
	ErrSprintEmpty         = apperror.WithCode(apperror.KindValidation, "sprint.empty", "El sprint no tiene incidencias; añade alguna o inicialo forzando")
	ErrSprintNotFound      = apperror.WithCode(apperror.KindNotFound, "sprint.not_found", "Sprint no encontrado")
	ErrIssueNotInProject   = apperror.WithCode(apperror.KindNotFound, "sprint.issue_not_in_project", "Incidencia no encontrada en este proyecto")
	ErrSprintNotPlanned    = apperror.WithCode(apperror.KindConflict, "sprint.not_started", "Solo se puede iniciar un sprint planificado")
	ErrSprintNotDeletable  = apperror.WithCode(apperror.KindConflict, "sprint.not_planned", "Solo se puede borrar un sprint planificado")
	ErrSprintNotActive     = apperror.WithCode(apperror.KindConflict, "sprint.not_active", "Solo se puede completar un sprint activo")
	ErrActiveSprintExists  = apperror.WithCode(apperror.KindConflict, "sprint.already_active", "Ya hay otro sprint activo en este proyecto")
	ErrSprintClosed        = apperror.WithCode(apperror.KindConflict, "sprint.closed", "Un sprint cerrado ya no admite cambios")
	ErrInvalidCloseOption  = apperror.WithCode(apperror.KindValidation, "sprint.invalid_close_option", "Opcion de cierre no valida: usa backlog, new o sprint:<id>")
	ErrInvalidTargetSprint = apperror.WithCode(apperror.KindValidation, "sprint.invalid_target", "El sprint de destino debe estar planificado y ser del mismo proyecto")
)

// Sprint es una iteracion de trabajo de un proyecto. Nace planificado, se inicia (congelando el alcance
// comprometido) y se completa una sola vez.
type Sprint struct {
	ID              uuid.UUID
	ProjectID       uuid.UUID
	BoardID         *uuid.UUID
	Name            string
	Goal            string
	Status          Status
	StartDate       *time.Time
	EndDate         *time.Time
	StartedAt       *time.Time
	CompletedAt     *time.Time
	ClosedAt        *time.Time
	CommittedPoints *float64
	CompletedPoints *float64
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

// NewSprint valida los datos de un sprint nuevo. Nace planificado; el id lo asigna la base de datos.
func NewSprint(now time.Time, projectID uuid.UUID, name, goal string, start, end *time.Time) (*Sprint, error) {
	name, goal, err := validateText(name, goal)
	if err != nil {
		return nil, err
	}
	if projectID == uuid.Nil {
		return nil, ErrInvalidSprint
	}
	if err := validateDates(start, end); err != nil {
		return nil, err
	}
	return &Sprint{
		ProjectID: projectID, Name: name, Goal: goal, Status: StatusPlanned,
		StartDate: start, EndDate: end, CreatedAt: now, UpdatedAt: now,
	}, nil
}

// Patch son los cambios opcionales de un sprint. Un puntero a cero (time.Time{}) en las fechas las borra.
type Patch struct {
	Name      *string
	Goal      *string
	StartDate *time.Time
	EndDate   *time.Time
}

// Edit aplica los cambios. Un sprint cerrado no se edita; uno activo no puede perder sus fechas.
func (s *Sprint) Edit(now time.Time, p Patch) error {
	if s.Status == StatusClosed {
		return ErrSprintClosed
	}
	name, goal := s.Name, s.Goal
	if p.Name != nil {
		name = *p.Name
	}
	if p.Goal != nil {
		goal = *p.Goal
	}
	name, goal, err := validateText(name, goal)
	if err != nil {
		return err
	}
	start, end := s.StartDate, s.EndDate
	if p.StartDate != nil {
		start = zeroToNil(p.StartDate)
	}
	if p.EndDate != nil {
		end = zeroToNil(p.EndDate)
	}
	if err := validateDates(start, end); err != nil {
		return err
	}
	if s.Status == StatusActive && (start == nil || end == nil) {
		return ErrSprintWithoutDates
	}
	s.Name, s.Goal, s.StartDate, s.EndDate, s.UpdatedAt = name, goal, start, end, now
	return nil
}

// StartParams son los datos que se confirman al iniciar el sprint; los nil conservan lo planificado.
type StartParams struct {
	Name      *string
	Goal      *string
	StartDate *time.Time
	EndDate   *time.Time
}

// Start activa un sprint planificado con sus datos definitivos. Sin fechas, empieza hoy y dura dos semanas.
func (s *Sprint) Start(now time.Time, p StartParams) error {
	if s.Status != StatusPlanned {
		return ErrSprintNotPlanned
	}
	patch := Patch{Name: p.Name, Goal: p.Goal, StartDate: p.StartDate, EndDate: p.EndDate}
	// Edit rejects closed sprints only, so it works on the planned sprint before the status changes.
	if err := s.Edit(now, patch); err != nil {
		return err
	}
	if s.StartDate == nil {
		today := dateOf(now)
		s.StartDate = &today
	}
	if s.EndDate == nil {
		end := s.StartDate.AddDate(0, 0, defaultSprintDays)
		s.EndDate = &end
	}
	if err := validateDates(s.StartDate, s.EndDate); err != nil {
		return err
	}
	s.Status = StatusActive
	s.StartedAt = &now
	return nil
}

// Complete cierra un sprint activo y fija sus puntos completados.
func (s *Sprint) Complete(now time.Time, completedPoints float64) error {
	if s.Status != StatusActive {
		return ErrSprintNotActive
	}
	s.Status = StatusClosed
	s.CompletedAt = &now
	s.ClosedAt = &now
	s.CompletedPoints = &completedPoints
	s.UpdatedAt = now
	return nil
}

// CheckDeletable indica si el sprint puede borrarse: solo los planificados.
func (s *Sprint) CheckDeletable() error {
	if s.Status != StatusPlanned {
		return ErrSprintNotDeletable
	}
	return nil
}

func validateText(name, goal string) (string, string, error) {
	name = strings.TrimSpace(name)
	goal = strings.TrimSpace(goal)
	if name == "" || utf8.RuneCountInString(name) > 80 || utf8.RuneCountInString(goal) > 500 {
		return "", "", ErrInvalidSprint
	}
	return name, goal, nil
}

func validateDates(start, end *time.Time) error {
	if start != nil && end != nil && end.Before(*start) {
		return ErrInvalidSprintDates
	}
	return nil
}

func zeroToNil(t *time.Time) *time.Time {
	if t == nil || t.IsZero() {
		return nil
	}
	return t
}

// dateOf trunca un instante a su fecha UTC.
func dateOf(t time.Time) time.Time {
	t = t.UTC()
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
}
