package http

import (
	"time"

	"github.com/jhonsferg/corelog/internal/project/application/port"
	"github.com/jhonsferg/corelog/internal/project/domain"
)

// createProjectRequest: el creador queda como lider. template es opcional si viene type (legado);
// visibility es opcional; si falta se usa default_project_visibility de la organizacion.
type createProjectRequest struct {
	OrganizationID        string   `json:"organization_id" validate:"required,uuid"`
	Key                   string   `json:"key" validate:"required,min=1,max=20"`
	Name                  string   `json:"name" validate:"required,min=2,max=120"`
	Description           string   `json:"description" validate:"max=5000"`
	Template              string   `json:"template" validate:"max=40"`
	Type                  string   `json:"type" validate:"omitempty,oneof=kanban scrum"`
	Visibility            string   `json:"visibility" validate:"omitempty,oneof=private open team"`
	Color                 string   `json:"color" validate:"max=20"`
	Icon                  string   `json:"icon" validate:"max=40"`
	DefaultAssigneePolicy string   `json:"default_assignee_policy" validate:"max=20"`
	MemberIDs             []string `json:"member_ids" validate:"max=100,dive,uuid"`
	TeamIDs               []string `json:"team_ids" validate:"max=100,dive,uuid"`
}

type updateProjectRequest struct {
	Name                  string `json:"name" validate:"required,min=2,max=120"`
	Description           string `json:"description" validate:"max=5000"`
	Type                  string `json:"type" validate:"required,oneof=kanban scrum"`
	LeadID                string `json:"lead_id" validate:"required,uuid"`
	Visibility            string `json:"visibility" validate:"required,oneof=private open team"`
	Color                 string `json:"color" validate:"max=20"`
	Icon                  string `json:"icon" validate:"max=40"`
	DefaultAssigneePolicy string `json:"default_assignee_policy" validate:"max=20"`
}

// deleteProjectRequest carries the typed confirmation: the exact project name.
type deleteProjectRequest struct {
	Confirmation string `json:"confirmation"`
}

type projectResponse struct {
	ID                    string     `json:"id"`
	OrganizationID        string     `json:"organization_id"`
	Key                   string     `json:"key"`
	Name                  string     `json:"name"`
	Description           string     `json:"description"`
	Type                  string     `json:"type"`
	Template              string     `json:"template"`
	LeadID                string     `json:"lead_id"`
	Visibility            string     `json:"visibility"`
	Color                 string     `json:"color"`
	Icon                  string     `json:"icon"`
	DefaultAssigneePolicy string     `json:"default_assignee_policy"`
	ArchivedAt            *time.Time `json:"archived_at"`
	CreatedAt             time.Time  `json:"created_at"`
	UpdatedAt             time.Time  `json:"updated_at"`
}

// projectKeyAvailabilityResponse is the body of the key availability check.
type projectKeyAvailabilityResponse struct {
	Available bool   `json:"available"`
	Key       string `json:"key"`
}

type projectMemberRequest struct {
	UserID string `json:"user_id" validate:"required,uuid"`
	Role   string `json:"role" validate:"required,oneof=lead contributor viewer"`
}

type updateProjectMemberRequest struct {
	Role string `json:"role" validate:"required,oneof=lead contributor viewer"`
}

type projectMemberResponse struct {
	ProjectID string `json:"project_id"`
	UserID    string `json:"user_id"`
	// Role is the explicit role of a direct member, or the team grant role for a team-only user.
	Role string `json:"role"`
	// EffectiveRole is the best of the explicit role and every team grant of the user.
	EffectiveRole string     `json:"effective_role"`
	ViaTeam       bool       `json:"via_team"`
	TeamID        *string    `json:"team_id,omitempty"`
	TeamName      string     `json:"team_name,omitempty"`
	CreatedAt     *time.Time `json:"created_at,omitempty"`
}

// teamGrantRequest sets the role of one team on a project.
type teamGrantRequest struct {
	TeamID string `json:"team_id" validate:"required,uuid"`
	Role   string `json:"role" validate:"required,oneof=lead contributor viewer"`
}

type teamGrantResponse struct {
	ProjectID string    `json:"project_id"`
	TeamID    string    `json:"team_id"`
	TeamName  string    `json:"team_name"`
	Role      string    `json:"role"`
	CreatedAt time.Time `json:"created_at"`
}

type labelRequest struct {
	Name  string `json:"name" validate:"required,min=1,max=40"`
	Color string `json:"color" validate:"omitempty,max=20"`
}

// labelUpdateRequest carries the fields of a label rename or recolor. Absent fields keep their value.
type labelUpdateRequest struct {
	Name  *string `json:"name" validate:"omitempty,min=1,max=40"`
	Color *string `json:"color" validate:"omitempty,max=20"`
}

type labelResponse struct {
	ID         string    `json:"id"`
	ProjectID  string    `json:"project_id"`
	Name       string    `json:"name"`
	Color      string    `json:"color"`
	UsageCount int       `json:"usage_count"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

func toProjectResponse(p *domain.Project) projectResponse {
	return projectResponse{
		ID: p.ID.String(), OrganizationID: p.OrganizationID.String(), Key: p.Key, Name: p.Name, Description: p.Description,
		Type: string(p.Type), Template: string(p.Template), LeadID: p.LeadID.String(), Visibility: string(p.Visibility),
		Color: p.Color, Icon: p.Icon, DefaultAssigneePolicy: string(p.DefaultAssigneePolicy),
		ArchivedAt: p.ArchivedAt, CreatedAt: p.CreatedAt, UpdatedAt: p.UpdatedAt,
	}
}

func toProjectResponses(projects []*domain.Project) []projectResponse {
	out := make([]projectResponse, 0, len(projects))
	for _, p := range projects {
		out = append(out, toProjectResponse(p))
	}
	return out
}

func toProjectMemberResponse(member port.Member) projectMemberResponse {
	return projectMemberResponse{
		ProjectID:     member.ProjectID.String(),
		UserID:        member.UserID.String(),
		Role:          member.Role,
		EffectiveRole: member.Role,
		CreatedAt:     timePtr(member.CreatedAt),
	}
}

func toProjectMemberEntryResponses(entries []port.MemberEntry) []projectMemberResponse {
	out := make([]projectMemberResponse, 0, len(entries))
	for _, entry := range entries {
		response := toProjectMemberResponse(entry.Member)
		response.EffectiveRole = entry.EffectiveRole
		response.ViaTeam = entry.ViaTeam
		if entry.TeamID != nil {
			teamID := entry.TeamID.String()
			response.TeamID = &teamID
		}
		response.TeamName = entry.TeamName
		if entry.ViaTeam {
			response.CreatedAt = nil
		}
		out = append(out, response)
	}
	return out
}

func toTeamGrantResponse(grant port.TeamGrant) teamGrantResponse {
	return teamGrantResponse{
		ProjectID: grant.ProjectID.String(),
		TeamID:    grant.TeamID.String(),
		TeamName:  grant.TeamName,
		Role:      grant.Role,
		CreatedAt: grant.CreatedAt,
	}
}

func toTeamGrantResponses(grants []port.TeamGrant) []teamGrantResponse {
	out := make([]teamGrantResponse, 0, len(grants))
	for _, grant := range grants {
		out = append(out, toTeamGrantResponse(grant))
	}
	return out
}

func timePtr(value time.Time) *time.Time {
	if value.IsZero() {
		return nil
	}
	return &value
}

func toLabelResponse(label domain.Label) labelResponse {
	return labelResponse{
		ID: label.ID.String(), ProjectID: label.ProjectID.String(), Name: label.Name, Color: label.Color,
		UsageCount: label.UsageCount,
		CreatedAt:  label.CreatedAt, UpdatedAt: label.UpdatedAt,
	}
}

func toLabelResponses(labels []domain.Label) []labelResponse {
	out := make([]labelResponse, 0, len(labels))
	for _, label := range labels {
		out = append(out, toLabelResponse(label))
	}
	return out
}
