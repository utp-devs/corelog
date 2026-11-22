package http_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/jhonsferg/corelog/internal/platform/apperror"
	"github.com/jhonsferg/corelog/internal/platform/authtoken"
	tickethttp "github.com/jhonsferg/corelog/internal/ticket/adapter/in/http"
	ticketport "github.com/jhonsferg/corelog/internal/ticket/application/port"
	"github.com/jhonsferg/corelog/internal/ticket/application/usecase"
	"github.com/jhonsferg/corelog/internal/ticket/domain"
)

type memoryTicketRepo struct {
	tickets map[uuid.UUID]*domain.Ticket
}

func newMemoryTicketRepo() *memoryTicketRepo {
	return &memoryTicketRepo{tickets: make(map[uuid.UUID]*domain.Ticket)}
}

func (r *memoryTicketRepo) Save(_ context.Context, t *domain.Ticket) error {
	copied := *t
	r.tickets[t.ID] = &copied
	return nil
}

func (r *memoryTicketRepo) FindByID(_ context.Context, id uuid.UUID) (*domain.Ticket, error) {
	t, ok := r.tickets[id]
	if !ok {
		return nil, domain.ErrTicketNotFound
	}
	copied := *t
	return &copied, nil
}

func (r *memoryTicketRepo) FindAll(_ context.Context, filter ticketport.ListFilter) ([]*domain.Ticket, error) {
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
	return list, nil
}

func (r *memoryTicketRepo) Update(_ context.Context, t *domain.Ticket) error {
	if _, ok := r.tickets[t.ID]; !ok {
		return domain.ErrTicketNotFound
	}
	copied := *t
	r.tickets[t.ID] = &copied
	return nil
}

type memoryDirectory struct {
	teams map[uuid.UUID]*uuid.UUID
}

type memorySavedFilterRepository struct {
	filters map[uuid.UUID]*domain.SavedFilter
}

func (r *memorySavedFilterRepository) SaveSavedFilter(_ context.Context, filter *domain.SavedFilter) error {
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
	return &memoryDirectory{teams: make(map[uuid.UUID]*uuid.UUID)}
}

func (d *memoryDirectory) TeamOf(_ context.Context, userID uuid.UUID) (*uuid.UUID, error) {
	teamID, ok := d.teams[userID]
	if !ok {
		return nil, nil
	}
	return teamID, nil
}

func setupTicketRouter() (*chi.Mux, string, *memoryTicketRepo, *memoryDirectory) {
	jwtSecret := "test-secret-key-that-is-long-enough-32-bytes"
	repo := newMemoryTicketRepo()
	dir := newMemoryDirectory()
	service := usecase.NewTicketService(repo, dir, projectRoleResolver{}, &memorySavedFilterRepository{})
	handler := tickethttp.NewHandler(service, jwtSecret)

	r := chi.NewRouter()
	handler.RegisterRoutes(r)
	return r, jwtSecret, repo, dir
}

func TestTicketHTTP_Handlers(t *testing.T) {
	router, jwtSecret, repo, dir := setupTicketRouter()

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
		body := `{"title":"Issue","description":"Desc","priority":"high"}`
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

	t.Run("POST /issues creates ticket (201)", func(t *testing.T) {
		body := `{"title":"Server crash","description":"Server crashes under load","priority":"critical"}`
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
		body := `{"title":"","description":"","priority":"invalid"}`
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

	// Create a ticket for status & assign tests
	ticket, _ := domain.NewTicket("Ticket for transitions", "Desc", domain.PriorityMedium, userA)
	ticket.ProjectID = uuid.MustParse("00000000-0000-4000-8000-000000000001")
	_ = repo.Save(context.Background(), ticket)

	t.Run("GET /issues/{id} returns 200 for existing ticket", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/issues/"+ticket.ID.String(), nil)
		req.Header.Set("Authorization", "Bearer "+tokenUserA)
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("got status %d, want %d", rec.Code, http.StatusOK)
		}
	})

	t.Run("GET /issues/{id} returns 404 for unknown ticket", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/issues/"+uuid.New().String(), nil)
		req.Header.Set("Authorization", "Bearer "+tokenUserA)
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusNotFound {
			t.Errorf("got status %d, want %d", rec.Code, http.StatusNotFound)
		}
	})

	t.Run("PATCH /issues/{id}/status returns 409 on illegal transition", func(t *testing.T) {
		// Open to Resolved is illegal
		body := `{"status":"resolved"}`
		req := httptest.NewRequest(http.MethodPatch, "/issues/"+ticket.ID.String()+"/status", bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+tokenUserA)
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusConflict {
			t.Errorf("got status %d, want %d", rec.Code, http.StatusConflict)
		}
	})

	t.Run("PATCH /issues/{id}/status returns 200 on valid transition", func(t *testing.T) {
		body := `{"status":"in_progress"}`
		req := httptest.NewRequest(http.MethodPatch, "/issues/"+ticket.ID.String()+"/status", bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+tokenUserA)
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("got status %d, want %d; body: %s", rec.Code, http.StatusOK, rec.Body.String())
		}
	})

	t.Run("PATCH /issues/{id}/assign returns 403 on cross-team assignment", func(t *testing.T) {
		body := fmt.Sprintf(`{"assignee_id":"%s"}`, userOtherTeam.String())
		req := httptest.NewRequest(http.MethodPatch, "/issues/"+ticket.ID.String()+"/assign", bytes.NewBufferString(body))
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
		req := httptest.NewRequest(http.MethodPatch, "/issues/"+ticket.ID.String()+"/assign", bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+tokenUserA)
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("got status %d, want %d; body: %s", rec.Code, http.StatusOK, rec.Body.String())
		}
	})
}
