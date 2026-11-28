package usecase

import (
	"context"

	"github.com/google/uuid"

	"github.com/jhonsferg/corelog/internal/notification/application/port"
	"github.com/jhonsferg/corelog/internal/notification/domain"
	"github.com/jhonsferg/corelog/internal/platform/apperror"
	"github.com/jhonsferg/corelog/internal/platform/clock"
)

const (
	actionEditProject = "edit_project"
	roleMember        = "member"
	roleAdmin         = "admin"
)

// SchemeService implements port.SchemeService.
type SchemeService struct {
	repo     port.SchemeRepository
	projects port.ProjectAuthorizer
	orgs     port.OrgRoles
	clock    clock.Clock
}

var _ port.SchemeService = (*SchemeService)(nil)

// NewSchemeService wires the service.
func NewSchemeService(repo port.SchemeRepository, projects port.ProjectAuthorizer, orgs port.OrgRoles, clk clock.Clock) *SchemeService {
	return &SchemeService{repo: repo, projects: projects, orgs: orgs, clock: clk}
}

func errProjectNotFound() error {
	return apperror.WithCode(apperror.KindNotFound, "project.not_found", "Proyecto no encontrado")
}

func (s *SchemeService) projectScope(ctx context.Context, actorID uuid.UUID, role string, projectID uuid.UUID) (uuid.UUID, error) {
	if err := s.projects.Authorize(ctx, actionEditProject, actorID, role, projectID); err != nil {
		return uuid.Nil, err
	}
	orgID, err := s.repo.ProjectOrg(ctx, projectID)
	if err != nil {
		return uuid.Nil, err
	}
	if orgID == uuid.Nil {
		return uuid.Nil, errProjectNotFound()
	}
	return orgID, nil
}

func (s *SchemeService) view(ctx context.Context, orgID uuid.UUID, projectID *uuid.UUID) (port.SchemeView, error) {
	pid := uuid.Nil
	if projectID != nil {
		pid = *projectID
	}
	effective, err := effectiveScheme(ctx, s.repo, orgID, pid)
	if err != nil {
		return port.SchemeView{}, err
	}
	view := port.SchemeView{OrganizationID: orgID, ProjectID: projectID, Scheme: effective, Overridden: map[domain.Event]bool{}}
	stored, err := s.repo.Get(ctx, orgID, projectID)
	if err != nil {
		return port.SchemeView{}, err
	}
	if stored != nil {
		for _, r := range stored.Scheme.Rules {
			view.Overridden[r.Event] = true
		}
		at := stored.UpdatedAt
		view.UpdatedAt = &at
	}
	return view, nil
}

// GetProject returns the scheme in force for a project. It needs edit_project.
func (s *SchemeService) GetProject(ctx context.Context, actorID uuid.UUID, role string, projectID uuid.UUID) (port.SchemeView, error) {
	orgID, err := s.projectScope(ctx, actorID, role, projectID)
	if err != nil {
		return port.SchemeView{}, err
	}
	return s.view(ctx, orgID, &projectID)
}

// PutProject replaces the rules a project overrides.
func (s *SchemeService) PutProject(ctx context.Context, actorID uuid.UUID, role string, projectID uuid.UUID, scheme domain.Scheme) (port.SchemeView, error) {
	orgID, err := s.projectScope(ctx, actorID, role, projectID)
	if err != nil {
		return port.SchemeView{}, err
	}
	clean, err := scheme.Validate()
	if err != nil {
		return port.SchemeView{}, err
	}
	if _, err := s.repo.Save(ctx, orgID, &projectID, clean, actorID, s.clock.Now()); err != nil {
		return port.SchemeView{}, err
	}
	return s.view(ctx, orgID, &projectID)
}

// ResetProject removes the project overrides: the project follows the organization default again.
func (s *SchemeService) ResetProject(ctx context.Context, actorID uuid.UUID, role string, projectID uuid.UUID) (port.SchemeView, error) {
	orgID, err := s.projectScope(ctx, actorID, role, projectID)
	if err != nil {
		return port.SchemeView{}, err
	}
	if _, err := s.repo.Delete(ctx, orgID, &projectID); err != nil {
		return port.SchemeView{}, err
	}
	return s.view(ctx, orgID, &projectID)
}

// GetOrganization returns the organization default. Any member may read it.
func (s *SchemeService) GetOrganization(ctx context.Context, actorID, orgID uuid.UUID) (port.SchemeView, error) {
	if err := s.orgs.RequireRole(ctx, actorID, orgID, roleMember); err != nil {
		return port.SchemeView{}, err
	}
	return s.view(ctx, orgID, nil)
}

// PutOrganization replaces the organization default. It needs an administrator.
func (s *SchemeService) PutOrganization(ctx context.Context, actorID, orgID uuid.UUID, scheme domain.Scheme) (port.SchemeView, error) {
	if err := s.orgs.RequireRole(ctx, actorID, orgID, roleAdmin); err != nil {
		return port.SchemeView{}, err
	}
	clean, err := scheme.Validate()
	if err != nil {
		return port.SchemeView{}, err
	}
	if _, err := s.repo.Save(ctx, orgID, nil, clean, actorID, s.clock.Now()); err != nil {
		return port.SchemeView{}, err
	}
	return s.view(ctx, orgID, nil)
}

// ResetOrganization removes the organization default: the built-in scheme applies again.
func (s *SchemeService) ResetOrganization(ctx context.Context, actorID, orgID uuid.UUID) (port.SchemeView, error) {
	if err := s.orgs.RequireRole(ctx, actorID, orgID, roleAdmin); err != nil {
		return port.SchemeView{}, err
	}
	if _, err := s.repo.Delete(ctx, orgID, nil); err != nil {
		return port.SchemeView{}, err
	}
	return s.view(ctx, orgID, nil)
}
