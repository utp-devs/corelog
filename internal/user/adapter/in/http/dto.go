package http

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"

	"github.com/jhonsferg/corelog/internal/user/application/port"
	"github.com/jhonsferg/corelog/internal/user/domain"
)

type registerRequest struct {
	// FirstName and LastName are the current identity fields; Name is the legacy single field, used only when
	// first_name is empty. The names are validated in the domain (code user.invalid_name).
	FirstName string `json:"first_name" validate:"omitempty,max=80"`
	LastName  string `json:"last_name" validate:"omitempty,max=120"`
	Name      string `json:"name" validate:"omitempty,max=120"`
	Email     string `json:"email" validate:"required,email"`
	Password  string `json:"password" validate:"required,min=8"`
	// PasswordConfirmation is compared with Password before any other validation.
	PasswordConfirmation string        `json:"password_confirmation"`
	Phone                *phoneRequest `json:"phone"`
	Locale               string        `json:"locale" validate:"omitempty,max=10"`
	// Consents are the legal document versions accepted; they must cover every required current document.
	Consents []consentRefRequest `json:"consents"`
}

// phoneRequest is the phone input: country is the ISO 3166-1 alpha-2 code; number may be national or carry
// the international prefix.
type phoneRequest struct {
	Country string `json:"country" validate:"required,len=2"`
	Number  string `json:"number" validate:"required,max=32"`
}

// optionalPhone tells an omitted phone (keep) from an explicit null (clear) in a partial update.
type optionalPhone struct {
	Present bool
	Value   *phoneRequest
}

func (o *optionalPhone) UnmarshalJSON(b []byte) error {
	o.Present = true
	if string(b) == "null" {
		o.Value = nil
		return nil
	}
	return json.Unmarshal(b, &o.Value)
}

type loginRequest struct {
	Email    string `json:"email" validate:"required,email"`
	Password string `json:"password" validate:"required"`
}

// updateProfileRequest is a partial update: omitted fields keep their value. Timezone is
// validated in the domain (IANA name), so an invalid value answers user.timezone_invalid.
type updateProfileRequest struct {
	Name      *string `json:"name" validate:"omitempty,min=2,max=120"`
	FirstName *string `json:"first_name" validate:"omitempty,max=80"`
	LastName  *string `json:"last_name" validate:"omitempty,max=120"`
	// Phone: omitted keeps the number, null removes it, an object replaces it (clearing the verification).
	Phone       optionalPhone `json:"phone"`
	Locale      *string       `json:"locale" validate:"omitempty,max=10"`
	Email       *string       `json:"email" validate:"omitempty,email"`
	DisplayName *string       `json:"display_name" validate:"omitempty,max=120"`
	JobTitle    *string       `json:"job_title" validate:"omitempty,max=120"`
	Department  *string       `json:"department" validate:"omitempty,max=120"`
	Timezone    *string       `json:"timezone" validate:"omitempty,max=64"`
}

type changePasswordRequest struct {
	CurrentPassword string `json:"current_password" validate:"required"`
	NewPassword     string `json:"new_password" validate:"required,min=8"`
	// NewPasswordConfirmation is compared with NewPassword before any other validation.
	NewPasswordConfirmation string `json:"new_password_confirmation"`
}

type deactivateRequest struct {
	Password string `json:"password" validate:"required"`
}

type sessionResponse struct {
	ID         string `json:"id"`
	UserAgent  string `json:"user_agent"`
	CreatedAt  string `json:"created_at"`
	LastSeenAt string `json:"last_seen_at"`
	Current    bool   `json:"current"`
}

func toSessionResponse(v port.SessionView) sessionResponse {
	return sessionResponse{ID: v.ID, UserAgent: v.UserAgent, CreatedAt: v.CreatedAt, LastSeenAt: v.LastSeenAt, Current: v.Current}
}

type setTeamRequest struct {
	TeamID *string `json:"team_id" validate:"omitempty,uuid4"`
}

type userResponse struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
	Locale    string `json:"locale"`
	// Phone is present only for the user themself and for admins and owners; absent when there is none.
	Phone           *phoneResponse `json:"phone,omitempty"`
	Email           string         `json:"email"`
	Role            string         `json:"role"`
	DisplayName     string         `json:"display_name"`
	JobTitle        string         `json:"job_title"`
	Department      string         `json:"department"`
	AvatarKey       *string        `json:"avatar_key"`
	Timezone        string         `json:"timezone"`
	TeamID          *string        `json:"team_id,omitempty"`
	CreatedAt       time.Time      `json:"created_at"`
	LastLoginAt     *time.Time     `json:"last_login_at"`
	EmailVerifiedAt *time.Time     `json:"email_verified_at"`
}

// phoneResponse is the phone of a user. verified_at is absent until the number is verified.
type phoneResponse struct {
	Country    string     `json:"country"`
	Number     string     `json:"number"`
	E164       string     `json:"e164"`
	Verified   bool       `json:"verified"`
	VerifiedAt *time.Time `json:"verified_at,omitempty"`
}

type phoneVerifyRequest struct {
	Code string `json:"code" validate:"required,len=6,numeric"`
}

type phoneVerificationResponse struct {
	Destination string    `json:"destination"`
	ExpiresAt   time.Time `json:"expires_at"`
}

type authResponse struct {
	Token string       `json:"token"`
	User  userResponse `json:"user"`
	// PendingConsents lists the required legal documents the user has not accepted at their current version.
	PendingConsents []pendingConsentResponse `json:"pending_consents"`
}

// toUserResponse is the public view of a user: it never carries the phone. toPrivateUserResponse adds it
// and is used for the user themself and for admins and owners.
func toUserResponse(u *domain.User) userResponse {
	resp := userResponse{
		ID:              u.ID.String(),
		Name:            u.Name,
		FirstName:       u.FirstName,
		LastName:        u.LastName,
		Locale:          u.Locale,
		Email:           u.Email,
		Role:            string(u.Role),
		DisplayName:     u.DisplayName,
		JobTitle:        u.JobTitle,
		Department:      u.Department,
		AvatarKey:       u.AvatarKey,
		Timezone:        u.Timezone,
		CreatedAt:       u.CreatedAt,
		LastLoginAt:     u.LastLoginAt,
		EmailVerifiedAt: u.EmailVerifiedAt,
	}
	if u.TeamID != nil {
		id := u.TeamID.String()
		resp.TeamID = &id
	}
	return resp
}

func toPrivateUserResponse(u *domain.User) userResponse {
	resp := toUserResponse(u)
	if u.Phone != nil {
		resp.Phone = &phoneResponse{
			Country: u.Phone.Country, Number: u.Phone.National, E164: u.Phone.E164,
			Verified: u.PhoneVerifiedAt != nil, VerifiedAt: u.PhoneVerifiedAt,
		}
	}
	return resp
}

// toUserResponses renders a directory. Phones are included only when showPhones is true (admins and
// owners); every user always sees their own.
func toUserResponses(users []*domain.User, showPhones bool, viewer uuid.UUID) []userResponse {
	out := make([]userResponse, 0, len(users))
	for _, u := range users {
		if showPhones || u.ID == viewer {
			out = append(out, toPrivateUserResponse(u))
		} else {
			out = append(out, toUserResponse(u))
		}
	}
	return out
}
