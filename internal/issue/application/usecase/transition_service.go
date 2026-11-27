package usecase

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/jhonsferg/corelog/internal/issue/application/port"
	"github.com/jhonsferg/corelog/internal/issue/domain"
	"github.com/jhonsferg/corelog/internal/platform/apperror"
	"github.com/jhonsferg/corelog/internal/platform/events"
)

var _ port.TransitionService = (*IssueService)(nil)
var _ port.TransitionProvider = (*IssueService)(nil)

// permTransitionIssue is the permission that lets a caller run workflow transitions.
const permTransitionIssue = "transition_issue"

// customFieldKeyPrefix starts the workflow field name of a custom field: "cf_" and the 32 hex digits of its id.
const customFieldKeyPrefix = "cf_"

// WithWorkflowEngine fixes the workflow engine and the persistence of transitions. Without them the issue
// view lists no transitions and every transition fails.
func (s *IssueService) WithWorkflowEngine(engine port.WorkflowEngine, repo port.TransitionRepository) *IssueService {
	s.engine = engine
	s.transitionRepo = repo
	return s
}

// CustomFieldKey returns the workflow field name of a custom field id.
func CustomFieldKey(id uuid.UUID) string {
	return customFieldKeyPrefix + hex.EncodeToString(id[:])
}

// requireTransition checks transition_issue (the assignee holder counts), or edit_issue without a permission
// checker.
func (s *IssueService) requireTransition(ctx context.Context, issue *domain.Issue, actorID uuid.UUID, globalRole string) error {
	if s.perms == nil {
		return s.requireIssueEdit(ctx, issue.ProjectID, actorID, globalRole)
	}
	ok, err := s.perms.Can(ctx, actorID, globalRole, issue.ProjectID, permTransitionIssue, holdersFor(issue, actorID)...)
	if err != nil {
		return err
	}
	if ok {
		return nil
	}
	return s.deniedOrArchived(ctx, permTransitionIssue, issue.ProjectID, actorID, globalRole)
}

// engineRole is the project role the workflow conditions see. A platform administrator counts as lead.
func (s *IssueService) engineRole(ctx context.Context, projectID, actorID uuid.UUID, globalRole string) (string, error) {
	role, err := s.transitionRole(ctx, projectID, actorID, globalRole)
	if err != nil {
		return "", err
	}
	if role == "admin" {
		return "lead", nil
	}
	return role, nil
}

// transitionContext builds the actor and the issue snapshot the engine evaluates.
func (s *IssueService) transitionContext(ctx context.Context, issue *domain.Issue, actorID uuid.UUID, globalRole string) (port.TransitionActor, port.TransitionIssue, error) {
	role, err := s.engineRole(ctx, issue.ProjectID, actorID, globalRole)
	if err != nil {
		return port.TransitionActor{}, port.TransitionIssue{}, err
	}
	actor := port.TransitionActor{UserID: actorID, Role: role}
	if s.defaults != nil {
		if _, lead, err := s.defaults.DefaultAssigneePolicy(ctx, issue.ProjectID); err != nil {
			return port.TransitionActor{}, port.TransitionIssue{}, err
		} else if lead != nil {
			actor.LeadID = *lead
		}
	}
	if actor.Permissions, err = s.effectivePermissions(ctx, issue, actorID, globalRole); err != nil {
		return port.TransitionActor{}, port.TransitionIssue{}, err
	}
	snapshot := port.TransitionIssue{StatusID: issue.StatusID, ReporterID: issue.ReporterID, Fields: transitionFields(issue)}
	if issue.AssigneeID != nil {
		snapshot.AssigneeID = *issue.AssigneeID
	}
	if issue.ResolutionID != nil {
		snapshot.ResolutionID = *issue.ResolutionID
	}
	if s.transitionRepo != nil {
		if snapshot.OpenSubtasks, err = s.transitionRepo.OpenSubtasks(ctx, issue.ID); err != nil {
			return port.TransitionActor{}, port.TransitionIssue{}, err
		}
	}
	return actor, snapshot, nil
}

// transitionFields lists the field values the engine can test, by workflow field name.
func transitionFields(issue *domain.Issue) map[string]string {
	fields := map[string]string{}
	if issue.StoryPoints != nil {
		fields["story_points"] = strconv.FormatFloat(*issue.StoryPoints, 'f', -1, 64)
	}
	if issue.DueDate != nil {
		fields["due_date"] = domain.FormatDate(issue.DueDate)
	}
	if issue.StartDate != nil {
		fields["start_date"] = domain.FormatDate(issue.StartDate)
	}
	if issue.Flagged {
		fields["flagged"] = "true"
	}
	for key, value := range issue.Custom {
		id, err := uuid.Parse(key)
		if err != nil || value == nil {
			continue
		}
		fields[CustomFieldKey(id)] = customValueText(value)
	}
	return fields
}

// customValueText renders a normalized custom value as text.
func customValueText(value any) string {
	switch typed := value.(type) {
	case string:
		return typed
	case []string:
		return strings.Join(typed, ",")
	case []any:
		parts := make([]string, 0, len(typed))
		for _, item := range typed {
			parts = append(parts, fmt.Sprint(item))
		}
		return strings.Join(parts, ",")
	case float64:
		return strconv.FormatFloat(typed, 'f', -1, 64)
	default:
		return fmt.Sprint(typed)
	}
}

// AvailableTransitions implements the TransitionProvider of the issue view with the workflow engine. A caller
// who cannot run transitions on the issue has none.
func (s *IssueService) AvailableTransitions(ctx context.Context, issue *domain.Issue, actorID uuid.UUID, globalRole string) ([]port.Transition, error) {
	out := []port.Transition{}
	options, err := s.transitionOptions(ctx, issue, actorID, globalRole)
	if err != nil {
		return nil, err
	}
	for _, option := range options {
		out = append(out, port.Transition{
			ID: option.ID, Name: option.Name, To: domain.LegacyStatusOfCategory(option.ToCategory),
			ToStatusID: option.ToStatusID, ToStatusName: option.ToStatusName, ToCategory: option.ToCategory,
			RequiredFields: option.RequiredFields, RequiresComment: option.RequiresComment,
			RequiresResolution: option.RequiresResolution, RequiresAssignee: option.RequiresAssignee,
			NeedsScreen: option.NeedsScreen(),
		})
	}
	return out, nil
}

// transitionOptions lists the transitions the caller may run on the issue; none when the caller lacks the
// permission or the engine is not wired.
func (s *IssueService) transitionOptions(ctx context.Context, issue *domain.Issue, actorID uuid.UUID, globalRole string) ([]port.TransitionOption, error) {
	if s.engine == nil {
		return nil, nil
	}
	if err := s.requireTransition(ctx, issue, actorID, globalRole); err != nil {
		if apperror.KindOf(err) == apperror.KindInternal {
			return nil, err
		}
		return nil, nil
	}
	actor, snapshot, err := s.transitionContext(ctx, issue, actorID, globalRole)
	if err != nil {
		return nil, err
	}
	return s.engine.Available(ctx, issue.ProjectID, snapshot, actor)
}

// ListTransitions returns the transitions the caller may run on the issue, with their required fields.
func (s *IssueService) ListTransitions(ctx context.Context, issueID, actorID uuid.UUID, globalRole string) ([]port.TransitionOption, error) {
	if s.engine == nil {
		return nil, domain.ErrTransitionUnsupported
	}
	issue, err := s.repo.FindByID(ctx, issueID)
	if err != nil {
		return nil, err
	}
	if err := s.requireVisible(ctx, issue.ProjectID, actorID, globalRole); err != nil {
		return nil, err
	}
	if err := s.requireTransition(ctx, issue, actorID, globalRole); err != nil {
		return nil, err
	}
	options, err := s.transitionOptions(ctx, issue, actorID, globalRole)
	if err != nil {
		return nil, err
	}
	if options == nil {
		options = []port.TransitionOption{}
	}
	return options, nil
}

// TransitionIssue runs a workflow transition in one transaction: it locks the issue, evaluates the transition
// with the engine, applies the planned changes, writes the status history, one activity batch and the
// issue.transitioned event.
func (s *IssueService) TransitionIssue(ctx context.Context, req port.TransitionRequest) (*port.TransitionResult, error) {
	if s.engine == nil || s.transitionRepo == nil {
		return nil, domain.ErrTransitionUnsupported
	}
	var result *port.TransitionResult
	var assigned *uuid.UUID
	err := s.tx.InTx(ctx, func(ctx context.Context) error {
		res, newAssignee, err := s.transition(ctx, req)
		result, assigned = res, newAssignee
		return err
	})
	if err != nil {
		return nil, err
	}
	if assigned != nil {
		s.notifyAssignment(ctx, result.Issue, req.ActorID, *assigned)
	}
	return result, nil
}

func (s *IssueService) transition(ctx context.Context, req port.TransitionRequest) (*port.TransitionResult, *uuid.UUID, error) {
	// The status read before the lock is the one the caller saw; a different status after the lock means a
	// concurrent transition won.
	before, err := s.repo.FindByID(ctx, req.IssueID)
	if err != nil {
		return nil, nil, err
	}
	if s.lifecycle != nil {
		if err := s.lifecycle.LockForUpdate(ctx, req.IssueID); err != nil {
			return nil, nil, err
		}
	}
	issue, err := s.repo.FindByID(ctx, req.IssueID)
	if err != nil {
		return nil, nil, err
	}
	if err := s.requireVisible(ctx, issue.ProjectID, req.ActorID, req.GlobalRole); err != nil {
		return nil, nil, err
	}
	if err := s.requireTransition(ctx, issue, req.ActorID, req.GlobalRole); err != nil {
		return nil, nil, err
	}
	if issue.StatusID != before.StatusID {
		return nil, nil, domain.ErrStatusConflict
	}
	if req.ExpectedStatusID != uuid.Nil && req.ExpectedStatusID != issue.StatusID {
		return nil, nil, domain.ErrStatusConflict
	}
	actor, snapshot, err := s.transitionContext(ctx, issue, req.ActorID, req.GlobalRole)
	if err != nil {
		return nil, nil, err
	}
	plan, err := s.engine.Plan(ctx, issue.ProjectID, snapshot, actor, req.TransitionID, req.Input)
	if err != nil {
		return nil, nil, err
	}
	resolutionID, err := s.planResolution(ctx, issue, plan, req.Input)
	if err != nil {
		return nil, nil, err
	}

	fromName := ""
	if issue.StatusRef != nil {
		fromName = issue.StatusRef.Name
	}
	changes := []domain.Change{domain.NewChange("status", fromName, plan.ToStatusName)}
	fieldChanges, err := s.applyPlannedFields(ctx, issue, plan.Fields)
	if err != nil {
		return nil, nil, err
	}
	changes = append(changes, fieldChanges...)
	var assigned *uuid.UUID
	previousResolution := idString(issue.ResolutionID)
	previousAssignee := issue.AssigneeID
	if plan.AssigneeSet {
		if plan.AssigneeID == uuid.Nil {
			issue.ClearAssignee()
		} else {
			id := plan.AssigneeID
			issue.AssigneeID = &id
			if !sameAssignee(previousAssignee, issue.AssigneeID) {
				assigned = &id
			}
		}
		if !sameAssignee(previousAssignee, issue.AssigneeID) {
			changes = append(changes, domain.NewChange("assignee", idString(previousAssignee), idString(issue.AssigneeID)))
		}
	}
	now := s.now()
	issue.UpdatedAt = now
	if err := s.repo.Update(ctx, issue); err != nil {
		return nil, nil, err
	}

	toDone := plan.ToCategory == domain.StatusCategoryDone
	write := port.TransitionWrite{
		IssueID: issue.ID, FromStatusID: issue.StatusID, ToStatusID: plan.ToStatusID, TransitionID: plan.TransitionID,
		ActorID: req.ActorID, Legacy: domain.LegacyStatusOfCategory(plan.ToCategory), ToDone: toDone,
		ResolutionAction: plan.ResolutionAction, ResolutionID: resolutionID, At: now,
	}
	applied, err := s.transitionRepo.ApplyStatus(ctx, write)
	if err != nil {
		return nil, nil, err
	}
	if !applied {
		return nil, nil, domain.ErrStatusConflict
	}
	if plan.ResolutionAction != port.PlanResolutionNone {
		changes = append(changes, domain.NewChange("resolution", previousResolution, idString(resolutionID)))
	}

	if err := s.transitionComments(ctx, issue, req.ActorID, plan); err != nil {
		return nil, nil, err
	}
	batchID := uuid.New()
	if err := s.recordBatch(ctx, issue, req.ActorID, batchID, changes); err != nil {
		return nil, nil, err
	}
	updated, err := s.repo.FindByID(ctx, issue.ID)
	if err != nil {
		return nil, nil, err
	}
	if err := s.publishTransitioned(ctx, updated, req.ActorID, batchID, plan, fromName); err != nil {
		return nil, nil, err
	}
	return &port.TransitionResult{Issue: updated, Plan: plan}, assigned, nil
}

// planResolution resolves the resolution the plan wants: the caller's choice, a post-function's id or the
// organization default Done. It returns nil when the plan leaves the resolution alone or clears it.
func (s *IssueService) planResolution(ctx context.Context, issue *domain.Issue, plan port.TransitionPlan, input port.TransitionInput) (*uuid.UUID, error) {
	if plan.ResolutionAction != port.PlanResolutionSet {
		return nil, nil
	}
	if s.catalog == nil {
		if plan.ResolutionID == uuid.Nil {
			return nil, nil
		}
		id := plan.ResolutionID
		return &id, nil
	}
	selector := port.CatalogSelector{ID: plan.ResolutionID}
	if plan.UseDefaultDone || plan.ResolutionID == uuid.Nil {
		selector = port.CatalogSelector{Default: true}
	}
	item, err := s.catalog.Select(ctx, issue.ProjectID, domain.CatalogResolution, selector)
	if err != nil {
		if apperror.CodeOf(err) == domain.ErrCatalogNotFound.Code && input.ResolutionID != uuid.Nil {
			return nil, domain.ErrResolutionNotFound
		}
		return nil, err
	}
	id := item.ID
	return &id, nil
}

// transitionComments saves the comment typed on the transition and the automatic comments of its post-functions.
func (s *IssueService) transitionComments(ctx context.Context, issue *domain.Issue, actorID uuid.UUID, plan port.TransitionPlan) error {
	if s.collaboration == nil {
		return nil
	}
	bodies := make([]string, 0, 1+len(plan.Comments))
	if strings.TrimSpace(plan.Comment) != "" {
		bodies = append(bodies, plan.Comment)
	}
	bodies = append(bodies, plan.Comments...)
	for _, body := range bodies {
		// Transition comments are public and carry no mentions: they are written by the workflow, not by a person.
		comment, err := domain.NewComment(issue.ID, actorID, body, s.now())
		if err != nil {
			return err
		}
		if err := s.collaboration.SaveComment(ctx, comment); err != nil {
			return err
		}
	}
	return nil
}

// publishTransitioned writes the issue.transitioned event in the transaction of the transition.
func (s *IssueService) publishTransitioned(ctx context.Context, issue *domain.Issue, actorID, batchID uuid.UUID, plan port.TransitionPlan, fromName string) error {
	if s.events == nil {
		return nil
	}
	projectID, issueID, actor := issue.ProjectID, issue.ID, actorID
	payload := map[string]any{
		"issue_id": issue.ID.String(), "key": issue.Key(), "batch_id": batchID.String(),
		"transition_id": plan.TransitionID, "transition": plan.TransitionName,
		"from_status_id": plan.FromStatusID.String(), "from_status": fromName,
		"to_status_id": plan.ToStatusID.String(), "to_status": plan.ToStatusName, "to_category": plan.ToCategory,
	}
	if len(plan.Events) > 0 {
		payload["post_function_events"] = plan.Events
	}
	return s.events.Publish(ctx, events.Event{
		ProjectID: &projectID, EntityID: &issueID, ActorID: &actor, Type: "issue.transitioned", EntityType: "issue",
		Payload: payload, CreatedAt: s.now(),
	})
}

// applyPlannedFields writes the field values of a plan into the issue and returns the changes. Standard fields
// are story_points, due_date, start_date and flagged; custom fields use CustomFieldKey names.
func (s *IssueService) applyPlannedFields(ctx context.Context, issue *domain.Issue, fields []port.PlannedField) ([]domain.Change, error) {
	var changes []domain.Change
	custom := map[string]string{}
	var problems []apperror.FieldError
	for _, planned := range fields {
		value := strings.TrimSpace(planned.Value)
		switch planned.Field {
		case "story_points":
			var points *float64
			if value != "" {
				parsed, err := strconv.ParseFloat(value, 64)
				if err != nil {
					problems = append(problems, apperror.FieldError{Field: "fields.story_points", Message: "debe ser un número"})
					continue
				}
				points = &parsed
			}
			if err := domain.ValidateStoryPoints(points); err != nil {
				problems = append(problems, apperror.FieldError{Field: "fields.story_points", Message: "los puntos de historia no son válidos"})
				continue
			}
			before := ""
			if issue.StoryPoints != nil {
				before = strconv.FormatFloat(*issue.StoryPoints, 'f', -1, 64)
			}
			issue.StoryPoints = points
			changes = append(changes, domain.NewChange("story_points", before, value))
		case "due_date", "start_date":
			var date *time.Time
			if value != "" {
				parsed, err := time.Parse("2006-01-02", value)
				if err != nil {
					problems = append(problems, apperror.FieldError{Field: "fields." + planned.Field, Message: "debe ser una fecha AAAA-MM-DD"})
					continue
				}
				date = &parsed
			}
			target := &issue.DueDate
			if planned.Field == "start_date" {
				target = &issue.StartDate
			}
			changes = append(changes, domain.NewChange(planned.Field, dateText(*target), dateText(date)))
			*target = date
		case "flagged":
			flag := value == "true"
			if flag != issue.Flagged {
				changes = append(changes, domain.NewChange("flagged", strconv.FormatBool(issue.Flagged), strconv.FormatBool(flag)))
			}
			issue.Flagged = flag
		default:
			if strings.HasPrefix(planned.Field, customFieldKeyPrefix) {
				custom[planned.Field] = planned.Value
				continue
			}
			problems = append(problems, apperror.FieldError{Field: "fields." + planned.Field, Message: "el campo no se puede escribir en una transición"})
		}
	}
	if len(problems) > 0 {
		return nil, apperror.WithCode(apperror.KindValidation, "workflow.validation_failed", "la transición no cumple los requisitos").WithDetails(problems...)
	}
	if err := domain.ValidateDates(issue.StartDate, issue.DueDate); err != nil {
		return nil, err
	}
	if len(custom) > 0 {
		customChanges, err := s.applyCustomPlanned(ctx, issue, custom)
		if err != nil {
			return nil, err
		}
		changes = append(changes, customChanges...)
	}
	return changes, nil
}

func dateText(t *time.Time) string {
	if t == nil {
		return ""
	}
	return domain.FormatDate(t)
}

// applyCustomPlanned writes custom field values by their workflow names. Values are text; the type of the
// field decides how they are read.
func (s *IssueService) applyCustomPlanned(ctx context.Context, issue *domain.Issue, values map[string]string) ([]domain.Change, error) {
	if s.customScope == nil {
		return nil, apperror.WithCode(apperror.KindValidation, "workflow.validation_failed", "la transición no cumple los requisitos").
			WithDetails(apperror.FieldError{Field: "fields", Message: "los campos personalizados no están disponibles"})
	}
	scope, err := s.customScope.ScopeFor(ctx, issue.ProjectID, issue.IssueTypeID)
	if err != nil {
		return nil, err
	}
	byKey := map[string]domain.FieldType{}
	for _, entry := range scope {
		byKey[CustomFieldKey(entry.Field.ID)] = entry.Field.Type
	}
	input := map[string]json.RawMessage{}
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		fieldType, ok := byKey[key]
		if !ok {
			return nil, apperror.WithCode(apperror.KindValidation, "workflow.validation_failed", "la transición no cumple los requisitos").
				WithDetails(apperror.FieldError{Field: "fields." + key, Message: "el campo no existe en este proyecto"})
		}
		raw, err := customRaw(fieldType, strings.TrimSpace(values[key]))
		if err != nil {
			return nil, apperror.WithCode(apperror.KindValidation, "workflow.validation_failed", "la transición no cumple los requisitos").
				WithDetails(apperror.FieldError{Field: "fields." + key, Message: "el valor no es válido"})
		}
		id, err := uuid.Parse(formatKeyUUID(key))
		if err != nil {
			return nil, err
		}
		input[id.String()] = raw
	}
	before := transitionFields(issue)
	if err := s.applyCustom(ctx, issue, false, issue.IssueTypeID, input); err != nil {
		return nil, err
	}
	after := transitionFields(issue)
	var changes []domain.Change
	for _, key := range keys {
		if before[key] != after[key] {
			changes = append(changes, domain.NewChange("custom."+key, before[key], after[key]))
		}
	}
	return changes, nil
}

// formatKeyUUID turns the 32 hex digits of a custom field key back into a canonical uuid string.
func formatKeyUUID(key string) string {
	digits := strings.TrimPrefix(key, customFieldKeyPrefix)
	if len(digits) != 32 {
		return digits
	}
	return digits[0:8] + "-" + digits[8:12] + "-" + digits[12:16] + "-" + digits[16:20] + "-" + digits[20:]
}

// customRaw converts the text of a workflow field value into the JSON the custom field writer reads.
func customRaw(fieldType domain.FieldType, value string) (json.RawMessage, error) {
	if value == "" {
		return json.RawMessage("null"), nil
	}
	switch fieldType {
	case domain.FieldNumber:
		number, err := strconv.ParseFloat(value, 64)
		if err != nil {
			return nil, err
		}
		return json.Marshal(number)
	case domain.FieldCheckbox:
		flag, err := strconv.ParseBool(value)
		if err != nil {
			return nil, err
		}
		return json.Marshal(flag)
	case domain.FieldMultiSelect, domain.FieldLabels:
		parts := strings.Split(value, ",")
		items := make([]string, 0, len(parts))
		for _, part := range parts {
			if part = strings.TrimSpace(part); part != "" {
				items = append(items, part)
			}
		}
		return json.Marshal(items)
	default:
		return json.Marshal(value)
	}
}

// MoveIssueToColumn runs the transition that leads the issue to the statuses of a board column. It fails with
// issue.transition_unavailable when no transition the caller may run reaches the column.
func (s *IssueService) MoveIssueToColumn(ctx context.Context, actorID uuid.UUID, globalRole string, projectID, issueID, columnID uuid.UUID) error {
	if s.engine == nil || s.transitionRepo == nil {
		return domain.ErrTransitionUnsupported
	}
	issue, err := s.repo.FindByID(ctx, issueID)
	if err != nil {
		return err
	}
	if issue.ProjectID != projectID {
		return domain.ErrIssueNotFound
	}
	if err := s.requireTransition(ctx, issue, actorID, globalRole); err != nil {
		return err
	}
	targets, err := s.engine.ColumnStatusIDs(ctx, columnID)
	if err != nil {
		return err
	}
	options, err := s.transitionOptions(ctx, issue, actorID, globalRole)
	if err != nil {
		return err
	}
	for _, option := range options {
		for _, target := range targets {
			if option.ToStatusID != target {
				continue
			}
			_, err := s.TransitionIssue(ctx, port.TransitionRequest{
				IssueID: issueID, ActorID: actorID, GlobalRole: globalRole, TransitionID: option.ID,
				ExpectedStatusID: issue.StatusID,
			})
			return err
		}
	}
	return domain.ErrTransitionUnavailable
}
