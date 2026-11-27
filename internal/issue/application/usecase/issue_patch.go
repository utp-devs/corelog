package usecase

import (
	"context"
	"encoding/json"
	"reflect"
	"strconv"
	"time"

	"github.com/google/uuid"

	"github.com/jhonsferg/corelog/internal/issue/application/port"
	"github.com/jhonsferg/corelog/internal/issue/domain"
	"github.com/jhonsferg/corelog/internal/platform/apperror"
	"github.com/jhonsferg/corelog/internal/platform/clock"
	"github.com/jhonsferg/corelog/internal/platform/events"
)

// Permission and holder names used for per-field checks. They match the permission catalog of the project
// permission schemes.
const (
	permEditIssue    = "edit_issue"
	permEditOwnIssue = "edit_own_issue"
	permAssignIssue  = "assign_issue"
	permDeleteIssue  = "delete_issue"

	holderReporter = "reporter"
	holderAssignee = "assignee"
)

// maxAncestorDepth bounds the walk up the parent chain of the cycle check.
const maxAncestorDepth = 8

// WithClock fixes the time source of timestamps written by the use cases. Without it the system clock is used.
func (s *IssueService) WithClock(c clock.Clock) *IssueService {
	if c != nil {
		s.clock = c
	}
	return s
}

// WithEvents fixes the publisher of issue events. Events are written in the transaction of the change.
func (s *IssueService) WithEvents(p events.Publisher) *IssueService {
	s.events = p
	return s
}

// WithPermissions fixes the per-permission checker. Without it every field edit needs edit_issue through the
// project authorizer and the view model lists no permissions.
func (s *IssueService) WithPermissions(p port.IssuePermissionChecker) *IssueService {
	s.perms = p
	return s
}

// WithTransitions replaces the legacy transition provider.
func (s *IssueService) WithTransitions(p port.TransitionProvider) *IssueService {
	s.transitions = p
	return s
}

// WithRequiredFields fixes the reader of the system fields a layout requires on creation.
func (s *IssueService) WithRequiredFields(r port.RequiredFieldReader) *IssueService {
	s.requiredFields = r
	return s
}

// WithProjectDefaults fixes the reader of project issue defaults (assignee policy).
func (s *IssueService) WithProjectDefaults(d port.ProjectDefaults) *IssueService {
	s.defaults = d
	return s
}

func (s *IssueService) now() time.Time {
	return s.clock.Now().UTC().Truncate(time.Microsecond)
}

// requireVisible checks that the caller can see the issue. An issue the caller cannot see looks missing.
func (s *IssueService) requireVisible(ctx context.Context, projectID, actorID uuid.UUID, globalRole string) error {
	err := s.requireIssueView(ctx, projectID, actorID, globalRole)
	if apperror.KindOf(err) == apperror.KindForbidden {
		return domain.ErrIssueNotFound
	}
	return err
}

func forbiddenIssueOperation() error {
	return apperror.New(apperror.KindForbidden, "forbidden project operation")
}

func holdersFor(issue *domain.Issue, actorID uuid.UUID) []string {
	var holders []string
	if issue.ReporterID == actorID {
		holders = append(holders, holderReporter)
	}
	if issue.AssigneeID != nil && *issue.AssigneeID == actorID {
		holders = append(holders, holderAssignee)
	}
	return holders
}

// requireFieldEdit checks edit_issue, or edit_own_issue when the caller reported the issue.
func (s *IssueService) requireFieldEdit(ctx context.Context, issue *domain.Issue, actorID uuid.UUID, globalRole string) error {
	if s.perms == nil {
		return s.requireIssueEdit(ctx, issue.ProjectID, actorID, globalRole)
	}
	holders := holdersFor(issue, actorID)
	ok, err := s.perms.Can(ctx, actorID, globalRole, issue.ProjectID, permEditIssue, holders...)
	if err != nil {
		return err
	}
	if !ok && issue.ReporterID == actorID {
		ok, err = s.perms.Can(ctx, actorID, globalRole, issue.ProjectID, permEditOwnIssue, holders...)
		if err != nil {
			return err
		}
	}
	if ok {
		return nil
	}
	return s.deniedOrArchived(ctx, permEditIssue, issue.ProjectID, actorID, globalRole)
}

// requireAssign checks assign_issue.
func (s *IssueService) requireAssign(ctx context.Context, issue *domain.Issue, actorID uuid.UUID, globalRole string) error {
	if s.perms == nil {
		return s.requireIssueEdit(ctx, issue.ProjectID, actorID, globalRole)
	}
	ok, err := s.perms.Can(ctx, actorID, globalRole, issue.ProjectID, permAssignIssue, holdersFor(issue, actorID)...)
	if err != nil {
		return err
	}
	if ok {
		return nil
	}
	return s.deniedOrArchived(ctx, permAssignIssue, issue.ProjectID, actorID, globalRole)
}

// requireDelete checks delete_issue.
func (s *IssueService) requireDelete(ctx context.Context, issue *domain.Issue, actorID uuid.UUID, globalRole string) error {
	if s.perms == nil {
		return s.authorize(ctx, permDeleteIssue, issue.ProjectID, actorID, globalRole)
	}
	ok, err := s.perms.Can(ctx, actorID, globalRole, issue.ProjectID, permDeleteIssue, holdersFor(issue, actorID)...)
	if err != nil {
		return err
	}
	if ok {
		return nil
	}
	return s.deniedOrArchived(ctx, permDeleteIssue, issue.ProjectID, actorID, globalRole)
}

// deniedOrArchived returns the error of a denied permission: a conflict when the project is archived (the
// authorizer reports it), forbidden otherwise.
func (s *IssueService) deniedOrArchived(ctx context.Context, action string, projectID, actorID uuid.UUID, globalRole string) error {
	if err := s.authorize(ctx, action, projectID, actorID, globalRole); err != nil && apperror.KindOf(err) != apperror.KindForbidden {
		return err
	}
	return forbiddenIssueOperation()
}

// Patch applies a partial update in one transaction: field permissions, stale check, hierarchy rules, the write,
// one activity batch and one issue.updated event.
func (s *IssueService) Patch(ctx context.Context, id, actorID uuid.UUID, globalRole string, in port.PatchInput) (*port.PatchResult, error) {
	var result *port.PatchResult
	var assigned *uuid.UUID
	err := s.tx.InTx(ctx, func(ctx context.Context) error {
		res, newAssignee, err := s.patch(ctx, id, actorID, globalRole, in)
		result, assigned = res, newAssignee
		return err
	})
	if err != nil {
		return nil, err
	}
	if assigned != nil {
		s.notifyAssignment(ctx, result.Issue, actorID, *assigned)
	}
	return result, nil
}

func (s *IssueService) patch(ctx context.Context, id, actorID uuid.UUID, globalRole string, in port.PatchInput) (*port.PatchResult, *uuid.UUID, error) {
	if s.lifecycle != nil {
		if err := s.lifecycle.LockForUpdate(ctx, id); err != nil {
			return nil, nil, err
		}
	}
	issue, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, nil, err
	}
	visible := s.requireIssueView
	if in.HideForbidden {
		visible = s.requireVisible
	}
	if err := visible(ctx, issue.ProjectID, actorID, globalRole); err != nil {
		return nil, nil, err
	}
	editing, assigning := patchTouches(in)
	if editing {
		if err := s.requireFieldEdit(ctx, issue, actorID, globalRole); err != nil {
			return nil, nil, err
		}
	}
	if assigning {
		if err := s.requireAssign(ctx, issue, actorID, globalRole); err != nil {
			return nil, nil, err
		}
	}
	if in.ExpectedUpdatedAt != nil && !in.ExpectedUpdatedAt.UTC().Truncate(time.Microsecond).Equal(issue.UpdatedAt.UTC().Truncate(time.Microsecond)) {
		return nil, nil, domain.ErrStale
	}

	previousAssignee := issue.AssigneeID
	changes, err := s.applyPatch(ctx, issue, actorID, in)
	if err != nil {
		return nil, nil, err
	}
	result := &port.PatchResult{Issue: issue, Changes: changes}
	if len(changes) == 0 {
		return result, nil, nil
	}

	issue.UpdatedAt = s.now()
	if err := s.repo.Update(ctx, issue); err != nil {
		return nil, nil, err
	}
	var newAssignee *uuid.UUID
	if in.AssigneeID.Set && issue.AssigneeID != nil && !sameAssignee(previousAssignee, issue.AssigneeID) {
		newAssignee = issue.AssigneeID
		if s.watchers != nil {
			if err := s.watchers.Add(ctx, issue.ID, *issue.AssigneeID); err != nil {
				return nil, nil, err
			}
		}
	}
	result.BatchID = uuid.New()
	if err := s.recordBatch(ctx, issue, actorID, result.BatchID, changes); err != nil {
		return nil, nil, err
	}
	if err := s.publishUpdated(ctx, issue, actorID, result.BatchID, changes); err != nil {
		return nil, nil, err
	}
	return result, newAssignee, nil
}

// patchTouches tells whether the patch edits fields (edit_issue) and whether it changes the assignee.
func patchTouches(in port.PatchInput) (editing, assigning bool) {
	editing = in.Summary.Set || in.Description.Set || in.IssueTypeID.Set || in.IssueType.Set ||
		in.PriorityID.Set || in.Priority.Set || in.ParentID.Set || in.StoryPoints.Set ||
		in.OriginalEstimateSeconds.Set || in.RemainingEstimateSeconds.Set || in.StartDate.Set ||
		in.DueDate.Set || in.Flagged.Set || len(in.Custom) > 0
	return editing, in.AssigneeID.Set
}

// applyPatch mutates issue with the provided fields and returns the changes. It validates every rule but does
// not write.
func (s *IssueService) applyPatch(ctx context.Context, issue *domain.Issue, actorID uuid.UUID, in port.PatchInput) ([]domain.Change, error) {
	var changes []domain.Change
	add := func(field, from, to string) {
		if from != to {
			changes = append(changes, domain.NewChange(field, from, to))
		}
	}
	previousType := issue.IssueTypeID
	previousLevel := issue.HierarchyLevel()
	previousParent := issue.ParentID

	if in.Summary.Set {
		if in.Summary.Value == nil {
			return nil, domain.ErrFieldNotClearable
		}
		summary, err := domain.NormalizeSummary(*in.Summary.Value)
		if err != nil {
			return nil, err
		}
		add("summary", issue.Summary, summary)
		issue.Summary = summary
	}
	if in.Description.Set {
		text := ""
		if in.Description.Value != nil {
			text = *in.Description.Value
		}
		description, err := domain.NormalizeDescription(text)
		if err != nil {
			return nil, err
		}
		add("description", issue.Description, description)
		issue.Description = description
	}
	if err := s.applyTypeAndPriority(ctx, issue, in, add); err != nil {
		return nil, err
	}
	if in.StoryPoints.Set {
		if err := domain.ValidateStoryPoints(in.StoryPoints.Value); err != nil {
			return nil, err
		}
		add("story_points", formatFloat(issue.StoryPoints), formatFloat(in.StoryPoints.Value))
		issue.StoryPoints = in.StoryPoints.Value
	}
	if in.OriginalEstimateSeconds.Set {
		if err := domain.ValidateEstimate(in.OriginalEstimateSeconds.Value); err != nil {
			return nil, err
		}
		add("original_estimate", formatInt(issue.OriginalEstimateSeconds), formatInt(in.OriginalEstimateSeconds.Value))
		issue.OriginalEstimateSeconds = in.OriginalEstimateSeconds.Value
	}
	if in.RemainingEstimateSeconds.Set {
		if err := domain.ValidateEstimate(in.RemainingEstimateSeconds.Value); err != nil {
			return nil, err
		}
		add("remaining_estimate", formatInt(issue.RemainingEstimateSeconds), formatInt(in.RemainingEstimateSeconds.Value))
		issue.RemainingEstimateSeconds = in.RemainingEstimateSeconds.Value
	}
	if in.StartDate.Set {
		date := dateValue(in.StartDate.Value)
		add("start_date", domain.FormatDate(issue.StartDate), domain.FormatDate(date))
		issue.StartDate = date
	}
	if in.DueDate.Set {
		date := dateValue(in.DueDate.Value)
		add("due_date", domain.FormatDate(issue.DueDate), domain.FormatDate(date))
		issue.DueDate = date
	}
	if err := domain.ValidateDates(issue.StartDate, issue.DueDate); err != nil {
		return nil, err
	}
	if in.Flagged.Set {
		if in.Flagged.Value == nil {
			return nil, domain.ErrFieldNotClearable
		}
		add("flagged", strconv.FormatBool(issue.Flagged), strconv.FormatBool(*in.Flagged.Value))
		issue.Flagged = *in.Flagged.Value
	}
	if in.ParentID.Set {
		add("parent", idString(issue.ParentID), idString(in.ParentID.Value))
		issue.ParentID = in.ParentID.Value
	}
	if in.AssigneeID.Set {
		if err := s.applyAssignee(ctx, issue, actorID, in.AssigneeID.Value, add); err != nil {
			return nil, err
		}
	}
	if in.ParentID.Set || previousLevel != issue.HierarchyLevel() {
		if err := s.checkHierarchy(ctx, issue, previousLevel != issue.HierarchyLevel(), in.ParentID.Set || previousParent != issue.ParentID); err != nil {
			return nil, err
		}
	}
	if in.Custom != nil || previousType != issue.IssueTypeID {
		before := make(map[string]any, len(issue.Custom))
		for k, v := range issue.Custom {
			before[k] = v
		}
		if err := s.applyCustom(ctx, issue, false, previousType, in.Custom); err != nil {
			return nil, err
		}
		changes = append(changes, customChanges(before, issue.Custom)...)
	}
	return changes, nil
}

func (s *IssueService) applyTypeAndPriority(ctx context.Context, issue *domain.Issue, in port.PatchInput, add func(field, from, to string)) error {
	if in.IssueTypeID.Set || in.IssueType.Set {
		if (in.IssueTypeID.Set && in.IssueTypeID.Value == nil) || (in.IssueType.Set && in.IssueType.Value == nil) {
			return domain.ErrFieldNotClearable
		}
		before := typeKey(issue)
		var legacy domain.IssueType
		if in.IssueType.Set {
			legacy = *in.IssueType.Value
		}
		if s.catalog != nil {
			var id *uuid.UUID
			if in.IssueTypeID.Set {
				id = in.IssueTypeID.Value
			}
			if err := s.resolveTypeInto(ctx, issue, id, legacy); err != nil {
				return err
			}
		} else {
			if !legacy.IsValid() {
				return domain.ErrInvalidIssueType
			}
			issue.IssueType = legacy
		}
		add("issue_type", before, typeKey(issue))
	}
	if in.PriorityID.Set || in.Priority.Set {
		if (in.PriorityID.Set && in.PriorityID.Value == nil) || (in.Priority.Set && in.Priority.Value == nil) {
			return domain.ErrFieldNotClearable
		}
		before := priorityKey(issue)
		var legacy domain.Priority
		if in.Priority.Set {
			legacy = *in.Priority.Value
		}
		if s.catalog != nil {
			var id *uuid.UUID
			if in.PriorityID.Set {
				id = in.PriorityID.Value
			}
			if err := s.resolvePriorityInto(ctx, issue, id, legacy); err != nil {
				return err
			}
		} else {
			if !legacy.IsValid() {
				return domain.ErrInvalidPriority
			}
			issue.Priority = legacy
		}
		add("priority", before, priorityKey(issue))
	}
	return nil
}

func typeKey(issue *domain.Issue) string {
	if issue.IssueTypeID != uuid.Nil {
		return issue.IssueTypeID.String()
	}
	return string(issue.IssueType)
}

func priorityKey(issue *domain.Issue) string {
	if issue.PriorityID != uuid.Nil {
		return issue.PriorityID.String()
	}
	return string(issue.Priority)
}

func (s *IssueService) applyAssignee(ctx context.Context, issue *domain.Issue, actorID uuid.UUID, next *uuid.UUID, add func(field, from, to string)) error {
	if sameAssignee(issue.AssigneeID, next) {
		return nil
	}
	if next != nil {
		sameTeam, err := s.directory.SharesTeam(ctx, actorID, *next)
		if err != nil {
			return err
		}
		if !sameTeam {
			return domain.ErrCrossTeamAssignment
		}
	}
	add("assignee", idString(issue.AssigneeID), idString(next))
	issue.AssigneeID = next
	return nil
}

// checkHierarchy validates the parent of the issue and, when its level changed, the children it already has.
func (s *IssueService) checkHierarchy(ctx context.Context, issue *domain.Issue, levelChanged, parentChanged bool) error {
	node := domain.HierarchyNode{ID: issue.ID, ProjectID: issue.ProjectID, Level: issue.HierarchyLevel(), ParentID: issue.ParentID}
	if parentChanged || levelChanged {
		var parent *domain.HierarchyNode
		var ancestors []uuid.UUID
		if issue.ParentID != nil {
			found, err := s.repo.FindByID(ctx, *issue.ParentID)
			if err != nil {
				if apperror.KindOf(err) == apperror.KindNotFound {
					return domain.ErrParentNotFound
				}
				return err
			}
			parent = &domain.HierarchyNode{ID: found.ID, ProjectID: found.ProjectID, Level: found.HierarchyLevel(), ParentID: found.ParentID}
			next := found.ParentID
			for depth := 0; next != nil && depth < maxAncestorDepth; depth++ {
				ancestors = append(ancestors, *next)
				up, err := s.repo.FindByID(ctx, *next)
				if err != nil {
					break
				}
				next = up.ParentID
			}
		}
		if err := domain.CheckParent(node, parent, ancestors); err != nil {
			return err
		}
	}
	if levelChanged && s.hierarchy != nil && issue.ID != uuid.Nil {
		levels, err := s.hierarchy.ChildLevels(ctx, issue.ID)
		if err != nil {
			return err
		}
		return domain.CheckChildren(node.Level, levels)
	}
	return nil
}

// customChanges lists the custom values that differ between before and after, keyed by field id.
func customChanges(before, after map[string]any) []domain.Change {
	var out []domain.Change
	seen := map[string]bool{}
	for key, value := range after {
		seen[key] = true
		if old, ok := before[key]; !ok || !reflect.DeepEqual(old, value) {
			out = append(out, domain.NewChange(key, jsonText(before[key]), jsonText(value)))
		}
	}
	for key, old := range before {
		if !seen[key] {
			out = append(out, domain.NewChange(key, jsonText(old), ""))
		}
	}
	sortChanges(out)
	return out
}

func sortChanges(changes []domain.Change) {
	for i := 1; i < len(changes); i++ {
		for j := i; j > 0 && changes[j].Field < changes[j-1].Field; j-- {
			changes[j], changes[j-1] = changes[j-1], changes[j]
		}
	}
}

func jsonText(v any) string {
	if v == nil {
		return ""
	}
	if text, ok := v.(string); ok {
		return text
	}
	raw, err := json.Marshal(v)
	if err != nil {
		return ""
	}
	return string(raw)
}

func formatFloat(v *float64) string {
	if v == nil {
		return ""
	}
	return strconv.FormatFloat(*v, 'f', -1, 64)
}

func formatInt(v *int64) string {
	if v == nil {
		return ""
	}
	return strconv.FormatInt(*v, 10)
}

func idString(id *uuid.UUID) string {
	if id == nil {
		return ""
	}
	return id.String()
}

func dateValue(v *time.Time) *time.Time {
	if v == nil {
		return nil
	}
	d := domain.DateOnly(*v)
	return &d
}

// recordBatch writes one activity per change, all sharing batchID.
func (s *IssueService) recordBatch(ctx context.Context, issue *domain.Issue, actorID, batchID uuid.UUID, changes []domain.Change) error {
	if s.collaboration == nil && s.activityBatch == nil {
		return nil
	}
	at := s.now()
	activities := make([]*domain.Activity, 0, len(changes))
	for _, change := range changes {
		activities = append(activities, domain.NewFieldActivity(issue.ID, actorID, batchID, change, at))
	}
	if s.activityBatch != nil {
		return s.activityBatch.SaveActivities(ctx, activities)
	}
	for _, activity := range activities {
		if err := s.collaboration.SaveActivity(ctx, activity); err != nil {
			return err
		}
	}
	return nil
}

func (s *IssueService) publishUpdated(ctx context.Context, issue *domain.Issue, actorID, batchID uuid.UUID, changes []domain.Change) error {
	if s.events == nil {
		return nil
	}
	list := make([]map[string]any, 0, len(changes))
	for _, change := range changes {
		list = append(list, map[string]any{"field": change.Field, "from": change.From, "to": change.To})
	}
	projectID, issueID, actor := issue.ProjectID, issue.ID, actorID
	return s.events.Publish(ctx, events.Event{
		ProjectID:  &projectID,
		EntityID:   &issueID,
		ActorID:    &actor,
		Type:       "issue.updated",
		EntityType: "issue",
		Payload: map[string]any{
			"issue_id": issue.ID.String(), "key": issue.Key(), "batch_id": batchID.String(), "changes": list,
		},
		CreatedAt: s.now(),
	})
}

// Delete soft-deletes an issue. The caller needs delete_issue; an issue the caller cannot see looks missing.
func (s *IssueService) Delete(ctx context.Context, id, actorID uuid.UUID, globalRole string) error {
	return s.tx.InTx(ctx, func(ctx context.Context) error {
		if s.lifecycle == nil {
			return apperror.New(apperror.KindInternal, "issue lifecycle is unavailable")
		}
		issue, err := s.repo.FindByID(ctx, id)
		if err != nil {
			return err
		}
		if err := s.requireVisible(ctx, issue.ProjectID, actorID, globalRole); err != nil {
			return err
		}
		if err := s.requireDelete(ctx, issue, actorID, globalRole); err != nil {
			return err
		}
		if err := s.lifecycle.SoftDelete(ctx, id, s.now()); err != nil {
			return err
		}
		return s.publishLifecycle(ctx, issue, actorID, "issue.deleted")
	})
}

// Restore brings back a soft-deleted issue. It needs delete_issue on the project, which only leads and
// administrators hold by default.
func (s *IssueService) Restore(ctx context.Context, id, actorID uuid.UUID, globalRole string) (*domain.Issue, error) {
	var restored *domain.Issue
	err := s.tx.InTx(ctx, func(ctx context.Context) error {
		if s.lifecycle == nil {
			return apperror.New(apperror.KindInternal, "issue lifecycle is unavailable")
		}
		issue, err := s.lifecycle.FindDeleted(ctx, id)
		if err != nil {
			return err
		}
		if err := s.requireVisible(ctx, issue.ProjectID, actorID, globalRole); err != nil {
			return err
		}
		if err := s.requireDelete(ctx, issue, actorID, globalRole); err != nil {
			return err
		}
		if err := s.lifecycle.Restore(ctx, id, s.now()); err != nil {
			return err
		}
		restored, err = s.repo.FindByID(ctx, id)
		if err != nil {
			return err
		}
		return s.publishLifecycle(ctx, restored, actorID, "issue.restored")
	})
	if err != nil {
		return nil, err
	}
	return restored, nil
}

func (s *IssueService) publishLifecycle(ctx context.Context, issue *domain.Issue, actorID uuid.UUID, kind string) error {
	if s.events == nil {
		return nil
	}
	projectID, issueID, actor := issue.ProjectID, issue.ID, actorID
	return s.events.Publish(ctx, events.Event{
		ProjectID: &projectID, EntityID: &issueID, ActorID: &actor, Type: kind, EntityType: "issue",
		Payload:   map[string]any{"issue_id": issue.ID.String(), "key": issue.Key()},
		CreatedAt: s.now(),
	})
}
