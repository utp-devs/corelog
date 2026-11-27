package http

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/jhonsferg/corelog/internal/favorite/application/port"
	"github.com/jhonsferg/corelog/internal/favorite/domain"
	"github.com/jhonsferg/corelog/internal/platform/authtoken"
)

// fakeFavorites implements the service with a configurable error and fixed entries.
type fakeFavorites struct {
	port.Service
	err     error
	entries []port.Entry
	limit   int
}

func (f *fakeFavorites) Star(context.Context, uuid.UUID, string, uuid.UUID) error {
	return f.err
}

func (f *fakeFavorites) Unstar(context.Context, uuid.UUID, string, uuid.UUID) error {
	return f.err
}

func (f *fakeFavorites) ListFavorites(context.Context, uuid.UUID, string) ([]port.Entry, error) {
	return f.entries, f.err
}

func (f *fakeFavorites) ListRecent(_ context.Context, _ uuid.UUID, _ string, limit int) ([]port.Entry, error) {
	f.limit = limit
	return f.entries, f.err
}

func favoriteRouter(service port.Service) http.Handler {
	r := chi.NewRouter()
	NewHandler(service, "test-secret").RegisterRoutes(r)
	return r
}

func favoriteAuth(t *testing.T) string {
	t.Helper()
	value, err := authtoken.Generate("test-secret", time.Hour, uuid.New(), "member")
	if err != nil {
		t.Fatal(err)
	}
	return "Bearer " + value
}

func favoriteSend(h http.Handler, method, path, auth string) *httptest.ResponseRecorder {
	request := httptest.NewRequest(method, path, strings.NewReader(""))
	if auth != "" {
		request.Header.Set("Authorization", auth)
	}
	recorder := httptest.NewRecorder()
	h.ServeHTTP(recorder, request)
	return recorder
}

func TestFavoriteRoutesRequireAuthentication(t *testing.T) {
	router := favoriteRouter(&fakeFavorites{})
	id := uuid.NewString()
	for _, tc := range []struct{ method, path string }{
		{http.MethodPut, "/favorites/project/" + id},
		{http.MethodDelete, "/favorites/project/" + id},
		{http.MethodGet, "/favorites?type=project"},
		{http.MethodGet, "/recent?type=project"},
	} {
		if got := favoriteSend(router, tc.method, tc.path, "").Code; got != http.StatusUnauthorized {
			t.Errorf("%s %s without token: status %d, want 401", tc.method, tc.path, got)
		}
	}
}

func TestStarAndUnstarAnswerNoContent(t *testing.T) {
	router := favoriteRouter(&fakeFavorites{})
	id := uuid.NewString()
	if got := favoriteSend(router, http.MethodPut, "/favorites/project/"+id, favoriteAuth(t)).Code; got != http.StatusNoContent {
		t.Fatalf("star status = %d, want 204", got)
	}
	if got := favoriteSend(router, http.MethodDelete, "/favorites/project/"+id, favoriteAuth(t)).Code; got != http.StatusNoContent {
		t.Fatalf("unstar status = %d, want 204", got)
	}
}

func TestStarInvisibleEntityAnswersNotFound(t *testing.T) {
	router := favoriteRouter(&fakeFavorites{err: domain.ErrEntityNotFound})
	recorder := favoriteSend(router, http.MethodPut, "/favorites/project/"+uuid.NewString(), favoriteAuth(t))
	if recorder.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", recorder.Code)
	}
}

func TestStarRejectsMalformedID(t *testing.T) {
	router := favoriteRouter(&fakeFavorites{})
	if got := favoriteSend(router, http.MethodPut, "/favorites/project/not-a-uuid", favoriteAuth(t)).Code; got != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want 422", got)
	}
}

func TestListFavoritesResponseShape(t *testing.T) {
	id := uuid.New()
	created := time.Date(2026, 11, 2, 8, 0, 0, 0, time.UTC)
	router := favoriteRouter(&fakeFavorites{entries: []port.Entry{
		{Type: domain.EntityProject, ID: id, Name: "Plataforma", Key: "PLT", Color: "blue", CreatedAt: created},
	}})
	recorder := favoriteSend(router, http.MethodGet, "/favorites?type=project", favoriteAuth(t))
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", recorder.Code)
	}
	var body struct {
		Data []map[string]any `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if len(body.Data) != 1 {
		t.Fatalf("data entries = %d, want 1", len(body.Data))
	}
	item := body.Data[0]
	if item["type"] != "project" || item["id"] != id.String() || item["key"] != "PLT" || item["name"] != "Plataforma" {
		t.Fatalf("unexpected item: %v", item)
	}
	if _, ok := item["created_at"]; !ok {
		t.Fatal("created_at missing")
	}
	if _, ok := item["viewed_at"]; ok {
		t.Fatal("viewed_at must be omitted for favorites")
	}
}

func TestListRecentIncludesViewedAtAndDefaultsLimit(t *testing.T) {
	viewed := time.Date(2026, 11, 2, 9, 0, 0, 0, time.UTC)
	service := &fakeFavorites{entries: []port.Entry{
		{Type: domain.EntityProject, ID: uuid.New(), Name: "A", Key: "AAA", CreatedAt: viewed, ViewedAt: &viewed},
	}}
	router := favoriteRouter(service)
	recorder := favoriteSend(router, http.MethodGet, "/recent?type=project", favoriteAuth(t))
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", recorder.Code)
	}
	if !strings.Contains(recorder.Body.String(), `"viewed_at"`) {
		t.Fatalf("viewed_at missing in %s", recorder.Body.String())
	}
	if service.limit != 0 {
		t.Fatalf("limit passed to service = %d, want 0 so the default applies", service.limit)
	}
}

func TestListRecentRejectsNonNumericLimit(t *testing.T) {
	router := favoriteRouter(&fakeFavorites{})
	if got := favoriteSend(router, http.MethodGet, "/recent?type=project&limit=abc", favoriteAuth(t)).Code; got != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want 422", got)
	}
}
