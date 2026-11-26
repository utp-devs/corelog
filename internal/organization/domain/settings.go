package domain

import (
	"math"
	"time"

	// Embedded IANA database so timezone validation does not depend on the host OS.
	_ "time/tzdata"

	"github.com/jhonsferg/corelog/internal/platform/apperror"
)

// Values accepted by the who_can_create_* settings.
const (
	CreatorMembers = "members"
	CreatorAdmins  = "admins"
)

// Values accepted by DefaultProjectVisibility.
const (
	VisibilityOpen    = "open"
	VisibilityPrivate = "private"
)

const (
	maxHoursPerDay     = 24
	maxDaysPerWeek     = 7
	maxUploadBytesCap  = 100 << 20 // 100 MiB
	defaultUploadBytes = 10 << 20  // 10 MiB
	defaultTimezone    = "UTC"
)

// Settings are the organization policies.
type Settings struct {
	WhoCanCreateProjects     string
	WhoCanCreateTeams        string
	DefaultProjectVisibility string
	Require2FA               bool
	MaxUploadBytes           int64
	HoursPerDay              float64
	DaysPerWeek              float64
	Timezone                 string
}

// DefaultSettings returns the settings of a new organization.
func DefaultSettings() Settings {
	return Settings{
		WhoCanCreateProjects:     CreatorMembers,
		WhoCanCreateTeams:        CreatorMembers,
		DefaultProjectVisibility: VisibilityOpen,
		Require2FA:               false,
		MaxUploadBytes:           defaultUploadBytes,
		HoursPerDay:              8,
		DaysPerWeek:              5,
		Timezone:                 defaultTimezone,
	}
}

// Validate checks every field and returns one ErrSettingsInvalid with a detail per invalid field.
func (s Settings) Validate() error {
	var details []apperror.FieldError
	addDetail := func(field, msg string) {
		details = append(details, apperror.FieldError{Field: field, Message: msg})
	}
	if !isCreatorValue(s.WhoCanCreateProjects) {
		addDetail("who_can_create_projects", "Debe ser 'members' o 'admins'")
	}
	if !isCreatorValue(s.WhoCanCreateTeams) {
		addDetail("who_can_create_teams", "Debe ser 'members' o 'admins'")
	}
	if s.DefaultProjectVisibility != VisibilityOpen && s.DefaultProjectVisibility != VisibilityPrivate {
		addDetail("default_project_visibility", "Debe ser 'open' o 'private'")
	}
	if s.MaxUploadBytes <= 0 || s.MaxUploadBytes > maxUploadBytesCap {
		addDetail("max_upload_bytes", "Debe estar entre 1 byte y 100 MiB")
	}
	// Written as negated comparisons so NaN is rejected too.
	if !(s.HoursPerDay > 0 && s.HoursPerDay <= maxHoursPerDay) || math.IsInf(s.HoursPerDay, 0) {
		addDetail("hours_per_day", "Debe estar entre 0 y 24")
	}
	if !(s.DaysPerWeek > 0 && s.DaysPerWeek <= maxDaysPerWeek) || math.IsInf(s.DaysPerWeek, 0) {
		addDetail("days_per_week", "Debe estar entre 0 y 7")
	}
	if !validTimezone(s.Timezone) {
		addDetail("timezone", "Debe ser una zona horaria IANA válida")
	}
	if len(details) == 0 {
		return nil
	}
	return apperror.WithCode(apperror.KindValidation, ErrSettingsInvalid.Code, ErrSettingsInvalid.Message).WithDetails(details...)
}

func isCreatorValue(v string) bool {
	return v == CreatorMembers || v == CreatorAdmins
}

// validTimezone accepts IANA names only; "Local" and empty strings are rejected.
func validTimezone(name string) bool {
	if name == "" || name == "Local" {
		return false
	}
	_, err := time.LoadLocation(name)
	return err == nil
}

// SettingsPatch is a partial change of Settings. A nil field keeps the current value.
type SettingsPatch struct {
	WhoCanCreateProjects     *string
	WhoCanCreateTeams        *string
	DefaultProjectVisibility *string
	Require2FA               *bool
	MaxUploadBytes           *int64
	HoursPerDay              *float64
	DaysPerWeek              *float64
	Timezone                 *string
}

// Apply returns current with the fields set in the patch replaced. It does not validate; callers pass
// the result to ApplySettings, which does.
func (p SettingsPatch) Apply(current Settings) Settings {
	s := current
	if p.WhoCanCreateProjects != nil {
		s.WhoCanCreateProjects = *p.WhoCanCreateProjects
	}
	if p.WhoCanCreateTeams != nil {
		s.WhoCanCreateTeams = *p.WhoCanCreateTeams
	}
	if p.DefaultProjectVisibility != nil {
		s.DefaultProjectVisibility = *p.DefaultProjectVisibility
	}
	if p.Require2FA != nil {
		s.Require2FA = *p.Require2FA
	}
	if p.MaxUploadBytes != nil {
		s.MaxUploadBytes = *p.MaxUploadBytes
	}
	if p.HoursPerDay != nil {
		s.HoursPerDay = *p.HoursPerDay
	}
	if p.DaysPerWeek != nil {
		s.DaysPerWeek = *p.DaysPerWeek
	}
	if p.Timezone != nil {
		s.Timezone = *p.Timezone
	}
	return s
}
