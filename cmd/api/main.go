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

	if err := ensureLegalDocuments(context.Background(), db, log); err != nil {
		return err
	}

	// Cada peticion autenticada confirma que su sesion sigue activa (revocacion y cuentas desactivadas).
	appmw.SetSessionValidator(userpostgres.NewSessionRepository(db).Validate)

	router, err := buildRouter(cfg, db, log)
	if err != nil {
		return err
	}

	// Comment mentions and replies become in-app notifications through the domain event outbox.
	dispatchCtx, stopDispatch := context.WithCancel(context.Background())
	defer stopDispatch()
	go func() {
		if err := startCommentMentionDispatcher(dispatchCtx, cfg, db, log); err != nil {
			log.Error("comment mention dispatcher stopped", "error", err)
		}
	}()

	// The audit log is written from the same outbox, and expired tokens, lockouts and audit rows are cleaned.
	go func() {
		if err := startSecurityRuntime(dispatchCtx, cfg, db, log); err != nil {
			log.Error("audit dispatcher stopped", "error", err)
		}
	}()

	// Webhook capture and automation rules consume the same outbox; deliveries and imports run in the background.
	go func() {
		if err := startIntegrationRuntime(dispatchCtx, db, log); err != nil {
			log.Error("integration runtime stopped", "error", err)
		}
	}()

	// One API instance (the leader) runs the scheduled jobs: email outbox, digests, filter subscriptions, cleanups.
	go startJobs(dispatchCtx, cfg, db, log)

	server := httpserver.New(cfg.Server, router)
	if rt := currentStream.Load(); rt != nil {
		// Open streams never end by themselves: close them on shutdown or Shutdown would wait for them.
		server.HTTPServer().RegisterOnShutdown(rt.hub.Close)
		go rt.bus.Run(dispatchCtx)
	}

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
