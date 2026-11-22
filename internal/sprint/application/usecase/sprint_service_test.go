package usecase

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/jhonsferg/corelog/internal/platform/apperror"
	"github.com/jhonsferg/corelog/internal/sprint/application/port"
	"github.com/jhonsferg/corelog/internal/sprint/domain"
)

// memoryRepo es un repositorio en memoria para los casos de uso.
type memoryRepo struct {
	sprints map[uuid.UUID]*domain.Sprint
	issues  map[uuid.UUID]*memoryIssue
}

type memoryIssue struct {
	projectID uuid.UUID
	sprintID  *uuid.UUID
	ref       port.IssueRef
}

func newMemoryRepo() *memoryRepo {
	return &memoryRepo{sprints: map[uuid.UUID]*domain.Sprint{}, issues: map[uuid.UUID]*memoryIssue{}}
}

func (r *memoryRepo) Save(_ context.Context, sprint *domain.Sprint) error {
	sprint.ID = uuid.New()
	r.sprints[sprint.ID] = sprint
	return nil
}

func (r *memoryRepo) FindByID(_ context.Context, id uuid.UUID) (*domain.Sprint, error) {
	sprint, ok := r.sprints[id]
	if !ok {
		return nil, domain.ErrSprintNotFound
	}
	return sprint, nil
}

func (r *memoryRepo) ListByProject(_ context.Context, projectID uuid.UUID) ([]*domain.Sprint, error) {
	var out []*domain.Sprint
	for _, sprint := range r.sprints {
		if sprint.ProjectID == projectID {
			out = append(out, sprint)
		}
	}
	return out, nil
}

func (r *memoryRepo) Update(_ context.Context, sprint *domain.Sprint) error {
	r.sprints[sprint.ID] = sprint
	return nil
}

func (r *memoryRepo) HasActive(_ context.Context, projectID, exceptID uuid.UUID) (bool, error) {
	for _, sprint := range r.sprints {
		if sprint.ProjectID == projectID && sprint.Status == domain.StatusActive && sprint.ID != exceptID {
			return true, nil
		}
	}
	return false, nil
}

func (r *memoryRepo) SetIssueSprint(_ context.Context, projectID, issueID uuid.UUID, sprintID *uuid.UUID) error {
	issue, ok := r.issues[issueID]
	if !ok || issue.projectID != projectID {
		return domain.ErrIssueNotInProject
	}
	issue.sprintID = sprintID
	return nil
}

func (r *memoryRepo) RemoveIssueFromSprint(_ context.Context, projectID, sprintID, issueID uuid.UUID) error {
	issue, ok := r.issues[issueID]
	if !ok || issue.projectID != projectID || issue.sprintID == nil || *issue.sprintID != sprintID {
		return domain.ErrIssueNotInProject
	}
	issue.sprintID = nil
	return nil
}

func (r *memoryRepo) ListIssues(_ context.Context, sprintID uuid.UUID) ([]port.IssueRef, error) {
	var out []port.IssueRef
	for _, issue := range r.issues {
		if issue.sprintID != nil && *issue.sprintID == sprintID {
			out = append(out, issue.ref)
		}
	}
	return out, nil
}

func (r *memoryRepo) MoveUnfinished(_ context.Context, sprintID uuid.UUID, target *uuid.UUID) error {
	for _, issue := range r.issues {
		if issue.sprintID != nil && *issue.sprintID == sprintID && issue.ref.Status != string(domain.StatusClosed) {
			issue.sprintID = target
		}
	}
	return nil
}

func (r *memoryRepo) Velocity(context.Context, uuid.UUID, int) ([]port.VelocityPoint, error) {
	return nil, nil
}

// roleAuthorizer permite gestionar sprints solo al lead y al admin, y ver a cualquier miembro.
type roleAuthorizer struct{}

func (roleAuthorizer) Authorize(_ context.Context, action string, _ uuid.UUID, globalRole string, _ uuid.UUID) error {
	if globalRole == "admin" || globalRole == "lead" {
		return nil
	}
	if action == port.ActionViewProject {
		return nil
	}
	return apperror.New(apperror.KindForbidden, "forbidden project operation")
}

func TestSprintServiceStartAllowsOneActivePerProject(t *testing.T) {
	ctx := context.Background()
	project := uuid.New()
	repo := newMemoryRepo()
	svc := NewSprintService(repo, roleAuthorizer{})

	first, err := svc.Create(ctx, uuid.New(), "lead", project, "Sprint 1", "", nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	second, err := svc.Create(ctx, uuid.New(), "lead", project, "Sprint 2", "", nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Start(ctx, uuid.New(), "lead", project, first.ID); err != nil {
		t.Fatalf("first sprint should start: %v", err)
	}
	if _, err := svc.Start(ctx, uuid.New(), "lead", project, second.ID); !errors.Is(err, domain.ErrActiveSprintExists) {
		t.Fatalf("second active sprint: got %v, want ErrActiveSprintExists", err)
	}
	if _, err := svc.Start(ctx, uuid.New(), "lead", project, first.ID); !errors.Is(err, domain.ErrSprintNotPlanned) {
		t.Fatalf("restarting a sprint: got %v, want ErrSprintNotPlanned", err)
	}
}

func TestSprintServiceCloseMovesUnfinishedToBacklogOrTarget(t *testing.T) {
	ctx := context.Background()
	project := uuid.New()
	repo := newMemoryRepo()
	svc := NewSprintService(repo, roleAuthorizer{})

	active, _ := svc.Create(ctx, uuid.New(), "lead", project, "Activo", "", nil, nil)
	planned, _ := svc.Create(ctx, uuid.New(), "lead", project, "Siguiente", "", nil, nil)
	foreign, _ := svc.Create(ctx, uuid.New(), "lead", uuid.New(), "Otro proyecto", "", nil, nil)
	_, _ = svc.Start(ctx, uuid.New(), "lead", project, active.ID)

	open := uuid.New()
	done := uuid.New()
	repo.issues[open] = &memoryIssue{projectID: project, sprintID: &active.ID, ref: port.IssueRef{ID: open, Status: "in_progress"}}
	repo.issues[done] = &memoryIssue{projectID: project, sprintID: &active.ID, ref: port.IssueRef{ID: done, Status: "closed"}}

	if _, err := svc.Close(ctx, uuid.New(), "lead", project, active.ID, port.CloseInput{Unfinished: port.UnfinishedToSprint, TargetSprintID: &foreign.ID}); !errors.Is(err, domain.ErrInvalidTargetSprint) {
		t.Fatalf("target from another project: got %v", err)
	}
	if _, err := svc.Close(ctx, uuid.New(), "lead", project, active.ID, port.CloseInput{Unfinished: port.UnfinishedToSprint}); !errors.Is(err, domain.ErrInvalidCloseOption) {
		t.Fatalf("sprint option without target: got %v", err)
	}

	closed, err := svc.Close(ctx, uuid.New(), "lead", project, active.ID, port.CloseInput{Unfinished: port.UnfinishedToSprint, TargetSprintID: &planned.ID})
	if err != nil {
		t.Fatalf("close to a planned sprint: %v", err)
	}
	if closed.Status != domain.StatusClosed || closed.ClosedAt == nil {
		t.Fatalf("sprint not closed: %+v", closed)
	}
	if repo.issues[open].sprintID == nil || *repo.issues[open].sprintID != planned.ID {
		t.Fatal("unfinished issue must move to the target sprint")
	}
	if repo.issues[done].sprintID == nil || *repo.issues[done].sprintID != active.ID {
		t.Fatal("closed issue must stay in the closed sprint")
	}
}

func TestSprintServiceAssignRequiresManagerAndOpenSprint(t *testing.T) {
	ctx := context.Background()
	project := uuid.New()
	repo := newMemoryRepo()
	svc := NewSprintService(repo, roleAuthorizer{})
	sprint, _ := svc.Create(ctx, uuid.New(), "lead", project, "Sprint", "", nil, nil)
	issue := uuid.New()
	repo.issues[issue] = &memoryIssue{projectID: project, ref: port.IssueRef{ID: issue, Status: "open"}}

	if err := svc.AssignIssue(ctx, uuid.New(), "requester", project, sprint.ID, issue); apperror.KindOf(err) != apperror.KindForbidden {
		t.Fatalf("viewer should not assign: got %v", err)
	}
	if err := svc.AssignIssue(ctx, uuid.New(), "lead", project, sprint.ID, issue); err != nil {
		t.Fatalf("lead assigns: %v", err)
	}
	if repo.issues[issue].sprintID == nil || *repo.issues[issue].sprintID != sprint.ID {
		t.Fatal("issue not assigned")
	}
	if err := svc.AssignIssue(ctx, uuid.New(), "lead", uuid.New(), sprint.ID, issue); !errors.Is(err, domain.ErrSprintNotFound) {
		t.Fatalf("sprint of another project must not be visible: got %v", err)
	}
	sprint.Status = domain.StatusClosed
	if err := svc.AssignIssue(ctx, uuid.New(), "lead", project, sprint.ID, issue); !errors.Is(err, domain.ErrSprintClosed) {
		t.Fatalf("closed sprint must not change: got %v", err)
	}
}

func TestSprintServiceBurndownTracksRemainingPoints(t *testing.T) {
	ctx := context.Background()
	project := uuid.New()
	repo := newMemoryRepo()
	svc := NewSprintService(repo, roleAuthorizer{})
	start := time.Date(2026, 11, 2, 0, 0, 0, 0, time.UTC)
	end := start.AddDate(0, 0, 2)
	sprint, _ := svc.Create(ctx, uuid.New(), "lead", project, "Sprint", "", &start, &end)

	three, five := 3.0, 5.0
	doneDay := start.AddDate(0, 0, 1).Add(5 * time.Hour)
	first, second := uuid.New(), uuid.New()
	repo.issues[first] = &memoryIssue{projectID: project, sprintID: &sprint.ID, ref: port.IssueRef{ID: first, Status: "closed", StoryPoints: &three, ClosedAt: &doneDay}}
	repo.issues[second] = &memoryIssue{projectID: project, sprintID: &sprint.ID, ref: port.IssueRef{ID: second, Status: "open", StoryPoints: &five}}

	points, err := svc.Burndown(ctx, uuid.New(), "requester", project, sprint.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(points) != 3 {
		t.Fatalf("days = %d, want 3", len(points))
	}
	if points[0].Remaining != 8 || points[1].Remaining != 5 || points[2].Remaining != 5 {
		t.Fatalf("remaining = %v, %v, %v; want 8, 5, 5", points[0].Remaining, points[1].Remaining, points[2].Remaining)
	}
	if points[0].Ideal != 8 || points[2].Ideal != 0 {
		t.Fatalf("ideal line = %v..%v, want 8..0", points[0].Ideal, points[2].Ideal)
	}

	undated, _ := svc.Create(ctx, uuid.New(), "lead", project, "Sin fechas", "", nil, nil)
	if _, err := svc.Burndown(ctx, uuid.New(), "requester", project, undated.ID); !errors.Is(err, domain.ErrSprintWithoutDates) {
		t.Fatalf("burndown without dates: got %v", err)
	}
}
