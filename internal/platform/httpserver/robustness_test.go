package httpserver

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/jhonsferg/corelog/internal/platform/config"
)

func TestServerTimeoutsAreConfigured(t *testing.T) {
	t.Parallel()
	configured := New(config.ServerConfig{
		Port: "0", ReadHeaderTimeout: 2 * time.Second, ReadTimeout: 3 * time.Second, WriteTimeout: 4 * time.Second, IdleTimeout: 5 * time.Second,
	}, http.NotFoundHandler()).HTTPServer()
	if configured.ReadHeaderTimeout != 2*time.Second || configured.ReadTimeout != 3*time.Second ||
		configured.WriteTimeout != 4*time.Second || configured.IdleTimeout != 5*time.Second {
		t.Fatalf("configured timeouts not applied: %+v", configured)
	}
	// An unconfigured server must never run without timeouts.
	defaults := New(config.ServerConfig{Port: "0"}, http.NotFoundHandler()).HTTPServer()
	for name, d := range map[string]time.Duration{
		"ReadHeaderTimeout": defaults.ReadHeaderTimeout, "ReadTimeout": defaults.ReadTimeout,
		"WriteTimeout": defaults.WriteTimeout, "IdleTimeout": defaults.IdleTimeout,
	} {
		if d <= 0 {
			t.Errorf("%s is not set by default", name)
		}
	}
}

func TestConfigLoadsTimeoutDefaults(t *testing.T) {
	t.Setenv("SERVER_READ_HEADER_TIMEOUT", "")
	t.Setenv("SERVER_IDLE_TIMEOUT", "")
	cfg, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Server.ReadHeaderTimeout != 5*time.Second || cfg.Server.IdleTimeout != 120*time.Second {
		t.Fatalf("defaults = %s / %s", cfg.Server.ReadHeaderTimeout, cfg.Server.IdleTimeout)
	}
}

// serve starts the real server on a loopback listener and returns its address.
func serve(t *testing.T, cfg config.ServerConfig, h http.Handler) string {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	srv := New(cfg, h).HTTPServer()
	go func() { _ = srv.Serve(l) }()
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = srv.Shutdown(ctx)
	})
	return l.Addr().String()
}

// waitClosed reports whether the server closes the connection within the limit.
func waitClosed(t *testing.T, conn net.Conn, limit time.Duration) bool {
	t.Helper()
	_ = conn.SetReadDeadline(time.Now().Add(limit))
	buf := make([]byte, 512)
	for {
		if _, err := conn.Read(buf); err != nil {
			ne, isNet := err.(net.Error)
			return !(isNet && ne.Timeout())
		}
	}
}

func TestSlowHeadersAreCut(t *testing.T) {
	t.Parallel()
	addr := serve(t, config.ServerConfig{Port: "0", ReadHeaderTimeout: 200 * time.Millisecond}, http.NotFoundHandler())
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = conn.Close() }()
	// A request line and a header that never ends.
	if _, err := conn.Write([]byte("GET /health HTTP/1.1\r\nHost: x\r\nX-Slow: ")); err != nil {
		t.Fatal(err)
	}
	if !waitClosed(t, conn, 3*time.Second) {
		t.Fatal("the server kept a connection that never finished its headers")
	}
}

func TestIdleConnectionsAreClosed(t *testing.T) {
	t.Parallel()
	addr := serve(t, config.ServerConfig{Port: "0", IdleTimeout: 200 * time.Millisecond}, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = conn.Close() }()
	if _, err := conn.Write([]byte("GET / HTTP/1.1\r\nHost: x\r\n\r\n")); err != nil {
		t.Fatal(err)
	}
	// The response arrives, then the kept alive connection must be closed by the idle timeout.
	if !waitClosed(t, conn, 3*time.Second) {
		t.Fatal("the server kept an idle connection open")
	}
}

func TestPanicIsAnswered500WithoutInternals(t *testing.T) {
	t.Parallel()
	var logs bytes.Buffer
	log := slog.New(slog.NewJSONHandler(&logs, nil))
	r := NewRouterWith(config.ServerConfig{}, RouterOptions{Logger: log})
	r.Get("/boom", func(w http.ResponseWriter, r *http.Request) {
		panic("secret-internal-detail postgres://user:pass@host/db")
	})
	req := httptest.NewRequest(http.MethodGet, "/boom?token=abc123", nil)
	req.Header.Set("X-Request-ID", "req-from-client-1")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d", rec.Code)
	}
	var body struct {
		Error struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil || body.Error.Message == "" {
		t.Fatalf("not the standard envelope: %q (%v)", rec.Body.String(), err)
	}
	for _, leak := range []string{"secret-internal-detail", "postgres://", "goroutine", ".go:", "panic"} {
		if strings.Contains(rec.Body.String(), leak) {
			t.Errorf("the body leaks %q: %s", leak, rec.Body.String())
		}
	}
	if got := rec.Header().Get("X-Request-ID"); got != "req-from-client-1" {
		t.Errorf("X-Request-ID = %q, want the id the client sent", got)
	}
	// The detail stays in the log, tied to the request, without the query string.
	logged := logs.String()
	if !strings.Contains(logged, "secret-internal-detail") || !strings.Contains(logged, "req-from-client-1") {
		t.Errorf("the panic was not logged with its request id: %s", logged)
	}
	if strings.Contains(logged, "abc123") {
		t.Errorf("the log keeps the query string: %s", logged)
	}
}

func TestRequestIDIsGeneratedAndLogged(t *testing.T) {
	t.Parallel()
	var logs bytes.Buffer
	r := NewRouterWith(config.ServerConfig{}, RouterOptions{Logger: slog.New(slog.NewJSONHandler(&logs, nil))})
	r.Get("/things/{id}", func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusNoContent) })
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/things/secret-invitation-token?key=feedtoken", nil))
	id := rec.Header().Get("X-Request-ID")
	if id == "" {
		t.Fatal("no X-Request-ID on the response")
	}
	line := logs.String()
	if !strings.Contains(line, id) || !strings.Contains(line, `"route":"/things/{id}"`) {
		t.Fatalf("request line without id or route pattern: %s", line)
	}
	if strings.Contains(line, "secret-invitation-token") || strings.Contains(line, "feedtoken") {
		t.Fatalf("the request log carries a path or query secret: %s", line)
	}
	// Two requests never share an id.
	other := httptest.NewRecorder()
	r.ServeHTTP(other, httptest.NewRequest(http.MethodGet, "/things/1", nil))
	if other.Header().Get("X-Request-ID") == id {
		t.Fatal("request ids repeat")
	}
}

func TestShutdownWaitsForInFlightRequests(t *testing.T) {
	t.Parallel()
	started := make(chan struct{})
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	srv := New(config.ServerConfig{Port: "0"}, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		close(started)
		time.Sleep(300 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	})).HTTPServer()
	go func() { _ = srv.Serve(l) }()

	result := make(chan int, 1)
	go func() {
		resp, err := http.Get("http://" + l.Addr().String() + "/slow")
		if err != nil {
			result <- -1
			return
		}
		_ = resp.Body.Close()
		result <- resp.StatusCode
	}()
	<-started
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		t.Fatalf("graceful shutdown: %v", err)
	}
	if got := <-result; got != http.StatusOK {
		t.Fatalf("the in-flight request was cut: %d", got)
	}
}
