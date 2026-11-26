package http

import (
	"time"

	"github.com/google/uuid"

	"github.com/jhonsferg/corelog/internal/organization/application/port"
	"github.com/jhonsferg/corelog/internal/organization/domain"
)

// Request bodies. Every field of a request is decoded strictly (unknown fields are rejected) and then
// validated with the struct tags below. Settings values are checked by the domain, which reports one
// detail per invalid field.

type createOrganizationRequest struct {
	Name        string `json:"name" validate:"required,max=120"`
	Slug        string `json:"slug" validate:"max=64"`
	Description string `json:"description" validate:"max=500"`
}

type updateOrganizationRequest struct {
	Name        *string        `json:"name" validate:"omitempty,max=120"`
	Description *string        `json:"description" validate:"omitempty,max=500"`
	Settings    *settingsPatch `json:"settings"`
}

// settingsPatch is a partial update of the organization settings. Omitted fields keep their value.
type settingsPatch struct {
	WhoCanCreateProjects     *string  `json:"who_can_create_projects"`
	WhoCanCreateTeams        *string  `json:"who_can_create_teams"`
	DefaultProjectVisibility *string  `json:"default_project_visibility"`
	Require2FA               *bool    `json:"require_2fa"`
	MaxUploadBytes           *int64   `json:"max_upload_bytes"`
	HoursPerDay              *float64 `json:"hours_per_day"`
	DaysPerWeek              *float64 `json:"days_per_week"`
	Timezone                 *string  `json:"timezone"`
}

type deleteOrganizationRequest struct {
	ConfirmSlug string `json:"confirm_slug" validate:"required,max=64"`
}

type transferOwnershipRequest struct {
	UserID string `json:"user_id" validate:"required,uuid"`
}

type changeMemberRoleRequest struct {
	Role string `json:"role" validate:"required,oneof=owner admin member guest"`
}

type inviteItemRequest struct {
	Email string `json:"email" validate:"required,max=255"`
	Role  string `json:"role" validate:"required,oneof=owner admin member guest"`
}

type createInvitationsRequest struct {
	Invitations []inviteItemRequest `json:"invitations" validate:"required,min=1,max=50,dive"`
}

// Response bodies.

type settingsResponse struct {
	WhoCanCreateProjects     string  `json:"who_can_create_projects"`
	WhoCanCreateTeams        string  `json:"who_can_create_teams"`
	DefaultProjectVisibility string  `json:"default_project_visibility"`
	Require2FA               bool    `json:"require_2fa"`
	MaxUploadBytes           int64   `json:"max_upload_bytes"`
	HoursPerDay              float64 `json:"hours_per_day"`
	DaysPerWeek              float64 `json:"days_per_week"`
	Timezone                 string  `json:"timezone"`
}

type organizationResponse struct {
	ID          string           `json:"id"`
	Slug        string           `json:"slug"`
	Name        string           `json:"name"`
	Description string           `json:"description"`
	Settings    settingsResponse `json:"settings"`
	Role        string           `json:"role,omitempty"`
	CreatedAt   time.Time        `json:"created_at"`
	UpdatedAt   time.Time        `json:"updated_at"`
}

type memberResponse struct {
	UserID   string    `json:"user_id"`
	Name     string    `json:"name"`
	Email    string    `json:"email"`
	Role     string    `json:"role"`
	JoinedAt time.Time `json:"joined_at"`
}

type membershipResponse struct {
	OrganizationID string    `json:"organization_id"`
	UserID         string    `json:"user_id"`
	Role           string    `json:"role"`
	JoinedAt       time.Time `json:"joined_at"`
}

type invitationResponse struct {
	ID         string     `json:"id"`
	Email      string     `json:"email"`
	Role       string     `json:"role"`
	Status     string     `json:"status"`
	InvitedBy  *string    `json:"invited_by"`
	ExpiresAt  time.Time  `json:"expires_at"`
	AcceptedAt *time.Time `json:"accepted_at"`
	RevokedAt  *time.Time `json:"revoked_at"`
	CreatedAt  time.Time  `json:"created_at"`
}

// invitationCreatedResponse is returned once, on create and resend: the plain token and the accept URL
// are never shown again.
type invitationCreatedResponse struct {
	invitationResponse
	Token     string `json:"token"`
	AcceptURL string `json:"accept_url"`
}

type createInvitationsResponse struct {
	Invitations []invitationCreatedResponse `json:"invitations"`
}

type invitationPreviewResponse struct {
	OrganizationName string    `json:"organization_name"`
	Role             string    `json:"role"`
	Email            string    `json:"email"`
	Status           string    `json:"status"`
	ExpiresAt        time.Time `json:"expires_at"`
}

func toSettingsResponse(s domain.Settings) settingsResponse {
	return settingsResponse{
		WhoCanCreateProjects:     s.WhoCanCreateProjects,
		WhoCanCreateTeams:        s.WhoCanCreateTeams,
		DefaultProjectVisibility: s.DefaultProjectVisibility,
		Require2FA:               s.Require2FA,
		MaxUploadBytes:           s.MaxUploadBytes,
		HoursPerDay:              s.HoursPerDay,
		DaysPerWeek:              s.DaysPerWeek,
		Timezone:                 s.Timezone,
	}
}

func toOrganizationResponse(o *domain.Organization, role domain.Role) organizationResponse {
	return organizationResponse{
		ID:          o.ID.String(),
		Slug:        o.Slug,
		Name:        o.Name,
		Description: o.Description,
		Settings:    toSettingsResponse(o.Settings),
		Role:        string(role),
		CreatedAt:   o.CreatedAt,
		UpdatedAt:   o.UpdatedAt,
	}
}

func toMemberResponses(views []port.MemberView) []memberResponse {
	out := make([]memberResponse, 0, len(views))
	for _, v := range views {
		out = append(out, memberResponse{
			UserID:   v.UserID.String(),
			Name:     v.Name,
			Email:    v.Email,
			Role:     string(v.Role),
			JoinedAt: v.JoinedAt,
		})
	}
	return out
}

func toInvitationResponse(inv *domain.Invitation, now time.Time) invitationResponse {
	var invitedBy *string
	if inv.InvitedBy != uuid.Nil {
		s := inv.InvitedBy.String()
		invitedBy = &s
	}
	return invitationResponse{
		ID:         inv.ID.String(),
		Email:      inv.Email,
		Role:       string(inv.Role),
		Status:     string(inv.Status(now)),
		InvitedBy:  invitedBy,
		ExpiresAt:  inv.ExpiresAt,
		AcceptedAt: inv.AcceptedAt,
		RevokedAt:  inv.RevokedAt,
		CreatedAt:  inv.CreatedAt,
	}
}

func toInvitationCreatedResponse(res port.InvitationResult, acceptURL string, now time.Time) invitationCreatedResponse {
	return invitationCreatedResponse{
		invitationResponse: toInvitationResponse(res.Invitation, now),
		Token:              res.PlainToken,
		AcceptURL:          acceptURL,
	}
}

// toDomain converts the request patch into the domain patch. A missing settings object yields nil.
func (p *settingsPatch) toDomain() *domain.SettingsPatch {
	if p == nil {
		return nil
	}
	return &domain.SettingsPatch{
		WhoCanCreateProjects:     p.WhoCanCreateProjects,
		WhoCanCreateTeams:        p.WhoCanCreateTeams,
		DefaultProjectVisibility: p.DefaultProjectVisibility,
		Require2FA:               p.Require2FA,
		MaxUploadBytes:           p.MaxUploadBytes,
		HoursPerDay:              p.HoursPerDay,
		DaysPerWeek:              p.DaysPerWeek,
		Timezone:                 p.Timezone,
	}
}
