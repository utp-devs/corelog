package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/go-chi/chi/v5"
	"github.com/jmoiron/sqlx"

	authpostgres "github.com/jhonsferg/corelog/internal/authorization/adapter/out/postgres"
	authusecase "github.com/jhonsferg/corelog/internal/authorization/application/usecase"
	"github.com/jhonsferg/corelog/internal/board"
	boardpostgres "github.com/jhonsferg/corelog/internal/board/adapter/out/postgres"
	"github.com/jhonsferg/corelog/internal/platform/config"
	"github.com/jhonsferg/corelog/internal/platform/database"
	"github.com/jhonsferg/corelog/internal/platform/httpserver"
	"github.com/jhonsferg/corelog/internal/platform/logger"
	"github.com/jhonsferg/corelog/internal/project"
	projectpostgres "github.com/jhonsferg/corelog/internal/project/adapter/out/postgres"
	projectport "github.com/jhonsferg/corelog/internal/project/application/port"
	"github.com/jhonsferg/corelog/internal/team"
	teampostgres "github.com/jhonsferg/corelog/internal/team/adapter/out/postgres"
	teamport "github.com/jhonsferg/corelog/internal/team/application/port"
	"github.com/jhonsferg/corelog/internal/ticket"
	ticketdirectory "github.com/jhonsferg/corelog/internal/ticket/adapter/out/directory"
	ticketfilesystem "github.com/jhonsferg/corelog/internal/ticket/adapter/out/filesystem"
	ticketmemory "github.com/jhonsferg/corelog/internal/ticket/adapter/out/memory"
	ticketpostgres "github.com/jhonsferg/corelog/internal/ticket/adapter/out/postgres"
	ticketport "github.com/jhonsferg/corelog/internal/ticket/application/port"
	"github.com/jhonsferg/corelog/internal/user"
	userpostgres "github.com/jhonsferg/corelog/internal/user/adapter/out/postgres"
	userport "github.com/jhonsferg/corelog/internal/user/application/port"
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

	log := logger.New(cfg.Server.Env)

	var db *sqlx.DB
	db, err = database.NewPostgres(cfg.Database)
	if err != nil {
		return err
	}
	defer func() { _ = db.Close() }()
	log.Info("connected to database", "driver", "postgresql")

	var userRepo userport.Repository
	var teamRepo teamport.Repository
	var ticketDBRepo ticketport.Repository
	var projectRepo projectport.Repository
	var projectMemberRepo projectport.MemberRepository
	userRepo = userpostgres.NewUserRepository(db)
	teamRepo = teampostgres.NewTeamRepository(db)
	ticketDBRepo = ticketpostgres.NewTicketRepository(db)
	projectRepo = projectpostgres.NewProjectRepository(db)
	projectMemberRepo = projectpostgres.NewMemberRepository(db)
	projectCatalogRepo := projectpostgres.NewCatalogRepository(db)

	userModule := user.NewModule(userRepo, cfg.Auth.JWTSecret, cfg.Auth.JWTExpiration)
	teamModule := team.NewModule(teamRepo, cfg.Auth.JWTSecret)
	projectAuthorizer := projectAuthorizerAdapter{authorizer: authusecase.NewAuthorizer(authpostgres.NewRoleLookup(db))}
	projectModule := project.NewModule(projectRepo, projectMemberRepo, projectCatalogRepo, cfg.Auth.JWTSecret, projectAuthorizer)
	boardModule := board.NewModule(boardpostgres.NewBoardRepository(db), projectModule.Service, cfg.Auth.JWTSecret)

	ticketRepo := ticketDBRepo
	if cfg.Features.TicketRepository == "memory" {
		ticketRepo = ticketmemory.NewTicketRepository()
		log.Warn("ticket module using in-memory repository")
	}
	userDirectory := ticketdirectory.NewUserDirectory(db)
	collaborationRepo := ticketpostgres.NewCollaborationRepository(db)
	issueLinkRepo := ticketpostgres.NewIssueLinkRepository(db)
	watcherRepo := ticketpostgres.NewWatcherRepository(db)
	savedFilterRepo := ticketpostgres.NewSavedFilterRepository(db)
	attachmentRepo := ticketpostgres.NewAttachmentRepository(db)
	attachmentRoot := os.Getenv("ATTACHMENTS_DIR")
	if attachmentRoot == "" {
		attachmentRoot = filepath.Join("data", "attachments")
	}
	fileStorage := ticketfilesystem.New(attachmentRoot)
	workflowRepo := ticketpostgres.NewWorkflowRepository(db)
	ticketModule := ticket.NewModule(ticketRepo, userDirectory, collaborationRepo, issueLinkRepo, workflowRepo, watcherRepo, savedFilterRepo, cfg.Auth.JWTSecret, projectAuthorizer, projectMemberRepo)
	ticketModule.Handler.WithAttachments(attachmentRepo, fileStorage)

	router := httpserver.NewRouter(cfg.Server)
	router.Route("/api/v1", func(r chi.Router) {
		httpserver.Mount(r, userModule.Handler, teamModule.Handler, projectModule.Handler, boardModule.Handler, ticketModule.Handler)
	})

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
