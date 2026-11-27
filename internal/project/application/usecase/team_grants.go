package usecase

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/jhonsferg/corelog/internal/project/application/port"
	"github.com/jhonsferg/corelog/internal/project/domain"
)

// ListTeamGrants returns the team roles of the project. Requires manage_members.
func (s *ProjectService) ListTeamGrants(ctx context.Context, actorID uuid.UUID, globalRole string, projectID uuid.UUID) ([]port.TeamGrant, error) {
	project, orgRole, err := s.loadVisible(ctx, actorID, globalRole, projectID)
	if err != nil {
		return nil, err
	}
	if err := s.requireManage(ctx, port.ActionManageMembers, project, actorID, globalRole, orgRole); err != nil {
		return nil, err
	}
	if s.teamGrants == nil {
		return []port.TeamGrant{}, nil
	}
	return s.teamGrants.ListTeamGrants(ctx, projectID)
}

// SetTeamGrant creates or replaces the role of a team. The team must belong to the project organization.
func (s *ProjectService) SetTeamGrant(ctx context.Context, actorID uuid.UUID, globalRole string, projectID, teamID uuid.UUID, role string) (*port.TeamGrant, error) {
	project, orgRole, err := s.loadVisible(ctx, actorID, globalRole, projectID)
	if err != nil {
		return nil, err
	}
	if err := s.requireManage(ctx, port.ActionManageMembers, project, actorID, globalRole, orgRole); err != nil {
		return nil, err
	}
	if err := validateProjectRole(role); err != nil {
		return nil, err
	}
	if s.teamGrants == nil {
		return nil, domain.ErrInvalidProject
	}
	name, found, err := s.teamGrants.TeamInOrganization(ctx, project.OrganizationID, teamID)
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, domain.ErrTeamNotInOrg
	}
	grant := port.TeamGrant{
		ProjectID: projectID,
		TeamID:    teamID,
		TeamName:  name,
		Role:      role,
		CreatedAt: time.Now().UTC(),
	}
	if err := s.teamGrants.UpsertTeamGrant(ctx, grant); err != nil {
		return nil, err
	}
	return &grant, nil
}

// RemoveTeamGrant deletes the team role of the project. Requires manage_members.
func (s *ProjectService) RemoveTeamGrant(ctx context.Context, actorID uuid.UUID, globalRole string, projectID, teamID uuid.UUID) error {
	project, orgRole, err := s.loadVisible(ctx, actorID, globalRole, projectID)
	if err != nil {
		return err
	}
	if err := s.requireManage(ctx, port.ActionManageMembers, project, actorID, globalRole, orgRole); err != nil {
		return err
	}
	if s.teamGrants == nil {
		return domain.ErrTeamGrantNotFound
	}
	return s.teamGrants.RemoveTeamGrant(ctx, projectID, teamID)
}
