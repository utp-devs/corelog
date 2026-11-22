package httpserver

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
)

type fakeRegistrar struct{ called bool }

func (f *fakeRegistrar) RegisterRoutes(r chi.Router) {
	f.called = true
	r.Get("/ping", func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte("pong")) })
}

func TestMountRegistersEveryModule(t *testing.T) {
	first, second := &fakeRegistrar{}, &fakeRegistrar{}
	r := chi.NewRouter()
	Mount(r, first, second)

	if !first.called || !second.called {
		t.Fatal("every registrar must be asked for its routes")
	}
	recorder := httptest.NewRecorder()
	r.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/ping", nil))
	if recorder.Body.String() != "pong" {
		t.Fatalf("mounted route body = %q", recorder.Body.String())
	}
}
