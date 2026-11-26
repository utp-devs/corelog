package http

import (
	"time"

	"github.com/jhonsferg/corelog/internal/user/application/port"
	"github.com/jhonsferg/corelog/internal/user/domain"
)

type registerRequest struct {
	Name     string `json:"name" validate:"required,min=2,max=120"`
	Email    string `json:"email" validate:"required,email"`
	Password string `json:"password" validate:"required,min=8"`
}

type loginRequest struct {
	Email    string `json:"email" validate:"required,email"`
	Password string `json:"password" validate:"required"`
}

// updateProfileRequest is a partial update: omitted fields keep their value. Timezone is
// validated in the domain (IANA name), so an invalid value answers user.timezone_invalid.
type updateProfileRequest struct {
	Name        *string `json:"name" validate:"omitempty,min=2,max=120"`
	Email       *string `json:"email" validate:"omitempty,email"`
	DisplayName *string `json:"display_name" validate:"omitempty,max=120"`
	JobTitle    *string `json:"job_title" validate:"omitempty,max=120"`
	Department  *string `json:"department" validate:"omitempty,max=120"`
	Timezone    *string `json:"timezone" validate:"omitempty,max=64"`
}

type changePasswordRequest struct {
	CurrentPassword string `json:"current_password" validate:"required"`
	NewPassword     string `json:"new_password" validate:"required,min=8"`
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
	ID              string     `json:"id"`
	Name            string     `json:"name"`
	Email           string     `json:"email"`
	Role            string     `json:"role"`
	DisplayName     string     `json:"display_name"`
	JobTitle        string     `json:"job_title"`
	Department      string     `json:"department"`
	AvatarKey       *string    `json:"avatar_key"`
	Timezone        string     `json:"timezone"`
	TeamID          *string    `json:"team_id,omitempty"`
	CreatedAt       time.Time  `json:"created_at"`
	LastLoginAt     *time.Time `json:"last_login_at"`
	EmailVerifiedAt *time.Time `json:"email_verified_at"`
}

type authResponse struct {
	Token string       `json:"token"`
	User  userResponse `json:"user"`
}

func toUserResponse(u *domain.User) userResponse {
	resp := userResponse{
		ID:              u.ID.String(),
		Name:            u.Name,
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

func toUserResponses(users []*domain.User) []userResponse {
	out := make([]userResponse, 0, len(users))
	for _, u := range users {
		out = append(out, toUserResponse(u))
	}
	return out
}
