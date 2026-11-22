package http_test

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/jhonsferg/corelog/internal/platform/authtoken"
	teamhttp "github.com/jhonsferg/corelog/internal/team/adapter/in/http"
	"github.com/jhonsferg/corelog/internal/team/application/usecase"
	"github.com/jhonsferg/corelog/internal/team/domain"
)

type memoryTeamRepo struct {
	teams map[uuid.UUID]*domain.Team
}

func newMemoryTeamRepo() *memoryTeamRepo {
	return &memoryTeamRepo{teams: make(map[uuid.UUID]*domain.Team)}
}

func (r *memoryTeamRepo) Save(_ context.Context, t *domain.Team) error {
	if t.ID == uuid.Nil {
		t.ID = uuid.New()
	}
	copied := *t
	r.teams[t.ID] = &copied
	return nil
}

func (r *memoryTeamRepo) FindByID(_ context.Context, id uuid.UUID) (*domain.Team, error) {
	t, ok := r.teams[id]
	if !ok {
		return nil, domain.ErrTeamNotFound
	}
	if t.ID == uuid.Nil {
		t.ID = uuid.New()
	}
	copied := *t
	return &copied, nil
}

func (r *memoryTeamRepo) FindByName(_ context.Context, name string) (*domain.Team, error) {
	for _, t := range r.teams {
		if t.Name == name {
			copied := *t
			return &copied, nil
		}
	}
	return nil, domain.ErrTeamNotFound
}

func (r *memoryTeamRepo) FindAll(_ context.Context) ([]*domain.Team, error) {
	var list []*domain.Team
	for _, t := range r.teams {
		copied := *t
		list = append(list, &copied)
	}
	return list, nil
}

func (r *memoryTeamRepo) Update(_ context.Context, t *domain.Team) error {
	if _, ok := r.teams[t.ID]; !ok {
		return domain.ErrTeamNotFound
	}
	if t.ID == uuid.Nil {
		t.ID = uuid.New()
	}
	copied := *t
	r.teams[t.ID] = &copied
	return nil
}

func (r *memoryTeamRepo) Delete(_ context.Context, id uuid.UUID) error {
	if _, ok := r.teams[id]; !ok {
		return domain.ErrTeamNotFound
	}
	delete(r.teams, id)
	return nil
}

func setupTeamRouter() (*chi.Mux, string, *memoryTeamRepo) {
	jwtSecret := "test-secret-key-that-is-long-enough-32-bytes"
	repo := newMemoryTeamRepo()
	service := usecase.NewTeamService(repo)
	handler := teamhttp.NewHandler(service, jwtSecret)

	r := chi.NewRouter()
	handler.RegisterRoutes(r)
	return r, jwtSecret, repo
}

func TestTeamHTTP_Handlers(t *testing.T) {
	router, jwtSecret, repo := setupTeamRouter()

	adminToken, _ := authtoken.Generate(jwtSecret, time.Hour, uuid.New(), "admin")
	requesterToken, _ := authtoken.Generate(jwtSecret, time.Hour, uuid.New(), "requester")

	t.Run("POST /teams requires authentication (401)", func(t *testing.T) {
		body := `{"name":"Security"}`
		req := httptest.NewRequest(http.MethodPost, "/teams", bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Errorf("got status %d, want %d", rec.Code, http.StatusUnauthorized)
		}
	})

	t.Run("POST /teams requires admin role (403)", func(t *testing.T) {
		body := `{"name":"Security"}`
		req := httptest.NewRequest(http.MethodPost, "/teams", bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+requesterToken)
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusForbidden {
			t.Errorf("got status %d, want %d", rec.Code, http.StatusForbidden)
		}
	})

	t.Run("POST /teams succeeds with admin token (201)", func(t *testing.T) {
		body := `{"name":"Engineering"}`
		req := httptest.NewRequest(http.MethodPost, "/teams", bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+adminToken)
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusCreated {
			t.Fatalf("got status %d, want %d; body: %s", rec.Code, http.StatusCreated, rec.Body.String())
		}
	})

	t.Run("POST /teams rejects duplicate name (409)", func(t *testing.T) {
		body := `{"name":"Engineering"}`
		req := httptest.NewRequest(http.MethodPost, "/teams", bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+adminToken)
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusConflict {
			t.Errorf("got status %d, want %d", rec.Code, http.StatusConflict)
		}
	})

	t.Run("POST /teams rejects invalid body (422)", func(t *testing.T) {
		body := `{"name":""}`
		req := httptest.NewRequest(http.MethodPost, "/teams", bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+adminToken)
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusUnprocessableEntity {
			t.Errorf("got status %d, want %d", rec.Code, http.StatusUnprocessableEntity)
		}
	})

	t.Run("GET /teams returns 200 for authenticated user", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/teams", nil)
		req.Header.Set("Authorization", "Bearer "+requesterToken)
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("got status %d, want %d", rec.Code, http.StatusOK)
		}
	})

	t.Run("GET /teams/{id} returns 404 for non-existent team", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/teams/"+uuid.New().String(), nil)
		req.Header.Set("Authorization", "Bearer "+requesterToken)
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusNotFound {
			t.Errorf("got status %d, want %d", rec.Code, http.StatusNotFound)
		}
	})

	t.Run("GET /teams/{id} returns 200 for existing team", func(t *testing.T) {
		created, _ := domain.NewTeam("Product")
		_ = repo.Save(context.Background(), created)

		req := httptest.NewRequest(http.MethodGet, "/teams/"+created.ID.String(), nil)
		req.Header.Set("Authorization", "Bearer "+requesterToken)
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("got status %d, want %d", rec.Code, http.StatusOK)
		}
	})
}
