package http_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// doJSON envia una peticion JSON con un token opcional y devuelve la respuesta.
func doJSON(t *testing.T, router http.Handler, method, path, body, bearer string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	if bearer != "" {
		req.Header.Set("Authorization", "Bearer "+bearer)
	}
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec
}

func loginToken(t *testing.T, router http.Handler, email, password string) string {
	t.Helper()
	rec := doJSON(t, router, http.MethodPost, "/auth/login", `{"email":"`+email+`","password":"`+password+`"}`, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("login status %d: %s", rec.Code, rec.Body.String())
	}
	var body struct {
		Data struct {
			Token string `json:"token"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	return body.Data.Token
}

func TestUserHTTP_SessionsAndDeactivation(t *testing.T) {
	router, _, _ := setupUserRouter()
	const email, password = "sesiones@example.com", "password123"
	doJSON(t, router, http.MethodPost, "/auth/register", `{"name":"Sesiones","email":"`+email+`","password":"`+password+`","password_confirmation":"`+password+`"}`, "")

	token := loginToken(t, router, email, password)

	t.Run("lists the open session", func(t *testing.T) {
		rec := doJSON(t, router, http.MethodGet, "/users/me/sessions", "", token)
		if rec.Code != http.StatusOK {
			t.Fatalf("status %d: %s", rec.Code, rec.Body.String())
		}
		var body struct {
			Data []struct {
				ID string `json:"id"`
			} `json:"data"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
			t.Fatal(err)
		}
		if len(body.Data) != 1 {
			t.Fatalf("want 1 session, got %d", len(body.Data))
		}
	})

	t.Run("closes a session and rejects an unknown one", func(t *testing.T) {
		list := doJSON(t, router, http.MethodGet, "/users/me/sessions", "", token)
		var body struct {
			Data []struct {
				ID string `json:"id"`
			} `json:"data"`
		}
		_ = json.Unmarshal(list.Body.Bytes(), &body)

		if rec := doJSON(t, router, http.MethodDelete, "/users/me/sessions/"+body.Data[0].ID, "", token); rec.Code != http.StatusNoContent {
			t.Fatalf("revoke status %d", rec.Code)
		}
		if rec := doJSON(t, router, http.MethodDelete, "/users/me/sessions/not-a-uuid", "", token); rec.Code != http.StatusUnprocessableEntity {
			t.Fatalf("invalid id status %d, want 422", rec.Code)
		}
		if rec := doJSON(t, router, http.MethodPost, "/users/me/sessions/revoke-others", "", token); rec.Code != http.StatusNoContent {
			t.Fatalf("revoke-others status %d", rec.Code)
		}
	})

	t.Run("deactivation needs the password and then blocks login", func(t *testing.T) {
		if rec := doJSON(t, router, http.MethodPost, "/users/me/deactivate", `{"password":"otra"}`, token); rec.Code != http.StatusUnauthorized {
			t.Fatalf("wrong password status %d, want 401", rec.Code)
		}
		if rec := doJSON(t, router, http.MethodPost, "/users/me/deactivate", `{"password":"`+password+`"}`, token); rec.Code != http.StatusNoContent {
			t.Fatalf("deactivate status %d: %s", rec.Code, rec.Body.String())
		}
		if rec := doJSON(t, router, http.MethodPost, "/auth/login", `{"email":"`+email+`","password":"`+password+`"}`, ""); rec.Code != http.StatusUnauthorized {
			t.Fatalf("login after deactivation status %d, want 401", rec.Code)
		}
	})
}
