package usecase

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"

	authorization "github.com/jhonsferg/corelog/internal/authorization/domain"
	"github.com/jhonsferg/corelog/internal/platform/apperror"
	"github.com/jhonsferg/corelog/internal/project/application/port"
	"github.com/jhonsferg/corelog/internal/project/domain"
)

type ProjectService struct {
	repo        port.Repository
	memberRepo  port.MemberRepository
	catalogRepo port.CatalogRepository
	policy      authorization.Policy
}

func NewProjectService(repo port.Repository, memberRepo port.MemberRepository, catalogRepo ...port.CatalogRepository) *ProjectService {
	var catalog port.CatalogRepository
	if len(catalogRepo) > 0 {
		catalog = catalogRepo[0]
	}
	return &ProjectService{repo: repo, memberRepo: memberRepo, catalogRepo: catalog, policy: authorization.Policy{}}
}

var _ port.Service = (*ProjectService)(nil)

func (s *ProjectService) Create(ctx context.Context, key, name, description string, projectType domain.ProjectType, leadID uuid.UUID, visibility domain.Visibility) (*domain.Project, error) {
	project, err := domain.NewProject(key, name, description, projectType, leadID, visibility)
	if err != nil {
		return nil, err
	}
	if _, err := s.repo.FindByKey(ctx, project.Key); err == nil {
		return nil, domain.ErrProjectKeyTaken
	} else if !errors.Is(err, domain.ErrProjectNotFound) {
		return nil, err
	}
	// El ID del proyecto lo asigna la BD al persistirlo; el lider se vincula despues.
	lead := port.Member{
		UserID:    leadID,
		Role:      string(authorization.ProjectRoleLead),
		CreatedAt: time.Now().UTC(),
	}
	if creator, ok := s.repo.(port.ProjectCreationRepository); ok {
		if err := creator.CreateWithDefaults(ctx, project, lead); err != nil {
			return nil, err
		}
	} else {
		if err := s.repo.Save(ctx, project); err != nil {
			return nil, err
		}
		lead.ProjectID = project.ID
		if err := s.memberRepo.Add(ctx, lead); err != nil {
			return nil, err
		}
	}
	return project, nil
}

func (s *ProjectService) Get(ctx context.Context, actorID uuid.UUID, globalRole string, id uuid.UUID) (*domain.Project, error) {
	project, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	projectRole, err := s.projectRole(ctx, id, actorID)
	if err != nil {
		return nil, err
	}
	if !s.policy.CanViewProject(globalRole, projectRole) {
		return nil, apperror.New(apperror.KindForbidden, "forbidden project operation")
	}
	return project, nil
}

func (s *ProjectService) List(ctx context.Context, actorID uuid.UUID, globalRole string) ([]*domain.Project, error) {
	projects, err := s.repo.FindAll(ctx)
	if err != nil {
		return nil, err
	}
	if globalRole == "admin" {
		return projects, nil
	}
	visible := make([]*domain.Project, 0, len(projects))
	for _, project := range projects {
		projectRole, err := s.projectRole(ctx, project.ID, actorID)
		if err != nil {
			return nil, err
		}
		if s.policy.CanViewProject(globalRole, projectRole) {
			visible = append(visible, project)
		}
	}
	return visible, nil
}

func (s *ProjectService) Update(ctx context.Context, actorID uuid.UUID, globalRole string, id uuid.UUID, input domain.UpdateInput) (*domain.Project, error) {
	project, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	projectRole, err := s.projectRole(ctx, id, actorID)
	if err != nil {
		return nil, err
	}
	if !s.policy.CanEditProject(globalRole, projectRole) {
		return nil, apperror.New(apperror.KindForbidden, "forbidden project operation")
	}
	if err := project.Update(input); err != nil {
		return nil, err
	}
	if err := s.repo.Update(ctx, project); err != nil {
		return nil, err
	}
	return project, nil
}

func (s *ProjectService) Delete(ctx context.Context, actorID uuid.UUID, globalRole string, id uuid.UUID) error {
	projectRole, err := s.projectRole(ctx, id, actorID)
	if err != nil {
		return err
	}
	if !s.policy.CanEditProject(globalRole, projectRole) {
		return apperror.New(apperror.KindForbidden, "forbidden project operation")
	}
	return s.repo.Delete(ctx, id)
}

func (s *ProjectService) projectRole(ctx context.Context, projectID, actorID uuid.UUID) (*authorization.ProjectRole, error) {
	role, err := s.memberRepo.FindRole(ctx, projectID, actorID)
	if err != nil {
		return nil, err
	}
	if role == nil {
		return nil, nil
	}
	projectRole := authorization.ProjectRole(*role)
	switch projectRole {
	case authorization.ProjectRoleLead, authorization.ProjectRoleContributor, authorization.ProjectRoleViewer:
		return &projectRole, nil
	default:
		return nil, apperror.New(apperror.KindValidation, "invalid project role")
	}
}

func (s *ProjectService) requireMembersPermission(ctx context.Context, actorID uuid.UUID, globalRole string, projectID uuid.UUID) error {
	projectRole, err := s.projectRole(ctx, projectID, actorID)
	if err != nil {
		return err
	}
	if !s.policy.CanManageProjectMembers(globalRole, projectRole) {
		return apperror.New(apperror.KindForbidden, "forbidden project operation")
	}
	return nil
}

func (s *ProjectService) ListMembers(ctx context.Context, actorID uuid.UUID, globalRole string, projectID uuid.UUID) ([]port.Member, error) {
	projectRole, err := s.projectRole(ctx, projectID, actorID)
	if err != nil {
		return nil, err
	}
	if !s.policy.CanViewProject(globalRole, projectRole) {
		return nil, apperror.New(apperror.KindForbidden, "forbidden project operation")
	}
	return s.memberRepo.List(ctx, projectID)
}

func (s *ProjectService) AddMember(ctx context.Context, actorID uuid.UUID, globalRole string, projectID, userID uuid.UUID, role string) (*port.Member, error) {
	if err := s.requireMembersPermission(ctx, actorID, globalRole, projectID); err != nil {
		return nil, err
	}
	projectRole := authorization.ProjectRole(role)
	switch projectRole {
	case authorization.ProjectRoleLead, authorization.ProjectRoleContributor, authorization.ProjectRoleViewer:
	default:
		return nil, apperror.New(apperror.KindValidation, "invalid project role")
	}
	member := port.Member{
		ProjectID: projectID,
		UserID:    userID,
		Role:      role,
		CreatedAt: time.Now().UTC(),
	}
	if err := s.memberRepo.Add(ctx, member); err != nil {
		return nil, err
	}
	return &member, nil
}

func (s *ProjectService) UpdateMemberRole(ctx context.Context, actorID uuid.UUID, globalRole string, projectID, userID uuid.UUID, role string) (*port.Member, error) {
	if err := s.requireMembersPermission(ctx, actorID, globalRole, projectID); err != nil {
		return nil, err
	}
	projectRole := authorization.ProjectRole(role)
	switch projectRole {
	case authorization.ProjectRoleLead, authorization.ProjectRoleContributor, authorization.ProjectRoleViewer:
	default:
		return nil, apperror.New(apperror.KindValidation, "invalid project role")
	}
	if err := s.memberRepo.UpdateRole(ctx, projectID, userID, role); err != nil {
		return nil, err
	}
	return &port.Member{ProjectID: projectID, UserID: userID, Role: role}, nil
}

func (s *ProjectService) RemoveMember(ctx context.Context, actorID uuid.UUID, globalRole string, projectID, userID uuid.UUID) error {
	if err := s.requireMembersPermission(ctx, actorID, globalRole, projectID); err != nil {
		return err
	}
	return s.memberRepo.Remove(ctx, projectID, userID)
}

func (s *ProjectService) ListLabels(ctx context.Context, projectID uuid.UUID) ([]domain.Label, error) {
	if s.catalogRepo == nil {
		return []domain.Label{}, nil
	}
	return s.catalogRepo.ListLabels(ctx, projectID)
}

func (s *ProjectService) CreateLabel(ctx context.Context, projectID uuid.UUID, name, color string) (*domain.Label, error) {
	if s.catalogRepo == nil {
		return nil, domain.ErrInvalidProject
	}
	label, err := domain.NewLabel(projectID, name, color)
	if err != nil {
		return nil, err
	}
	if err := s.catalogRepo.CreateLabel(ctx, label); err != nil {
		return nil, err
	}
	return label, nil
}

func (s *ProjectService) DeleteLabel(ctx context.Context, projectID, labelID uuid.UUID) error {
	if s.catalogRepo == nil {
		return domain.ErrInvalidProject
	}
	return s.catalogRepo.DeleteLabel(ctx, projectID, labelID)
}

func (s *ProjectService) ListComponents(ctx context.Context, projectID uuid.UUID) ([]domain.Component, error) {
	if s.catalogRepo == nil {
		return []domain.Component{}, nil
	}
	return s.catalogRepo.ListComponents(ctx, projectID)
}

func (s *ProjectService) CreateComponent(ctx context.Context, projectID uuid.UUID, name, description string) (*domain.Component, error) {
	if s.catalogRepo == nil {
		return nil, domain.ErrInvalidProject
	}
	component, err := domain.NewComponent(projectID, name, description)
	if err != nil {
		return nil, err
	}
	if err := s.catalogRepo.CreateComponent(ctx, component); err != nil {
		return nil, err
	}
	return component, nil
}

func (s *ProjectService) DeleteComponent(ctx context.Context, projectID, componentID uuid.UUID) error {
	if s.catalogRepo == nil {
		return domain.ErrInvalidProject
	}
	return s.catalogRepo.DeleteComponent(ctx, projectID, componentID)
}
