package usecase

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"

	"github.com/jhonsferg/corelog/internal/issue/application/port"
	"github.com/jhonsferg/corelog/internal/issue/domain"
	"github.com/jhonsferg/corelog/internal/platform/apperror"
)

// fakeStep is one transition of the fake workflow engine.
type fakeStep struct {
	id             string
	name           string
	from           []uuid.UUID
	to             uuid.UUID
	toName         string
	toCategory     string
	requireComment bool
	plan           func(*port.TransitionPlan)
}

// fakeEngine evaluates a fixed list of steps. It stands in for the workflow module.
type fakeEngine struct {
	steps     []fakeStep
	columns   map[uuid.UUID][]uuid.UUID
	lastActor port.TransitionActor
	lastIssue port.TransitionIssue
}

func (e *fakeEngine) Available(_ context.Context, _ uuid.UUID, issue port.TransitionIssue, actor port.TransitionActor) ([]port.TransitionOption, error) {
	e.lastActor, e.lastIssue = actor, issue
	out := []port.TransitionOption{}
	for _, step := range e.steps {
		if !stepFrom(step, issue.StatusID) {
			continue
		}
		out = append(out, port.TransitionOption{
			ID: step.id, Name: step.name, ToStatusID: step.to, ToStatusName: step.toName, ToCategory: step.toCategory,
			RequiresComment: step.requireComment,
		})
	}
	return out, nil
}

func stepFrom(step fakeStep, id uuid.UUID) bool {
	for _, from := range step.from {
		if from == id {
			return true
		}
	}
	return false
}

func (e *fakeEngine) Plan(_ context.Context, _ uuid.UUID, issue port.TransitionIssue, actor port.TransitionActor, transitionID string, input port.TransitionInput) (port.TransitionPlan, error) {
	e.lastActor, e.lastIssue = actor, issue
	for _, step := range e.steps {
		if step.id != transitionID || !stepFrom(step, issue.StatusID) {
			continue
		}
		if step.requireComment && input.Comment == "" {
			return port.TransitionPlan{}, apperror.WithCode(apperror.KindValidation, "workflow.validation_failed", "la transición no cumple los requisitos").
				WithDetails(apperror.FieldError{Field: "comment", Message: "el comentario es obligatorio para esta transición"})
		}
		plan := port.TransitionPlan{
			TransitionID: step.id, TransitionName: step.name, FromStatusID: issue.StatusID, ToStatusID: step.to,
			ToStatusName: step.toName, ToCategory: step.toCategory, Comment: input.Comment,
		}
		if step.plan != nil {
			step.plan(&plan)
		}
		return plan, nil
	}
	return port.TransitionPlan{}, apperror.WithCode(apperror.KindForbidden, "workflow.transition_not_allowed", "la transición no está disponible desde el estado actual")
}

func (e *fakeEngine) ColumnStatusIDs(_ context.Context, columnID uuid.UUID) ([]uuid.UUID, error) {
	return e.columns[columnID], nil
}

// fakeTransitionRepo applies the status part of a transition on the in-memory issue store.
type fakeTransitionRepo struct {
	repo     *lifecycleRepo
	subtasks int
	writes   []port.TransitionWrite
	// raceOnce changes the status of the issue right before the next ApplyStatus, like a concurrent transition.
	raceOnce uuid.UUID
}

func (r *fakeTransitionRepo) OpenSubtasks(context.Context, uuid.UUID) (int, error) {
	return r.subtasks, nil
}

func (r *fakeTransitionRepo) StatusInfo(context.Context, uuid.UUID) (string, string, error) {
	return "", "", nil
}

func (r *fakeTransitionRepo) ApplyStatus(_ context.Context, w port.TransitionWrite) (bool, error) {
	stored := r.repo.issues[w.IssueID]
	if r.raceOnce != uuid.Nil {
		stored.StatusID, r.raceOnce = r.raceOnce, uuid.Nil
	}
	if stored.StatusID != w.FromStatusID {
		return false, nil
	}
	stored.StatusID = w.ToStatusID
	stored.Status = w.Legacy
	switch w.ResolutionAction {
	case port.PlanResolutionSet:
		stored.ResolutionID = w.ResolutionID
		at := w.At
		stored.ResolvedAt = &at
	case port.PlanResolutionClear:
		stored.ResolutionID, stored.ResolvedAt = nil, nil
	}
	stored.StatusRef = &domain.StatusRef{ID: w.ToStatusID, Name: "x", Category: "x"}
	r.writes = append(r.writes, w)
	return true, nil
}

// permRoles reads the project role of a user from the permission fake.
type permRoles struct{ *permFake }

func (p permRoles) FindRole(_ context.Context, _, userID uuid.UUID) (*string, error) {
	role, ok := p.roles[userID]
	if !ok {
		return nil, nil
	}
	name := string(role)
	return &name, nil
}

func (p permRoles) ListProjectIDs(context.Context, uuid.UUID) ([]uuid.UUID, error) { return nil, nil }

type transitionFixture struct {
	*patchFixture
	engine                       *fakeEngine
	store                        *fakeTransitionRepo
	todo, progress, review, done uuid.UUID
}

func newTransitionFixture(t *testing.T) *transitionFixture {
	t.Helper()
	pf := newPatchFixture(t)
	f := &transitionFixture{patchFixture: pf, todo: uuid.New(), progress: uuid.New(), review: uuid.New(), done: uuid.New()}
	f.engine = &fakeEngine{columns: map[uuid.UUID][]uuid.UUID{}}
	f.store = &fakeTransitionRepo{repo: pf.repo}
	f.engine.steps = []fakeStep{
		{id: "start", name: "Iniciar", from: []uuid.UUID{f.todo}, to: f.progress, toName: "En curso", toCategory: "in_progress"},
		{id: "review", name: "Revisar", from: []uuid.UUID{f.progress}, to: f.review, toName: "En revisión", toCategory: "in_progress", requireComment: true},
		{id: "complete", name: "Completar", from: []uuid.UUID{f.todo, f.progress}, to: f.done, toName: "Hecho", toCategory: "done"},
	}
	stored := pf.repo.issues[pf.issue.ID]
	stored.StatusID = f.todo
	stored.StatusRef = &domain.StatusRef{ID: f.todo, Name: "Por hacer", Category: "todo"}
	pf.svc.projectRoles = permRoles{pf.perms}
	pf.svc.WithWorkflowEngine(f.engine, f.store)
	return f
}

func (f *transitionFixture) run(actor uuid.UUID, transition string, mod func(*port.TransitionRequest)) (*port.TransitionResult, error) {
	req := port.TransitionRequest{IssueID: f.issue.ID, ActorID: actor, GlobalRole: "user", TransitionID: transition}
	if mod != nil {
		mod(&req)
	}
	return f.svc.TransitionIssue(context.Background(), req)
}

func TestTransitionIssue_AppliesStatusHistoryActivityAndEvent(t *testing.T) {
	f := newTransitionFixture(t)
	res, err := f.run(f.lead, "start", nil)
	if err != nil {
		t.Fatal(err)
	}
	if res.Issue.StatusID != f.progress || res.Issue.Status != domain.StatusInProgress {
		t.Fatalf("issue after transition: status %v legacy %s", res.Issue.StatusID, res.Issue.Status)
	}
	if len(f.store.writes) != 1 || f.store.writes[0].TransitionID != "start" || f.store.writes[0].FromStatusID != f.todo || f.store.writes[0].ActorID != f.lead {
		t.Fatalf("status writes = %+v", f.store.writes)
	}
	if len(f.collab.activities) != 1 || (f.collab.activities[0].FieldName == nil || *f.collab.activities[0].FieldName != "status") {
		t.Fatalf("activities = %+v, want one status change", f.collab.activities)
	}
	if len(f.events.list) != 1 || f.events.list[0].Type != "issue.transitioned" {
		t.Fatalf("events = %+v, want one issue.transitioned", f.events.list)
	}
	payload := f.events.list[0].Payload
	if payload["transition_id"] != "start" || payload["to_status_id"] != f.progress.String() {
		t.Fatalf("event payload = %v", payload)
	}
	if f.engine.lastActor.Role != "lead" {
		t.Fatalf("engine saw role %q, want lead", f.engine.lastActor.Role)
	}
}

func TestTransitionIssue_PermissionsAndVisibility(t *testing.T) {
	f := newTransitionFixture(t)
	if _, err := f.run(f.viewer, "start", nil); apperror.KindOf(err) != apperror.KindForbidden {
		t.Fatalf("viewer: %v, want forbidden", err)
	}
	if _, err := f.run(uuid.New(), "start", nil); apperror.KindOf(err) != apperror.KindNotFound {
		t.Fatalf("stranger: %v, want not found", err)
	}
	if _, err := f.run(f.owner, "start", nil); err != nil {
		t.Fatalf("contributor: %v", err)
	}
	if len(f.store.writes) != 1 {
		t.Fatalf("only the contributor must have written, got %d writes", len(f.store.writes))
	}
	if _, err := f.run(f.admin, "start", func(r *port.TransitionRequest) { r.GlobalRole = "admin" }); apperror.CodeOf(err) != "workflow.transition_not_allowed" {
		t.Fatalf("admin on the wrong source status: %v, want workflow.transition_not_allowed", err)
	}
	if f.engine.lastActor.Role != "lead" {
		t.Fatalf("platform admin role = %q, want lead", f.engine.lastActor.Role)
	}
}

func TestTransitionIssue_ValidationFailureWritesNothing(t *testing.T) {
	f := newTransitionFixture(t)
	if _, err := f.run(f.lead, "start", nil); err != nil {
		t.Fatal(err)
	}
	f.events.list, f.collab.activities, f.store.writes = nil, nil, nil
	_, err := f.run(f.lead, "review", nil)
	if apperror.KindOf(err) != apperror.KindValidation || apperror.CodeOf(err) != "workflow.validation_failed" {
		t.Fatalf("review without comment: %v", err)
	}
	var appErr *apperror.Error
	if !errors.As(err, &appErr) || len(appErr.Details) != 1 || appErr.Details[0].Field != "comment" {
		t.Fatalf("details = %+v, want one detail on comment", appErr)
	}
	if len(f.store.writes) != 0 || len(f.events.list) != 0 || len(f.collab.activities) != 0 {
		t.Fatal("a failed validation must not write anything")
	}
	res, err := f.run(f.lead, "review", func(r *port.TransitionRequest) { r.Input.Comment = "Listo para revisar" })
	if err != nil {
		t.Fatal(err)
	}
	if res.Issue.StatusID != f.review {
		t.Fatalf("status = %v, want review", res.Issue.StatusID)
	}
	if len(f.collab.comments) != 1 || f.collab.comments[0].Body != "Listo para revisar" {
		t.Fatalf("comments = %+v, want the transition comment", f.collab.comments)
	}
}

func TestTransitionIssue_PostFunctionsApplyResolutionAssigneeFieldsAndComments(t *testing.T) {
	f := newTransitionFixture(t)
	resolution := uuid.New()
	f.engine.steps[2].plan = func(p *port.TransitionPlan) {
		p.ResolutionAction, p.ResolutionID = port.PlanResolutionSet, resolution
		p.AssigneeSet, p.AssigneeID = true, f.other
		p.Fields = []port.PlannedField{{Field: "story_points", Value: "5"}, {Field: "due_date", Value: "2026-12-01"}, {Field: "flagged", Value: "true"}}
		p.Comments = []string{"Cerrada automáticamente"}
	}
	res, err := f.run(f.lead, "complete", nil)
	if err != nil {
		t.Fatal(err)
	}
	stored := f.repo.issues[f.issue.ID]
	if stored.ResolutionID == nil || *stored.ResolutionID != resolution || stored.ResolvedAt == nil {
		t.Fatalf("resolution = %v at %v", stored.ResolutionID, stored.ResolvedAt)
	}
	if stored.AssigneeID == nil || *stored.AssigneeID != f.other {
		t.Fatalf("assignee = %v, want %v", stored.AssigneeID, f.other)
	}
	if stored.StoryPoints == nil || *stored.StoryPoints != 5 || stored.DueDate == nil || !stored.Flagged {
		t.Fatalf("fields not applied: points=%v due=%v flagged=%v", stored.StoryPoints, stored.DueDate, stored.Flagged)
	}
	if res.Issue.Status != domain.StatusClosed {
		t.Fatalf("legacy mirror of a done status = %s, want closed", res.Issue.Status)
	}
	fields := map[string]bool{}
	for _, activity := range f.collab.activities {
		if activity.FieldName != nil {
			fields[*activity.FieldName] = true
		}
	}
	for _, want := range []string{"status", "resolution", "assignee", "story_points", "due_date", "flagged"} {
		if !fields[want] {
			t.Errorf("activity batch misses a %s change: %v", want, fields)
		}
	}
	if len(f.collab.comments) != 1 || f.collab.comments[0].Body != "Cerrada automáticamente" {
		t.Fatalf("comments = %+v", f.collab.comments)
	}

	// Reopening clears the resolution.
	f.engine.steps = append(f.engine.steps, fakeStep{id: "reopen", name: "Reabrir", from: []uuid.UUID{f.done}, to: f.todo, toName: "Por hacer", toCategory: "todo",
		plan: func(p *port.TransitionPlan) { p.ResolutionAction = port.PlanResolutionClear }})
	if _, err := f.run(f.lead, "reopen", nil); err != nil {
		t.Fatal(err)
	}
	stored = f.repo.issues[f.issue.ID]
	if stored.ResolutionID != nil || stored.ResolvedAt != nil {
		t.Fatal("reopening must clear the resolution")
	}
}

func TestTransitionIssue_InvalidPlannedFieldsAreRejected(t *testing.T) {
	f := newTransitionFixture(t)
	for name, field := range map[string]port.PlannedField{
		"unknown field":        {Field: "colour", Value: "x"},
		"bad story points":     {Field: "story_points", Value: "abc"},
		"bad date":             {Field: "due_date", Value: "tomorrow"},
		"custom without scope": {Field: customFieldKeyPrefix + "00000000000000000000000000000001", Value: "x"},
	} {
		field := field
		f.engine.steps[0].plan = func(p *port.TransitionPlan) { p.Fields = []port.PlannedField{field} }
		_, err := f.run(f.lead, "start", nil)
		if apperror.KindOf(err) != apperror.KindValidation || apperror.CodeOf(err) != "workflow.validation_failed" {
			t.Errorf("%s: %v, want workflow.validation_failed", name, err)
		}
	}
	if len(f.store.writes) != 0 {
		t.Fatal("rejected fields must not write a status")
	}
}

func TestTransitionIssue_StaleStatusIsAConflict(t *testing.T) {
	f := newTransitionFixture(t)
	_, err := f.run(f.lead, "start", func(r *port.TransitionRequest) { r.ExpectedStatusID = f.progress })
	if apperror.KindOf(err) != apperror.KindConflict || apperror.CodeOf(err) != "issue.status_conflict" {
		t.Fatalf("stale expected status: %v, want issue.status_conflict", err)
	}
	// A concurrent transition wins between the plan and the write: the guarded update affects no row.
	f.store.raceOnce = f.review
	_, err = f.run(f.lead, "start", func(r *port.TransitionRequest) { r.ExpectedStatusID = f.todo })
	if apperror.CodeOf(err) != "issue.status_conflict" {
		t.Fatalf("lost race: %v, want issue.status_conflict", err)
	}
	if len(f.store.writes) != 0 {
		t.Fatal("a conflicting transition must not write history")
	}
}

func TestTransitionIssue_UnsupportedWithoutEngine(t *testing.T) {
	f := newPatchFixture(t)
	if _, err := f.svc.TransitionIssue(context.Background(), port.TransitionRequest{IssueID: f.issue.ID, ActorID: f.lead}); apperror.CodeOf(err) != "issue.transition_unsupported" {
		t.Fatalf("no engine: %v", err)
	}
	if _, err := f.svc.ListTransitions(context.Background(), f.issue.ID, f.lead, "user"); apperror.CodeOf(err) != "issue.transition_unsupported" {
		t.Fatalf("list without engine: %v", err)
	}
}

func TestListTransitionsAndViewShowWhatTheCallerMayRun(t *testing.T) {
	f := newTransitionFixture(t)
	ctx := context.Background()
	options, err := f.svc.ListTransitions(ctx, f.issue.ID, f.lead, "user")
	if err != nil {
		t.Fatal(err)
	}
	if len(options) != 2 || options[0].ID != "start" || options[1].ID != "complete" {
		t.Fatalf("options = %+v, want start and complete", options)
	}
	if _, err := f.svc.ListTransitions(ctx, f.issue.ID, f.viewer, "user"); apperror.KindOf(err) != apperror.KindForbidden {
		t.Fatalf("viewer list: %v, want forbidden", err)
	}
	if _, err := f.svc.ListTransitions(ctx, f.issue.ID, uuid.New(), "user"); apperror.KindOf(err) != apperror.KindNotFound {
		t.Fatalf("stranger list: %v, want not found", err)
	}
	view, err := f.svc.GetView(ctx, f.issue.ID, f.lead, "user")
	if err != nil {
		t.Fatal(err)
	}
	if len(view.Transitions) != 2 || view.Transitions[0].To != domain.StatusInProgress || view.Transitions[0].ToStatusID != f.progress {
		t.Fatalf("view transitions = %+v", view.Transitions)
	}
	viewerView, err := f.svc.GetView(ctx, f.issue.ID, f.viewer, "user")
	if err != nil || len(viewerView.Transitions) != 0 {
		t.Fatalf("viewer view transitions = %+v err %v", viewerView.Transitions, err)
	}
}

func TestMoveIssueToColumn(t *testing.T) {
	f := newTransitionFixture(t)
	ctx := context.Background()
	column, noWay, other := uuid.New(), uuid.New(), uuid.New()
	f.engine.columns[column] = []uuid.UUID{f.progress}
	f.engine.columns[noWay] = []uuid.UUID{f.review}

	if err := f.svc.MoveIssueToColumn(ctx, f.lead, "user", uuid.New(), f.issue.ID, column); !errors.Is(err, domain.ErrIssueNotFound) {
		t.Fatalf("other project: %v, want not found", err)
	}
	if err := f.svc.MoveIssueToColumn(ctx, f.lead, "user", f.issue.ProjectID, f.issue.ID, noWay); apperror.CodeOf(err) != "issue.transition_unavailable" || apperror.KindOf(err) != apperror.KindConflict {
		t.Fatalf("unreachable column: %v, want 409 issue.transition_unavailable", err)
	}
	if err := f.svc.MoveIssueToColumn(ctx, f.lead, "user", f.issue.ProjectID, f.issue.ID, other); apperror.CodeOf(err) != "issue.transition_unavailable" {
		t.Fatalf("unbound column: %v, want issue.transition_unavailable", err)
	}
	if err := f.svc.MoveIssueToColumn(ctx, f.viewer, "user", f.issue.ProjectID, f.issue.ID, column); apperror.KindOf(err) != apperror.KindForbidden {
		t.Fatalf("viewer: %v, want forbidden", err)
	}
	if err := f.svc.MoveIssueToColumn(ctx, f.lead, "user", f.issue.ProjectID, f.issue.ID, column); err != nil {
		t.Fatal(err)
	}
	if got := f.repo.issues[f.issue.ID].StatusID; got != f.progress {
		t.Fatalf("status after the column move = %v, want in progress", got)
	}
	// The next move needs a comment on the review transition, so the column move reports a validation failure.
	f.engine.columns[noWay] = []uuid.UUID{f.review}
	if err := f.svc.MoveIssueToColumn(ctx, f.lead, "user", f.issue.ProjectID, f.issue.ID, noWay); apperror.CodeOf(err) != "workflow.validation_failed" {
		t.Fatalf("move that needs a screen: %v, want workflow.validation_failed", err)
	}
}

func TestBulkTransitionChoosesTheTransitionByIdStatusOrLegacyValue(t *testing.T) {
	f := newTransitionFixture(t)
	ctx := context.Background()
	res, err := f.svc.BulkChange(ctx, f.lead, "user", port.BulkInput{Refs: []string{f.issue.ID.String()}, Action: domain.BulkTransition, TransitionID: "start"})
	if err != nil || len(res.Succeeded) != 1 || res.Succeeded[0].StatusID != f.progress {
		t.Fatalf("by transition id: %+v err %v", res, err)
	}
	res, err = f.svc.BulkChange(ctx, f.lead, "user", port.BulkInput{Refs: []string{f.issue.ID.String()}, Action: domain.BulkTransition, StatusID: f.done})
	if err != nil || len(res.Succeeded) != 1 || res.Succeeded[0].StatusID != f.done {
		t.Fatalf("by status id: %+v err %v", res, err)
	}
	f.engine.steps = append(f.engine.steps, fakeStep{id: "reopen", name: "Reabrir", from: []uuid.UUID{f.done}, to: f.todo, toName: "Por hacer", toCategory: "todo"})
	res, err = f.svc.BulkChange(ctx, f.lead, "user", port.BulkInput{Refs: []string{f.issue.ID.String()}, Action: domain.BulkTransition, Status: domain.StatusOpen})
	if err != nil || len(res.Succeeded) != 1 || res.Succeeded[0].StatusID != f.todo {
		t.Fatalf("by legacy status: %+v err %v", res, err)
	}
	res, err = f.svc.BulkChange(ctx, f.lead, "user", port.BulkInput{Refs: []string{f.issue.ID.String()}, Action: domain.BulkTransition, Status: domain.StatusResolved})
	if err != nil || len(res.Failed) != 1 || res.Failed[0].Code != "issue.transition_unavailable" {
		t.Fatalf("unreachable legacy status: %+v err %v", res, err)
	}
}
