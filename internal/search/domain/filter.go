package domain

import (
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"

	"github.com/jhonsferg/corelog/internal/platform/apperror"
)

// Limits of saved filters.
const (
	MaxFilterNameLength        = 120
	MaxFilterDescriptionLength = 500
	MaxFilterQueryLength       = 4000
	MaxFilterSortLength        = 200
	MaxFilterColumns           = 30
	MaxFilterShares            = 100
	MaxColumnNameLength        = 80
)

var (
	ErrFilterNotFound  = apperror.WithCode(apperror.KindNotFound, CodeFilterNotFound, "filtro no encontrado")
	ErrFilterNameTaken = apperror.WithCode(apperror.KindConflict, CodeFilterNameTaken,
		"ya tienes un filtro con ese nombre")
	ErrFilterForbidden = apperror.WithCode(apperror.KindForbidden, CodeFilterForbidden,
		"no tienes permiso para modificar este filtro")
	ErrSubscriptionNotFound = apperror.WithCode(apperror.KindNotFound, CodeSubscriptionNotFound,
		"no tienes una suscripción a este filtro")
)

// Permission is what a share allows.
type Permission string

const (
	PermissionView Permission = "view"
	PermissionEdit Permission = "edit"
)

// IsValid reports whether p is a known permission.
func (p Permission) IsValid() bool { return p == PermissionView || p == PermissionEdit }

// ShareType is the kind of target a filter is shared with.
type ShareType string

const (
	ShareUser         ShareType = "user"
	ShareTeam         ShareType = "team"
	ShareProject      ShareType = "project"
	ShareOrganization ShareType = "organization"
)

// IsValid reports whether t is a known share type.
func (t ShareType) IsValid() bool {
	switch t {
	case ShareUser, ShareTeam, ShareProject, ShareOrganization:
		return true
	}
	return false
}

// Access is what a viewer can do with a filter, from nothing to everything.
type Access int

const (
	AccessNone Access = iota
	AccessView
	AccessEdit
	AccessOwner
)

// CanView reports whether the viewer can read and run the filter.
func (a Access) CanView() bool { return a >= AccessView }

// CanEdit reports whether the viewer can change the name, query, columns and sort.
func (a Access) CanEdit() bool { return a >= AccessEdit }

// CanManage reports whether the viewer can delete the filter and change its shares.
func (a Access) CanManage() bool { return a == AccessOwner }

// ResolveAccess combines ownership with the best permission among the shares that reach the viewer. An empty
// shared permission means no share reaches the viewer.
func ResolveAccess(owner bool, shared Permission) Access {
	switch {
	case owner:
		return AccessOwner
	case shared == PermissionEdit:
		return AccessEdit
	case shared == PermissionView:
		return AccessView
	}
	return AccessNone
}

// Share grants a permission on a filter to one target.
type Share struct {
	Type       ShareType
	TargetID   uuid.UUID
	Permission Permission
}

// Filter is a saved CoreQL query.
type Filter struct {
	ID             uuid.UUID
	OwnerID        uuid.UUID
	OrganizationID uuid.UUID
	Name           string
	Description    string
	QueryText      string
	Columns        []string
	Sort           string
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

// FilterFields are the user-editable fields of a filter.
type FilterFields struct {
	Name        string
	Description string
	QueryText   string
	Columns     []string
	Sort        string
}

// Normalize trims the fields and validates their shape (not the CoreQL syntax, which the use case checks).
func (f FilterFields) Normalize() (FilterFields, error) {
	f.Name = strings.TrimSpace(f.Name)
	f.Description = strings.TrimSpace(f.Description)
	f.QueryText = strings.TrimSpace(f.QueryText)
	f.Sort = strings.TrimSpace(f.Sort)
	var details []apperror.FieldError
	if f.Name == "" || utf8.RuneCountInString(f.Name) > MaxFilterNameLength {
		details = append(details, apperror.FieldError{Field: "name", Message: "el nombre es obligatorio y admite hasta 120 caracteres"})
	}
	if utf8.RuneCountInString(f.Description) > MaxFilterDescriptionLength {
		details = append(details, apperror.FieldError{Field: "description", Message: "la descripción admite hasta 500 caracteres"})
	}
	if utf8.RuneCountInString(f.QueryText) > MaxFilterQueryLength {
		details = append(details, apperror.FieldError{Field: "query", Message: "la consulta admite hasta 4000 caracteres"})
	}
	if utf8.RuneCountInString(f.Sort) > MaxFilterSortLength {
		details = append(details, apperror.FieldError{Field: "sort", Message: "el orden admite hasta 200 caracteres"})
	}
	cols := make([]string, 0, len(f.Columns))
	seen := map[string]bool{}
	for _, c := range f.Columns {
		c = strings.TrimSpace(c)
		if c == "" || utf8.RuneCountInString(c) > MaxColumnNameLength {
			details = append(details, apperror.FieldError{Field: "columns", Message: "cada columna debe tener entre 1 y 80 caracteres"})
			break
		}
		if !seen[strings.ToLower(c)] {
			seen[strings.ToLower(c)] = true
			cols = append(cols, c)
		}
	}
	if len(cols) > MaxFilterColumns {
		details = append(details, apperror.FieldError{Field: "columns", Message: "se admiten hasta 30 columnas"})
	}
	f.Columns = cols
	if len(details) > 0 {
		return f, apperror.WithCode(apperror.KindValidation, CodeFilterInvalid, "el filtro no es válido").WithDetails(details...)
	}
	return f, nil
}

// NormalizeShares validates a share list: known types and permissions, non-nil targets, no duplicates.
func NormalizeShares(shares []Share) ([]Share, error) {
	if len(shares) > MaxFilterShares {
		return nil, apperror.WithCode(apperror.KindValidation, CodeShareInvalid, "se admiten hasta 100 destinos de uso compartido")
	}
	seen := map[string]bool{}
	out := make([]Share, 0, len(shares))
	for _, s := range shares {
		if !s.Type.IsValid() || s.TargetID == uuid.Nil {
			return nil, apperror.WithCode(apperror.KindValidation, CodeShareInvalid, "destino de uso compartido no válido")
		}
		if s.Permission == "" {
			s.Permission = PermissionView
		}
		if !s.Permission.IsValid() {
			return nil, apperror.WithCode(apperror.KindValidation, CodeShareInvalid, "permiso de uso compartido no válido")
		}
		key := string(s.Type) + ":" + s.TargetID.String()
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, s)
	}
	return out, nil
}

// Frequency is how often a subscription is delivered.
type Frequency string

const (
	FrequencyDaily  Frequency = "daily"
	FrequencyWeekly Frequency = "weekly"
)

// Schedule is when a subscription is delivered. Weekday uses time.Weekday numbering (Sunday = 0) and only
// applies to weekly schedules.
type Schedule struct {
	Frequency Frequency `json:"frequency"`
	Hour      int       `json:"hour"`
	Weekday   int       `json:"weekday"`
	Timezone  string    `json:"timezone"`
}

// Validate checks the schedule and resolves its time zone. An empty time zone means UTC.
func (s Schedule) Validate() (Schedule, error) {
	invalid := func(msg string) error {
		return apperror.WithCode(apperror.KindValidation, CodeSubscriptionInvalid, msg)
	}
	if s.Frequency != FrequencyDaily && s.Frequency != FrequencyWeekly {
		return s, invalid("la frecuencia debe ser daily o weekly")
	}
	if s.Hour < 0 || s.Hour > 23 {
		return s, invalid("la hora debe estar entre 0 y 23")
	}
	if s.Frequency == FrequencyWeekly && (s.Weekday < 0 || s.Weekday > 6) {
		return s, invalid("el día de la semana debe estar entre 0 (domingo) y 6 (sábado)")
	}
	if s.Frequency == FrequencyDaily {
		s.Weekday = 0
	}
	if s.Timezone == "" {
		s.Timezone = "UTC"
	}
	if _, err := time.LoadLocation(s.Timezone); err != nil {
		return s, invalid("la zona horaria no es válida")
	}
	return s, nil
}

// LastOccurrence returns the most recent scheduled instant that is not after now. The schedule must be valid.
func (s Schedule) LastOccurrence(now time.Time) time.Time {
	loc, err := time.LoadLocation(s.Timezone)
	if err != nil {
		loc = time.UTC
	}
	local := now.In(loc)
	at := time.Date(local.Year(), local.Month(), local.Day(), s.Hour, 0, 0, 0, loc)
	if s.Frequency == FrequencyWeekly {
		back := (int(local.Weekday()) - s.Weekday + 7) % 7
		at = at.AddDate(0, 0, -back)
	}
	if at.After(now) {
		step := 1
		if s.Frequency == FrequencyWeekly {
			step = 7
		}
		at = at.AddDate(0, 0, -step)
	}
	return at.UTC()
}

// Subscription delivers a filter to a user on a schedule.
type Subscription struct {
	ID         uuid.UUID
	FilterID   uuid.UUID
	UserID     uuid.UUID
	Schedule   Schedule
	LastSentAt *time.Time
	CreatedAt  time.Time
}

// IsDue reports whether a delivery is pending at now: the latest scheduled instant is after both the last
// delivery and the creation of the subscription, so a new subscription waits for its next slot.
func (s Subscription) IsDue(now time.Time) bool {
	occurrence := s.Schedule.LastOccurrence(now)
	baseline := s.CreatedAt
	if s.LastSentAt != nil && s.LastSentAt.After(baseline) {
		baseline = *s.LastSentAt
	}
	return occurrence.After(baseline)
}
