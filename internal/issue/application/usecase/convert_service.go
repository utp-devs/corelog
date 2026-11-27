package usecase

import (
	"context"

	"github.com/google/uuid"

	"github.com/jhonsferg/corelog/internal/issue/application/port"
	"github.com/jhonsferg/corelog/internal/issue/domain"
)

// ConvertIssue changes the type of an issue. It is a patch of the type and the parent, so the hierarchy rules,
// the custom field rules and the history are the ones of PATCH /issues/{id}. Before the patch, two cases that
// need an explicit parent answer with a clear code: a subtask needs a parent, and a parent that stays must be
// an epic when the target is not a subtask.
func (s *IssueService) ConvertIssue(ctx context.Context, id, actorID uuid.UUID, globalRole string, in port.ConvertInput) (*port.PatchResult, error) {
	if in.IssueTypeID == uuid.Nil {
		return nil, domain.ErrInvalidIssueType
	}
	if s.catalog != nil && !in.ParentID.Set {
		if err := s.checkConvertParent(ctx, id, actorID, globalRole, in.IssueTypeID); err != nil {
			return nil, err
		}
	}
	return s.Patch(ctx, id, actorID, globalRole, port.PatchInput{
		IssueTypeID:   port.Some(in.IssueTypeID),
		ParentID:      in.ParentID,
		HideForbidden: true,
	})
}

// checkConvertParent answers the conversions that need the caller to name the parent: a subtask target without a
// parent, and a non-subtask target with a parent that is not an epic.
func (s *IssueService) checkConvertParent(ctx context.Context, id, actorID uuid.UUID, globalRole string, typeID uuid.UUID) error {
	issue, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return err
	}
	if err := s.requireVisible(ctx, issue.ProjectID, actorID, globalRole); err != nil {
		return err
	}
	target, err := s.catalog.Select(ctx, issue.ProjectID, domain.CatalogIssueType, port.CatalogSelector{ID: typeID})
	if err != nil {
		return err
	}
	if target.HierarchyLevel == domain.HierarchySubtask {
		if issue.ParentID == nil {
			return domain.ErrSubtaskNeedsParent
		}
		return nil
	}
	if issue.ParentID == nil {
		return nil
	}
	parent, err := s.repo.FindByID(ctx, *issue.ParentID)
	if err != nil {
		return err
	}
	if parent.HierarchyLevel() != domain.HierarchyEpic {
		return domain.ErrConvertParentRequired
	}
	return nil
}
