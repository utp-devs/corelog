package domain

import (
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

func (r Role) IsValid() bool {
	switch r {
	case RoleAdmin, RoleUser:
		return true
	default:
		return false
	}
}

type User struct {
	ID           uuid.UUID
	Name         string
	Email        string
	PasswordHash string
	Role         Role
	DisplayName  string
	JobTitle     string
	Department   string
	AvatarKey    *string
	Timezone     string
	TeamID       *uuid.UUID
	CreatedAt    time.Time
	UpdatedAt    time.Time
	LastLoginAt  *time.Time
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

	now := time.Now().UTC()
	return &User{
		Name:         name,
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
	u.Name = name
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
