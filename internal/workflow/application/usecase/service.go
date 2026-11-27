// Package usecase holds the workflow use cases: loading the workflow of a project, evaluating transitions
// and managing the status catalog of an organization.
package usecase

import (
	"context"
	"strings"
	"unicode/utf8"

	"github.com/google/uuid"

	"github.com/jhonsferg/corelog/internal/platform/clock"
	"github.com/jhonsferg/corelog/internal/workflow/application/port"
	"github.com/jhonsferg/corelog/internal/workflow/domain"
)

// maxStatusNameLength bounds the name of an organization status.
const maxStatusNameLength = 60

// Service is the workflow application service.
type Service struct {
	repo  port.Repository
	clock clock.Clock
}

// NewService builds the service. A nil clock means the system clock.
func NewService(repo port.Repository, c clock.Clock) *Service {
	if c == nil {
		c = clock.RealClock{}
	}
	return &Service{repo: repo, clock: c}
}

// GetWorkflow returns the workflow of a project and its version.
func (s *Service) GetWorkflow(ctx context.Context, projectID uuid.UUID) (domain.Workflow, int, error) {
	record, err := s.repo.FindWorkflow(ctx, projectID)
	if err != nil {
		return domain.Workflow{}, 0, err
	}
	return record.Workflow, record.Version, nil
}

// Available lists, in workflow order, the transitions the actor may run on the issue.
func (s *Service) Available(ctx context.Context, projectID uuid.UUID, issue domain.IssueSnapshot, actor domain.ActorContext) ([]domain.AvailableTransition, error) {
	w, _, err := s.GetWorkflow(ctx, projectID)
	if err != nil {
		return nil, err
	}
	return domain.AvailableTransitions(w, issue, actor), nil
}

// Plan evaluates a transition and returns the changes it plans. It never writes.
func (s *Service) Plan(ctx context.Context, projectID uuid.UUID, issue domain.IssueSnapshot, actor domain.ActorContext, transitionID string, input domain.TransitionInput) (domain.Result, string, error) {
	w, _, err := s.GetWorkflow(ctx, projectID)
	if err != nil {
		return domain.Result{}, "", err
	}
	plan, err := domain.Evaluate(w, issue, actor, transitionID, input)
	if err != nil {
		return domain.Result{}, "", err
	}
	name := ""
	if t, ok := w.Transition(transitionID); ok {
		name = t.Name
	}
	return plan, name, nil
}

// StatusOf returns the status of the workflow with the given id.
func (s *Service) StatusOf(ctx context.Context, projectID uuid.UUID, statusID string) (domain.Status, error) {
	w, _, err := s.GetWorkflow(ctx, projectID)
	if err != nil {
		return domain.Status{}, err
	}
	status, ok := w.Status(statusID)
	if !ok {
		return domain.Status{}, port.ErrStatusNotFound
	}
	return status, nil
}

// ColumnStatusIDs returns the statuses a board column shows.
func (s *Service) ColumnStatusIDs(ctx context.Context, columnID uuid.UUID) ([]uuid.UUID, error) {
	return s.repo.ColumnStatusIDs(ctx, columnID)
}

// ReplaceWorkflow validates and stores a workflow when the stored version is expectedVersion.
func (s *Service) ReplaceWorkflow(ctx context.Context, projectID uuid.UUID, w domain.Workflow, expectedVersion int) (int, error) {
	if err := w.Check(); err != nil {
		return 0, err
	}
	return s.repo.SaveWorkflow(ctx, projectID, w, expectedVersion, s.clock.Now().UTC())
}

// ListStatuses lists the status catalog of an organization.
func (s *Service) ListStatuses(ctx context.Context, orgID uuid.UUID) ([]port.OrgStatus, error) {
	return s.repo.ListStatuses(ctx, orgID)
}

func validateStatus(name string, category domain.Category) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" || utf8.RuneCountInString(name) > maxStatusNameLength {
		return "", port.ErrStatusNameInvalid
	}
	if !category.IsValid() {
		return "", port.ErrStatusCategory
	}
	return name, nil
}

// CreateStatus adds a status to the catalog of an organization.
func (s *Service) CreateStatus(ctx context.Context, orgID uuid.UUID, name string, category domain.Category) (port.OrgStatus, error) {
	name, err := validateStatus(name, category)
	if err != nil {
		return port.OrgStatus{}, err
	}
	return s.repo.CreateStatus(ctx, port.OrgStatus{OrganizationID: orgID, Name: name, Category: category})
}

// UpdateStatus renames a status or changes its category.
func (s *Service) UpdateStatus(ctx context.Context, status port.OrgStatus) error {
	name, err := validateStatus(status.Name, status.Category)
	if err != nil {
		return err
	}
	status.Name = name
	return s.repo.UpdateStatus(ctx, status)
}

// DeleteStatus removes an unused status from the catalog.
func (s *Service) DeleteStatus(ctx context.Context, orgID, id uuid.UUID) error {
	return s.repo.DeleteStatus(ctx, orgID, id)
}
