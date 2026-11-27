package http_test

import (
	"context"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/jhonsferg/corelog/internal/platform/database"
	userhttp "github.com/jhonsferg/corelog/internal/user/adapter/in/http"
	"github.com/jhonsferg/corelog/internal/user/application/usecase"
)

// soleOwnerDetacher reports one organization owned solely by the account being closed.
type soleOwnerDetacher struct{}

func (soleOwnerDetacher) SoleOwnedOrganizations(context.Context, uuid.UUID) (int, error) {
	return 1, nil
}

func (soleOwnerDetacher) DetachMemberships(context.Context, uuid.UUID) error { return nil }

// TestUserHTTP_DeactivationBlockedForSoleOwner checks that closing an account that solely owns an
// organization answers 409 with the stable code and keeps the account usable.
func TestUserHTTP_DeactivationBlockedForSoleOwner(t *testing.T) {
	jwtSecret := "test-secret-key-that-is-long-enough-32-bytes"
	service := usecase.NewUserService(newMemoryUserRepo(), newMemorySessionRepo()).
		WithAccountClosure(&database.FakeTxManager{}, soleOwnerDetacher{})
	r := chi.NewRouter()
	userhttp.NewHandler(service, jwtSecret, 24*time.Hour).RegisterRoutes(r)

	const email, password = "unico-dueno@example.com", "password123"
	doJSON(t, r, http.MethodPost, "/auth/register", `{"name":"Dueno","email":"`+email+`","password":"`+password+`","password_confirmation":"`+password+`"}`, "")
	token := loginToken(t, r, email, password)

	rec := doJSON(t, r, http.MethodPost, "/users/me/deactivate", `{"password":"`+password+`"}`, token)
	if rec.Code != http.StatusConflict {
		t.Fatalf("status %d, want 409: %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "user.sole_owner") {
		t.Fatalf("body lacks user.sole_owner: %s", rec.Body.String())
	}
	if rec := doJSON(t, r, http.MethodGet, "/users/me/sessions", "", token); rec.Code != http.StatusOK {
		t.Fatalf("account must stay usable after the blocked closure, status %d", rec.Code)
	}
}
