package httpserver

import (
	"context"
	"net/http"
	"time"

	"github.com/jhonsferg/corelog/internal/platform/config"
)

// Fallbacks used when a timeout is not configured: a server without them is open to slow clients.
const (
	defaultReadHeaderTimeout = 5 * time.Second
	defaultReadTimeout       = 10 * time.Second
	defaultWriteTimeout      = 30 * time.Second
	defaultIdleTimeout       = 120 * time.Second
)

type Server struct {
	httpServer *http.Server
}

func New(cfg config.ServerConfig, handler http.Handler) *Server {
	return &Server{
		httpServer: &http.Server{
			Addr:              ":" + cfg.Port,
			Handler:           handler,
			ReadHeaderTimeout: orDefault(cfg.ReadHeaderTimeout, defaultReadHeaderTimeout),
			ReadTimeout:       orDefault(cfg.ReadTimeout, defaultReadTimeout),
			WriteTimeout:      orDefault(cfg.WriteTimeout, defaultWriteTimeout),
			IdleTimeout:       orDefault(cfg.IdleTimeout, defaultIdleTimeout),
		},
	}
}

func orDefault(d, fallback time.Duration) time.Duration {
	if d <= 0 {
		return fallback
	}
	return d
}

func (s *Server) Start() error {
	return s.httpServer.ListenAndServe()
}

func (s *Server) Shutdown(ctx context.Context) error {
	return s.httpServer.Shutdown(ctx)
}

func (s *Server) Addr() string {
	return s.httpServer.Addr
}

// HTTPServer exposes the underlying server, for tests that serve on their own listener.
func (s *Server) HTTPServer() *http.Server { return s.httpServer }
