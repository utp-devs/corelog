package http_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/jhonsferg/corelog/internal/platform/authtoken"
	"github.com/jhonsferg/corelog/internal/user/domain"
)

type profileEnvelope struct {
	Data  map[string]any `json:"data"`
	Error struct {
		Code string `json:"code"`
	} `json:"error"`
}

func decodeProfileEnvelope(t *testing.T, rec *httptest.ResponseRecorder) profileEnvelope {
	t.Helper()
	var env profileEnvelope
	if err := json.Unmarshal(rec.Body.Bytes(), &env); err != nil {
		t.Fatalf("decode body %q: %v", rec.Body.String(), err)
	}
	return env
}

func seedUser(t *testing.T, repo *memoryUserRepo, name string, role domain.Role) (*domain.User, string) {
	t.Helper()
	user, err := domain.NewUser(name, strings.ToLower(strings.ReplaceAll(name, " ", ""))+"@example.com", "hash", role)
	if err != nil {
		t.Fatal(err)
	}
	if err := repo.Save(context.Background(), user); err != nil {
		t.Fatal(err)
	}
	token, err := authtoken.Generate("test-secret-key-that-is-long-enough-32-bytes", time.Hour, user.ID, string(user.Role))
	if err != nil {
		t.Fatal(err)
	}
	return user, token
}

func TestUserHTTP_ProfileFieldsAndValidation(t *testing.T) {
	router, _, repo := setupUserRouter()
	user, token := seedUser(t, repo, "Ana Prueba", domain.RoleUser)

	t.Run("GET /users/me returns the profile fields with defaults", func(t *testing.T) {
		rec := doJSON(t, router, http.MethodGet, "/users/me", "", token)
		if rec.Code != http.StatusOK {
			t.Fatalf("status %d: %s", rec.Code, rec.Body.String())
		}
		data := decodeProfileEnvelope(t, rec).Data
		if data["display_name"] != "Ana Prueba" || data["timezone"] != "UTC" || data["role"] != "user" {
			t.Errorf("unexpected defaults: %v", data)
		}
		for _, key := range []string{"job_title", "department", "avatar_key", "last_login_at", "email_verified_at"} {
			if _, ok := data[key]; !ok {
				t.Errorf("response must include %q", key)
			}
		}
	})

	t.Run("PATCH /users/me with a bad timezone answers 422 user.timezone_invalid", func(t *testing.T) {
		rec := doJSON(t, router, http.MethodPatch, "/users/me", `{"timezone":"Mars/Olympus"}`, token)
		if rec.Code != http.StatusUnprocessableEntity {
			t.Fatalf("status %d, want 422: %s", rec.Code, rec.Body.String())
		}
		if code := decodeProfileEnvelope(t, rec).Error.Code; code != "user.timezone_invalid" {
			t.Errorf("code %q, want user.timezone_invalid", code)
		}
	})

	t.Run("PATCH /users/me applies partial profile changes", func(t *testing.T) {
		rec := doJSON(t, router, http.MethodPatch, "/users/me",
			`{"display_name":"Ana M.","job_title":"Analista","department":"Operaciones","timezone":"America/Bogota"}`, token)
		if rec.Code != http.StatusOK {
			t.Fatalf("status %d: %s", rec.Code, rec.Body.String())
		}
		data := decodeProfileEnvelope(t, rec).Data
		if data["display_name"] != "Ana M." || data["job_title"] != "Analista" || data["department"] != "Operaciones" || data["timezone"] != "America/Bogota" {
			t.Errorf("profile not updated: %v", data)
		}
		if data["email"] != user.Email {
			t.Errorf("omitted email must keep its value")
		}
	})

	t.Run("PATCH /users/me rejects an empty display name", func(t *testing.T) {
		rec := doJSON(t, router, http.MethodPatch, "/users/me", `{"display_name":""}`, token)
		if rec.Code != http.StatusUnprocessableEntity {
			t.Fatalf("status %d, want 422: %s", rec.Code, rec.Body.String())
		}
	})
}

func TestUserHTTP_SearchRequiresOrganizationForNonAdmins(t *testing.T) {
	router, _, repo := setupUserRouter()
	_, adminToken := seedUser(t, repo, "Root Admin", domain.RoleAdmin)
	_, userToken := seedUser(t, repo, "Plain User", domain.RoleUser)

	rec := doJSON(t, router, http.MethodGet, "/users/search?q=a", "", userToken)
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("non-admin without organization_id: status %d, want 422", rec.Code)
	}
	if code := decodeProfileEnvelope(t, rec).Error.Code; code != "validation.failed" {
		t.Errorf("code %q, want validation.failed", code)
	}

	if rec := doJSON(t, router, http.MethodGet, "/users/search", "", adminToken); rec.Code != http.StatusOK {
		t.Errorf("platform admin without organization_id keeps the legacy search: status %d", rec.Code)
	}

	if rec := doJSON(t, router, http.MethodGet, "/users/search?organization_id=not-a-uuid", "", userToken); rec.Code != http.StatusUnprocessableEntity {
		t.Errorf("invalid organization_id: status %d, want 422", rec.Code)
	}
}

func TestUserHTTP_SearchIsScopedToOrganization(t *testing.T) {
	router, _, repo := setupUserRouter()
	org := uuid.New()
	member, memberToken := seedUser(t, repo, "Member Beto", domain.RoleUser)
	colleague, _ := seedUser(t, repo, "Colleague Carla", domain.RoleUser)
	outsider, outsiderToken := seedUser(t, repo, "Outsider Dani", domain.RoleUser)

	repo.memberships[org] = map[uuid.UUID]string{member.ID: "member", colleague.ID: "member"}

	rec := doJSON(t, router, http.MethodGet, "/users/search?organization_id="+org.String(), "", memberToken)
	if rec.Code != http.StatusOK {
		t.Fatalf("member: status %d: %s", rec.Code, rec.Body.String())
	}
	var list struct {
		Data []map[string]any `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &list); err != nil {
		t.Fatal(err)
	}
	if len(list.Data) != 2 {
		t.Errorf("member must see the 2 members of the org, got %d", len(list.Data))
	}
	for _, item := range list.Data {
		if item["id"] == outsider.ID.String() {
			t.Error("outsider must not appear in the org search")
		}
	}

	rec = doJSON(t, router, http.MethodGet, "/users/search?organization_id="+org.String(), "", outsiderToken)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("outsider: status %d, want 404", rec.Code)
	}
	if code := decodeProfileEnvelope(t, rec).Error.Code; code != "organization.not_found" {
		t.Errorf("code %q, want organization.not_found", code)
	}
}

func TestUserHTTP_LoginRecordsLastLogin(t *testing.T) {
	router, _, _ := setupUserRouter()

	if rec := doJSON(t, router, http.MethodPost, "/auth/register", `{"name":"Eva Login","email":"eva@example.com","password":"password123"}`, ""); rec.Code != http.StatusCreated {
		t.Fatalf("register: status %d: %s", rec.Code, rec.Body.String())
	}
	if rec := doJSON(t, router, http.MethodPost, "/auth/login", `{"email":"eva@example.com","password":"wrong-password"}`, ""); rec.Code == http.StatusOK {
		t.Fatal("login with a wrong password must not succeed")
	}
	rec := doJSON(t, router, http.MethodPost, "/auth/login", `{"email":"eva@example.com","password":"password123"}`, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("login: status %d: %s", rec.Code, rec.Body.String())
	}
	var resp struct {
		Data struct {
			User map[string]any `json:"user"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if resp.Data.User["last_login_at"] == nil {
		t.Error("successful login must return last_login_at")
	}
	if resp.Data.User["role"] != "admin" {
		t.Errorf("first registered user must be admin, got %v", resp.Data.User["role"])
	}
}
