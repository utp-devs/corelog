package httpserver

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestTimeoutExemptsOnlyTheLiveStream(t *testing.T) {
	probe := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-r.Context().Done():
			w.WriteHeader(http.StatusGatewayTimeout)
		case <-time.After(150 * time.Millisecond):
			w.WriteHeader(http.StatusOK)
		}
	})
	h := timeoutExceptStreams(20 * time.Millisecond)(probe)
	for path, want := range map[string]int{
		"/api/v1/notifications/stream": http.StatusOK,
		"/api/v1/notifications":        http.StatusGatewayTimeout,
	} {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
		if rec.Code != want {
			t.Errorf("GET %s = %d, want %d", path, rec.Code, want)
		}
	}
}
