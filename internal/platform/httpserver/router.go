package httpserver

import (
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"

	"github.com/jhonsferg/corelog/internal/platform/config"
	appmw "github.com/jhonsferg/corelog/internal/platform/middleware"
)

type RouteRegistrar interface {
	RegisterRoutes(r chi.Router)
}

// RouterOptions carries the hardening settings of the router. The zero value is a safe default.
type RouterOptions struct {
	// HSTS adds Strict-Transport-Security; enable it only behind TLS.
	HSTS bool
	// MaxJSONBodyBytes caps non-multipart request bodies (default 1 MiB).
	MaxJSONBodyBytes int64
	// TrustedProxies decides whose X-Forwarded-For is believed for the client address of events.
	TrustedProxies *appmw.TrustedProxies
	// Logger receives the request log and the recovered panics (default slog.Default()).
	Logger *slog.Logger
}

// NewRouter builds the router with the default hardening options.
func NewRouter(cfg config.ServerConfig) *chi.Mux { return NewRouterWith(cfg, RouterOptions{}) }

// SafeOrigins removes wildcard and empty entries: the API answers with credentials, so a wildcard origin is never valid.
func SafeOrigins(origins []string) []string {
	out := make([]string, 0, len(origins))
	for _, o := range origins {
		if o = strings.TrimSpace(o); o != "" && !strings.Contains(o, "*") {
			out = append(out, o)
		}
	}
	return out
}

// NewRouterWith builds the router with explicit hardening options.
func NewRouterWith(cfg config.ServerConfig, opts RouterOptions) *chi.Mux {
	if opts.MaxJSONBodyBytes <= 0 {
		opts.MaxJSONBodyBytes = 1 << 20
	}
	r := chi.NewRouter()

	r.Use(middleware.RequestID)
	r.Use(appmw.EchoRequestID)
	r.Use(appmw.SecurityHeaders(opts.HSTS))
	r.Use(appmw.RequestMeta(opts.TrustedProxies))
	r.Use(appmw.RequestLogger(opts.Logger))
	r.Use(appmw.Recoverer(opts.Logger))
	r.Use(middleware.Timeout(30 * time.Second))
	r.Use(appmw.BodyLimit(opts.MaxJSONBodyBytes))
	// With no valid origin no CORS headers are sent at all (the library would default to every origin).
	if origins := SafeOrigins(cfg.Origins()); len(origins) > 0 {
		r.Use(cors.Handler(cors.Options{
			AllowedOrigins:   origins,
			AllowedMethods:   []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
			AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-Request-ID"},
			AllowCredentials: true,
			MaxAge:           300,
		}))
	}

	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	})

	return r
}

func Mount(r chi.Router, registrars ...RouteRegistrar) {
	for _, reg := range registrars {
		reg.RegisterRoutes(r)
	}
}
