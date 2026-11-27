package usecase

import (
	"context"
	"errors"
	"strings"

	"github.com/google/uuid"

	"github.com/jhonsferg/corelog/internal/issue/application/port"
	"github.com/jhonsferg/corelog/internal/issue/domain"
	"github.com/jhonsferg/corelog/internal/platform/apperror"
)

// operationDeps holds the out-ports of the bulk, clone, move and export operations. They are optional: an
// operation without its port answers with an internal error.
type operationDeps struct {
	moves        port.MoveRepository
	clones       port.CloneRepository
	labels       port.BulkLabelRepository
	exportLabels port.ExportLabelReader
	attachments  port.AttachmentRepository
	files        port.FileStorage
}

// WithOperations fixes the adapters of the bulk, clone, move and export operations. attachments and files are
// only needed to copy attachments on clone.
func (s *IssueService) WithOperations(repo port.OperationsRepository, attachments port.AttachmentRepository, files port.FileStorage) *IssueService {
	deps := &operationDeps{attachments: attachments, files: files}
	if repo != nil {
		deps.moves, deps.clones, deps.labels, deps.exportLabels = repo, repo, repo, repo
	}
	s.ops = deps
	return s
}

// errOperationsUnavailable is the answer of an operation whose adapter was not wired.
func errOperationsUnavailable() error {
	return apperror.New(apperror.KindInternal, "issue operations are unavailable")
}

var _ port.IssueOperations = (*IssueService)(nil)

// BulkChange applies one action to each referenced issue, in order. Every issue runs in its own transaction and
// with the same checks as the single-issue endpoints, so a forbidden issue does not stop the others.
func (s *IssueService) BulkChange(ctx context.Context, actorID uuid.UUID, globalRole string, in port.BulkInput) (port.BulkResult, error) {
	if err := domain.ValidateBulkRefs(in.Refs); err != nil {
		return port.BulkResult{}, err
	}
	if err := validateBulkInput(in); err != nil {
		return port.BulkResult{}, err
	}
	result := port.BulkResult{Succeeded: []*domain.Issue{}, Failed: []port.BulkFailure{}}
	for _, ref := range domain.DedupeRefs(in.Refs) {
		issue, err := s.applyBulkRef(ctx, ref, actorID, globalRole, in)
		if err != nil {
			result.Failed = append(result.Failed, bulkFailure(ref, err))
			continue
		}
		result.Succeeded = append(result.Succeeded, issue)
	}
	return result, nil
}

// validateBulkInput checks the parameters of the action. A malformed request fails before any issue changes.
func validateBulkInput(in port.BulkInput) error {
	switch in.Action {
	case domain.BulkAssign:
		if !in.Assignee.Set {
			return domain.ErrBulkParams
		}
	case domain.BulkSetPriority:
		if in.PriorityID == uuid.Nil {
			return domain.ErrBulkParams
		}
	case domain.BulkSetLabels:
		return domain.ValidateLabelChange(in.LabelsAdd, in.LabelsRemove)
	case domain.BulkSetField:
		if editing, assigning := patchTouches(in.Fields); !editing && !assigning {
			return domain.ErrBulkParams
		}
	case domain.BulkTransition:
		chosen := 0
		if in.TransitionID != "" {
			chosen++
		}
		if in.StatusID != uuid.Nil {
			chosen++
		}
		if in.Status != "" {
			chosen++
			if !in.Status.IsValid() {
				return domain.ErrInvalidStatus
			}
		}
		if chosen != 1 {
			return domain.ErrBulkParams
		}
	case domain.BulkMove:
		if in.Move.ProjectID == uuid.Nil {
			return domain.ErrBulkParams
		}
	case domain.BulkDelete:
	default:
		return domain.ErrBulkActionUnknown
	}
	return nil
}

// applyBulkRef resolves one reference and applies the action to the issue in its own transaction.
func (s *IssueService) applyBulkRef(ctx context.Context, ref string, actorID uuid.UUID, globalRole string, in port.BulkInput) (*domain.Issue, error) {
	id, err := s.resolveBulkRef(ctx, ref, actorID, globalRole)
	if err != nil {
		return nil, err
	}
	var issue *domain.Issue
	err = s.tx.InTx(ctx, func(ctx context.Context) error {
		var err error
		issue, err = s.applyBulkAction(ctx, id, actorID, globalRole, in)
		return err
	})
	if err != nil {
		return nil, err
	}
	return issue, nil
}

// resolveBulkRef turns a UUID or a key into an issue id. Keys are resolved among the issues the caller can see.
func (s *IssueService) resolveBulkRef(ctx context.Context, ref string, actorID uuid.UUID, globalRole string) (uuid.UUID, error) {
	if id, err := uuid.Parse(ref); err == nil {
		return id, nil
	}
	return s.ResolveKey(ctx, actorID, globalRole, uuid.Nil, ref)
}

func (s *IssueService) applyBulkAction(ctx context.Context, id, actorID uuid.UUID, globalRole string, in port.BulkInput) (*domain.Issue, error) {
	switch in.Action {
	case domain.BulkAssign:
		if in.Assignee.Value == nil {
			return s.bulkUnassign(ctx, id, actorID, globalRole)
		}
		return s.bulkAssign(ctx, id, actorID, globalRole, *in.Assignee.Value)
	case domain.BulkSetPriority:
		return s.bulkPatch(ctx, id, actorID, globalRole, port.PatchInput{PriorityID: port.Some(in.PriorityID)})
	case domain.BulkSetField:
		return s.bulkPatch(ctx, id, actorID, globalRole, in.Fields)
	case domain.BulkSetLabels:
		return s.bulkLabels(ctx, id, actorID, globalRole, in.LabelsAdd, in.LabelsRemove)
	case domain.BulkTransition:
		return s.bulkTransition(ctx, id, actorID, globalRole, in)
	case domain.BulkMove:
		result, err := s.MoveIssue(ctx, id, actorID, globalRole, in.Move)
		if err != nil {
			return nil, err
		}
		return result.Issue, nil
	case domain.BulkDelete:
		issue, err := s.repo.FindByID(ctx, id)
		if err != nil {
			return nil, err
		}
		if err := s.Delete(ctx, id, actorID, globalRole); err != nil {
			return nil, err
		}
		return issue, nil
	default:
		return nil, domain.ErrBulkActionUnknown
	}
}

// bulkPatch applies a partial update with the single-issue patch rules. Issues the caller cannot see look missing.
func (s *IssueService) bulkPatch(ctx context.Context, id, actorID uuid.UUID, globalRole string, in port.PatchInput) (*domain.Issue, error) {
	in.HideForbidden = true
	result, err := s.Patch(ctx, id, actorID, globalRole, in)
	if err != nil {
		return nil, err
	}
	return result.Issue, nil
}

// bulkAssign assigns like PATCH /issues/{id}/assign: the same permission, team rule, status change and history.
// The assignment and its event are written in the transaction of the issue.
func (s *IssueService) bulkAssign(ctx context.Context, id, actorID uuid.UUID, globalRole string, assigneeID uuid.UUID) (*domain.Issue, error) {
	before, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	issue, err := s.Assign(ctx, id, actorID, assigneeID, globalRole)
	if err != nil {
		return nil, err
	}
	if err := s.commitAssigneeChange(ctx, issue, actorID, before.AssigneeID); err != nil {
		return nil, err
	}
	return issue, nil
}

// bulkUnassign removes the assignee like PATCH /issues/{id}/assign with a null assignee.
func (s *IssueService) bulkUnassign(ctx context.Context, id, actorID uuid.UUID, globalRole string) (*domain.Issue, error) {
	before, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	issue, err := s.Unassign(ctx, id, actorID, globalRole)
	if err != nil {
		return nil, err
	}
	if err := s.commitAssigneeChange(ctx, issue, actorID, before.AssigneeID); err != nil {
		return nil, err
	}
	return issue, nil
}

// commitAssigneeChange emits the issue.updated event of an assignee change. Nothing is emitted when the
// assignee did not change; the history entry itself is written by Assign and Unassign.
func (s *IssueService) commitAssigneeChange(ctx context.Context, issue *domain.Issue, actorID uuid.UUID, previous *uuid.UUID) error {
	if sameAssignee(previous, issue.AssigneeID) {
		return nil
	}
	change := domain.NewChange("assignee", idString(previous), idString(issue.AssigneeID))
	return s.publishUpdated(ctx, issue, actorID, uuid.New(), []domain.Change{change})
}

// bulkLabels adds and removes labels of one issue. Added labels must belong to the issue's project.
func (s *IssueService) bulkLabels(ctx context.Context, id, actorID uuid.UUID, globalRole string, add, remove []uuid.UUID) (*domain.Issue, error) {
	if s.ops == nil || s.ops.labels == nil {
		return nil, errOperationsUnavailable()
	}
	if s.lifecycle != nil {
		if err := s.lifecycle.LockForUpdate(ctx, id); err != nil {
			return nil, err
		}
	}
	issue, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if err := s.requireVisible(ctx, issue.ProjectID, actorID, globalRole); err != nil {
		return nil, err
	}
	if err := s.requireFieldEdit(ctx, issue, actorID, globalRole); err != nil {
		return nil, err
	}
	valid, err := s.ops.labels.ProjectLabelIDs(ctx, issue.ProjectID, add)
	if err != nil {
		return nil, err
	}
	for _, labelID := range add {
		if _, ok := valid[labelID]; !ok {
			return nil, domain.ErrLabelNotInProject
		}
	}
	before, err := s.ops.labels.IssueLabelNames(ctx, id)
	if err != nil {
		return nil, err
	}
	if len(add) > 0 {
		if err := s.ops.labels.AddIssueLabels(ctx, id, add); err != nil {
			return nil, err
		}
	}
	if len(remove) > 0 {
		if err := s.ops.labels.RemoveIssueLabels(ctx, id, remove); err != nil {
			return nil, err
		}
	}
	after, err := s.ops.labels.IssueLabelNames(ctx, id)
	if err != nil {
		return nil, err
	}
	from, to := strings.Join(before, ", "), strings.Join(after, ", ")
	if from == to {
		return issue, nil
	}
	if err := s.commitChanges(ctx, issue, actorID, []domain.Change{domain.NewChange("labels", from, to)}); err != nil {
		return nil, err
	}
	return issue, nil
}

// commitChanges writes one activity batch and one issue.updated event for changes that were already applied.
func (s *IssueService) commitChanges(ctx context.Context, issue *domain.Issue, actorID uuid.UUID, changes []domain.Change) error {
	if len(changes) == 0 {
		return nil
	}
	batchID := uuid.New()
	if err := s.recordBatch(ctx, issue, actorID, batchID, changes); err != nil {
		return err
	}
	return s.publishUpdated(ctx, issue, actorID, batchID, changes)
}

// bulkTransition runs a workflow transition on one issue through the single-issue transition use case. The
// transition is chosen by id, by target status or by the legacy status the target mirrors; a status choice
// takes the first transition the caller may run that leads there.
func (s *IssueService) bulkTransition(ctx context.Context, id, actorID uuid.UUID, globalRole string, in port.BulkInput) (*domain.Issue, error) {
	transitionID := in.TransitionID
	if transitionID == "" {
		issue, err := s.repo.FindByID(ctx, id)
		if err != nil {
			return nil, err
		}
		if err := s.requireVisible(ctx, issue.ProjectID, actorID, globalRole); err != nil {
			return nil, err
		}
		options, err := s.transitionOptions(ctx, issue, actorID, globalRole)
		if err != nil {
			return nil, err
		}
		for _, option := range options {
			if (in.StatusID != uuid.Nil && option.ToStatusID == in.StatusID) ||
				(in.Status != "" && domain.LegacyStatusOfCategory(option.ToCategory) == in.Status) {
				transitionID = option.ID
				break
			}
		}
		if transitionID == "" {
			return nil, domain.ErrTransitionUnavailable
		}
	}
	result, err := s.TransitionIssue(ctx, port.TransitionRequest{IssueID: id, ActorID: actorID, GlobalRole: globalRole, TransitionID: transitionID})
	if err != nil {
		return nil, err
	}
	return result.Issue, nil
}

// bulkFailure describes why one reference could not be changed. Internal causes do not leave the server.
func bulkFailure(ref string, err error) port.BulkFailure {
	var appErr *apperror.Error
	if !errors.As(err, &appErr) || appErr.Kind == apperror.KindInternal {
		return port.BulkFailure{Ref: ref, Code: "issue.internal_error", Message: "no se pudo aplicar el cambio a la incidencia"}
	}
	code := appErr.Code
	if code == "" {
		code = fallbackCode(appErr.Kind)
	}
	return port.BulkFailure{Ref: ref, Code: code, Message: appErr.Message}
}

// fallbackCode names the failure of an error that carries no code of its own.
func fallbackCode(kind apperror.Kind) string {
	switch kind {
	case apperror.KindForbidden:
		return "issue.forbidden"
	case apperror.KindNotFound:
		return "issue.not_found"
	case apperror.KindConflict:
		return "issue.conflict"
	case apperror.KindValidation:
		return "issue.invalid"
	default:
		return "issue.operation_failed"
	}
}
