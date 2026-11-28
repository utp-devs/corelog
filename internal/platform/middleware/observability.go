package middleware

import (
	"log/slog"
	"net/http"
	"runtime/debug"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

// panicBody is the standard error envelope of an unexpected failure. It never carries the panic value or the stack.
const panicBody = `{"error":{"message":"an unexpected error occurred"}}`

// Recoverer turns a panic of a handler into the standard 500 envelope. The panic value and the stack go to the log
// together with the request id, never to the client. http.ErrAbortHandler is passed on: it is how a handler aborts
// a response on purpose.
func Recoverer(log *slog.Logger) func(http.Handler) http.Handler {
	if log == nil {
		log = slog.Default()
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				rec := recover()
				if rec == nil {
					return
				}
				if rec == http.ErrAbortHandler { //nolint:errorlint // the sentinel is compared by identity on purpose
					panic(rec)
				}
				log.Error("panic recovered", "panic", rec, "stack", string(debug.Stack()),
					"request_id", middleware.GetReqID(r.Context()), "method", r.Method, "route", routePattern(r))
				if r.Header.Get("Connection") == "Upgrade" {
					return
				}
				w.Header().Set("Content-Type", "application/json; charset=utf-8")
				w.WriteHeader(http.StatusInternalServerError)
				_, _ = w.Write([]byte(panicBody))
			}()
			next.ServeHTTP(w, r)
		})
	}
}

// RequestLogger writes one structured line per request: method, route pattern (never the raw path or the query
// string, which can carry invitation, feed or reset tokens), status, size, duration and request id. The health
// check is not logged.
func RequestLogger(log *slog.Logger) func(http.Handler) http.Handler {
	if log == nil {
		log = slog.Default()
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/health" {
				next.ServeHTTP(w, r)
				return
			}
			ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)
			began := time.Now()
			next.ServeHTTP(ww, r)
			status := ww.Status()
			if status == 0 {
				status = http.StatusOK
			}
			level := slog.LevelInfo
			if status >= http.StatusInternalServerError {
				level = slog.LevelError
			}
			log.Log(r.Context(), level, "http request", "method", r.Method, "route", routePattern(r), "status", status,
				"bytes", ww.BytesWritten(), "duration_ms", time.Since(began).Milliseconds(),
				"request_id", middleware.GetReqID(r.Context()))
		})
	}
}

// routePattern is the chi pattern that matched the request ("" before routing or for an unknown path).
func routePattern(r *http.Request) string {
	if rc := chi.RouteContext(r.Context()); rc != nil {
		return rc.RoutePattern()
	}
	return ""
}

// EchoRequestID sets the X-Request-ID response header to the request id (the one the caller sent or the one chi
// generated), so a client can quote it when it reports a problem. It must run after chi's RequestID middleware.
func EchoRequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if id := middleware.GetReqID(r.Context()); id != "" {
			w.Header().Set("X-Request-ID", id)
		}
		next.ServeHTTP(w, r)
	})
}
