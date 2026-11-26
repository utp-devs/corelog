package main

import (
	"log/slog"
	"os"
	"path/filepath"

	"github.com/go-chi/chi/v5"
	"github.com/jmoiron/sqlx"

	authpostgres "github.com/jhonsferg/corelog/internal/authorization/adapter/out/postgres"
	authusecase "github.com/jhonsferg/corelog/internal/authorization/application/usecase"
	"github.com/jhonsferg/corelog/internal/board"
	boardpostgres "github.com/jhonsferg/corelog/internal/board/adapter/out/postgres"
	"github.com/jhonsferg/corelog/internal/calendar"
	calendarpostgres "github.com/jhonsferg/corelog/internal/calendar/adapter/out/postgres"
	"github.com/jhonsferg/corelog/internal/dashboard"
	dashboardpostgres "github.com/jhonsferg/corelog/internal/dashboard/adapter/out/postgres"
	"github.com/jhonsferg/corelog/internal/notification"
	notificationpostgres "github.com/jhonsferg/corelog/internal/notification/adapter/out/postgres"
	"github.com/jhonsferg/corelog/internal/platform/config"
	"github.com/jhonsferg/corelog/internal/platform/database"
	"github.com/jhonsferg/corelog/internal/platform/httpserver"
	"github.com/jhonsferg/corelog/internal/project"
	projectpostgres "github.com/jhonsferg/corelog/internal/project/adapter/out/postgres"
	projectport "github.com/jhonsferg/corelog/internal/project/application/port"
	"github.com/jhonsferg/corelog/internal/search"
	searchpostgres "github.com/jhonsferg/corelog/internal/search/adapter/out/postgres"
	"github.com/jhonsferg/corelog/internal/sprint"
	sprintpostgres "github.com/jhonsferg/corelog/internal/sprint/adapter/out/postgres"
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

// buildRouter assembles the HTTP router with every module mounted under /api/v1. It only constructs
// adapters and services; it does not register the global session validator (run() does that) and it
// does not open connections, so it can be called with a nil db to inspect the route table.
func buildRouter(cfg *config.Config, db *sqlx.DB, log *slog.Logger) (chi.Router, error) {
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
	sessionRepo := userpostgres.NewSessionRepository(db)

	userModule := user.NewModule(userRepo, sessionRepo, cfg.Auth.JWTSecret, cfg.Auth.JWTExpiration)
	orgs := newOrgDirectory(db)
	teamModule := team.NewModule(teamRepo, orgs, database.NewTxManager(db), cfg.Auth.JWTSecret)
	projectAuthorizer := projectAuthorizerAdapter{authorizer: authusecase.NewAuthorizer(authpostgres.NewRoleLookup(db))}
	projectModule := project.NewModule(projectRepo, projectMemberRepo, projectCatalogRepo, orgs, cfg.Auth.JWTSecret, projectAuthorizer)

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
	notificationModule := notification.NewModule(notificationpostgres.NewNotificationRepository(db), cfg.Auth.JWTSecret)
	ticketModule := ticket.NewModule(ticketRepo, userDirectory, collaborationRepo, issueLinkRepo, workflowRepo, watcherRepo, savedFilterRepo, cfg.Auth.JWTSecret, projectAuthorizer, notificationModule.Service, projectMemberRepo)
	searchModule := search.NewModule(searchpostgres.NewSearchIndex(db), cfg.Auth.JWTSecret)
	dashboardModule := dashboard.NewModule(dashboardpostgres.NewDashboardReader(db), cfg.Auth.JWTSecret)
	calendarModule := calendar.NewModule(calendarpostgres.NewCalendarRepository(db), projectAuthorizer, cfg.Auth.JWTSecret)
	sprintModule := sprint.NewModule(sprintpostgres.NewSprintRepository(db), projectAuthorizer, cfg.Auth.JWTSecret)
	boardModule := board.NewModule(boardpostgres.NewBoardRepository(db), projectModule.Service, projectAuthorizer, boardIssueMover{issues: ticketModule.Service}, cfg.Auth.JWTSecret)
	ticketModule.Handler.WithAttachments(attachmentRepo, fileStorage)

	router := httpserver.NewRouter(cfg.Server)
	router.Route("/api/v1", func(r chi.Router) {
		httpserver.Mount(r, userModule.Handler, teamModule.Handler, projectModule.Handler, boardModule.Handler, ticketModule.Handler, sprintModule.Handler, notificationModule.Handler, calendarModule.Handler, searchModule.Handler, dashboardModule.Handler)
	})
	return router, nil
}
