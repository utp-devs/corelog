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

	"github.com/jhonsferg/corelog/internal/issue/application/port"
	"github.com/jhonsferg/corelog/internal/issue/domain"
	"github.com/jhonsferg/corelog/internal/platform/apperror"
)

type fakeIssueRepo struct {
	issues map[uuid.UUID]*domain.Issue
}

func newFakeIssueRepo() *fakeIssueRepo {
	return &fakeIssueRepo{issues: make(map[uuid.UUID]*domain.Issue)}
}

func (r *fakeIssueRepo) Save(_ context.Context, t *domain.Issue) error {
	if t.ID == uuid.Nil {
		t.ID = uuid.New()
	}
	r.nextKey(t)
	copied := *t
	r.issues[t.ID] = &copied
	return nil
}

// nextKey gives a new issue the next number of its project, as the real repository does.
func (r *fakeIssueRepo) nextKey(t *domain.Issue) {
	if t.KeyNumber != 0 {
		return
	}
	for _, other := range r.issues {
		if other.ProjectID == t.ProjectID && other.KeyNumber >= t.KeyNumber {
			t.KeyNumber = other.KeyNumber
		}
	}
	t.KeyNumber++
	if t.ProjectKey == "" {
		t.ProjectKey = "CORE"
	}
}

// FindKeyCandidates matches on project key and number only; organization reach is covered by the postgres tests.
func (r *fakeIssueRepo) FindKeyCandidates(_ context.Context, projectKey string, number int, _, _ uuid.UUID, _ bool) ([]*domain.Issue, error) {
	var out []*domain.Issue
	for _, t := range r.issues {
		if t.ProjectKey == projectKey && t.KeyNumber == number {
			copied := *t
			out = append(out, &copied)
		}
	}
	return out, nil
}

func (r *fakeIssueRepo) FindByID(_ context.Context, id uuid.UUID) (*domain.Issue, error) {
	t, ok := r.issues[id]
	if !ok {
		return nil, domain.ErrIssueNotFound
	}
	if t.ID == uuid.Nil {
		t.ID = uuid.New()
	}
	copied := *t
	return &copied, nil
}

func (r *fakeIssueRepo) Count(ctx context.Context, filter port.ListFilter) (int, error) {
	unpaged := filter
	unpaged.Page = 1
	unpaged.PageSize = math.MaxInt32
	all, err := r.FindAll(ctx, unpaged)
	if err != nil {
		return 0, err
	}
	return len(all), nil
}

func (r *fakeIssueRepo) FindAll(_ context.Context, filter port.ListFilter) ([]*domain.Issue, error) {
	var list []*domain.Issue
	projectIDs := make(map[uuid.UUID]struct{}, len(filter.ProjectIDs))
	for _, projectID := range filter.ProjectIDs {
		projectIDs[projectID] = struct{}{}
	}
	for _, t := range r.issues {
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
		if filter.Unassigned && t.AssigneeID != nil {
			continue
		}
		if filter.AssigneeID != nil && (t.AssigneeID == nil || *t.AssigneeID != *filter.AssigneeID) {
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

func (r *fakeIssueRepo) Update(_ context.Context, t *domain.Issue) error {
	if _, ok := r.issues[t.ID]; !ok {
		return domain.ErrIssueNotFound
	}
	if t.ID == uuid.Nil {
		t.ID = uuid.New()
	}
	copied := *t
	r.issues[t.ID] = &copied
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

func TestIssueService_Create(t *testing.T) {
	ctx := context.Background()
	repo := newFakeIssueRepo()
	dir := newFakeUserDirectory()
	svc := NewIssueService(repo, dir, contributorRoleResolver())

	reporterID := uuid.New()

	t.Run("successfully creates issue", func(t *testing.T) {
		issue, err := svc.Create(ctx, port.CreateInput{
			Summary:     "Login bug",
			Description: "Cannot log in with valid credentials",
			Priority:    domain.PriorityHigh,
			ReporterID:  reporterID,
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if issue.Summary != "Login bug" {
			t.Errorf("got summary %s, want 'Login bug'", issue.Summary)
		}
		if issue.Status != domain.StatusOpen {
			t.Errorf("got status %v, want %v", issue.Status, domain.StatusOpen)
		}

		// Verify stored in repo
		stored, err := repo.FindByID(ctx, issue.ID)
		if err != nil {
			t.Fatalf("expected issue to be found in repo: %v", err)
		}
		if stored.Summary != "Login bug" {
			t.Errorf("got stored summary %s, want 'Login bug'", stored.Summary)
		}
	})

	t.Run("fails on invalid domain data", func(t *testing.T) {
		_, err := svc.Create(ctx, port.CreateInput{
			Summary:     "",
			Description: "Desc",
			Priority:    domain.PriorityLow,
			ReporterID:  reporterID,
		})
		if !errors.Is(err, domain.ErrInvalidSummary) {
			t.Errorf("expected ErrInvalidSummary, got %v", err)
		}
	})
}

func TestIssueService_GetAndList(t *testing.T) {
	ctx := context.Background()
	repo := newFakeIssueRepo()
	dir := newFakeUserDirectory()
	svc := NewIssueService(repo, dir, contributorRoleResolver())

	reporterID := uuid.New()
	t1, _ := svc.Create(ctx, port.CreateInput{
		Summary:     "Issue 1",
		Description: "Desc 1",
		Priority:    domain.PriorityLow,
		ReporterID:  reporterID,
	})
	t2, _ := svc.Create(ctx, port.CreateInput{
		Summary:     "Issue 2",
		Description: "Desc 2",
		Priority:    domain.PriorityCritical,
		ReporterID:  reporterID,
	})
	privateProjectIssue, _ := domain.NewIssue("Private issue", "Hidden description", domain.PriorityLow, reporterID)
	privateProjectIssue.ProjectID = uuid.New()
	_ = privateProjectIssue.ChangeStatus(domain.StatusInProgress)
	_ = privateProjectIssue.ChangeStatus(domain.StatusResolved)
	if err := repo.Save(ctx, privateProjectIssue); err != nil {
		t.Fatal(err)
	}
	_ = svc.repo.Update(ctx, &domain.Issue{
		ID:          t2.ID,
		ProjectID:   t2.ProjectID,
		Summary:     t2.Summary,
		Description: t2.Description,
		Status:      domain.StatusResolved,
		Priority:    t2.Priority,
		ReporterID:  reporterID,
		CreatedAt:   t2.CreatedAt,
		UpdatedAt:   t2.UpdatedAt,
	})

	t.Run("Get returns issue by ID", func(t *testing.T) {
		got, err := svc.Get(ctx, t1.ID, reporterID, "agent")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got.ID != t1.ID {
			t.Errorf("got ID %v, want %v", got.ID, t1.ID)
		}
	})

	t.Run("Get returns not found error for non-existent issue", func(t *testing.T) {
		_, err := svc.Get(ctx, uuid.New(), reporterID, "agent")
		if !errors.Is(err, domain.ErrIssueNotFound) {
			t.Errorf("expected ErrIssueNotFound, got %v", err)
		}
	})

	t.Run("Get forbids an issue outside the member's projects", func(t *testing.T) {
		_, err := svc.Get(ctx, privateProjectIssue.ID, reporterID, "agent")
		if apperror.KindOf(err) != apperror.KindForbidden {
			t.Fatalf("got error kind %q, want forbidden", apperror.KindOf(err))
		}
	})

	t.Run("List with default pagination and filter", func(t *testing.T) {
		result, err := svc.List(ctx, reporterID, "agent", port.ListFilter{Status: domain.StatusResolved})
		items := result.Items
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(items) != 1 {
			t.Fatalf("got %d items, want only the member project's issue", len(items))
		}
		if items[0].ID != t2.ID {
			t.Errorf("got issue ID %v, want %v", items[0].ID, t2.ID)
		}
	})
}

func TestIssueService_UpdateRank(t *testing.T) {
	ctx := context.Background()
	repo := newFakeIssueRepo()
	dir := newFakeUserDirectory()
	svc := NewIssueService(repo, dir, contributorRoleResolver())

	issue, _ := svc.Create(ctx, port.CreateInput{
		Summary:     "Ranked issue",
		Description: "Desc",
		Priority:    domain.PriorityMedium,
		ReporterID:  uuid.New(),
	})

	updated, err := svc.UpdateRank(ctx, issue.ID, uuid.New(), "admin", "00000001")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if updated.Rank != "00000001" {
		t.Fatalf("got rank %q, want %q", updated.Rank, "00000001")
	}
}

func TestIssueService_UpdateDetailsAuthorization(t *testing.T) {
	ctx := context.Background()
	repo := newFakeIssueRepo()
	dir := newFakeUserDirectory()
	reporterID := uuid.New()
	issue, err := domain.NewIssue("Issue", "Description", domain.PriorityLow, reporterID)
	if err != nil {
		t.Fatal(err)
	}
	issue.ProjectID = uuid.MustParse("00000000-0000-4000-8000-000000000001")
	if err := repo.Save(ctx, issue); err != nil {
		t.Fatal(err)
	}
	input := port.UpdateDetailsInput{
		Summary: "Updated issue", Description: "Updated description", Priority: domain.PriorityHigh,
		IssueType: domain.IssueTypeBug,
	}

	t.Run("project contributor can edit", func(t *testing.T) {
		role := "contributor"
		svc := NewIssueService(repo, dir, fakeProjectRoleResolver{role: &role})
		updated, err := svc.UpdateDetails(ctx, issue.ID, uuid.New(), "agent", input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if updated.Summary != input.Summary {
			t.Fatalf("got summary %q, want %q", updated.Summary, input.Summary)
		}
	})

	t.Run("project viewer cannot edit", func(t *testing.T) {
		role := "viewer"
		svc := NewIssueService(repo, dir, fakeProjectRoleResolver{role: &role})
		_, err := svc.UpdateDetails(ctx, issue.ID, uuid.New(), "agent", input)
		if apperror.KindOf(err) != apperror.KindForbidden {
			t.Fatalf("got error kind %q, want forbidden", apperror.KindOf(err))
		}
	})

	t.Run("non-member cannot edit", func(t *testing.T) {
		svc := NewIssueService(repo, dir, fakeProjectRoleResolver{})
		_, err := svc.UpdateDetails(ctx, issue.ID, uuid.New(), "agent", input)
		if apperror.KindOf(err) != apperror.KindForbidden {
			t.Fatalf("got error kind %q, want forbidden", apperror.KindOf(err))
		}
	})

	t.Run("platform admin has read-only support access without project membership", func(t *testing.T) {
		// Platform admins get viewer access for support; editing needs a project or org role.
		svc := NewIssueService(repo, dir, fakeProjectRoleResolver{})
		_, err := svc.UpdateDetails(ctx, issue.ID, uuid.New(), "admin", input)
		if err == nil {
			t.Fatal("expected forbidden error for a platform admin without a project role")
		}
	})
}

func TestIssueService_Assign(t *testing.T) {
	ctx := context.Background()
	repo := newFakeIssueRepo()
	dir := newFakeUserDirectory()
	watchers := newFakeWatcherRepository()
	svc := NewIssueService(repo, dir, contributorRoleResolver(), watchers)

	teamA := uuid.New()
	teamB := uuid.New()

	assigner := uuid.New()
	assigneeTeamA := uuid.New()
	assigneeTeamB := uuid.New()
	userNoTeam := uuid.New()

	dir.userTeams[assigner] = &teamA
	dir.userTeams[assigneeTeamA] = &teamA
	dir.userTeams[assigneeTeamB] = &teamB

	issue, _ := svc.Create(ctx, port.CreateInput{
		Summary:     "Issue to assign",
		Description: "Desc",
		Priority:    domain.PriorityLow,
		ReporterID:  assigner,
	})

	t.Run("assigning to member of same team succeeds and keeps the status", func(t *testing.T) {
		assigned, err := svc.Assign(ctx, issue.ID, assigner, assigneeTeamA, "admin")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if assigned.AssigneeID == nil || *assigned.AssigneeID != assigneeTeamA {
			t.Errorf("got assignee %v, want %v", assigned.AssigneeID, assigneeTeamA)
		}
		if assigned.Status != domain.StatusOpen {
			t.Errorf("got status %v, want %v", assigned.Status, domain.StatusOpen)
		}
		state, err := watchers.State(ctx, issue.ID, assigneeTeamA)
		if err != nil || !state.IsWatching {
			t.Errorf("assigned user should automatically follow issue, state=%+v err=%v", state, err)
		}
	})

	t.Run("rejects cross-team assignment", func(t *testing.T) {
		_, err := svc.Assign(ctx, issue.ID, assigner, assigneeTeamB, "admin")
		if !errors.Is(err, domain.ErrCrossTeamAssignment) {
			t.Errorf("expected ErrCrossTeamAssignment, got %v", err)
		}
	})

	t.Run("rejects assignment when assignee has no team", func(t *testing.T) {
		_, err := svc.Assign(ctx, issue.ID, assigner, userNoTeam, "admin")
		if !errors.Is(err, domain.ErrCrossTeamAssignment) {
			t.Errorf("expected ErrCrossTeamAssignment, got %v", err)
		}
	})

	t.Run("returns not found for non-existent issue", func(t *testing.T) {
		_, err := svc.Assign(ctx, uuid.New(), assigner, assigneeTeamA, "admin")
		if !errors.Is(err, domain.ErrIssueNotFound) {
			t.Errorf("expected ErrIssueNotFound, got %v", err)
		}
	})
}

func TestIssueService_WatchIssueRequiresProjectVisibility(t *testing.T) {
	ctx := context.Background()
	repo := newFakeIssueRepo()
	dir := newFakeUserDirectory()
	watchers := newFakeWatcherRepository()
	svc := NewIssueService(repo, dir, contributorRoleResolver(), watchers)
	issue, err := svc.Create(ctx, port.CreateInput{Summary: "Watch issue", Description: "Desc", Priority: domain.PriorityLow, ReporterID: uuid.New()})
	if err != nil {
		t.Fatal(err)
	}
	actor := uuid.New()

	state, err := svc.GetWatcherState(ctx, issue.ID, actor, "requester")
	// The reporter follows the issue from creation.
	if err != nil || state.IsWatching || state.Count != 1 {
		t.Fatalf("initial watcher state=%+v err=%v", state, err)
	}
	state, err = svc.WatchIssue(ctx, issue.ID, actor, "requester")
	if err != nil || !state.IsWatching || state.Count != 2 {
		t.Fatalf("watch state=%+v err=%v", state, err)
	}
	state, err = svc.WatchIssue(ctx, issue.ID, actor, "requester")
	if err != nil || !state.IsWatching || state.Count != 2 {
		t.Fatalf("duplicate watch should be idempotent: state=%+v err=%v", state, err)
	}
	state, err = svc.UnwatchIssue(ctx, issue.ID, actor, "requester")
	if err != nil || state.IsWatching || state.Count != 1 {
		t.Fatalf("unwatch state=%+v err=%v", state, err)
	}

	nonMemberService := NewIssueService(repo, dir, fakeProjectRoleResolver{}, watchers)
	_, err = nonMemberService.WatchIssue(ctx, issue.ID, uuid.New(), "requester")
	if apperror.KindOf(err) != apperror.KindForbidden {
		t.Fatalf("non-member watch error=%v, want forbidden", err)
	}
}

func TestIssueService_SavedFiltersArePrivateToOwner(t *testing.T) {
	repository := &fakeSavedFilterRepository{}
	service := NewIssueService(newFakeIssueRepo(), newFakeUserDirectory(), repository)
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

func TestIssueService_ListReportsTotalAcrossPages(t *testing.T) {
	ctx := context.Background()
	repo := newFakeIssueRepo()
	svc := NewIssueService(repo, newFakeUserDirectory(), contributorRoleResolver())
	reporterID := uuid.New()
	for i := 0; i < 5; i++ {
		if _, err := svc.Create(ctx, port.CreateInput{
			Summary:     "Issue",
			Description: "Desc",
			Priority:    domain.PriorityLow,
			ReporterID:  reporterID,
		}); err != nil {
			t.Fatal(err)
		}
	}

	result, err := svc.List(ctx, reporterID, "agent", port.ListFilter{Page: 2, PageSize: 2})
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

func TestIssueService_ListFiltersByAssignee(t *testing.T) {
	ctx := context.Background()
	repo := newFakeIssueRepo()
	svc := NewIssueService(repo, newFakeUserDirectory(), contributorRoleResolver())
	reporterID := uuid.New()
	assigneeID := uuid.New()

	unassigned, _ := domain.NewIssue("Unassigned", "Desc", domain.PriorityLow, reporterID)
	assigned, _ := domain.NewIssue("Assigned", "Desc", domain.PriorityLow, reporterID)
	assigned.AssigneeID = &assigneeID
	for _, issue := range []*domain.Issue{unassigned, assigned} {
		if err := repo.Save(ctx, issue); err != nil {
			t.Fatal(err)
		}
	}

	none, err := svc.List(ctx, reporterID, "admin", port.ListFilter{Unassigned: true, Page: 1, PageSize: 20})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if none.Total != 1 || len(none.Items) != 1 || none.Items[0].ID != unassigned.ID {
		t.Errorf("unassigned filter returned total=%d items=%d, want only the unassigned issue", none.Total, len(none.Items))
	}

	byAssignee, err := svc.List(ctx, reporterID, "admin", port.ListFilter{AssigneeID: &assigneeID, Page: 1, PageSize: 20})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if byAssignee.Total != 1 || len(byAssignee.Items) != 1 || byAssignee.Items[0].ID != assigned.ID {
		t.Errorf("assignee filter returned total=%d items=%d, want only the assigned issue", byAssignee.Total, len(byAssignee.Items))
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

type fakeBacklogRepo struct {
	items []*domain.Issue
	saved map[uuid.UUID]string
}

func (r *fakeBacklogRepo) ListBacklog(context.Context, uuid.UUID) ([]*domain.Issue, error) {
	return r.items, nil
}

func (r *fakeBacklogRepo) SaveRanks(_ context.Context, _ uuid.UUID, ranks map[uuid.UUID]string) error {
	r.saved = ranks
	return nil
}

// TestIssueService_MoveInBacklogRenumbersRanks comprueba el reordenado, la renumeracion de rank,
// la validacion de posicion y los permisos (viewer sin acceso a mover).
func TestIssueService_MoveInBacklogRenumbersRanks(t *testing.T) {
	ctx := context.Background()
	first := &domain.Issue{ID: uuid.New(), ProjectID: domain.DefaultProjectID, Summary: "primera", Rank: "000001"}
	second := &domain.Issue{ID: uuid.New(), ProjectID: domain.DefaultProjectID, Summary: "segunda", Rank: "000002"}
	third := &domain.Issue{ID: uuid.New(), ProjectID: domain.DefaultProjectID, Summary: "tercera", Rank: "000003"}
	newRepo := func() *fakeBacklogRepo {
		return &fakeBacklogRepo{items: []*domain.Issue{first, second, third}}
	}

	repo := newRepo()
	svc := NewIssueService(newFakeIssueRepo(), newFakeUserDirectory(), repo, contributorRoleResolver())
	ordered, err := svc.MoveInBacklog(ctx, uuid.New(), "agent", domain.DefaultProjectID, first.ID, 2)
	if err != nil {
		t.Fatalf("contributor should reorder the backlog, got %v", err)
	}
	if ordered[0].ID != second.ID || ordered[1].ID != third.ID || ordered[2].ID != first.ID {
		t.Fatalf("unexpected order: %s, %s, %s", ordered[0].Summary, ordered[1].Summary, ordered[2].Summary)
	}
	if repo.saved[first.ID] != "000003" || repo.saved[second.ID] != "000001" || repo.saved[third.ID] != "000002" {
		t.Fatalf("saved ranks = %v", repo.saved)
	}
	if first.Rank != "000003" {
		t.Fatalf("rank not updated in memory: %s", first.Rank)
	}

	outOfRange := NewIssueService(newFakeIssueRepo(), newFakeUserDirectory(), newRepo(), contributorRoleResolver())
	if _, err := outOfRange.MoveInBacklog(ctx, uuid.New(), "agent", domain.DefaultProjectID, second.ID, 3); apperror.KindOf(err) != apperror.KindValidation {
		t.Fatalf("position out of range: want validation, got %v", err)
	}

	unknown := NewIssueService(newFakeIssueRepo(), newFakeUserDirectory(), newRepo(), contributorRoleResolver())
	if _, err := unknown.MoveInBacklog(ctx, uuid.New(), "agent", domain.DefaultProjectID, uuid.New(), 0); !errors.Is(err, domain.ErrIssueNotFound) {
		t.Fatalf("unknown issue: got %v", err)
	}

	viewer := fakeProjectRoleResolver{role: stringPtr("viewer")}
	denied := NewIssueService(newFakeIssueRepo(), newFakeUserDirectory(), newRepo(), viewer)
	if _, err := denied.MoveInBacklog(ctx, uuid.New(), "requester", domain.DefaultProjectID, first.ID, 0); apperror.KindOf(err) != apperror.KindForbidden {
		t.Fatalf("viewer should be forbidden, got %v", err)
	}
}

func stringPtr(value string) *string {
	return &value
}

// fakeSharedRepo cumple AssigneeWatcherRepository y BacklogRepository a la vez, como el repositorio de
// Postgres. Protege contra que un case del switch de wiring tape al otro.
type fakeSharedRepo struct {
	fakeBacklogRepo
}

func (fakeSharedRepo) UpdateAndWatchAssignee(context.Context, *domain.Issue, uuid.UUID) error {
	return nil
}

// TestIssueService_BacklogWiredWhenRepositoryAlsoImplementsOtherPorts es la regresion del bug en que el
// backlog no se conectaba porque el repositorio tambien cumple AssigneeWatcherRepository.
func TestIssueService_BacklogWiredWhenRepositoryAlsoImplementsOtherPorts(t *testing.T) {
	shared := &fakeSharedRepo{fakeBacklogRepo: fakeBacklogRepo{items: []*domain.Issue{}}}
	svc := NewIssueService(newFakeIssueRepo(), newFakeUserDirectory(), shared, contributorRoleResolver())
	if _, err := svc.Backlog(context.Background(), uuid.New(), "agent", domain.DefaultProjectID); err != nil {
		t.Fatalf("backlog must be wired, got %v", err)
	}
}

type fakeNotifier struct {
	calls []uuid.UUID
}

func (f *fakeNotifier) Notify(_ context.Context, userID uuid.UUID, _, _, _ string) error {
	f.calls = append(f.calls, userID)
	return nil
}

// TestIssueService_AssignNotifiesAssignee comprueba que asignar a otra persona la avisa, y que asignarse a si
// mismo no genera aviso.
func TestIssueService_AssignNotifiesAssignee(t *testing.T) {
	ctx := context.Background()
	repo := newFakeIssueRepo()
	dir := newFakeUserDirectory()
	team := uuid.New()
	assigner, assignee := uuid.New(), uuid.New()
	dir.userTeams[assigner] = &team
	dir.userTeams[assignee] = &team
	notifier := &fakeNotifier{}
	svc := NewIssueService(repo, dir, contributorRoleResolver(), newFakeWatcherRepository()).WithNotifier(notifier)

	issue, _ := svc.Create(ctx, port.CreateInput{Summary: "Avisame", Description: "Desc", Priority: domain.PriorityLow, ReporterID: uuid.New()})
	if _, err := svc.Assign(ctx, issue.ID, assigner, assignee, "agent"); err != nil {
		t.Fatalf("assign: %v", err)
	}
	if len(notifier.calls) != 1 || notifier.calls[0] != assignee {
		t.Fatalf("notifications = %v, want one for the assignee", notifier.calls)
	}
	if _, err := svc.Assign(ctx, issue.ID, assignee, assignee, "agent"); err != nil {
		t.Fatalf("self assign: %v", err)
	}
	if len(notifier.calls) != 1 {
		t.Fatalf("self assignment must not notify, calls = %d", len(notifier.calls))
	}
}
