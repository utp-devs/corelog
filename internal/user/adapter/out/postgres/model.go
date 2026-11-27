package postgres

import (
	"time"

	"github.com/google/uuid"

	"github.com/jhonsferg/corelog/internal/user/domain"
)

type userRow struct {
	ID              uuid.UUID  `db:"id"`
	Name            string     `db:"name"`
	FirstName       string     `db:"first_name"`
	LastName        string     `db:"last_name"`
	Locale          string     `db:"locale"`
	PhoneCountry    *string    `db:"phone_country"`
	PhoneNumber     *string    `db:"phone_number"`
	PhoneE164       *string    `db:"phone_e164"`
	PhoneVerifiedAt *time.Time `db:"phone_verified_at"`
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
	u := &domain.User{
		ID:              r.ID,
		Name:            r.Name,
		FirstName:       r.FirstName,
		LastName:        r.LastName,
		Locale:          r.Locale,
		PhoneVerifiedAt: r.PhoneVerifiedAt,
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
	if r.PhoneE164 != nil && r.PhoneCountry != nil && r.PhoneNumber != nil {
		u.Phone = &domain.Phone{Country: *r.PhoneCountry, National: *r.PhoneNumber, E164: *r.PhoneE164}
	}
	return u
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
	first, last := u.FirstName, u.LastName
	if first == "" && last == "" {
		first, last = domain.SplitName(u.Name)
	}
	locale := u.Locale
	if locale == "" {
		locale = domain.DefaultLocale
	}
	row := userRow{
		ID:              u.ID,
		Name:            u.Name,
		FirstName:       first,
		LastName:        last,
		Locale:          locale,
		PhoneVerifiedAt: u.PhoneVerifiedAt,
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
	if u.Phone != nil {
		row.PhoneCountry, row.PhoneNumber, row.PhoneE164 = &u.Phone.Country, &u.Phone.National, &u.Phone.E164
		// A number never keeps a verification it was not given for.
	} else {
		row.PhoneVerifiedAt = nil
	}
	return row
}
