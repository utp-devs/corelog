package http

import (
	"time"

	"github.com/google/uuid"

	"github.com/jhonsferg/corelog/internal/team/domain"
)

type createTeamRequest struct {
	OrganizationID string `json:"organization_id" validate:"required,uuid"`
	Name           string `json:"name" validate:"required,min=2,max=120"`
	Description    string `json:"description" validate:"max=500"`
	AvatarColor    string `json:"avatar_color" validate:"omitempty,hexcolor"`
}

type updateTeamRequest struct {
	Name        *string `json:"name" validate:"omitempty,min=2,max=120"`
	Description *string `json:"description" validate:"omitempty,max=500"`
	AvatarColor *string `json:"avatar_color" validate:"omitempty,hexcolor"`
}

type addMemberRequest struct {
	UserID string `json:"user_id" validate:"required,uuid"`
	Role   string `json:"role" validate:"omitempty,oneof=maintainer member"`
}

type updateMemberRequest struct {
	Role string `json:"role" validate:"required,oneof=maintainer member"`
}

type teamResponse struct {
	ID             string    `json:"id"`
	OrganizationID string    `json:"organization_id"`
	Name           string    `json:"name"`
	Description    string    `json:"description"`
	AvatarColor    *string   `json:"avatar_color"`
	CreatedBy      *string   `json:"created_by"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
	MemberCount    int       `json:"member_count"`
	MyRole         *string   `json:"my_role"`
}

type memberResponse struct {
	TeamID    string    `json:"team_id"`
	UserID    string    `json:"user_id"`
	Role      string    `json:"role"`
	CreatedAt time.Time `json:"created_at"`
}

func optionalString(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func toTeamResponse(t *domain.Team) teamResponse {
	var createdBy *string
	if t.CreatedBy != uuid.Nil {
		s := t.CreatedBy.String()
		createdBy = &s
	}
	return teamResponse{
		ID:             t.ID.String(),
		OrganizationID: t.OrganizationID.String(),
		Name:           t.Name,
		Description:    t.Description,
		AvatarColor:    optionalString(t.AvatarColor),
		CreatedBy:      createdBy,
		CreatedAt:      t.CreatedAt,
		UpdatedAt:      t.UpdatedAt,
		MemberCount:    t.MemberCount,
		MyRole:         optionalString(t.MyRole),
	}
}

func toTeamResponses(teams []*domain.Team) []teamResponse {
	out := make([]teamResponse, 0, len(teams))
	for _, t := range teams {
		out = append(out, toTeamResponse(t))
	}
	return out
}

func toMemberResponse(m *domain.Member) memberResponse {
	return memberResponse{
		TeamID:    m.TeamID.String(),
		UserID:    m.UserID.String(),
		Role:      m.Role,
		CreatedAt: m.CreatedAt,
	}
}

func toMemberResponses(members []*domain.Member) []memberResponse {
	out := make([]memberResponse, 0, len(members))
	for _, m := range members {
		out = append(out, toMemberResponse(m))
	}
	return out
}
