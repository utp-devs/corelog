package search

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/jhonsferg/corelog/internal/platform/authtoken"
	"github.com/jhonsferg/corelog/internal/search/application/port"
	"github.com/jhonsferg/corelog/internal/search/domain"
)

type stubIndex struct{}

func (stubIndex) Search(context.Context, uuid.UUID, bool, string, int) ([]domain.Hit, error) {
	return nil, nil
}

type stubSearcher struct{}

func (stubSearcher) SearchIssues(context.Context, port.IssueQuery) (port.IssuePage, error) {
	return port.IssuePage{}, nil
}

type stubFilters struct{ port.FilterRepository }

type stubGlobal struct{}

func (stubGlobal) SearchAll(context.Context, port.GlobalRequest) ([]domain.GlobalHit, error) {
	return nil, nil
}

func status(t *testing.T, m *Module, method, path, token string) int {
	t.Helper()
	r := chi.NewRouter()
	r.Route("/api/v1", func(r chi.Router) { m.Handler.RegisterRoutes(r) })
	req := httptest.NewRequest(method, "/api/v1"+path, nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	return rec.Code
}

func TestModuleWiresFeaturesOnDemand(t *testing.T) {
	const secret = "test-secret-with-enough-length-0123456789"
	token, err := authtoken.Generate(secret, time.Hour, uuid.New(), "user")
	if err != nil {
		t.Fatal(err)
	}
	m := NewModule(stubIndex{}, secret)
	if m.Issues != nil || m.Filters != nil || m.Global != nil {
		t.Fatal("features must start disabled")
	}
	// Filters need the issue search first: without it the call is a no-op.
	if m.WithFilters(stubFilters{}, nil); m.Filters != nil {
		t.Fatal("filters enabled without the issue search")
	}
	if got := status(t, m, "GET", "/issues/search", token); got != http.StatusNotFound && got != http.StatusMethodNotAllowed {
		t.Fatalf("issue search answered %d before being enabled", got)
	}

	m.WithIssueSearch(IssueSearchDeps{Searcher: stubSearcher{}}).WithFilters(stubFilters{}, nil).WithGlobal(stubGlobal{}, nil)
	if m.Issues == nil || m.Filters == nil || m.Global == nil {
		t.Fatal("features not enabled")
	}
	if got := status(t, m, "GET", "/issues/search", token); got != http.StatusOK {
		t.Fatalf("issue search: %d", got)
	}
	if got := status(t, m, "POST", "/search/validate", token); got != http.StatusUnprocessableEntity {
		t.Fatalf("validate without a body: %d", got)
	}
	if got := status(t, m, "GET", "/search?q=ab", token); got != http.StatusOK {
		t.Fatalf("global: %d", got)
	}
	if got := status(t, m, "GET", "/search?q=ab", "bad"); got != http.StatusUnauthorized {
		t.Fatalf("auth: %d", got)
	}
}
