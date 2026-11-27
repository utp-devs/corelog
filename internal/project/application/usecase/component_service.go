package usecase

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/jhonsferg/corelog/internal/platform/apperror"
	"github.com/jhonsferg/corelog/internal/project/application/port"
	"github.com/jhonsferg/corelog/internal/project/domain"
)

// CreateComponent adds a component. Managing components needs manage_components. A lead must be a member
// of the project.
func (s *ProjectService) CreateComponent(ctx context.Context, actorID uuid.UUID, globalRole string, projectID uuid.UUID, in domain.ComponentInput) (*domain.Component, error) {
	project, orgRole, err := s.loadVisible(ctx, actorID, globalRole, projectID)
	if err != nil {
		return nil, err
	}
	if err := s.requireManage(ctx, port.ActionManageComponents, project, actorID, globalRole, orgRole); err != nil {
		return nil, err
	}
	if s.catalogRepo == nil {
		return nil, domain.ErrInvalidProject
	}
	component, err := domain.NewComponent(projectID, in.Name, in.Description)
	if err != nil {
		return nil, err
	}
	policy := in.Policy
	if policy == "" {
		policy = domain.AssigneeNone
	}
	if err := component.Update(domain.ComponentPatch{LeadIDSet: true, LeadID: in.LeadID, Policy: &policy}, s.now()); err != nil {
		return nil, err
	}
	if err := s.checkLead(ctx, projectID, component.LeadID); err != nil {
		return nil, err
	}
	if err := s.catalogRepo.CreateComponent(ctx, component); err != nil {
		return nil, err
	}
	return component, nil
}

// UpdateComponent changes a component. The lead, when changed, must be a member of the project.
func (s *ProjectService) UpdateComponent(ctx context.Context, actorID uuid.UUID, globalRole string, projectID, componentID uuid.UUID, patch domain.ComponentPatch) (*domain.Component, error) {
	project, orgRole, err := s.loadVisible(ctx, actorID, globalRole, projectID)
	if err != nil {
		return nil, err
	}
	if err := s.requireManage(ctx, port.ActionManageComponents, project, actorID, globalRole, orgRole); err != nil {
		return nil, err
	}
	if s.catalogRepo == nil {
		return nil, domain.ErrInvalidProject
	}
	component, err := s.catalogRepo.FindComponent(ctx, projectID, componentID)
	if err != nil {
		return nil, err
	}
	if patch.LeadIDSet {
		if err := s.checkLead(ctx, projectID, patch.LeadID); err != nil {
			return nil, err
		}
	}
	if err := component.Update(patch, s.now()); err != nil {
		return nil, err
	}
	if err := s.catalogRepo.UpdateComponent(ctx, component); err != nil {
		return nil, err
	}
	return s.catalogRepo.FindComponent(ctx, projectID, componentID)
}

// DeleteComponent removes a component. With moveTo the issues of the component are linked to the target first,
// both in one transaction.
func (s *ProjectService) DeleteComponent(ctx context.Context, actorID uuid.UUID, globalRole string, projectID, componentID uuid.UUID, moveTo *uuid.UUID) error {
	project, orgRole, err := s.loadVisible(ctx, actorID, globalRole, projectID)
	if err != nil {
		return err
	}
	if err := s.requireManage(ctx, port.ActionManageComponents, project, actorID, globalRole, orgRole); err != nil {
		return err
	}
	if s.catalogRepo == nil {
		return domain.ErrInvalidProject
	}
	return s.inTx(ctx, func(ctx context.Context) error {
		if _, err := s.catalogRepo.FindComponent(ctx, projectID, componentID); err != nil {
			return err
		}
		if moveTo != nil {
			if *moveTo == componentID {
				return domain.ErrComponentMoveTarget
			}
			if _, err := s.catalogRepo.FindComponent(ctx, projectID, *moveTo); err != nil {
				if apperror.CodeOf(err) == domain.ErrComponentNotFound.Code {
					return domain.ErrComponentMoveTarget
				}
				return err
			}
			if err := s.catalogRepo.MoveComponentIssues(ctx, componentID, *moveTo); err != nil {
				return err
			}
		}
		return s.catalogRepo.DeleteComponent(ctx, projectID, componentID)
	})
}

// checkLead accepts a nil lead (no lead) and otherwise requires an active project membership.
func (s *ProjectService) checkLead(ctx context.Context, projectID uuid.UUID, leadID *uuid.UUID) error {
	if leadID == nil {
		return nil
	}
	role, err := s.memberRepo.FindRole(ctx, projectID, *leadID)
	if err != nil {
		return err
	}
	if role == nil {
		return domain.ErrComponentLead
	}
	return nil
}

// now is the clock of the component writes. Components keep the timestamps the domain sets.
func (s *ProjectService) now() time.Time {
	if s.clock == nil {
		return time.Now().UTC()
	}
	return s.clock.Now()
}
