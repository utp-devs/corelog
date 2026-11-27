package usecase

import (
	"context"

	"github.com/google/uuid"

	"github.com/jhonsferg/corelog/internal/issue/application/port"
	"github.com/jhonsferg/corelog/internal/issue/domain"
)

// ListChildren returns the live direct children of an issue with the done count. Children live in the parent
// project, so the caller's browse permission on the parent covers them.
func (s *IssueService) ListChildren(ctx context.Context, issueID, actorID uuid.UUID, globalRole string) (port.ChildrenView, error) {
	issue, err := s.repo.FindByID(ctx, issueID)
	if err != nil {
		return port.ChildrenView{}, err
	}
	if err := s.requireVisible(ctx, issue.ProjectID, actorID, globalRole); err != nil {
		return port.ChildrenView{}, err
	}
	view := port.ChildrenView{Children: []*domain.Issue{}, AssigneeNames: map[uuid.UUID]string{}}
	if s.children == nil {
		return view, nil
	}
	children, err := s.children.ListChildren(ctx, issueID)
	if err != nil {
		return port.ChildrenView{}, err
	}
	view.Children = children
	view.Progress = domain.ComputeChildProgress(children)

	var assignees []uuid.UUID
	seen := map[uuid.UUID]bool{}
	for _, child := range children {
		if child.AssigneeID != nil && !seen[*child.AssigneeID] {
			seen[*child.AssigneeID] = true
			assignees = append(assignees, *child.AssigneeID)
		}
	}
	if len(assignees) > 0 && s.directory != nil {
		names, err := s.directory.NamesOf(ctx, assignees)
		if err != nil {
			return port.ChildrenView{}, err
		}
		view.AssigneeNames = names
	}
	return view, nil
}
