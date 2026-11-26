package http_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/jhonsferg/corelog/internal/platform/authtoken"
)

func teamRequest(router http.Handler, body, bearer string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, "/teams", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+bearer)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec
}

func TestTeamHTTP_ErrorCodes(t *testing.T) {
	router, jwtSecret, _ := setupTeamRouter()
	adminToken, _ := authtoken.Generate(jwtSecret, time.Hour, uuid.New(), "admin")

	if rec := teamRequest(router, `{"name":"Plataforma"}`, adminToken); rec.Code != http.StatusCreated {
		t.Fatalf("first create status %d: %s", rec.Code, rec.Body.String())
	}

	t.Run("duplicate name returns team.name_taken", func(t *testing.T) {
		rec := teamRequest(router, `{"name":"Plataforma"}`, adminToken)
		if rec.Code != http.StatusConflict {
			t.Fatalf("status %d, want 409", rec.Code)
		}
		var env struct {
			Error struct {
				Code string `json:"code"`
			} `json:"error"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &env); err != nil {
			t.Fatal(err)
		}
		if env.Error.Code != "team.name_taken" {
			t.Fatalf("code %q, want team.name_taken", env.Error.Code)
		}
	})
}
