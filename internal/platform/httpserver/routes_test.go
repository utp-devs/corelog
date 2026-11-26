package httpserver

import (
	"net/http"
	"sort"
	"testing"

	"github.com/go-chi/chi/v5"
)

func TestRoutesListsMethodsAndNormalisedPaths(t *testing.T) {
	r := chi.NewRouter()
	r.Get("/health", func(http.ResponseWriter, *http.Request) {})
	r.Route("/api/v1", func(r chi.Router) {
		r.Get("/projects/{id}", func(http.ResponseWriter, *http.Request) {})
		r.Put("/projects/{projectID}/labels/{label_id}", func(http.ResponseWriter, *http.Request) {})
		r.Options("/projects", func(http.ResponseWriter, *http.Request) {})
	})

	got, err := Routes(r)
	if err != nil {
		t.Fatalf("Routes: %v", err)
	}

	want := []Route{
		{Method: http.MethodGet, Path: "/health"},
		{Method: http.MethodGet, Path: "/api/v1/projects/{}"},
		{Method: http.MethodPut, Path: "/api/v1/projects/{}/labels/{}"},
	}
	if len(got) != len(want) {
		t.Fatalf("Routes() = %v, want %v", got, want)
	}
	sortRoutes(got)
	sortRoutes(want)
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("Routes()[%d] = %+v, want %+v (all: %v)", i, got[i], want[i], got)
		}
	}
}

func sortRoutes(routes []Route) {
	sort.Slice(routes, func(i, j int) bool {
		if routes[i].Path != routes[j].Path {
			return routes[i].Path < routes[j].Path
		}
		return routes[i].Method < routes[j].Method
	})
}
