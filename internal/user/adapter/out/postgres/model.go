package postgres

import (
	"time"

	"github.com/google/uuid"

	"github.com/jhonsferg/corelog/internal/user/domain"
)

type userRow struct {
	ID              uuid.UUID  `db:"id"`
	Name            string     `db:"name"`
	Email           string     `db:"email"`
	PasswordHash    string     `db:"password_hash"`
	Role            string     `db:"role"`
	DisplayName     string     `db:"display_name"`
	JobTitle        string     `db:"job_title"`
	Department      string     `db:"department"`
	AvatarKey       *string    `db:"avatar_key"`
	Timezone        string     `db:"timezone"`
	TeamID          *uuid.UUID `db:"team_id"`
	CreatedAt       time.Time  `db:"created_at"`
	UpdatedAt       time.Time  `db:"updated_at"`
	LastLoginAt     *time.Time `db:"last_login_at"`
	EmailVerifiedAt *time.Time `db:"email_verified_at"`
	DeactivatedAt   *time.Time `db:"deactivated_at"`
}

func (r userRow) toDomain() *domain.User {
	return &domain.User{
		ID:              r.ID,
		Name:            r.Name,
		Email:           r.Email,
		PasswordHash:    r.PasswordHash,
		Role:            domain.Role(r.Role),
		DisplayName:     r.DisplayName,
		JobTitle:        r.JobTitle,
		Department:      r.Department,
		AvatarKey:       r.AvatarKey,
		Timezone:        r.Timezone,
		TeamID:          r.TeamID,
		CreatedAt:       r.CreatedAt,
		UpdatedAt:       r.UpdatedAt,
		LastLoginAt:     r.LastLoginAt,
		EmailVerifiedAt: r.EmailVerifiedAt,
		DeactivatedAt:   r.DeactivatedAt,
	}
}

// fromDomain fills the columns that domain constructors may leave empty: display_name
// falls back to name and timezone to UTC, because both columns are NOT NULL.
func fromDomain(u *domain.User) userRow {
	displayName := u.DisplayName
	if displayName == "" {
		displayName = u.Name
	}
	timezone := u.Timezone
	if timezone == "" {
		timezone = domain.DefaultTimezone
	}
	return userRow{
		ID:              u.ID,
		Name:            u.Name,
		Email:           u.Email,
		PasswordHash:    u.PasswordHash,
		Role:            string(u.Role),
		DisplayName:     displayName,
		JobTitle:        u.JobTitle,
		Department:      u.Department,
		AvatarKey:       u.AvatarKey,
		Timezone:        timezone,
		TeamID:          u.TeamID,
		CreatedAt:       u.CreatedAt,
		UpdatedAt:       u.UpdatedAt,
		LastLoginAt:     u.LastLoginAt,
		EmailVerifiedAt: u.EmailVerifiedAt,
		DeactivatedAt:   u.DeactivatedAt,
	}
}
