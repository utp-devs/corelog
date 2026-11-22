package usecase

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"

	"github.com/jhonsferg/corelog/internal/ticket/application/port"
	"github.com/jhonsferg/corelog/internal/ticket/domain"
)

type fakeTicketRepo struct {
	tickets map[uuid.UUID]*domain.Ticket
}

func newFakeTicketRepo() *fakeTicketRepo {
	return &fakeTicketRepo{tickets: make(map[uuid.UUID]*domain.Ticket)}
}

func (r *fakeTicketRepo) Save(_ context.Context, t *domain.Ticket) error {
	copied := *t
	r.tickets[t.ID] = &copied
	return nil
}

func (r *fakeTicketRepo) FindByID(_ context.Context, id uuid.UUID) (*domain.Ticket, error) {
	t, ok := r.tickets[id]
	if !ok {
		return nil, domain.ErrTicketNotFound
	}
	copied := *t
	return &copied, nil
}

func (r *fakeTicketRepo) FindAll(_ context.Context, filter port.ListFilter) ([]*domain.Ticket, error) {
	var list []*domain.Ticket
	for _, t := range r.tickets {
		if filter.Status != "" && t.Status != filter.Status {
			continue
		}
		if filter.Priority != "" && t.Priority != filter.Priority {
			continue
		}
		copied := *t
		list = append(list, &copied)
	}
	return list, nil
}

func (r *fakeTicketRepo) Update(_ context.Context, t *domain.Ticket) error {
	if _, ok := r.tickets[t.ID]; !ok {
		return domain.ErrTicketNotFound
	}
	copied := *t
	r.tickets[t.ID] = &copied
	return nil
}

type fakeUserDirectory struct {
	userTeams map[uuid.UUID]*uuid.UUID
}

func newFakeUserDirectory() *fakeUserDirectory {
	return &fakeUserDirectory{userTeams: make(map[uuid.UUID]*uuid.UUID)}
}

func (d *fakeUserDirectory) TeamOf(_ context.Context, userID uuid.UUID) (*uuid.UUID, error) {
	teamID, ok := d.userTeams[userID]
	if !ok {
		return nil, nil
	}
	return teamID, nil
}

func TestTicketService_Create(t *testing.T) {
	ctx := context.Background()
	repo := newFakeTicketRepo()
	dir := newFakeUserDirectory()
	svc := NewTicketService(repo, dir)

	requesterID := uuid.New()

	t.Run("successfully creates ticket", func(t *testing.T) {
		ticket, err := svc.Create(ctx, port.CreateInput{
			Title:       "Login bug",
			Description: "Cannot log in with valid credentials",
			Priority:    domain.PriorityHigh,
			RequesterID: requesterID,
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if ticket.Title != "Login bug" {
			t.Errorf("got title %s, want 'Login bug'", ticket.Title)
		}
		if ticket.Status != domain.StatusOpen {
			t.Errorf("got status %v, want %v", ticket.Status, domain.StatusOpen)
		}

		// Verify stored in repo
		stored, err := repo.FindByID(ctx, ticket.ID)
		if err != nil {
			t.Fatalf("expected ticket to be found in repo: %v", err)
		}
		if stored.Title != "Login bug" {
			t.Errorf("got stored title %s, want 'Login bug'", stored.Title)
		}
	})

	t.Run("fails on invalid domain data", func(t *testing.T) {
		_, err := svc.Create(ctx, port.CreateInput{
			Title:       "",
			Description: "Desc",
			Priority:    domain.PriorityLow,
			RequesterID: requesterID,
		})
		if !errors.Is(err, domain.ErrInvalidTitle) {
			t.Errorf("expected ErrInvalidTitle, got %v", err)
		}
	})
}

func TestTicketService_GetAndList(t *testing.T) {
	ctx := context.Background()
	repo := newFakeTicketRepo()
	dir := newFakeUserDirectory()
	svc := NewTicketService(repo, dir)

	requesterID := uuid.New()
	t1, _ := svc.Create(ctx, port.CreateInput{
		Title:       "Issue 1",
		Description: "Desc 1",
		Priority:    domain.PriorityLow,
		RequesterID: requesterID,
	})
	t2, _ := svc.Create(ctx, port.CreateInput{
		Title:       "Issue 2",
		Description: "Desc 2",
		Priority:    domain.PriorityCritical,
		RequesterID: requesterID,
	})
	_ = svc.repo.Update(ctx, &domain.Ticket{
		ID:          t2.ID,
		Title:       t2.Title,
		Description: t2.Description,
		Status:      domain.StatusResolved,
		Priority:    t2.Priority,
		RequesterID: requesterID,
		CreatedAt:   t2.CreatedAt,
		UpdatedAt:   t2.UpdatedAt,
	})

	t.Run("Get returns ticket by ID", func(t *testing.T) {
		got, err := svc.Get(ctx, t1.ID)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got.ID != t1.ID {
			t.Errorf("got ID %v, want %v", got.ID, t1.ID)
		}
	})

	t.Run("Get returns not found error for non-existent ticket", func(t *testing.T) {
		_, err := svc.Get(ctx, uuid.New())
		if !errors.Is(err, domain.ErrTicketNotFound) {
			t.Errorf("expected ErrTicketNotFound, got %v", err)
		}
	})

	t.Run("List with default pagination and filter", func(t *testing.T) {
		items, err := svc.List(ctx, port.ListFilter{Status: domain.StatusResolved})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(items) != 1 {
			t.Fatalf("got %d items, want 1", len(items))
		}
		if items[0].ID != t2.ID {
			t.Errorf("got ticket ID %v, want %v", items[0].ID, t2.ID)
		}
	})
}

func TestTicketService_UpdateStatus(t *testing.T) {
	ctx := context.Background()
	repo := newFakeTicketRepo()
	dir := newFakeUserDirectory()
	svc := NewTicketService(repo, dir)

	requesterID := uuid.New()
	t1, _ := svc.Create(ctx, port.CreateInput{
		Title:       "Issue",
		Description: "Desc",
		Priority:    domain.PriorityMedium,
		RequesterID: requesterID,
	})

	t.Run("valid transition", func(t *testing.T) {
		updated, err := svc.UpdateStatus(ctx, t1.ID, domain.StatusInProgress)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if updated.Status != domain.StatusInProgress {
			t.Errorf("got status %v, want %v", updated.Status, domain.StatusInProgress)
		}

		stored, _ := repo.FindByID(ctx, t1.ID)
		if stored.Status != domain.StatusInProgress {
			t.Errorf("got stored status %v, want %v", stored.Status, domain.StatusInProgress)
		}
	})

	t.Run("illegal transition", func(t *testing.T) {
		// InProgress -> Open is illegal
		_, err := svc.UpdateStatus(ctx, t1.ID, domain.StatusOpen)
		if !errors.Is(err, domain.ErrIllegalTransition) {
			t.Errorf("expected ErrIllegalTransition, got %v", err)
		}
	})

	t.Run("ticket not found", func(t *testing.T) {
		_, err := svc.UpdateStatus(ctx, uuid.New(), domain.StatusClosed)
		if !errors.Is(err, domain.ErrTicketNotFound) {
			t.Errorf("expected ErrTicketNotFound, got %v", err)
		}
	})
}

func TestTicketService_UpdateRank(t *testing.T) {
	ctx := context.Background()
	repo := newFakeTicketRepo()
	dir := newFakeUserDirectory()
	svc := NewTicketService(repo, dir)

	ticket, _ := svc.Create(ctx, port.CreateInput{
		Title:       "Ranked issue",
		Description: "Desc",
		Priority:    domain.PriorityMedium,
		RequesterID: uuid.New(),
	})

	updated, err := svc.UpdateRank(ctx, ticket.ID, "00000001")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if updated.Rank != "00000001" {
		t.Fatalf("got rank %q, want %q", updated.Rank, "00000001")
	}
}

type fakeWorkflowRepo struct {
	allowed bool
}

func (r *fakeWorkflowRepo) CanTransition(_ context.Context, _ uuid.UUID, _, _ domain.Status, _ string) (bool, error) {
	return r.allowed, nil
}

func TestTicketService_WorkflowPolicy(t *testing.T) {
	ctx := context.Background()
	repo := newFakeTicketRepo()
	dir := newFakeUserDirectory()
	workflowRepo := &fakeWorkflowRepo{allowed: false}
	svc := NewTicketService(repo, dir, workflowRepo)

	ticket, _ := svc.Create(ctx, port.CreateInput{
		Title:       "Workflow issue",
		Description: "Desc",
		Priority:    domain.PriorityMedium,
		RequesterID: uuid.New(),
	})

	_, err := svc.UpdateStatus(ctx, ticket.ID, domain.StatusInProgress, "requester")
	if !errors.Is(err, domain.ErrIllegalTransition) {
		t.Fatalf("expected ErrIllegalTransition, got %v", err)
	}

	workflowRepo.allowed = true
	updated, err := svc.UpdateStatus(ctx, ticket.ID, domain.StatusInProgress, "admin")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if updated.Status != domain.StatusInProgress {
		t.Fatalf("got status %v, want %v", updated.Status, domain.StatusInProgress)
	}
}

func TestTicketService_Assign(t *testing.T) {
	ctx := context.Background()
	repo := newFakeTicketRepo()
	dir := newFakeUserDirectory()
	svc := NewTicketService(repo, dir)

	teamA := uuid.New()
	teamB := uuid.New()

	assigner := uuid.New()
	assigneeTeamA := uuid.New()
	assigneeTeamB := uuid.New()
	userNoTeam := uuid.New()

	dir.userTeams[assigner] = &teamA
	dir.userTeams[assigneeTeamA] = &teamA
	dir.userTeams[assigneeTeamB] = &teamB

	ticket, _ := svc.Create(ctx, port.CreateInput{
		Title:       "Ticket to assign",
		Description: "Desc",
		Priority:    domain.PriorityLow,
		RequesterID: assigner,
	})

	t.Run("assigning to member of same team succeeds and moves open to in_progress", func(t *testing.T) {
		assigned, err := svc.Assign(ctx, ticket.ID, assigner, assigneeTeamA)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if assigned.AssigneeID == nil || *assigned.AssigneeID != assigneeTeamA {
			t.Errorf("got assignee %v, want %v", assigned.AssigneeID, assigneeTeamA)
		}
		if assigned.Status != domain.StatusInProgress {
			t.Errorf("got status %v, want %v", assigned.Status, domain.StatusInProgress)
		}
	})

	t.Run("rejects cross-team assignment", func(t *testing.T) {
		_, err := svc.Assign(ctx, ticket.ID, assigner, assigneeTeamB)
		if !errors.Is(err, domain.ErrCrossTeamAssignment) {
			t.Errorf("expected ErrCrossTeamAssignment, got %v", err)
		}
	})

	t.Run("rejects assignment when assignee has no team", func(t *testing.T) {
		_, err := svc.Assign(ctx, ticket.ID, assigner, userNoTeam)
		if !errors.Is(err, domain.ErrCrossTeamAssignment) {
			t.Errorf("expected ErrCrossTeamAssignment, got %v", err)
		}
	})

	t.Run("returns not found for non-existent ticket", func(t *testing.T) {
		_, err := svc.Assign(ctx, uuid.New(), assigner, assigneeTeamA)
		if !errors.Is(err, domain.ErrTicketNotFound) {
			t.Errorf("expected ErrTicketNotFound, got %v", err)
		}
	})
}
