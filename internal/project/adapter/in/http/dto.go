package http

import (
	"time"

	"github.com/google/uuid"

	"github.com/jhonsferg/corelog/internal/project/application/port"
	"github.com/jhonsferg/corelog/internal/project/domain"
)

type createProjectRequest struct {
	Key         string `json:"key" validate:"required,min=1,max=20"`
	Name        string `json:"name" validate:"required,min=2,max=120"`
	Description string `json:"description" validate:"max=5000"`
	Type        string `json:"type" validate:"required,oneof=kanban scrum"`
	LeadID      string `json:"lead_id" validate:"required,uuid"`
	Visibility  string `json:"visibility" validate:"required,oneof=private team"`
}

type updateProjectRequest struct {
	Name        string `json:"name" validate:"required,min=2,max=120"`
	Description string `json:"description" validate:"max=5000"`
	Type        string `json:"type" validate:"required,oneof=kanban scrum"`
	LeadID      string `json:"lead_id" validate:"required,uuid"`
	Visibility  string `json:"visibility" validate:"required,oneof=private team"`
}

type projectResponse struct {
	ID          string     `json:"id"`
	Key         string     `json:"key"`
	Name        string     `json:"name"`
	Description string     `json:"description"`
	Type        string     `json:"type"`
	LeadID      string     `json:"lead_id"`
	Visibility  string     `json:"visibility"`
	ArchivedAt  *time.Time `json:"archived_at"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
}

type projectMemberRequest struct {
	UserID string `json:"user_id" validate:"required,uuid"`
	Role   string `json:"role" validate:"required,oneof=lead contributor viewer"`
}

type updateProjectMemberRequest struct {
	Role string `json:"role" validate:"required,oneof=lead contributor viewer"`
}

type projectMemberResponse struct {
	ProjectID string    `json:"project_id"`
	UserID    string    `json:"user_id"`
	Role      string    `json:"role"`
	CreatedAt time.Time `json:"created_at"`
}

func toProjectResponse(p *domain.Project) projectResponse {
	return projectResponse{
		ID: p.ID.String(), Key: p.Key, Name: p.Name, Description: p.Description,
		Type: string(p.Type), LeadID: p.LeadID.String(), Visibility: string(p.Visibility),
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
		ProjectID: member.ProjectID.String(),
		UserID:    member.UserID.String(),
		Role:      member.Role,
		CreatedAt: member.CreatedAt,
	}
}

func toProjectMemberResponses(members []port.Member) []projectMemberResponse {
	out := make([]projectMemberResponse, 0, len(members))
	for _, member := range members {
		out = append(out, toProjectMemberResponse(member))
	}
	return out
}

func parseProjectID(value string) (uuid.UUID, error) {
	return uuid.Parse(value)
}
