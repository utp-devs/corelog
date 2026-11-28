package http_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/jhonsferg/corelog/internal/platform/authtoken"
	userhttp "github.com/jhonsferg/corelog/internal/user/adapter/in/http"
	userport "github.com/jhonsferg/corelog/internal/user/application/port"
	"github.com/jhonsferg/corelog/internal/user/application/usecase"
	"github.com/jhonsferg/corelog/internal/user/domain"
)

type memoryUserRepo struct {
	users       map[uuid.UUID]*domain.User
	memberships map[uuid.UUID]map[uuid.UUID]string // org -> user -> role
}

func newMemoryUserRepo() *memoryUserRepo {
	return &memoryUserRepo{
		users:       make(map[uuid.UUID]*domain.User),
		memberships: make(map[uuid.UUID]map[uuid.UUID]string),
	}
}

func (r *memoryUserRepo) RecordLogin(_ context.Context, userID uuid.UUID, at time.Time) error {
	u, ok := r.users[userID]
	if !ok {
		return domain.ErrUserNotFound
	}
	u.LastLoginAt = &at
	return nil
}

func (r *memoryUserRepo) FindOrganizationRole(_ context.Context, orgID, userID uuid.UUID) (string, bool, error) {
	role, ok := r.memberships[orgID][userID]
	return role, ok, nil
}

func (r *memoryUserRepo) SearchOrganizationMembers(_ context.Context, filter userport.OrganizationSearchFilter) ([]*domain.User, error) {
	var list []*domain.User
	for id, role := range r.memberships[filter.OrganizationID] {
		if filter.ActorIsGuest && role == "guest" {
			continue
		}
		if u, ok := r.users[id]; ok {
			copied := *u
			list = append(list, &copied)
		}
	}
	return list, nil
}

func (r *memoryUserRepo) Save(_ context.Context, u *domain.User) error {
	if u.ID == uuid.Nil {
		u.ID = uuid.New()
	}
	copied := *u
	r.users[u.ID] = &copied
	return nil
}

func (r *memoryUserRepo) FindByID(_ context.Context, id uuid.UUID) (*domain.User, error) {
	u, ok := r.users[id]
	if !ok {
		return nil, domain.ErrUserNotFound
	}
	if u.ID == uuid.Nil {
		u.ID = uuid.New()
	}
	copied := *u
	return &copied, nil
}

func (r *memoryUserRepo) FindByEmail(_ context.Context, email string) (*domain.User, error) {
	for _, u := range r.users {
		if strings.EqualFold(u.Email, email) {
			copied := *u
			return &copied, nil
		}
	}
	return nil, domain.ErrUserNotFound
}

func (r *memoryUserRepo) Update(_ context.Context, u *domain.User) error {
	if _, ok := r.users[u.ID]; !ok {
		return domain.ErrUserNotFound
	}
	if u.ID == uuid.Nil {
		u.ID = uuid.New()
	}
	copied := *u
	r.users[u.ID] = &copied
	return nil
}

func (r *memoryUserRepo) Count(_ context.Context) (int, error) {
	return len(r.users), nil
}

func (r *memoryUserRepo) Search(_ context.Context, filter userport.SearchFilter) ([]*domain.User, error) {
	var list []*domain.User
	for _, u := range r.users {
		if filter.TeamID != nil {
			if u.TeamID == nil || *u.TeamID != *filter.TeamID {
				continue
			}
		}
		if filter.Query != "" {
			q := strings.ToLower(filter.Query)
			if !strings.Contains(strings.ToLower(u.Name), q) && !strings.Contains(strings.ToLower(u.Email), q) {
				continue
			}
		}
		copied := *u
		list = append(list, &copied)
	}
	return list, nil
}

func setupUserRouter() (*chi.Mux, string, *memoryUserRepo) {
	jwtSecret := "test-secret-key-that-is-long-enough-32-bytes"
	repo := newMemoryUserRepo()
	service := usecase.NewUserService(repo, newMemorySessionRepo())
	handler := userhttp.NewHandler(service, jwtSecret, 24*time.Hour)

	r := chi.NewRouter()
	handler.RegisterRoutes(r)
	return r, jwtSecret, repo
}

func TestUserHTTP_Register(t *testing.T) {
	router, _, _ := setupUserRouter()

	t.Run("returns 201 on valid registration", func(t *testing.T) {
		body := `{"name":"Alice","email":"alice@example.com","password":"Secure-Pass-123x","password_confirmation":"Secure-Pass-123x"}`
		req := httptest.NewRequest(http.MethodPost, "/auth/register", bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusCreated {
			t.Fatalf("got status %d, want %d; body: %s", rec.Code, http.StatusCreated, rec.Body.String())
		}
	})

	t.Run("returns 422 on validation error", func(t *testing.T) {
		body := `{"name":"","email":"invalid-email","password":"123","password_confirmation":"123"}`
		req := httptest.NewRequest(http.MethodPost, "/auth/register", bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusUnprocessableEntity {
			t.Errorf("got status %d, want %d; body: %s", rec.Code, http.StatusUnprocessableEntity, rec.Body.String())
		}
	})

	t.Run("returns 409 on duplicate email", func(t *testing.T) {
		body := `{"name":"Alice","email":"alice@example.com","password":"Secure-Pass-123x","password_confirmation":"Secure-Pass-123x"}`
		req := httptest.NewRequest(http.MethodPost, "/auth/register", bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusConflict {
			t.Errorf("got status %d, want %d; body: %s", rec.Code, http.StatusConflict, rec.Body.String())
		}
	})
}

func TestUserHTTP_Login(t *testing.T) {
	router, _, _ := setupUserRouter()

	// Seed user
	regBody := `{"name":"Bob","email":"bob@example.com","password":"Tr1cky-Pass9x","password_confirmation":"Tr1cky-Pass9x"}`
	regReq := httptest.NewRequest(http.MethodPost, "/auth/register", bytes.NewBufferString(regBody))
	regReq.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(httptest.NewRecorder(), regReq)

	t.Run("returns 200 with JWT token on valid credentials", func(t *testing.T) {
		body := `{"email":"bob@example.com","password":"Tr1cky-Pass9x"}`
		req := httptest.NewRequest(http.MethodPost, "/auth/login", bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("got status %d, want %d; body: %s", rec.Code, http.StatusOK, rec.Body.String())
		}

		var resp struct {
			Data struct {
				Token string `json:"token"`
			} `json:"data"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}
		if resp.Data.Token == "" {
			t.Errorf("expected non-empty token")
		}
	})

	t.Run("returns 401 on invalid credentials", func(t *testing.T) {
		body := `{"email":"bob@example.com","password":"wrongpassword"}`
		req := httptest.NewRequest(http.MethodPost, "/auth/login", bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Errorf("got status %d, want %d", rec.Code, http.StatusUnauthorized)
		}
	})
}

func TestUserHTTP_AuthAndRoles(t *testing.T) {
	router, jwtSecret, repo := setupUserRouter()

	adminUser, _ := domain.NewUser("Admin", "admin@example.com", "hash", domain.RoleAdmin)
	requesterUser, _ := domain.NewUser("Requester", "req@example.com", "hash", domain.RoleUser)
	_ = repo.Save(context.Background(), adminUser)
	_ = repo.Save(context.Background(), requesterUser)

	adminToken, _ := authtoken.Generate(jwtSecret, time.Hour, adminUser.ID, string(adminUser.Role))
	requesterToken, _ := authtoken.Generate(jwtSecret, time.Hour, requesterUser.ID, string(requesterUser.Role))

	t.Run("returns 401 when token is missing", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/users/me", nil)
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Errorf("got status %d, want %d", rec.Code, http.StatusUnauthorized)
		}
	})

	t.Run("returns 200 on /users/me with valid token", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/users/me", nil)
		req.Header.Set("Authorization", "Bearer "+requesterToken)
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("got status %d, want %d; body: %s", rec.Code, http.StatusOK, rec.Body.String())
		}
	})

	t.Run("returns 403 when non-admin accesses admin endpoint", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/users", nil)
		req.Header.Set("Authorization", "Bearer "+requesterToken)
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusForbidden {
			t.Errorf("got status %d, want %d; body: %s", rec.Code, http.StatusForbidden, rec.Body.String())
		}
	})

	t.Run("returns 200 when admin accesses admin endpoint", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/users", nil)
		req.Header.Set("Authorization", "Bearer "+adminToken)
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("got status %d, want %d; body: %s", rec.Code, http.StatusOK, rec.Body.String())
		}
	})
}
