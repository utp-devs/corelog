package domain

import (
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"

	"github.com/jhonsferg/corelog/internal/platform/apperror"
)

// Stable codes of the event errors.
const (
	CodeInvalidEvent      = "calendar.invalid_event"
	CodeEventNotFound     = "calendar.event_not_found"
	CodeNotAttendee       = "calendar.not_attendee"
	CodeInvalidOccurrence = "calendar.invalid_occurrence"
	CodeFeedNotFound      = "calendar.feed_not_found"
)

// Field limits.
const (
	MaxTitle       = 200
	MaxDescription = 8000
	MaxLocation    = 300
	MaxAttendees   = 100
	MaxExceptions  = 500
)

// Visibility says who can see an event besides its owner and attendees.
type Visibility string

const (
	VisibilityPrivate      Visibility = "private"
	VisibilityProject      Visibility = "project"
	VisibilityOrganization Visibility = "organization"
)

// Response is the answer of an attendee.
type Response string

const (
	ResponseNeedsAction Response = "needs_action"
	ResponseAccepted    Response = "accepted"
	ResponseDeclined    Response = "declined"
	ResponseTentative   Response = "tentative"
)

// ValidResponse reports whether r is one of the four responses.
func ValidResponse(r Response) bool {
	switch r {
	case ResponseNeedsAction, ResponseAccepted, ResponseDeclined, ResponseTentative:
		return true
	}
	return false
}

var palette = map[string]bool{
	"gray": true, "red": true, "orange": true, "amber": true, "green": true, "teal": true,
	"blue": true, "indigo": true, "purple": true, "pink": true,
}

// Attendee is a guest of an event.
type Attendee struct {
	UserID   uuid.UUID
	Name     string
	Response Response
}

// Event is a calendar event owned by a user. All-day events keep UTC midnights and an exclusive end.
type Event struct {
	ID             uuid.UUID
	OwnerID        uuid.UUID
	OrganizationID uuid.UUID
	ProjectID      *uuid.UUID
	Title          string
	Description    string
	Location       string
	StartsAt       time.Time
	EndsAt         time.Time
	AllDay         bool
	Timezone       string
	RecurrenceRule *string
	Exceptions     []Exception
	Visibility     Visibility
	Color          string
	CreatedAt      time.Time
	UpdatedAt      time.Time
	Attendees      []Attendee
}

func invalidEvent(field, message string) *apperror.Error {
	return apperror.WithCode(apperror.KindValidation, CodeInvalidEvent, "evento no válido: "+message).
		WithDetails(apperror.FieldError{Field: field, Message: message})
}

// Normalize trims text fields, fills defaults and validates the event.
func (e *Event) Normalize() error {
	e.Title = strings.TrimSpace(e.Title)
	e.Location = strings.TrimSpace(e.Location)
	if e.Title == "" {
		return invalidEvent("title", "el título es obligatorio")
	}
	if utf8.RuneCountInString(e.Title) > MaxTitle {
		return invalidEvent("title", "el título admite como máximo 200 caracteres")
	}
	if utf8.RuneCountInString(e.Description) > MaxDescription {
		return invalidEvent("description", "la descripción admite como máximo 8000 caracteres")
	}
	if utf8.RuneCountInString(e.Location) > MaxLocation {
		return invalidEvent("location", "el lugar admite como máximo 300 caracteres")
	}
	if e.Timezone == "" {
		e.Timezone = "UTC"
	}
	if _, err := time.LoadLocation(e.Timezone); err != nil || len(e.Timezone) > 64 {
		return invalidEvent("timezone", "la zona horaria no es válida")
	}
	if e.Color == "" {
		e.Color = "blue"
	}
	if !palette[e.Color] {
		return invalidEvent("color", "el color no es válido")
	}
	switch e.Visibility {
	case "":
		e.Visibility = VisibilityPrivate
	case VisibilityPrivate, VisibilityProject, VisibilityOrganization:
	default:
		return invalidEvent("visibility", "la visibilidad debe ser private, project u organization")
	}
	if e.Visibility == VisibilityProject && e.ProjectID == nil {
		return invalidEvent("project_id", "un evento de proyecto necesita project_id")
	}
	e.StartsAt, e.EndsAt = e.StartsAt.UTC(), e.EndsAt.UTC()
	if e.AllDay {
		if !isMidnight(e.StartsAt) || !isMidnight(e.EndsAt) {
			return invalidEvent("starts_at", "los eventos de día completo usan fechas (medianoche UTC)")
		}
		if !e.EndsAt.After(e.StartsAt) {
			return invalidEvent("ends_at", "el fin debe ser posterior al inicio")
		}
	} else if e.EndsAt.Before(e.StartsAt) {
		return invalidEvent("ends_at", "el fin no puede ser anterior al inicio")
	}
	if e.RecurrenceRule != nil {
		rule, err := ParseRule(*e.RecurrenceRule)
		if err != nil {
			return err
		}
		canonical := rule.String()
		e.RecurrenceRule = &canonical
	}
	if len(e.Exceptions) > MaxExceptions {
		return invalidEvent("recurrence_exceptions", "demasiadas excepciones")
	}
	if len(e.Attendees) > MaxAttendees {
		return invalidEvent("attendees", "un evento admite como máximo 100 invitados")
	}
	return nil
}

func isMidnight(t time.Time) bool {
	return t.Hour() == 0 && t.Minute() == 0 && t.Second() == 0 && t.Nanosecond() == 0
}

// Location returns the zone the wall clock of the event is kept in: UTC for all-day events.
func (e *Event) location() *time.Location {
	if e.AllDay {
		return time.UTC
	}
	loc, err := time.LoadLocation(e.Timezone)
	if err != nil {
		return time.UTC
	}
	return loc
}

// Series builds the expansion input of the event.
func (e *Event) Series() Series {
	s := Series{Start: e.StartsAt, End: e.EndsAt, Location: e.location(), Exceptions: e.Exceptions}
	if e.RecurrenceRule != nil {
		if rule, err := ParseRule(*e.RecurrenceRule); err == nil {
			s.Rule = &rule
		}
	}
	return s
}

// Recurring reports whether the event has a recurrence rule.
func (e *Event) Recurring() bool { return e.RecurrenceRule != nil }

// FindException returns the exception of the occurrence that starts at original.
func (e *Event) FindException(original time.Time) (int, bool) {
	for i, x := range e.Exceptions {
		if x.OriginalStart.Equal(original) {
			return i, true
		}
	}
	return -1, false
}

// SetException adds or replaces the exception of one occurrence.
func (e *Event) SetException(x Exception) error {
	if i, ok := e.FindException(x.OriginalStart); ok {
		e.Exceptions[i] = x
		return nil
	}
	if len(e.Exceptions) >= MaxExceptions {
		return invalidEvent("recurrence_exceptions", "demasiadas excepciones")
	}
	e.Exceptions = append(e.Exceptions, x)
	return nil
}

// IsOccurrence reports whether original is one of the instants the series generates (so an exception cannot
// point at a date that is not part of the series).
func (e *Event) IsOccurrence(original time.Time) bool {
	s := e.Series()
	s.Exceptions = nil
	dur := e.EndsAt.Sub(e.StartsAt)
	if s.Rule == nil {
		return original.Equal(e.StartsAt)
	}
	from := original.Add(-time.Second)
	occ, _ := Series{Start: s.Start, End: s.Start.Add(dur), Location: s.Location, Rule: s.Rule}.Expand(from, original.Add(time.Second+dur))
	for _, o := range occ {
		if o.OriginalStart.Equal(original) {
			return true
		}
	}
	return false
}

// ErrInvalidOccurrence is returned when an occurrence is not part of the series.
var ErrInvalidOccurrence = apperror.WithCode(apperror.KindValidation, CodeInvalidOccurrence, "la ocurrencia no pertenece a la serie del evento")

// ErrEventNotFound hides events the caller cannot see.
var ErrEventNotFound = apperror.WithCode(apperror.KindNotFound, CodeEventNotFound, "evento no encontrado")

// ErrNotAttendee is returned when someone who is not a guest answers an invitation.
var ErrNotAttendee = apperror.WithCode(apperror.KindForbidden, CodeNotAttendee, "no estás invitado a este evento")

// ErrFeedNotFound is returned for an unknown or revoked feed token.
var ErrFeedNotFound = apperror.WithCode(apperror.KindNotFound, CodeFeedNotFound, "calendario no encontrado")
