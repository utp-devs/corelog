// Package domain holds the versions (releases) of a project: their lifecycle, validation and the release notes
// built from the issues that fix them.
package domain

import (
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/jhonsferg/corelog/internal/platform/apperror"
)

// DateLayout is the wire format of the date-only fields of a version.
const DateLayout = "2006-01-02"

// Length limits of the version fields (characters, not bytes).
const (
	MaxVersionNameLength        = 80
	MaxVersionDescriptionLength = 4000
)

// Status is the lifecycle state of a version.
type Status string

const (
	// StatusUnreleased is the initial state: the version collects fix issues.
	StatusUnreleased Status = "unreleased"
	// StatusReleased marks a published version. It takes no new fix issues unless an administrator overrides it.
	StatusReleased Status = "released"
	// StatusArchived is terminal: the version is kept for history and rejects edits.
	StatusArchived Status = "archived"
)

// Valid reports whether s is a known status.
func (s Status) Valid() bool {
	return s == StatusUnreleased || s == StatusReleased || s == StatusArchived
}

// Typed errors. The codes are part of the API contract.
var (
	ErrVersionNotFound = apperror.WithCode(apperror.KindNotFound, "version.not_found", "Versión no encontrada")
	ErrVersionInvalid  = apperror.WithCode(apperror.KindValidation, "version.invalid",
		"La versión no es válida: el nombre es obligatorio (80 caracteres como máximo) y la descripción admite hasta 4000")
	ErrVersionDates = apperror.WithCode(apperror.KindValidation, "version.invalid_dates",
		"La fecha de publicación no puede ser anterior a la de inicio")
	ErrVersionNameTaken = apperror.WithCode(apperror.KindConflict, "version.name_taken",
		"Ya existe una versión con ese nombre en el proyecto")
	ErrVersionTransition = apperror.WithCode(apperror.KindConflict, "version.invalid_transition",
		"La versión no admite esa transición de estado")
	ErrVersionArchived = apperror.WithCode(apperror.KindConflict, "version.archived",
		"La versión está archivada y no admite cambios")
	ErrVersionMoveTarget = apperror.WithCode(apperror.KindValidation, "version.move_target_invalid",
		"La versión de destino debe ser de este proyecto y seguir sin publicar")
	ErrVersionStatusInvalid = apperror.WithCode(apperror.KindValidation, "version.status_invalid",
		"El estado solicitado no es válido")
)

// Version is a named release of a project. Progress is filled by the read paths.
type Version struct {
	ID          uuid.UUID
	ProjectID   uuid.UUID
	Name        string
	Description string
	StartDate   *time.Time
	ReleaseDate *time.Time
	Status      Status
	ReleasedAt  *time.Time
	ArchivedAt  *time.Time
	CreatedAt   time.Time
	UpdatedAt   time.Time
	Progress    Progress
}

// NewVersion validates the input of a new version. Status starts as unreleased.
func NewVersion(projectID uuid.UUID, name, description string, start, release *time.Time, now time.Time) (*Version, error) {
	if projectID == uuid.Nil {
		return nil, ErrVersionInvalid
	}
	v := &Version{
		ProjectID: projectID, Status: StatusUnreleased, CreatedAt: now.UTC(), UpdatedAt: now.UTC(),
	}
	if err := v.setName(name); err != nil {
		return nil, err
	}
	if err := v.setDescription(description); err != nil {
		return nil, err
	}
	v.StartDate = dayOf(start)
	v.ReleaseDate = dayOf(release)
	if err := v.checkDates(); err != nil {
		return nil, err
	}
	return v, nil
}

// VersionPatch is a partial update. A nil pointer keeps the field. The Set flags distinguish a dated field
// from one the client clears with null.
type VersionPatch struct {
	Name           *string
	Description    *string
	StartDateSet   bool
	StartDate      *time.Time
	ReleaseDateSet bool
	ReleaseDate    *time.Time
	// Status may only ask for "unreleased" (reopening a released version). Released and archived are reached
	// through the release and archive actions.
	Status *Status
}

// Update applies the patch to a version that is not archived. Reopening a released version clears its release
// date and the release timestamp.
func (v *Version) Update(p VersionPatch, now time.Time) error {
	if v.Status == StatusArchived {
		return ErrVersionArchived
	}
	if p.Name != nil {
		if err := v.setName(*p.Name); err != nil {
			return err
		}
	}
	if p.Description != nil {
		if err := v.setDescription(*p.Description); err != nil {
			return err
		}
	}
	if p.StartDateSet {
		v.StartDate = dayOf(p.StartDate)
	}
	if p.ReleaseDateSet {
		v.ReleaseDate = dayOf(p.ReleaseDate)
	}
	if err := v.checkDates(); err != nil {
		return err
	}
	if p.Status != nil {
		if *p.Status != StatusUnreleased {
			if !p.Status.Valid() {
				return ErrVersionStatusInvalid
			}
			return ErrVersionTransition
		}
		if err := v.Unrelease(now); err != nil {
			return err
		}
	}
	v.UpdatedAt = now.UTC()
	return nil
}

// Release publishes an unreleased version. A nil releaseDate defaults to the day of now.
func (v *Version) Release(releaseDate *time.Time, now time.Time) error {
	if v.Status != StatusUnreleased {
		return transitionError(v.Status, StatusReleased)
	}
	now = now.UTC()
	v.Status = StatusReleased
	v.ReleasedAt = &now
	if releaseDate != nil {
		v.ReleaseDate = dayOf(releaseDate)
	} else {
		v.ReleaseDate = dayOf(&now)
	}
	if err := v.checkDates(); err != nil {
		return err
	}
	v.UpdatedAt = now
	return nil
}

// Archive moves an unreleased or released version to the terminal archived state.
func (v *Version) Archive(now time.Time) error {
	if v.Status == StatusArchived {
		return transitionError(v.Status, StatusArchived)
	}
	now = now.UTC()
	v.Status = StatusArchived
	v.ArchivedAt = &now
	v.UpdatedAt = now
	return nil
}

// Unrelease moves a released version back to unreleased so more fix issues can land in it.
func (v *Version) Unrelease(now time.Time) error {
	if v.Status != StatusReleased {
		return transitionError(v.Status, StatusUnreleased)
	}
	v.Status = StatusUnreleased
	v.ReleasedAt = nil
	v.ReleaseDate = nil
	v.UpdatedAt = now.UTC()
	return nil
}

// AcceptsFixIssues reports whether a fix link can be added without an administrator override.
func (v *Version) AcceptsFixIssues() bool {
	return v.Status == StatusUnreleased
}

// AcceptsAffectsIssues reports whether an affects link can be added without an override. Released versions
// still take affects links because bugs are often found after a release.
func (v *Version) AcceptsAffectsIssues() bool {
	return v.Status != StatusArchived
}

// AcceptsMoves reports whether issues can be moved into the version (move targets must stay unreleased).
func (v *Version) AcceptsMoves() bool {
	return v.Status == StatusUnreleased
}

func (v *Version) setName(raw string) error {
	name := strings.TrimSpace(raw)
	if name == "" || len([]rune(name)) > MaxVersionNameLength {
		return ErrVersionInvalid
	}
	v.Name = name
	return nil
}

func (v *Version) setDescription(raw string) error {
	description := strings.TrimSpace(raw)
	if len([]rune(description)) > MaxVersionDescriptionLength {
		return ErrVersionInvalid
	}
	v.Description = description
	return nil
}

func (v *Version) checkDates() error {
	if v.StartDate != nil && v.ReleaseDate != nil && v.ReleaseDate.Before(*v.StartDate) {
		return ErrVersionDates
	}
	return nil
}

// dayOf truncates a timestamp to the UTC day, the value of a DATE column. Nil stays nil.
func dayOf(t *time.Time) *time.Time {
	if t == nil {
		return nil
	}
	day := time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
	return &day
}

func transitionError(from, to Status) error {
	if from == StatusArchived {
		return ErrVersionArchived
	}
	return ErrVersionTransition
}

// FormatDate renders a date-only field, or "" when unset.
func FormatDate(t *time.Time) string {
	if t == nil {
		return ""
	}
	return t.Format(DateLayout)
}
