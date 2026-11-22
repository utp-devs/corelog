package http

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
)

func TestNotificationHTTPPreferences(t *testing.T) {
	h := router(&fakeService{})
	user := uuid.New()

	t.Run("reads the default preference", func(t *testing.T) {
		rec := do(t, h, http.MethodGet, "/notifications/preferences", user, true)
		if rec.Code != http.StatusOK {
			t.Fatalf("status %d, want 200", rec.Code)
		}
		var body struct {
			Data struct {
				InApp bool `json:"in_app"`
			} `json:"data"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
			t.Fatal(err)
		}
		if !body.Data.InApp {
			t.Fatal("in_app must default to true")
		}
	})

	t.Run("saves the preference sent by the user", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPut, "/notifications/preferences", bytes.NewBufferString(`{"in_app":false}`))
		req.Header.Set("Authorization", "Bearer "+token(t, user))
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("status %d, want 200; body %s", rec.Code, rec.Body.String())
		}
		var body struct {
			Data struct {
				InApp bool `json:"in_app"`
			} `json:"data"`
		}
		_ = json.Unmarshal(rec.Body.Bytes(), &body)
		if body.Data.InApp {
			t.Fatal("in_app must reflect the saved value false")
		}
	})

	t.Run("rejects a body without in_app", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPut, "/notifications/preferences", bytes.NewBufferString(`{}`))
		req.Header.Set("Authorization", "Bearer "+token(t, user))
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusUnprocessableEntity {
			t.Fatalf("status %d, want 422", rec.Code)
		}
	})

	t.Run("requires authentication", func(t *testing.T) {
		rec := do(t, h, http.MethodGet, "/notifications/preferences", user, false)
		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("status %d, want 401", rec.Code)
		}
	})
}
