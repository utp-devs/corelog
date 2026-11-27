package http_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	authport "github.com/jhonsferg/corelog/internal/authorization/application/port"
	authusecase "github.com/jhonsferg/corelog/internal/authorization/application/usecase"
	authdomain "github.com/jhonsferg/corelog/internal/authorization/domain"

	issuehttp "github.com/jhonsferg/corelog/internal/issue/adapter/in/http"
	issueport "github.com/jhonsferg/corelog/internal/issue/application/port"
	"github.com/jhonsferg/corelog/internal/issue/application/usecase"
	"github.com/jhonsferg/corelog/internal/issue/domain"
	"github.com/jhonsferg/corelog/internal/platform/apperror"
	"github.com/jhonsferg/corelog/internal/platform/authtoken"
)

type memoryIssueRepo struct {
	issues map[uuid.UUID]*domain.Issue
}

func newMemoryIssueRepo() *memoryIssueRepo {
	return &memoryIssueRepo{issues: make(map[uuid.UUID]*domain.Issue)}
}

func (r *memoryIssueRepo) Save(_ context.Context, t *domain.Issue) error {
	if t.ID == uuid.Nil {
		t.ID = uuid.New()
	}
	r.nextKey(t)
	copied := *t
	r.issues[t.ID] = &copied
	return nil
}

// nextKey gives a new issue the next number of its project, as the real repository does.
func (r *memoryIssueRepo) nextKey(t *domain.Issue) {
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
func (r *memoryIssueRepo) FindKeyCandidates(_ context.Context, projectKey string, number int, _, _ uuid.UUID, _ bool) ([]*domain.Issue, error) {
	var out []*domain.Issue
	for _, t := range r.issues {
		if t.ProjectKey == projectKey && t.KeyNumber == number {
			copied := *t
			out = append(out, &copied)
		}
	}
	return out, nil
}

func (r *memoryIssueRepo) FindByID(_ context.Context, id uuid.UUID) (*domain.Issue, error) {
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

func (r *memoryIssueRepo) Count(ctx context.Context, filter issueport.ListFilter) (int, error) {
	unpaged := filter
	unpaged.Page = 1
	unpaged.PageSize = math.MaxInt32
	all, err := r.FindAll(ctx, unpaged)
	if err != nil {
		return 0, err
	}
	return len(all), nil
}

func (r *memoryIssueRepo) FindAll(_ context.Context, filter issueport.ListFilter) ([]*domain.Issue, error) {
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

func (r *memoryIssueRepo) Update(_ context.Context, t *domain.Issue) error {
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

type memoryDirectory struct {
	teams map[uuid.UUID]*uuid.UUID
	names map[uuid.UUID]string
}

type memorySavedFilterRepository struct {
	filters map[uuid.UUID]*domain.SavedFilter
}

func (r *memorySavedFilterRepository) SaveSavedFilter(_ context.Context, filter *domain.SavedFilter) error {
	if filter.ID == uuid.Nil {
		filter.ID = uuid.New()
	}
	if r.filters == nil {
		r.filters = make(map[uuid.UUID]*domain.SavedFilter)
	}
	for _, existing := range r.filters {
		if existing.OwnerID == filter.OwnerID && existing.Name == filter.Name {
			return apperror.New(apperror.KindConflict, "duplicate name")
		}
	}
	r.filters[filter.ID] = filter
	return nil
}

func (r *memorySavedFilterRepository) ListSavedFilters(_ context.Context, ownerID uuid.UUID) ([]*domain.SavedFilter, error) {
	filters := make([]*domain.SavedFilter, 0)
	for _, filter := range r.filters {
		if filter.OwnerID == ownerID {
			filters = append(filters, filter)
		}
	}
	return filters, nil
}

func (r *memorySavedFilterRepository) DeleteSavedFilter(_ context.Context, ownerID, filterID uuid.UUID) error {
	filter, ok := r.filters[filterID]
	if !ok || filter.OwnerID != ownerID {
		return apperror.New(apperror.KindNotFound, "saved filter not found")
	}
	delete(r.filters, filterID)
	return nil
}

type projectRoleResolver struct{}

func (projectRoleResolver) FindRole(context.Context, uuid.UUID, uuid.UUID) (*string, error) {
	role := "contributor"
	return &role, nil
}

func (projectRoleResolver) ListProjectIDs(context.Context, uuid.UUID) ([]uuid.UUID, error) {
	return []uuid.UUID{uuid.MustParse("00000000-0000-4000-8000-000000000001")}, nil
}

func newMemoryDirectory() *memoryDirectory {
	return &memoryDirectory{teams: make(map[uuid.UUID]*uuid.UUID), names: make(map[uuid.UUID]string)}
}

func (d *memoryDirectory) NamesOf(_ context.Context, userIDs []uuid.UUID) (map[uuid.UUID]string, error) {
	out := make(map[uuid.UUID]string, len(userIDs))
	for _, id := range userIDs {
		if name, ok := d.names[id]; ok {
			out[id] = name
		}
	}
	return out, nil
}

func (d *memoryDirectory) SharesTeam(_ context.Context, userA, userB uuid.UUID) (bool, error) {
	teamA, okA := d.teams[userA]
	teamB, okB := d.teams[userB]
	return okA && okB && teamA != nil && teamB != nil && *teamA == *teamB, nil
}

func setupIssueRouter() (*chi.Mux, string, *memoryIssueRepo, *memoryDirectory) {
	jwtSecret := "test-secret-key-that-is-long-enough-32-bytes"
	repo := newMemoryIssueRepo()
	dir := newMemoryDirectory()
	service := usecase.NewIssueService(repo, dir, projectRoleResolver{}, &memorySavedFilterRepository{})
	handler := issuehttp.NewHandler(service, jwtSecret).WithComments(newStubComments())

	r := chi.NewRouter()
	handler.RegisterRoutes(r)
	return r, jwtSecret, repo, dir
}

func TestIssueHTTP_Handlers(t *testing.T) {
	router, jwtSecret, repo, dir := setupIssueRouter()

	userA := uuid.New()
	userB := uuid.New()
	userOtherTeam := uuid.New()

	team1 := uuid.New()
	team2 := uuid.New()
	dir.teams[userA] = &team1
	dir.teams[userB] = &team1
	dir.teams[userOtherTeam] = &team2

	tokenUserA, _ := authtoken.Generate(jwtSecret, time.Hour, userA, "requester")
	tokenUserB, _ := authtoken.Generate(jwtSecret, time.Hour, userB, "requester")

	t.Run("POST /issues requires authentication (401)", func(t *testing.T) {
		body := `{"summary":"Issue","description":"Desc","priority":"high"}`
		req := httptest.NewRequest(http.MethodPost, "/issues", bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Errorf("got status %d, want %d", rec.Code, http.StatusUnauthorized)
		}
	})

	t.Run("GET /saved-filters requires authentication (401)", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/saved-filters", nil)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("got status %d, want %d", rec.Code, http.StatusUnauthorized)
		}
	})

	t.Run("POST /issues creates issue (201)", func(t *testing.T) {
		body := `{"summary":"Server crash","description":"Server crashes under load","priority":"critical"}`
		req := httptest.NewRequest(http.MethodPost, "/issues", bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+tokenUserA)
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusCreated {
			t.Fatalf("got status %d, want %d; body: %s", rec.Code, http.StatusCreated, rec.Body.String())
		}
	})

	t.Run("POST /issues rejects validation error (422)", func(t *testing.T) {
		body := `{"summary":"","description":"","priority":"invalid"}`
		req := httptest.NewRequest(http.MethodPost, "/issues", bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+tokenUserA)
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusUnprocessableEntity {
			t.Errorf("got status %d, want %d", rec.Code, http.StatusUnprocessableEntity)
		}
	})

	t.Run("saved filters are private and deletions are owner-scoped", func(t *testing.T) {
		body := `{"name":"Open bugs","filters":{"status":"open","issue_type":"bug","query":"timeout"}}`
		createReq := httptest.NewRequest(http.MethodPost, "/saved-filters", bytes.NewBufferString(body))
		createReq.Header.Set("Content-Type", "application/json")
		createReq.Header.Set("Authorization", "Bearer "+tokenUserA)
		createRec := httptest.NewRecorder()
		router.ServeHTTP(createRec, createReq)
		if createRec.Code != http.StatusCreated {
			t.Fatalf("create saved filter status=%d body=%s", createRec.Code, createRec.Body.String())
		}
		var created struct {
			Data struct {
				ID string `json:"id"`
			} `json:"data"`
		}
		if err := json.Unmarshal(createRec.Body.Bytes(), &created); err != nil {
			t.Fatal(err)
		}

		listReq := httptest.NewRequest(http.MethodGet, "/saved-filters", nil)
		listReq.Header.Set("Authorization", "Bearer "+tokenUserB)
		listRec := httptest.NewRecorder()
		router.ServeHTTP(listRec, listReq)
		if listRec.Code != http.StatusOK || listRec.Body.String() != "{\"data\":[]}\n" {
			t.Fatalf("other user's saved filters status=%d body=%s", listRec.Code, listRec.Body.String())
		}

		ownerListReq := httptest.NewRequest(http.MethodGet, "/saved-filters", nil)
		ownerListReq.Header.Set("Authorization", "Bearer "+tokenUserA)
		ownerListRec := httptest.NewRecorder()
		router.ServeHTTP(ownerListRec, ownerListReq)
		if ownerListRec.Code != http.StatusOK || !bytes.Contains(ownerListRec.Body.Bytes(), []byte("Open bugs")) {
			t.Fatalf("owner's filter not returned: status=%d body=%s", ownerListRec.Code, ownerListRec.Body.String())
		}

		deleteOtherReq := httptest.NewRequest(http.MethodDelete, "/saved-filters/"+created.Data.ID, nil)
		deleteOtherReq.Header.Set("Authorization", "Bearer "+tokenUserB)
		deleteOtherRec := httptest.NewRecorder()
		router.ServeHTTP(deleteOtherRec, deleteOtherReq)
		if deleteOtherRec.Code != http.StatusNotFound {
			t.Fatalf("cross-owner delete status=%d body=%s", deleteOtherRec.Code, deleteOtherRec.Body.String())
		}

		deleteOwnerReq := httptest.NewRequest(http.MethodDelete, "/saved-filters/"+created.Data.ID, nil)
		deleteOwnerReq.Header.Set("Authorization", "Bearer "+tokenUserA)
		deleteOwnerRec := httptest.NewRecorder()
		router.ServeHTTP(deleteOwnerRec, deleteOwnerReq)
		if deleteOwnerRec.Code != http.StatusNoContent {
			t.Fatalf("owner delete status=%d body=%s", deleteOwnerRec.Code, deleteOwnerRec.Body.String())
		}
	})

	// Create a issue for status & assign tests
	issue, _ := domain.NewIssue("Issue for transitions", "Desc", domain.PriorityMedium, userA)
	issue.ProjectID = uuid.MustParse("00000000-0000-4000-8000-000000000001")
	_ = repo.Save(context.Background(), issue)

	t.Run("GET /issues/{id} returns 200 for existing issue", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/issues/"+issue.ID.String(), nil)
		req.Header.Set("Authorization", "Bearer "+tokenUserA)
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("got status %d, want %d", rec.Code, http.StatusOK)
		}
	})

	t.Run("GET /issues/{id} returns 404 for unknown issue", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/issues/"+uuid.New().String(), nil)
		req.Header.Set("Authorization", "Bearer "+tokenUserA)
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusNotFound {
			t.Errorf("got status %d, want %d", rec.Code, http.StatusNotFound)
		}
	})

	t.Run("PATCH /issues/{id}/assign returns 403 on cross-team assignment", func(t *testing.T) {
		body := fmt.Sprintf(`{"assignee_id":"%s"}`, userOtherTeam.String())
		req := httptest.NewRequest(http.MethodPatch, "/issues/"+issue.ID.String()+"/assign", bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+tokenUserA)
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusForbidden {
			t.Errorf("got status %d, want %d; body: %s", rec.Code, http.StatusForbidden, rec.Body.String())
		}
	})

	t.Run("PATCH /issues/{id}/assign returns 200 on same-team assignment", func(t *testing.T) {
		body := fmt.Sprintf(`{"assignee_id":"%s"}`, userB.String())
		req := httptest.NewRequest(http.MethodPatch, "/issues/"+issue.ID.String()+"/assign", bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+tokenUserA)
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("got status %d, want %d; body: %s", rec.Code, http.StatusOK, rec.Body.String())
		}
	})
}

func TestIssueHTTP_ListReturnsPaginationMeta(t *testing.T) {
	router, jwtSecret, _, _ := setupIssueRouter()
	token, _ := authtoken.Generate(jwtSecret, time.Hour, uuid.New(), "admin")

	for i := 0; i < 3; i++ {
		body := `{"summary":"Issue list","description":"Desc","priority":"low"}`
		req := httptest.NewRequest(http.MethodPost, "/issues", bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+token)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		if rec.Code != http.StatusCreated {
			t.Fatalf("create got status %d, want %d", rec.Code, http.StatusCreated)
		}
	}

	req := httptest.NewRequest(http.MethodGet, "/issues?page=1&page_size=2", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("list got status %d, want %d", rec.Code, http.StatusOK)
	}

	var payload struct {
		Data []map[string]any `json:"data"`
		Meta struct {
			Page     int `json:"page"`
			PageSize int `json:"page_size"`
			Total    int `json:"total"`
		} `json:"meta"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if len(payload.Data) != 2 {
		t.Errorf("got %d items on the page, want 2", len(payload.Data))
	}
	if payload.Meta.Total != 3 {
		t.Errorf("meta.total = %d, want 3 (all matches, not only this page)", payload.Meta.Total)
	}
	if payload.Meta.Page != 1 || payload.Meta.PageSize != 2 {
		t.Errorf("meta page/page_size = %d/%d, want 1/2", payload.Meta.Page, payload.Meta.PageSize)
	}
}

func TestIssueHTTP_ListFiltersByAssignee(t *testing.T) {
	router, jwtSecret, repo, _ := setupIssueRouter()
	token, _ := authtoken.Generate(jwtSecret, time.Hour, uuid.New(), "admin")
	assigneeID := uuid.New()

	unassigned, _ := domain.NewIssue("Unassigned list", "Desc", domain.PriorityLow, uuid.New())
	assigned, _ := domain.NewIssue("Assigned list", "Desc", domain.PriorityLow, uuid.New())
	assigned.AssigneeID = &assigneeID
	for _, issue := range []*domain.Issue{unassigned, assigned} {
		if err := repo.Save(context.Background(), issue); err != nil {
			t.Fatal(err)
		}
	}

	list := func(t *testing.T, query string) (int, []string) {
		t.Helper()
		req := httptest.NewRequest(http.MethodGet, "/issues?"+query, nil)
		req.Header.Set("Authorization", "Bearer "+token)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			return rec.Code, nil
		}
		var payload struct {
			Data []struct {
				ID string `json:"id"`
			} `json:"data"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
			t.Fatalf("invalid JSON: %v", err)
		}
		ids := make([]string, 0, len(payload.Data))
		for _, item := range payload.Data {
			ids = append(ids, item.ID)
		}
		return rec.Code, ids
	}

	t.Run("none returns unassigned issues", func(t *testing.T) {
		code, ids := list(t, "assignee_id=none")
		if code != http.StatusOK {
			t.Fatalf("status = %d, want %d", code, http.StatusOK)
		}
		if len(ids) != 1 || ids[0] != unassigned.ID.String() {
			t.Errorf("ids = %v, want only %s", ids, unassigned.ID)
		}
	})

	t.Run("uuid returns issues of that assignee", func(t *testing.T) {
		code, ids := list(t, "assignee_id="+assigneeID.String())
		if code != http.StatusOK {
			t.Fatalf("status = %d, want %d", code, http.StatusOK)
		}
		if len(ids) != 1 || ids[0] != assigned.ID.String() {
			t.Errorf("ids = %v, want only %s", ids, assigned.ID)
		}
	})

	t.Run("other values are rejected", func(t *testing.T) {
		for _, value := range []string{"nobody", "None", "null"} {
			code, _ := list(t, "assignee_id="+value)
			if code != http.StatusUnprocessableEntity {
				t.Errorf("assignee_id=%s status = %d, want %d", value, code, http.StatusUnprocessableEntity)
			}
		}
	})
}

func TestIssueHTTP_ResponsesIncludePersonNames(t *testing.T) {
	router, jwtSecret, _, dir := setupIssueRouter()
	reporter := uuid.New()
	dir.names[reporter] = "Ana Torres"
	token, _ := authtoken.Generate(jwtSecret, time.Hour, reporter, "admin")

	body := `{"summary":"Named issue","description":"Desc","priority":"low"}`
	create := httptest.NewRequest(http.MethodPost, "/issues", bytes.NewBufferString(body))
	create.Header.Set("Content-Type", "application/json")
	create.Header.Set("Authorization", "Bearer "+token)
	createRec := httptest.NewRecorder()
	router.ServeHTTP(createRec, create)
	if createRec.Code != http.StatusCreated {
		t.Fatalf("create got status %d, want %d", createRec.Code, http.StatusCreated)
	}
	var created struct {
		Data struct {
			ID           string `json:"id"`
			ReporterName string `json:"reporter_name"`
		} `json:"data"`
	}
	if err := json.Unmarshal(createRec.Body.Bytes(), &created); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if created.Data.ReporterName != "Ana Torres" {
		t.Errorf("create reporter_name = %q, want Ana Torres", created.Data.ReporterName)
	}

	get := httptest.NewRequest(http.MethodGet, "/issues/"+created.Data.ID, nil)
	get.Header.Set("Authorization", "Bearer "+token)
	getRec := httptest.NewRecorder()
	router.ServeHTTP(getRec, get)
	if getRec.Code != http.StatusOK {
		t.Fatalf("get got status %d, want %d", getRec.Code, http.StatusOK)
	}
	var fetched struct {
		Data struct {
			ReporterName string  `json:"reporter_name"`
			AssigneeName *string `json:"assignee_name"`
		} `json:"data"`
	}
	if err := json.Unmarshal(getRec.Body.Bytes(), &fetched); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if fetched.Data.ReporterName != "Ana Torres" {
		t.Errorf("get reporter_name = %q, want Ana Torres", fetched.Data.ReporterName)
	}
	if fetched.Data.AssigneeName != nil {
		t.Errorf("assignee_name = %q, want absent when unassigned", *fetched.Data.AssigneeName)
	}
}

// roleLookupOf adapta el resolutor de roles del modulo al puerto de autorizacion del hexagono authorization.
type roleLookupOf struct {
	resolver issueport.ProjectRoleResolver
}

func (l roleLookupOf) FindRole(ctx context.Context, projectID, userID uuid.UUID) (*authdomain.ProjectRole, error) {
	role, err := l.resolver.FindRole(ctx, projectID, userID)
	if err != nil || role == nil {
		return nil, err
	}
	value := authdomain.ProjectRole(*role)
	return &value, nil
}

func authorizeWithResolver(ctx context.Context, resolver issueport.ProjectRoleResolver, action string, actorID uuid.UUID, globalRole string, projectID uuid.UUID) error {
	return authusecase.NewAuthorizer(roleLookupOf{resolver: resolver}).Authorize(ctx, authport.Action(action), actorID, globalRole, projectID)
}

func (r projectRoleResolver) Authorize(ctx context.Context, action string, actorID uuid.UUID, globalRole string, projectID uuid.UUID) error {
	return authorizeWithResolver(ctx, r, action, actorID, globalRole, projectID)
}

func TestIssueHTTP_KeyAndFieldNames(t *testing.T) {
	router, jwtSecret, _, _ := setupIssueRouter()
	reporter := uuid.New()
	token, _ := authtoken.Generate(jwtSecret, time.Hour, reporter, "admin")

	do := func(method, path, body string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(method, path, bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+token)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		return rec
	}

	first := do(http.MethodPost, "/issues", `{"summary":"First","description":"Desc","priority":"low"}`)
	second := do(http.MethodPost, "/issues", `{"summary":"Second","description":"Desc","priority":"low"}`)
	if first.Code != http.StatusCreated || second.Code != http.StatusCreated {
		t.Fatalf("create statuses = %d, %d", first.Code, second.Code)
	}

	var fields map[string]map[string]any
	if err := json.Unmarshal(second.Body.Bytes(), &fields); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	data := fields["data"]
	for _, name := range []string{"summary", "reporter_id", "reporter_name", "key", "project_key"} {
		if _, ok := data[name]; !ok {
			t.Errorf("response is missing %q", name)
		}
	}
	for _, name := range []string{"title", "requester_id", "requester_name"} {
		if _, ok := data[name]; ok {
			t.Errorf("response still carries legacy field %q", name)
		}
	}
	if data["key"] != "CORE-2" || data["project_key"] != "CORE" {
		t.Errorf("key = %v, project_key = %v, want CORE-2 and CORE", data["key"], data["project_key"])
	}

	byKey := do(http.MethodGet, "/issues/CORE-2", "")
	if byKey.Code != http.StatusOK {
		t.Fatalf("get by key status = %d, body %s", byKey.Code, byKey.Body.String())
	}
	var fetched struct {
		Data struct {
			ID      string `json:"id"`
			Summary string `json:"summary"`
		} `json:"data"`
	}
	_ = json.Unmarshal(byKey.Body.Bytes(), &fetched)
	if fetched.Data.Summary != "Second" {
		t.Errorf("get by key returned %q, want Second", fetched.Data.Summary)
	}
	if byID := do(http.MethodGet, "/issues/"+fetched.Data.ID, ""); byID.Code != http.StatusOK {
		t.Errorf("get by uuid status = %d", byID.Code)
	}
	if rec := do(http.MethodGet, "/issues/CORE-2/comments", ""); rec.Code != http.StatusOK {
		t.Errorf("subresource by key status = %d, body %s", rec.Code, rec.Body.String())
	}
	if rec := do(http.MethodGet, "/issues/CORE-99", ""); rec.Code != http.StatusNotFound {
		t.Errorf("unknown key status = %d, want 404", rec.Code)
	}
	if rec := do(http.MethodGet, "/issues/core-2", ""); rec.Code != http.StatusUnprocessableEntity {
		t.Errorf("malformed key status = %d, want 422", rec.Code)
	}
	if rec := do(http.MethodGet, "/issues/CORE-2?organization_id=nope", ""); rec.Code != http.StatusUnprocessableEntity {
		t.Errorf("bad organization_id status = %d, want 422", rec.Code)
	}
}
