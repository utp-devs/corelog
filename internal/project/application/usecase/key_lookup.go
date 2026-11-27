package usecase

import (
	"context"

	"github.com/google/uuid"

	"github.com/jhonsferg/corelog/internal/project/domain"
)

// FindByKey resolves a project key among the projects the actor can see. With orgID the key is
// looked up in that organization only. Without it, the candidates are the live projects with the key
// in organizations the actor can reach; one visible candidate is returned, none is not found, and
// several are ambiguous.
func (s *ProjectService) FindByKey(ctx context.Context, actorID uuid.UUID, globalRole string, orgID uuid.UUID, key string) (*domain.Project, error) {
	normalized, err := domain.ValidateKey(key)
	if err != nil {
		return nil, err
	}
	if orgID != uuid.Nil {
		project, err := s.repo.FindByKey(ctx, orgID, normalized)
		if err != nil {
			return nil, err
		}
		visible, err := s.visibleTo(ctx, project, actorID, globalRole)
		if err != nil {
			return nil, err
		}
		if !visible {
			return nil, domain.ErrProjectNotFound
		}
		return project, nil
	}

	candidates, err := s.repo.FindKeyCandidates(ctx, normalized, actorID, globalRole == platformAdmin)
	if err != nil {
		return nil, err
	}
	var visible []*domain.Project
	for _, candidate := range candidates {
		ok, err := s.visibleTo(ctx, candidate, actorID, globalRole)
		if err != nil {
			return nil, err
		}
		if ok {
			visible = append(visible, candidate)
		}
	}
	switch len(visible) {
	case 0:
		return nil, domain.ErrProjectNotFound
	case 1:
		return visible[0], nil
	default:
		return nil, domain.ErrProjectKeyAmbiguous
	}
}

// visibleTo reports whether the actor may see the project, using the same rules as loadVisible.
func (s *ProjectService) visibleTo(ctx context.Context, project *domain.Project, actorID uuid.UUID, globalRole string) (bool, error) {
	orgRole, err := s.orgRoleOf(ctx, project.OrganizationID, actorID)
	if err != nil {
		return false, err
	}
	return s.canView(ctx, project, actorID, globalRole, orgRole)
}
