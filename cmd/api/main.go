package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/jmoiron/sqlx"

	"github.com/jhonsferg/corelog/internal/platform/config"
	"github.com/jhonsferg/corelog/internal/platform/database"
	"github.com/jhonsferg/corelog/internal/platform/httpserver"
	"github.com/jhonsferg/corelog/internal/platform/logger"
	appmw "github.com/jhonsferg/corelog/internal/platform/middleware"
	userpostgres "github.com/jhonsferg/corelog/internal/user/adapter/out/postgres"
)

func main() {
	if err := run(); err != nil {
		slog.Error("fatal startup error", "error", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	if err := requireTwoFactorKey(cfg); err != nil {
		return err
	}

	log := logger.New(cfg.Server.Env)

	var db *sqlx.DB
	db, err = database.NewPostgres(cfg.Database)
	if err != nil {
		return err
	}
	defer func() { _ = db.Close() }()
	log.Info("connected to database", "driver", "postgresql")

	// Cada peticion autenticada confirma que su sesion sigue activa (revocacion y cuentas desactivadas).
	appmw.SetSessionValidator(userpostgres.NewSessionRepository(db).Validate)

	router, err := buildRouter(cfg, db, log)
	if err != nil {
		return err
	}

	server := httpserver.New(cfg.Server, router)

	serverErrors := make(chan error, 1)
	go func() {
		log.Info("server listening", "addr", server.Addr(), "env", cfg.Server.Env)
		if err := server.Start(); err != nil && err != http.ErrServerClosed {
			serverErrors <- err
		}
	}()

	shutdown := make(chan os.Signal, 1)
	signal.Notify(shutdown, os.Interrupt, syscall.SIGTERM)

	select {
	case err := <-serverErrors:
		return err
	case sig := <-shutdown:
		log.Info("shutdown signal received", "signal", sig.String())

		ctx, cancel := context.WithTimeout(context.Background(), cfg.Server.ShutdownTimeout)
		defer cancel()

		if err := server.Shutdown(ctx); err != nil {
			return err
		}
		log.Info("server stopped gracefully")
	}

	return nil
}
