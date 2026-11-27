package domain

import (
	"regexp"
	"time"

	"github.com/google/uuid"
)

type Role string

const (
	RoleAdmin Role = "admin"
	RoleUser  Role = "user"
)

// DefaultTimezone is the timezone of users who never chose one.
const DefaultTimezone = "UTC"

// DefaultLocale is the language of users who never chose one.
const DefaultLocale = "es"

func (r Role) IsValid() bool {
	switch r {
	case RoleAdmin, RoleUser:
		return true
	default:
		return false
	}
}

type User struct {
	ID uuid.UUID
	// Name is the full name (first and last name); it stays in sync with them and every reader uses it.
	Name string
	// FirstName and LastName are the separate name parts. LastName may be empty for legacy single-word names.
	FirstName    string
	LastName     string
	Email        string
	PasswordHash string
	Role         Role
	DisplayName  string
	JobTitle     string
	Department   string
	AvatarKey    *string
	Timezone     string
	Locale       string
	// Phone is nil when the user has none. PhoneVerifiedAt is nil until the number is verified.
	Phone           *Phone
	PhoneVerifiedAt *time.Time
	TeamID          *uuid.UUID
	CreatedAt       time.Time
	UpdatedAt       time.Time
	LastLoginAt     *time.Time
	// EmailVerifiedAt is nil until the address is verified.
	EmailVerifiedAt *time.Time
	// DeactivatedAt marca una cuenta cerrada por el propio usuario (borrado logico). Nil = activa.
	DeactivatedAt *time.Time
}

func NewUser(name, email, passwordHash string, role Role) (*User, error) {
	if name == "" {
		return nil, ErrInvalidName
	}
	if email == "" {
		return nil, ErrInvalidEmail
	}
	if passwordHash == "" {
		return nil, ErrInvalidPassword
	}
	if !role.IsValid() {
		return nil, ErrInvalidRole
	}

	first, last := SplitName(name)
	now := time.Now().UTC()
	return &User{
		Name:         name,
		FirstName:    first,
		LastName:     last,
		Locale:       DefaultLocale,
		Email:        email,
		PasswordHash: passwordHash,
		Role:         role,
		DisplayName:  name,
		Timezone:     DefaultTimezone,
		CreatedAt:    now,
		UpdatedAt:    now,
	}, nil
}

// ValidateTimezone accepts IANA zone names known to the runtime. "Local" is rejected
// because it depends on the server, and the empty string is rejected because the
// caller must choose a value explicitly.
func ValidateTimezone(tz string) error {
	if tz == "" || tz == "Local" {
		return ErrInvalidTimezone
	}
	if _, err := time.LoadLocation(tz); err != nil {
		return ErrInvalidTimezone
	}
	return nil
}

func (u *User) UpdateProfile(name, email string) error {
	if name == "" {
		return ErrInvalidName
	}
	if email == "" {
		return ErrInvalidEmail
	}
	u.SetFullName(name)
	u.Email = email
	u.UpdatedAt = time.Now().UTC()
	return nil
}

// SetDisplayName changes the name shown in the UI. It must not be empty.
func (u *User) SetDisplayName(name string) error {
	if name == "" {
		return ErrInvalidDisplayName
	}
	u.DisplayName = name
	u.UpdatedAt = time.Now().UTC()
	return nil
}

// SetJobTitle sets the optional job title; an empty value clears it.
func (u *User) SetJobTitle(title string) {
	u.JobTitle = title
	u.UpdatedAt = time.Now().UTC()
}

// SetDepartment sets the optional department; an empty value clears it.
func (u *User) SetDepartment(department string) {
	u.Department = department
	u.UpdatedAt = time.Now().UTC()
}

// SetTimezone changes the user timezone after validating it.
func (u *User) SetTimezone(tz string) error {
	if err := ValidateTimezone(tz); err != nil {
		return err
	}
	u.Timezone = tz
	u.UpdatedAt = time.Now().UTC()
	return nil
}

func (u *User) ChangePasswordHash(hash string) {
	u.PasswordHash = hash
	u.UpdatedAt = time.Now().UTC()
}

func (u *User) AssignTeam(teamID *uuid.UUID) {
	u.TeamID = teamID
	u.UpdatedAt = time.Now().UTC()
}

// IsActive indica si la cuenta puede iniciar sesion y usar la API.
func (u *User) IsActive() bool {
	return u.DeactivatedAt == nil
}

// Deactivate cierra la cuenta. Es idempotente: una cuenta ya desactivada conserva su fecha.
func (u *User) Deactivate(now time.Time) {
	if u.DeactivatedAt != nil {
		return
	}
	at := now.UTC()
	u.DeactivatedAt = &at
	u.UpdatedAt = at
}

// FullName returns the first and last name joined; it falls back to Name for users without parts.
func (u *User) FullName() string {
	if full := JoinNames(u.FirstName, u.LastName); full != "" {
		return full
	}
	return u.Name
}

// SetNames replaces the first and last name (already validated) and refreshes Name. The display name
// follows the full name while the user has not chosen a different one.
func (u *User) SetNames(first, last string) {
	tracking := u.DisplayName == "" || u.DisplayName == u.FullName()
	u.FirstName, u.LastName = first, last
	u.Name = JoinNames(first, last)
	if tracking {
		u.DisplayName = u.Name
	}
	u.UpdatedAt = time.Now().UTC()
}

// SetFullName splits a legacy full name into first and last name.
func (u *User) SetFullName(full string) {
	first, last := SplitName(full)
	u.SetNames(first, last)
}

// SetPhone stores a new phone number, or clears it with nil. Changing the number clears the verification.
func (u *User) SetPhone(p *Phone) {
	switch {
	case p == nil && u.Phone == nil:
		return
	case p != nil && u.Phone != nil && p.E164 == u.Phone.E164:
		u.Phone = p
		return
	}
	u.Phone = p
	u.PhoneVerifiedAt = nil
	u.UpdatedAt = time.Now().UTC()
}

// MarkPhoneVerified records that the current number was verified at the given instant.
func (u *User) MarkPhoneVerified(at time.Time) {
	t := at.UTC()
	u.PhoneVerifiedAt = &t
	u.UpdatedAt = t
}

// PhoneVerified reports whether the user has a verified phone number.
func (u *User) PhoneVerified() bool { return u.Phone != nil && u.PhoneVerifiedAt != nil }

var localePattern = regexp.MustCompile(`^[a-z]{2}(-[A-Z]{2})?$`)

// ValidateLocale accepts language tags such as "es" or "es-PE".
func ValidateLocale(locale string) error {
	if !localePattern.MatchString(locale) {
		return ErrInvalidLocale
	}
	return nil
}

// SetLocale changes the interface language after validating it.
func (u *User) SetLocale(locale string) error {
	if err := ValidateLocale(locale); err != nil {
		return err
	}
	u.Locale = locale
	u.UpdatedAt = time.Now().UTC()
	return nil
}
