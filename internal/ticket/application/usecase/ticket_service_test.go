package usecase

import (
	"context"
	"errors"
	"math"
	"testing"

	"github.com/google/uuid"
	authport "github.com/jhonsferg/corelog/internal/authorization/application/port"
	authusecase "github.com/jhonsferg/corelog/internal/authorization/application/usecase"
	authdomain "github.com/jhonsferg/corelog/internal/authorization/domain"

	"github.com/jhonsferg/corelog/internal/platform/apperror"
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
	if t.ID == uuid.Nil {
		t.ID = uuid.New()
	}
	copied := *t
	r.tickets[t.ID] = &copied
	return nil
}

func (r *fakeTicketRepo) FindByID(_ context.Context, id uuid.UUID) (*domain.Ticket, error) {
	t, ok := r.tickets[id]
	if !ok {
		return nil, domain.ErrTicketNotFound
	}
	if t.ID == uuid.Nil {
		t.ID = uuid.New()
	}
	copied := *t
	return &copied, nil
}

func (r *fakeTicketRepo) Count(ctx context.Context, filter port.ListFilter) (int, error) {
	unpaged := filter
	unpaged.Page = 1
	unpaged.PageSize = math.MaxInt32
	all, err := r.FindAll(ctx, unpaged)
	if err != nil {
		return 0, err
	}
	return len(all), nil
}

func (r *fakeTicketRepo) FindAll(_ context.Context, filter port.ListFilter) ([]*domain.Ticket, error) {
	var list []*domain.Ticket
	projectIDs := make(map[uuid.UUID]struct{}, len(filter.ProjectIDs))
	for _, projectID := range filter.ProjectIDs {
		projectIDs[projectID] = struct{}{}
	}
	for _, t := range r.tickets {
		if len(projectIDs) > 0 {
			if _, ok := projectIDs[t.ProjectID]; !ok {
				continue
			}
		}
		if filter.Status != "" && t.Status != filter.Status {
			continue
		}
		if filter.Priority != "" && t.Priority != filter.Priority {
			continue
		}
		copied := *t
		list = append(list, &copied)
	}
	if filter.PageSize > 0 {
		start := (filter.Page - 1) * filter.PageSize
		if start < 0 {
			start = 0
		}
		if start > len(list) {
			start = len(list)
		}
		end := start + filter.PageSize
		if end > len(list) {
			end = len(list)
		}
		list = list[start:end]
	}
	return list, nil
}

func (r *fakeTicketRepo) Update(_ context.Context, t *domain.Ticket) error {
	if _, ok := r.tickets[t.ID]; !ok {
		return domain.ErrTicketNotFound
	}
	if t.ID == uuid.Nil {
		t.ID = uuid.New()
	}
	copied := *t
	r.tickets[t.ID] = &copied
	return nil
}

type fakeUserDirectory struct {
	userTeams map[uuid.UUID]*uuid.UUID
	names     map[uuid.UUID]string
}

type fakeProjectRoleResolver struct {
	role *string
}

func (r fakeProjectRoleResolver) FindRole(_ context.Context, projectID, _ uuid.UUID) (*string, error) {
	if projectID != uuid.MustParse("00000000-0000-4000-8000-000000000001") {
		return nil, nil
	}
	return r.role, nil
}

func (r fakeProjectRoleResolver) ListProjectIDs(context.Context, uuid.UUID) ([]uuid.UUID, error) {
	if r.role == nil {
		return nil, nil
	}
	return []uuid.UUID{uuid.MustParse("00000000-0000-4000-8000-000000000001")}, nil
}

func contributorRoleResolver() fakeProjectRoleResolver {
	role := "contributor"
	return fakeProjectRoleResolver{role: &role}
}

type fakeWatcherRepository struct {
	watching map[uuid.UUID]map[uuid.UUID]bool
}

type fakeSavedFilterRepository struct {
	filters map[uuid.UUID]*domain.SavedFilter
}

func (r *fakeSavedFilterRepository) SaveSavedFilter(_ context.Context, filter *domain.SavedFilter) error {
	if filter.ID == uuid.Nil {
		filter.ID = uuid.New()
	}
	if r.filters == nil {
		r.filters = make(map[uuid.UUID]*domain.SavedFilter)
	}
	for _, existing := range r.filters {
		if existing.OwnerID == filter.OwnerID && existing.Name == filter.Name {
			return apperror.New(apperror.KindConflict, "duplicate filter")
		}
	}
	r.filters[filter.ID] = filter
	return nil
}

func (r *fakeSavedFilterRepository) ListSavedFilters(_ context.Context, ownerID uuid.UUID) ([]*domain.SavedFilter, error) {
	filters := make([]*domain.SavedFilter, 0)
	for _, filter := range r.filters {
		if filter.OwnerID == ownerID {
			filters = append(filters, filter)
		}
	}
	return filters, nil
}

func (r *fakeSavedFilterRepository) DeleteSavedFilter(_ context.Context, ownerID, filterID uuid.UUID) error {
	filter, ok := r.filters[filterID]
	if !ok || filter.OwnerID != ownerID {
		return apperror.New(apperror.KindNotFound, "saved filter not found")
	}
	delete(r.filters, filterID)
	return nil
}

func newFakeWatcherRepository() *fakeWatcherRepository {
	return &fakeWatcherRepository{watching: make(map[uuid.UUID]map[uuid.UUID]bool)}
}

func (r *fakeWatcherRepository) State(_ context.Context, issueID, userID uuid.UUID) (port.WatcherState, error) {
	state := port.WatcherState{IsWatching: r.watching[issueID][userID]}
	for range r.watching[issueID] {
		state.Count++
	}
	return state, nil
}

func (r *fakeWatcherRepository) Add(_ context.Context, issueID, userID uuid.UUID) error {
	if r.watching[issueID] == nil {
		r.watching[issueID] = make(map[uuid.UUID]bool)
	}
	r.watching[issueID][userID] = true
	return nil
}

func (r *fakeWatcherRepository) Remove(_ context.Context, issueID, userID uuid.UUID) error {
	delete(r.watching[issueID], userID)
	return nil
}

func newFakeUserDirectory() *fakeUserDirectory {
	return &fakeUserDirectory{userTeams: make(map[uuid.UUID]*uuid.UUID), names: make(map[uuid.UUID]string)}
}

func (d *fakeUserDirectory) NamesOf(_ context.Context, userIDs []uuid.UUID) (map[uuid.UUID]string, error) {
	out := make(map[uuid.UUID]string, len(userIDs))
	for _, id := range userIDs {
		if name, ok := d.names[id]; ok {
			out[id] = name
		}
	}
	return out, nil
}

func (d *fakeUserDirectory) SharesTeam(_ context.Context, userA, userB uuid.UUID) (bool, error) {
	teamA, okA := d.userTeams[userA]
	teamB, okB := d.userTeams[userB]
	return okA && okB && teamA != nil && teamB != nil && *teamA == *teamB, nil
}

func TestTicketService_Create(t *testing.T) {
	ctx := context.Background()
	repo := newFakeTicketRepo()
	dir := newFakeUserDirectory()
	svc := NewTicketService(repo, dir, contributorRoleResolver())

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
	svc := NewTicketService(repo, dir, contributorRoleResolver())

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
	privateProjectTicket, _ := domain.NewTicket("Private issue", "Hidden description", domain.PriorityLow, requesterID)
	privateProjectTicket.ProjectID = uuid.New()
	_ = privateProjectTicket.ChangeStatus(domain.StatusInProgress)
	_ = privateProjectTicket.ChangeStatus(domain.StatusResolved)
	if err := repo.Save(ctx, privateProjectTicket); err != nil {
		t.Fatal(err)
	}
	_ = svc.repo.Update(ctx, &domain.Ticket{
		ID:          t2.ID,
		ProjectID:   t2.ProjectID,
		Title:       t2.Title,
		Description: t2.Description,
		Status:      domain.StatusResolved,
		Priority:    t2.Priority,
		RequesterID: requesterID,
		CreatedAt:   t2.CreatedAt,
		UpdatedAt:   t2.UpdatedAt,
	})

	t.Run("Get returns ticket by ID", func(t *testing.T) {
		got, err := svc.Get(ctx, t1.ID, requesterID, "agent")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got.ID != t1.ID {
			t.Errorf("got ID %v, want %v", got.ID, t1.ID)
		}
	})

	t.Run("Get returns not found error for non-existent ticket", func(t *testing.T) {
		_, err := svc.Get(ctx, uuid.New(), requesterID, "agent")
		if !errors.Is(err, domain.ErrTicketNotFound) {
			t.Errorf("expected ErrTicketNotFound, got %v", err)
		}
	})

	t.Run("Get forbids an issue outside the member's projects", func(t *testing.T) {
		_, err := svc.Get(ctx, privateProjectTicket.ID, requesterID, "agent")
		if apperror.KindOf(err) != apperror.KindForbidden {
			t.Fatalf("got error kind %q, want forbidden", apperror.KindOf(err))
		}
	})

	t.Run("List with default pagination and filter", func(t *testing.T) {
		result, err := svc.List(ctx, requesterID, "agent", port.ListFilter{Status: domain.StatusResolved})
		items := result.Items
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(items) != 1 {
			t.Fatalf("got %d items, want only the member project's issue", len(items))
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
	svc := NewTicketService(repo, dir, contributorRoleResolver())

	requesterID := uuid.New()
	t1, _ := svc.Create(ctx, port.CreateInput{
		Title:       "Issue",
		Description: "Desc",
		Priority:    domain.PriorityMedium,
		RequesterID: requesterID,
	})

	t.Run("valid transition", func(t *testing.T) {
		updated, err := svc.UpdateStatus(ctx, t1.ID, requesterID, "admin", domain.StatusInProgress)
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
		_, err := svc.UpdateStatus(ctx, t1.ID, requesterID, "admin", domain.StatusOpen)
		if !errors.Is(err, domain.ErrIllegalTransition) {
			t.Errorf("expected ErrIllegalTransition, got %v", err)
		}
	})

	t.Run("ticket not found", func(t *testing.T) {
		_, err := svc.UpdateStatus(ctx, uuid.New(), requesterID, "admin", domain.StatusClosed)
		if !errors.Is(err, domain.ErrTicketNotFound) {
			t.Errorf("expected ErrTicketNotFound, got %v", err)
		}
	})
}

func TestTicketService_UpdateRank(t *testing.T) {
	ctx := context.Background()
	repo := newFakeTicketRepo()
	dir := newFakeUserDirectory()
	svc := NewTicketService(repo, dir, contributorRoleResolver())

	ticket, _ := svc.Create(ctx, port.CreateInput{
		Title:       "Ranked issue",
		Description: "Desc",
		Priority:    domain.PriorityMedium,
		RequesterID: uuid.New(),
	})

	updated, err := svc.UpdateRank(ctx, ticket.ID, uuid.New(), "admin", "00000001")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if updated.Rank != "00000001" {
		t.Fatalf("got rank %q, want %q", updated.Rank, "00000001")
	}
}

func TestTicketService_UpdateDetailsAuthorization(t *testing.T) {
	ctx := context.Background()
	repo := newFakeTicketRepo()
	dir := newFakeUserDirectory()
	requesterID := uuid.New()
	ticket, err := domain.NewTicket("Issue", "Description", domain.PriorityLow, requesterID)
	if err != nil {
		t.Fatal(err)
	}
	ticket.ProjectID = uuid.MustParse("00000000-0000-4000-8000-000000000001")
	if err := repo.Save(ctx, ticket); err != nil {
		t.Fatal(err)
	}
	input := port.UpdateDetailsInput{
		Title: "Updated issue", Description: "Updated description", Priority: domain.PriorityHigh,
		IssueType: domain.IssueTypeBug,
	}

	t.Run("project contributor can edit", func(t *testing.T) {
		role := "contributor"
		svc := NewTicketService(repo, dir, fakeProjectRoleResolver{role: &role})
		updated, err := svc.UpdateDetails(ctx, ticket.ID, uuid.New(), "agent", input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if updated.Title != input.Title {
			t.Fatalf("got title %q, want %q", updated.Title, input.Title)
		}
	})

	t.Run("project viewer cannot edit", func(t *testing.T) {
		role := "viewer"
		svc := NewTicketService(repo, dir, fakeProjectRoleResolver{role: &role})
		_, err := svc.UpdateDetails(ctx, ticket.ID, uuid.New(), "agent", input)
		if apperror.KindOf(err) != apperror.KindForbidden {
			t.Fatalf("got error kind %q, want forbidden", apperror.KindOf(err))
		}
	})

	t.Run("non-member cannot edit", func(t *testing.T) {
		svc := NewTicketService(repo, dir, fakeProjectRoleResolver{})
		_, err := svc.UpdateDetails(ctx, ticket.ID, uuid.New(), "agent", input)
		if apperror.KindOf(err) != apperror.KindForbidden {
			t.Fatalf("got error kind %q, want forbidden", apperror.KindOf(err))
		}
	})

	t.Run("admin can edit without project membership", func(t *testing.T) {
		svc := NewTicketService(repo, dir, fakeProjectRoleResolver{})
		_, err := svc.UpdateDetails(ctx, ticket.ID, uuid.New(), "admin", input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})
}

type fakeWorkflowRepo struct {
	allowed  bool
	roles    []string
	workflow *port.Workflow
	replaced []port.WorkflowTransition
}

func (r *fakeWorkflowRepo) DefaultWorkflow(_ context.Context, _ uuid.UUID) (*port.Workflow, error) {
	return r.workflow, nil
}

func (r *fakeWorkflowRepo) ReplaceTransitions(_ context.Context, _ uuid.UUID, transitions []port.WorkflowTransition) error {
	r.replaced = transitions
	return nil
}

// TestTicketService_UpdateDefaultWorkflow comprueba permisos (solo lead y admin) y validaciones.
func TestTicketService_UpdateDefaultWorkflow(t *testing.T) {
	ctx := context.Background()
	valid := []port.WorkflowTransition{{From: domain.StatusOpen, To: domain.StatusInProgress, Roles: []string{"lead", "contributor"}}}
	newRepo := func() *fakeWorkflowRepo {
		return &fakeWorkflowRepo{workflow: &port.Workflow{ID: uuid.New(), ProjectID: domain.DefaultProjectID, Name: "Flujo"}}
	}
	lead := "lead"
	leadResolver := fakeProjectRoleResolver{role: &lead}

	repo := newRepo()
	svc := NewTicketService(newFakeTicketRepo(), newFakeUserDirectory(), repo, leadResolver)
	if _, err := svc.UpdateDefaultWorkflow(ctx, uuid.New(), "agent", domain.DefaultProjectID, valid); err != nil {
		t.Fatalf("lead should update the workflow, got %v", err)
	}
	if len(repo.replaced) != 1 {
		t.Fatalf("transitions were not replaced: %v", repo.replaced)
	}

	denied := NewTicketService(newFakeTicketRepo(), newFakeUserDirectory(), newRepo(), contributorRoleResolver())
	if _, err := denied.UpdateDefaultWorkflow(ctx, uuid.New(), "agent", domain.DefaultProjectID, valid); apperror.KindOf(err) != apperror.KindForbidden {
		t.Fatalf("contributor should be forbidden, got %v", err)
	}

	cases := map[string][]port.WorkflowTransition{
		"invalid status":  {{From: "open", To: "done", Roles: []string{"lead"}}},
		"self transition": {{From: domain.StatusOpen, To: domain.StatusOpen, Roles: []string{"lead"}}},
		"no roles":        {{From: domain.StatusOpen, To: domain.StatusClosed, Roles: nil}},
		"unknown role":    {{From: domain.StatusOpen, To: domain.StatusClosed, Roles: []string{"agent"}}},
		"duplicated":      {{From: domain.StatusOpen, To: domain.StatusClosed, Roles: []string{"lead"}}, {From: domain.StatusOpen, To: domain.StatusClosed, Roles: []string{"lead"}}},
	}
	for name, transitions := range cases {
		svc := NewTicketService(newFakeTicketRepo(), newFakeUserDirectory(), newRepo(), leadResolver)
		if _, err := svc.UpdateDefaultWorkflow(ctx, uuid.New(), "agent", domain.DefaultProjectID, transitions); apperror.KindOf(err) != apperror.KindValidation {
			t.Errorf("%s: want validation error, got %v", name, err)
		}
	}
}

// TestTicketService_DefaultWorkflow comprueba que lo puede leer un miembro, que un no miembro recibe
// prohibido y que un proyecto sin flujo responde no encontrado.
func TestTicketService_DefaultWorkflow(t *testing.T) {
	ctx := context.Background()
	workflowRepo := &fakeWorkflowRepo{workflow: &port.Workflow{Name: "Flujo", ProjectID: domain.DefaultProjectID}}

	member := NewTicketService(newFakeTicketRepo(), newFakeUserDirectory(), workflowRepo, contributorRoleResolver())
	workflow, err := member.DefaultWorkflow(ctx, uuid.New(), "agent", domain.DefaultProjectID)
	if err != nil || workflow == nil || workflow.Name != "Flujo" {
		t.Fatalf("member should read the workflow, got %v, %v", workflow, err)
	}

	outsider := NewTicketService(newFakeTicketRepo(), newFakeUserDirectory(), workflowRepo, fakeProjectRoleResolver{})
	if _, err := outsider.DefaultWorkflow(ctx, uuid.New(), "agent", domain.DefaultProjectID); apperror.KindOf(err) != apperror.KindForbidden {
		t.Fatalf("non member should be forbidden, got %v", err)
	}

	withoutFlow := NewTicketService(newFakeTicketRepo(), newFakeUserDirectory(), &fakeWorkflowRepo{}, contributorRoleResolver())
	if _, err := withoutFlow.DefaultWorkflow(ctx, uuid.New(), "agent", domain.DefaultProjectID); apperror.KindOf(err) != apperror.KindNotFound {
		t.Fatalf("project without workflow should be not found, got %v", err)
	}
}

func (r *fakeWorkflowRepo) CanTransition(_ context.Context, _ uuid.UUID, _, _ domain.Status, role string) (bool, error) {
	r.roles = append(r.roles, role)
	return r.allowed, nil
}

// TestTicketService_WorkflowEvaluatesProjectRole comprueba que el flujo recibe el rol del proyecto
// (o admin para el administrador global), no el rol global del actor.
func TestTicketService_WorkflowEvaluatesProjectRole(t *testing.T) {
	ctx := context.Background()
	repo := newFakeTicketRepo()
	workflowRepo := &fakeWorkflowRepo{allowed: true}
	svc := NewTicketService(repo, newFakeUserDirectory(), workflowRepo, contributorRoleResolver())

	ticket, _ := svc.Create(ctx, port.CreateInput{Title: "Role issue", Description: "Desc", Priority: domain.PriorityMedium, RequesterID: uuid.New()})

	if _, err := svc.UpdateStatus(ctx, ticket.ID, uuid.New(), "agent", domain.StatusInProgress); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, err := svc.UpdateStatus(ctx, ticket.ID, uuid.New(), "admin", domain.StatusResolved); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(workflowRepo.roles) != 2 || workflowRepo.roles[0] != "contributor" || workflowRepo.roles[1] != "admin" {
		t.Fatalf("workflow roles = %v, want [contributor admin]", workflowRepo.roles)
	}
}

func TestTicketService_WorkflowPolicy(t *testing.T) {
	ctx := context.Background()
	repo := newFakeTicketRepo()
	dir := newFakeUserDirectory()
	workflowRepo := &fakeWorkflowRepo{allowed: false}
	svc := NewTicketService(repo, dir, workflowRepo, contributorRoleResolver())

	ticket, _ := svc.Create(ctx, port.CreateInput{
		Title:       "Workflow issue",
		Description: "Desc",
		Priority:    domain.PriorityMedium,
		RequesterID: uuid.New(),
	})

	_, err := svc.UpdateStatus(ctx, ticket.ID, uuid.New(), "admin", domain.StatusInProgress)
	if !errors.Is(err, domain.ErrIllegalTransition) {
		t.Fatalf("expected ErrIllegalTransition, got %v", err)
	}

	workflowRepo.allowed = true
	updated, err := svc.UpdateStatus(ctx, ticket.ID, uuid.New(), "admin", domain.StatusInProgress)
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
	watchers := newFakeWatcherRepository()
	svc := NewTicketService(repo, dir, contributorRoleResolver(), watchers)

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
		assigned, err := svc.Assign(ctx, ticket.ID, assigner, assigneeTeamA, "admin")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if assigned.AssigneeID == nil || *assigned.AssigneeID != assigneeTeamA {
			t.Errorf("got assignee %v, want %v", assigned.AssigneeID, assigneeTeamA)
		}
		if assigned.Status != domain.StatusInProgress {
			t.Errorf("got status %v, want %v", assigned.Status, domain.StatusInProgress)
		}
		state, err := watchers.State(ctx, ticket.ID, assigneeTeamA)
		if err != nil || !state.IsWatching {
			t.Errorf("assigned user should automatically follow issue, state=%+v err=%v", state, err)
		}
	})

	t.Run("rejects cross-team assignment", func(t *testing.T) {
		_, err := svc.Assign(ctx, ticket.ID, assigner, assigneeTeamB, "admin")
		if !errors.Is(err, domain.ErrCrossTeamAssignment) {
			t.Errorf("expected ErrCrossTeamAssignment, got %v", err)
		}
	})

	t.Run("rejects assignment when assignee has no team", func(t *testing.T) {
		_, err := svc.Assign(ctx, ticket.ID, assigner, userNoTeam, "admin")
		if !errors.Is(err, domain.ErrCrossTeamAssignment) {
			t.Errorf("expected ErrCrossTeamAssignment, got %v", err)
		}
	})

	t.Run("returns not found for non-existent ticket", func(t *testing.T) {
		_, err := svc.Assign(ctx, uuid.New(), assigner, assigneeTeamA, "admin")
		if !errors.Is(err, domain.ErrTicketNotFound) {
			t.Errorf("expected ErrTicketNotFound, got %v", err)
		}
	})
}

func TestTicketService_WatchIssueRequiresProjectVisibility(t *testing.T) {
	ctx := context.Background()
	repo := newFakeTicketRepo()
	dir := newFakeUserDirectory()
	watchers := newFakeWatcherRepository()
	svc := NewTicketService(repo, dir, contributorRoleResolver(), watchers)
	ticket, err := svc.Create(ctx, port.CreateInput{Title: "Watch issue", Description: "Desc", Priority: domain.PriorityLow, RequesterID: uuid.New()})
	if err != nil {
		t.Fatal(err)
	}
	actor := uuid.New()

	state, err := svc.GetWatcherState(ctx, ticket.ID, actor, "requester")
	if err != nil || state.IsWatching || state.Count != 0 {
		t.Fatalf("initial watcher state=%+v err=%v", state, err)
	}
	state, err = svc.WatchIssue(ctx, ticket.ID, actor, "requester")
	if err != nil || !state.IsWatching || state.Count != 1 {
		t.Fatalf("watch state=%+v err=%v", state, err)
	}
	state, err = svc.WatchIssue(ctx, ticket.ID, actor, "requester")
	if err != nil || !state.IsWatching || state.Count != 1 {
		t.Fatalf("duplicate watch should be idempotent: state=%+v err=%v", state, err)
	}
	state, err = svc.UnwatchIssue(ctx, ticket.ID, actor, "requester")
	if err != nil || state.IsWatching || state.Count != 0 {
		t.Fatalf("unwatch state=%+v err=%v", state, err)
	}

	nonMemberService := NewTicketService(repo, dir, fakeProjectRoleResolver{}, watchers)
	_, err = nonMemberService.WatchIssue(ctx, ticket.ID, uuid.New(), "requester")
	if apperror.KindOf(err) != apperror.KindForbidden {
		t.Fatalf("non-member watch error=%v, want forbidden", err)
	}
}

func TestTicketService_SavedFiltersArePrivateToOwner(t *testing.T) {
	repository := &fakeSavedFilterRepository{}
	service := NewTicketService(newFakeTicketRepo(), newFakeUserDirectory(), repository)
	ownerID, otherID := uuid.New(), uuid.New()
	filter, err := service.CreateSavedFilter(context.Background(), ownerID, "Open bugs", domain.SavedFilterCriteria{Status: domain.StatusOpen, IssueType: domain.IssueTypeBug})
	if err != nil {
		t.Fatal(err)
	}
	ownerFilters, err := service.ListSavedFilters(context.Background(), ownerID)
	if err != nil || len(ownerFilters) != 1 || ownerFilters[0].ID != filter.ID {
		t.Fatalf("owner filters=%+v err=%v", ownerFilters, err)
	}
	otherFilters, err := service.ListSavedFilters(context.Background(), otherID)
	if err != nil || len(otherFilters) != 0 {
		t.Fatalf("other user filters=%+v err=%v", otherFilters, err)
	}
	if err := service.DeleteSavedFilter(context.Background(), otherID, filter.ID); apperror.KindOf(err) != apperror.KindNotFound {
		t.Fatalf("cross-owner delete error=%v, want not found", err)
	}
	if err := service.DeleteSavedFilter(context.Background(), ownerID, filter.ID); err != nil {
		t.Fatal(err)
	}
}

func TestTicketService_ListReportsTotalAcrossPages(t *testing.T) {
	ctx := context.Background()
	repo := newFakeTicketRepo()
	svc := NewTicketService(repo, newFakeUserDirectory(), contributorRoleResolver())
	requesterID := uuid.New()
	for i := 0; i < 5; i++ {
		if _, err := svc.Create(ctx, port.CreateInput{
			Title:       "Issue",
			Description: "Desc",
			Priority:    domain.PriorityLow,
			RequesterID: requesterID,
		}); err != nil {
			t.Fatal(err)
		}
	}

	result, err := svc.List(ctx, requesterID, "agent", port.ListFilter{Page: 2, PageSize: 2})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Items) != 2 {
		t.Errorf("got %d items on page 2, want 2", len(result.Items))
	}
	if result.Total != 5 {
		t.Errorf("total = %d, want 5 (all matches, not only the page)", result.Total)
	}
	if result.Page != 2 || result.PageSize != 2 {
		t.Errorf("page/page_size = %d/%d, want 2/2", result.Page, result.PageSize)
	}
}

// roleLookupOf adapta el resolutor de roles del modulo al puerto de autorizacion del hexagono authorization.
type roleLookupOf struct {
	resolver port.ProjectRoleResolver
}

func (l roleLookupOf) FindRole(ctx context.Context, projectID, userID uuid.UUID) (*authdomain.ProjectRole, error) {
	role, err := l.resolver.FindRole(ctx, projectID, userID)
	if err != nil || role == nil {
		return nil, err
	}
	value := authdomain.ProjectRole(*role)
	return &value, nil
}

func authorizeWithResolver(ctx context.Context, resolver port.ProjectRoleResolver, action string, actorID uuid.UUID, globalRole string, projectID uuid.UUID) error {
	return authusecase.NewAuthorizer(roleLookupOf{resolver: resolver}).Authorize(ctx, authport.Action(action), actorID, globalRole, projectID)
}

func (r fakeProjectRoleResolver) Authorize(ctx context.Context, action string, actorID uuid.UUID, globalRole string, projectID uuid.UUID) error {
	return authorizeWithResolver(ctx, r, action, actorID, globalRole, projectID)
}
