package usecase_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/jhonsferg/corelog/internal/platform/apperror"
	"github.com/jhonsferg/corelog/internal/platform/clock"
	"github.com/jhonsferg/corelog/internal/workflow"
	"github.com/jhonsferg/corelog/internal/workflow/application/port"
	"github.com/jhonsferg/corelog/internal/workflow/application/usecase"
	"github.com/jhonsferg/corelog/internal/workflow/domain"
)

// memRepo is an in-memory repository.
type memRepo struct {
	workflows map[uuid.UUID]port.Record
	statuses  []port.OrgStatus
	columns   map[uuid.UUID][]uuid.UUID
	savedAt   time.Time
}

func newMemRepo() *memRepo {
	return &memRepo{workflows: map[uuid.UUID]port.Record{}, columns: map[uuid.UUID][]uuid.UUID{}}
}

func (r *memRepo) FindWorkflow(_ context.Context, projectID uuid.UUID) (port.Record, error) {
	record, ok := r.workflows[projectID]
	if !ok {
		return port.Record{}, port.ErrWorkflowNotFound
	}
	return record, nil
}

func (r *memRepo) SaveWorkflow(_ context.Context, projectID uuid.UUID, w domain.Workflow, expected int, at time.Time) (int, error) {
	record, ok := r.workflows[projectID]
	if !ok {
		return 0, port.ErrWorkflowNotFound
	}
	if record.Version != expected {
		return 0, port.ErrVersionConflict
	}
	record.Workflow, record.Version = w, record.Version+1
	r.workflows[projectID], r.savedAt = record, at
	return record.Version, nil
}

func (r *memRepo) ListStatuses(_ context.Context, orgID uuid.UUID) ([]port.OrgStatus, error) {
	var out []port.OrgStatus
	for _, s := range r.statuses {
		if s.OrganizationID == orgID {
			out = append(out, s)
		}
	}
	return out, nil
}

func (r *memRepo) CreateStatus(_ context.Context, s port.OrgStatus) (port.OrgStatus, error) {
	for _, existing := range r.statuses {
		if existing.OrganizationID == s.OrganizationID && strings.EqualFold(existing.Name, s.Name) {
			return port.OrgStatus{}, port.ErrStatusNameTaken
		}
	}
	s.ID, s.Position = uuid.New(), len(r.statuses)
	r.statuses = append(r.statuses, s)
	return s, nil
}

func (r *memRepo) UpdateStatus(_ context.Context, s port.OrgStatus) error {
	for i := range r.statuses {
		if r.statuses[i].ID == s.ID {
			r.statuses[i] = s
			return nil
		}
	}
	return port.ErrStatusNotFound
}

func (r *memRepo) DeleteStatus(_ context.Context, _ uuid.UUID, id uuid.UUID) error {
	for i := range r.statuses {
		if r.statuses[i].ID == id {
			r.statuses = append(r.statuses[:i], r.statuses[i+1:]...)
			return nil
		}
	}
	return port.ErrStatusNotFound
}

func (r *memRepo) ColumnStatusIDs(_ context.Context, columnID uuid.UUID) ([]uuid.UUID, error) {
	return r.columns[columnID], nil
}

func newService(t *testing.T) (*usecase.Service, *memRepo, uuid.UUID, domain.Workflow) {
	t.Helper()
	repo := newMemRepo()
	w, err := domain.TemplateWorkflow(domain.TemplateScrum)
	if err != nil {
		t.Fatal(err)
	}
	project := uuid.New()
	repo.workflows[project] = port.Record{ProjectID: project, Workflow: w, Version: 1}
	svc := workflow.NewModule(repo, &clock.FakeClock{T: time.Date(2026, 11, 27, 9, 0, 0, 0, time.UTC)}).Service
	return svc, repo, project, w
}

func lead() domain.ActorContext {
	return domain.ActorContext{UserID: "u1", Role: domain.RoleLead, Permissions: []string{"transition_issue"}}
}

func TestGetWorkflowAndNotFound(t *testing.T) {
	svc, _, project, want := newService(t)
	got, version, err := svc.GetWorkflow(context.Background(), project)
	if err != nil || version != 1 || len(got.Statuses) != len(want.Statuses) {
		t.Fatalf("GetWorkflow = %d statuses, version %d, err %v", len(got.Statuses), version, err)
	}
	if _, _, err := svc.GetWorkflow(context.Background(), uuid.New()); apperror.CodeOf(err) != "workflow.not_found" || apperror.KindOf(err) != apperror.KindNotFound {
		t.Fatalf("unknown project: %v, want workflow.not_found", err)
	}
	if _, err := svc.Available(context.Background(), uuid.New(), domain.IssueSnapshot{}, lead()); apperror.CodeOf(err) != "workflow.not_found" {
		t.Fatalf("Available on an unknown project: %v", err)
	}
	if _, _, err := svc.Plan(context.Background(), uuid.New(), domain.IssueSnapshot{}, lead(), "start", domain.TransitionInput{}); apperror.CodeOf(err) != "workflow.not_found" {
		t.Fatalf("Plan on an unknown project: %v", err)
	}
}

func TestAvailableAndPlanFollowTheEngine(t *testing.T) {
	svc, _, project, w := newService(t)
	ctx := context.Background()
	issue := domain.IssueSnapshot{StatusID: w.Initial}

	available, err := svc.Available(ctx, project, issue, lead())
	if err != nil {
		t.Fatal(err)
	}
	ids := []string{}
	for _, a := range available {
		ids = append(ids, a.TransitionID)
	}
	if strings.Join(ids, ",") != "start,complete" {
		t.Fatalf("available from the initial status = %v, want start and complete", ids)
	}

	plan, name, err := svc.Plan(ctx, project, issue, lead(), "complete", domain.TransitionInput{})
	if err != nil || name != "Completar" || plan.ToStatusID != "done" || plan.Resolution.Action != domain.ResolutionSet || !plan.Resolution.UseDefaultDone {
		t.Fatalf("plan = %+v name %q err %v", plan, name, err)
	}
	if _, _, err := svc.Plan(ctx, project, issue, lead(), "approve", domain.TransitionInput{}); apperror.CodeOf(err) != domain.CodeTransitionNotAllowed {
		t.Fatalf("transition from the wrong status: %v", err)
	}
	if _, _, err := svc.Plan(ctx, project, issue, lead(), "missing", domain.TransitionInput{}); apperror.CodeOf(err) != domain.CodeTransitionNotAllowed {
		t.Fatalf("unknown transition: %v", err)
	}
}

func TestStatusOf(t *testing.T) {
	svc, _, project, _ := newService(t)
	status, err := svc.StatusOf(context.Background(), project, "in_review")
	if err != nil || status.Name != "En revisión" || status.Category != domain.CategoryInProgress {
		t.Fatalf("StatusOf = %+v, %v", status, err)
	}
	if _, err := svc.StatusOf(context.Background(), project, "nope"); apperror.CodeOf(err) != "workflow.status_not_found" {
		t.Fatalf("unknown status: %v", err)
	}
	if _, err := svc.StatusOf(context.Background(), uuid.New(), "todo"); apperror.CodeOf(err) != "workflow.not_found" {
		t.Fatalf("unknown project: %v", err)
	}
}

func TestReplaceWorkflowValidatesAndChecksTheVersion(t *testing.T) {
	svc, repo, project, w := newService(t)
	ctx := context.Background()

	broken := w
	broken.Initial = "ghost"
	if _, err := svc.ReplaceWorkflow(ctx, project, broken, 1); apperror.CodeOf(err) != domain.CodeInvalidWorkflow {
		t.Fatalf("invalid workflow: %v, want %s", err, domain.CodeInvalidWorkflow)
	}
	version, err := svc.ReplaceWorkflow(ctx, project, w, 1)
	if err != nil || version != 2 {
		t.Fatalf("replace = %d, %v", version, err)
	}
	if !repo.savedAt.Equal(time.Date(2026, 11, 27, 9, 0, 0, 0, time.UTC)) {
		t.Fatalf("saved at %v, want the injected clock time", repo.savedAt)
	}
	if _, err := svc.ReplaceWorkflow(ctx, project, w, 1); apperror.CodeOf(err) != "workflow.version_conflict" || apperror.KindOf(err) != apperror.KindConflict {
		t.Fatalf("stale version: %v, want workflow.version_conflict", err)
	}
}

func TestColumnStatusIDs(t *testing.T) {
	svc, repo, _, _ := newService(t)
	column, status := uuid.New(), uuid.New()
	repo.columns[column] = []uuid.UUID{status}
	ids, err := svc.ColumnStatusIDs(context.Background(), column)
	if err != nil || len(ids) != 1 || ids[0] != status {
		t.Fatalf("ColumnStatusIDs = %v, %v", ids, err)
	}
}

func TestStatusCatalogCRUD(t *testing.T) {
	svc, _, _, _ := newService(t)
	ctx := context.Background()
	org := uuid.New()

	created, err := svc.CreateStatus(ctx, org, "  En pruebas  ", domain.CategoryInProgress)
	if err != nil || created.Name != "En pruebas" || created.ID == uuid.Nil {
		t.Fatalf("CreateStatus = %+v, %v", created, err)
	}
	if _, err := svc.CreateStatus(ctx, org, "EN PRUEBAS", domain.CategoryTodo); apperror.CodeOf(err) != "workflow.status_name_taken" {
		t.Fatalf("duplicate name: %v", err)
	}
	for name, in := range map[string]struct {
		name     string
		category domain.Category
		code     string
	}{
		"blank":        {"   ", domain.CategoryTodo, "workflow.status_name_invalid"},
		"too long":     {strings.Repeat("x", 61), domain.CategoryTodo, "workflow.status_name_invalid"},
		"bad category": {"Nuevo", domain.Category("later"), "workflow.invalid_category"},
	} {
		if _, err := svc.CreateStatus(ctx, org, in.name, in.category); apperror.CodeOf(err) != in.code || apperror.KindOf(err) != apperror.KindValidation {
			t.Errorf("%s: %v, want %s", name, err, in.code)
		}
	}

	list, err := svc.ListStatuses(ctx, org)
	if err != nil || len(list) != 1 {
		t.Fatalf("ListStatuses = %v, %v", list, err)
	}
	created.Name, created.Category = "QA", domain.CategoryDone
	if err := svc.UpdateStatus(ctx, created); err != nil {
		t.Fatal(err)
	}
	created.Name = ""
	if err := svc.UpdateStatus(ctx, created); apperror.CodeOf(err) != "workflow.status_name_invalid" {
		t.Fatalf("blank rename: %v", err)
	}
	if err := svc.UpdateStatus(ctx, port.OrgStatus{ID: uuid.New(), OrganizationID: org, Name: "X", Category: domain.CategoryTodo}); !errors.Is(err, port.ErrStatusNotFound) {
		t.Fatalf("update of an unknown status: %v", err)
	}
	if err := svc.DeleteStatus(ctx, org, created.ID); err != nil {
		t.Fatal(err)
	}
	if err := svc.DeleteStatus(ctx, org, created.ID); apperror.CodeOf(err) != "workflow.status_not_found" {
		t.Fatalf("second delete: %v", err)
	}
}

func TestNewServiceDefaultsToTheSystemClock(t *testing.T) {
	repo := newMemRepo()
	w, _ := domain.TemplateWorkflow(domain.TemplateTaskTracking)
	project := uuid.New()
	repo.workflows[project] = port.Record{ProjectID: project, Workflow: w, Version: 1}
	svc := usecase.NewService(repo, nil)
	before := time.Now().Add(-time.Second)
	if _, err := svc.ReplaceWorkflow(context.Background(), project, w, 1); err != nil {
		t.Fatal(err)
	}
	if repo.savedAt.Before(before) {
		t.Fatalf("saved at %v, want the current time", repo.savedAt)
	}
}
