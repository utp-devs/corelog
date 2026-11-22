package http_test

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

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
	service := usecase.NewTicketService(repo, dir)
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

	t.Run("POST /tickets requires authentication (401)", func(t *testing.T) {
		body := `{"title":"Issue","description":"Desc","priority":"high"}`
		req := httptest.NewRequest(http.MethodPost, "/tickets", bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Errorf("got status %d, want %d", rec.Code, http.StatusUnauthorized)
		}
	})

	t.Run("POST /tickets creates ticket (201)", func(t *testing.T) {
		body := `{"title":"Server crash","description":"Server crashes under load","priority":"critical"}`
		req := httptest.NewRequest(http.MethodPost, "/tickets", bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+tokenUserA)
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusCreated {
			t.Fatalf("got status %d, want %d; body: %s", rec.Code, http.StatusCreated, rec.Body.String())
		}
	})

	t.Run("POST /tickets rejects validation error (422)", func(t *testing.T) {
		body := `{"title":"","description":"","priority":"invalid"}`
		req := httptest.NewRequest(http.MethodPost, "/tickets", bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+tokenUserA)
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusUnprocessableEntity {
			t.Errorf("got status %d, want %d", rec.Code, http.StatusUnprocessableEntity)
		}
	})

	// Create a ticket for status & assign tests
	ticket, _ := domain.NewTicket("Ticket for transitions", "Desc", domain.PriorityMedium, userA)
	_ = repo.Save(context.Background(), ticket)

	t.Run("GET /tickets/{id} returns 200 for existing ticket", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/tickets/"+ticket.ID.String(), nil)
		req.Header.Set("Authorization", "Bearer "+tokenUserA)
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("got status %d, want %d", rec.Code, http.StatusOK)
		}
	})

	t.Run("GET /tickets/{id} returns 404 for unknown ticket", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/tickets/"+uuid.New().String(), nil)
		req.Header.Set("Authorization", "Bearer "+tokenUserA)
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusNotFound {
			t.Errorf("got status %d, want %d", rec.Code, http.StatusNotFound)
		}
	})

	t.Run("PATCH /tickets/{id}/status returns 409 on illegal transition", func(t *testing.T) {
		// Open to Resolved is illegal
		body := `{"status":"resolved"}`
		req := httptest.NewRequest(http.MethodPatch, "/tickets/"+ticket.ID.String()+"/status", bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+tokenUserA)
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusConflict {
			t.Errorf("got status %d, want %d", rec.Code, http.StatusConflict)
		}
	})

	t.Run("PATCH /tickets/{id}/status returns 200 on valid transition", func(t *testing.T) {
		body := `{"status":"in_progress"}`
		req := httptest.NewRequest(http.MethodPatch, "/tickets/"+ticket.ID.String()+"/status", bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+tokenUserA)
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("got status %d, want %d; body: %s", rec.Code, http.StatusOK, rec.Body.String())
		}
	})

	t.Run("PATCH /tickets/{id}/assign returns 403 on cross-team assignment", func(t *testing.T) {
		body := fmt.Sprintf(`{"assignee_id":"%s"}`, userOtherTeam.String())
		req := httptest.NewRequest(http.MethodPatch, "/tickets/"+ticket.ID.String()+"/assign", bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+tokenUserA)
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusForbidden {
			t.Errorf("got status %d, want %d; body: %s", rec.Code, http.StatusForbidden, rec.Body.String())
		}
	})

	t.Run("PATCH /tickets/{id}/assign returns 200 on same-team assignment", func(t *testing.T) {
		body := fmt.Sprintf(`{"assignee_id":"%s"}`, userB.String())
		req := httptest.NewRequest(http.MethodPatch, "/tickets/"+ticket.ID.String()+"/assign", bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+tokenUserA)
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("got status %d, want %d; body: %s", rec.Code, http.StatusOK, rec.Body.String())
		}
	})
}
