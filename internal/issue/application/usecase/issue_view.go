package usecase

import (
	"context"

	"github.com/google/uuid"

	"github.com/jhonsferg/corelog/internal/issue/application/port"
	"github.com/jhonsferg/corelog/internal/issue/domain"
	"github.com/jhonsferg/corelog/internal/platform/apperror"
	"github.com/jhonsferg/corelog/internal/platform/validation"
)

var _ port.TransitionProvider = (*IssueService)(nil)

// issuePermissionNames are the permissions the view model reports, in display order.
var issuePermissionNames = []string{
	"edit_issue", "assign_issue", "transition_issue", "comment_issue", "delete_issue",
	"add_attachment", "link_issue", "watch_issue", "vote_issue", "log_work",
}

// legacyStatusNames are the Spanish names of the legacy statuses.
var legacyStatusNames = map[domain.Status]string{
	domain.StatusOpen:       "Abierta",
	domain.StatusInProgress: "En progreso",
	domain.StatusResolved:   "Resuelta",
	domain.StatusClosed:     "Cerrada",
}

// GetView returns the issue with the permissions of the caller, the transitions it can run, the collection
// counts and the breadcrumb. It costs a fixed number of statements however many comments or links exist.
func (s *IssueService) GetView(ctx context.Context, id, actorID uuid.UUID, globalRole string) (*port.IssueView, error) {
	issue, err := s.Get(ctx, id, actorID, globalRole)
	if err != nil {
		return nil, err
	}
	view := &port.IssueView{Issue: issue, Permissions: []string{}, Transitions: []port.Transition{}}
	if view.Permissions, err = s.effectivePermissions(ctx, issue, actorID, globalRole); err != nil {
		return nil, err
	}
	provider := s.transitions
	if provider == nil {
		provider = s
	}
	if view.Transitions, err = provider.AvailableTransitions(ctx, issue, actorID, globalRole); err != nil {
		return nil, err
	}
	if s.viewReader != nil {
		if view.Counts, view.Breadcrumb, err = s.viewReader.ViewExtras(ctx, issue.ID); err != nil {
			return nil, err
		}
	} else {
		view.Breadcrumb = domain.Breadcrumb{ProjectID: issue.ProjectID, ProjectKey: issue.ProjectKey}
		view.Counts.Votes = issue.VoteCount
	}
	if s.votes != nil {
		if view.ViewerHasVoted, err = s.votes.HasVoted(ctx, issue.ID, actorID); err != nil {
			return nil, err
		}
	}
	return view, nil
}

// effectivePermissions lists the issue permissions of the caller. Special holders (reporter, assignee) add the
// permissions the scheme grants to them; edit_own_issue of the reporter counts as edit_issue.
func (s *IssueService) effectivePermissions(ctx context.Context, issue *domain.Issue, actorID uuid.UUID, globalRole string) ([]string, error) {
	if s.perms == nil {
		return []string{}, nil
	}
	base, err := s.perms.Effective(ctx, actorID, globalRole, issue.ProjectID)
	if err != nil {
		return nil, err
	}
	held := make(map[string]bool, len(base))
	for _, name := range base {
		held[name] = true
	}
	if holders := holdersFor(issue, actorID); len(holders) > 0 {
		candidates := append([]string{permEditOwnIssue}, issuePermissionNames...)
		for _, name := range candidates {
			if held[name] {
				continue
			}
			ok, err := s.perms.Can(ctx, actorID, globalRole, issue.ProjectID, name, holders...)
			if err != nil {
				return nil, err
			}
			if ok {
				held[name] = true
			}
		}
	}
	if held[permEditOwnIssue] && issue.ReporterID == actorID {
		held[permEditIssue] = true
	}
	out := make([]string, 0, len(issuePermissionNames))
	for _, name := range issuePermissionNames {
		if held[name] {
			out = append(out, name)
		}
	}
	return out, nil
}

// AvailableTransitions is the legacy provider: the statuses the fixed status rules and the project workflow
// allow for the caller. A caller who cannot edit the issue has none.
func (s *IssueService) AvailableTransitions(ctx context.Context, issue *domain.Issue, actorID uuid.UUID, globalRole string) ([]port.Transition, error) {
	out := []port.Transition{}
	if err := s.requireFieldEdit(ctx, issue, actorID, globalRole); err != nil {
		if apperror.KindOf(err) == apperror.KindInternal {
			return nil, err
		}
		return out, nil
	}
	var workflow *port.Workflow
	role := ""
	if s.workflows != nil {
		var err error
		if role, err = s.transitionRole(ctx, issue.ProjectID, actorID, globalRole); err != nil {
			return nil, err
		}
		if workflow, err = s.workflows.DefaultWorkflow(ctx, issue.ProjectID); err != nil {
			return nil, err
		}
	}
	for _, next := range []domain.Status{domain.StatusOpen, domain.StatusInProgress, domain.StatusResolved, domain.StatusClosed} {
		if !issue.Status.CanTransitionTo(next) {
			continue
		}
		if s.workflows != nil && !workflowAllows(workflow, issue.Status, next, role) {
			continue
		}
		out = append(out, port.Transition{ID: string(next), Name: legacyStatusNames[next], To: next})
	}
	return out, nil
}

func workflowAllows(workflow *port.Workflow, from, to domain.Status, role string) bool {
	if workflow == nil || role == "" {
		return false
	}
	for _, transition := range workflow.Transitions {
		if transition.From != from || transition.To != to {
			continue
		}
		for _, allowed := range transition.Roles {
			if allowed == role {
				return true
			}
		}
	}
	return false
}

// finishCreate validates the fields beyond the legacy create input, applies the project defaults and writes the
// issue with its watchers and event in one transaction.
func (s *IssueService) finishCreate(ctx context.Context, issue *domain.Issue, in port.CreateInput) (*domain.Issue, error) {
	if err := domain.ValidateStoryPoints(in.StoryPoints); err != nil {
		return nil, err
	}
	if err := domain.ValidateEstimate(in.OriginalEstimateSeconds); err != nil {
		return nil, err
	}
	if err := domain.ValidateEstimate(in.RemainingEstimateSeconds); err != nil {
		return nil, err
	}
	issue.OriginalEstimateSeconds = in.OriginalEstimateSeconds
	issue.RemainingEstimateSeconds = in.RemainingEstimateSeconds
	issue.StartDate = dateValue(in.StartDate)
	issue.DueDate = dateValue(in.DueDate)
	if err := domain.ValidateDates(issue.StartDate, issue.DueDate); err != nil {
		return nil, err
	}
	if err := s.checkRequiredOnCreate(ctx, issue, in); err != nil {
		return nil, err
	}
	issue.UpdatedAt = s.now()
	issue.CreatedAt = issue.UpdatedAt

	err := s.tx.InTx(ctx, func(ctx context.Context) error {
		if err := s.checkHierarchy(ctx, issue, true, true); err != nil {
			return err
		}
		assignee, err := s.createAssignee(ctx, issue, in)
		if err != nil {
			return err
		}
		issue.AssigneeID = assignee
		if s.lifecycle != nil {
			rank, err := s.lifecycle.NextRank(ctx, issue.ProjectID)
			if err != nil {
				return err
			}
			issue.Rank = rank
		}
		if err := s.repo.Save(ctx, issue); err != nil {
			return err
		}
		if s.watchers != nil {
			if err := s.watchers.Add(ctx, issue.ID, issue.ReporterID); err != nil {
				return err
			}
			if issue.AssigneeID != nil && *issue.AssigneeID != issue.ReporterID {
				if err := s.watchers.Add(ctx, issue.ID, *issue.AssigneeID); err != nil {
					return err
				}
			}
		}
		return s.publishLifecycle(ctx, issue, issue.ReporterID, "issue.created")
	})
	if err != nil {
		return nil, err
	}
	if issue.AssigneeID != nil {
		s.notifyAssignment(ctx, issue, issue.ReporterID, *issue.AssigneeID)
	}
	return issue, nil
}

// createAssignee resolves the assignee of a new issue: the one the caller chose (which needs assign_issue) or
// the project policy default.
func (s *IssueService) createAssignee(ctx context.Context, issue *domain.Issue, in port.CreateInput) (*uuid.UUID, error) {
	if in.AssigneeID != nil {
		if s.perms != nil {
			if err := s.requireAssign(ctx, issue, issue.ReporterID, in.GlobalRole); err != nil {
				return nil, err
			}
		}
		return in.AssigneeID, nil
	}
	if s.defaults == nil {
		return nil, nil
	}
	policy, lead, err := s.defaults.DefaultAssigneePolicy(ctx, issue.ProjectID)
	if err != nil {
		return nil, err
	}
	if policy == "lead" {
		return lead, nil
	}
	return nil, nil
}

// checkRequiredOnCreate rejects a create that misses a system field the project layout requires. It only runs
// when the request says which fields it sent.
func (s *IssueService) checkRequiredOnCreate(ctx context.Context, issue *domain.Issue, in port.CreateInput) error {
	if s.requiredFields == nil || in.Provided == nil {
		return nil
	}
	required, err := s.requiredFields.RequiredSystemFields(ctx, issue.ProjectID, issue.IssueTypeID)
	if err != nil {
		return err
	}
	var problems []apperror.FieldError
	for _, key := range required {
		if key == "summary" || key == "labels" || in.Provided[key] {
			continue
		}
		problems = append(problems, apperror.FieldError{Field: key, Message: "es obligatorio"})
	}
	if len(problems) == 0 {
		return nil
	}
	return apperror.WithCode(apperror.KindValidation, validation.CodeFailed, "faltan campos obligatorios").WithDetails(problems...)
}
