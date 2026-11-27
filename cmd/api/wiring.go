package main

import (
	"context"
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
	"github.com/jhonsferg/corelog/internal/favorite"
	favoritepostgres "github.com/jhonsferg/corelog/internal/favorite/adapter/out/postgres"
	"github.com/jhonsferg/corelog/internal/issue"
	issuedirectory "github.com/jhonsferg/corelog/internal/issue/adapter/out/directory"
	issuefilesystem "github.com/jhonsferg/corelog/internal/issue/adapter/out/filesystem"
	issuepostgres "github.com/jhonsferg/corelog/internal/issue/adapter/out/postgres"
	issueport "github.com/jhonsferg/corelog/internal/issue/application/port"
	issueusecase "github.com/jhonsferg/corelog/internal/issue/application/usecase"
	"github.com/jhonsferg/corelog/internal/legal"
	legalpostgres "github.com/jhonsferg/corelog/internal/legal/adapter/out/postgres"
	"github.com/jhonsferg/corelog/internal/note"
	notepostgres "github.com/jhonsferg/corelog/internal/note/adapter/out/postgres"
	"github.com/jhonsferg/corelog/internal/notification"
	notificationpostgres "github.com/jhonsferg/corelog/internal/notification/adapter/out/postgres"
	"github.com/jhonsferg/corelog/internal/organization"
	organizationhttp "github.com/jhonsferg/corelog/internal/organization/adapter/in/http"
	organizationpostgres "github.com/jhonsferg/corelog/internal/organization/adapter/out/postgres"
	"github.com/jhonsferg/corelog/internal/platform/clock"
	"github.com/jhonsferg/corelog/internal/platform/config"
	"github.com/jhonsferg/corelog/internal/platform/database"
	"github.com/jhonsferg/corelog/internal/platform/events"
	"github.com/jhonsferg/corelog/internal/platform/filestorage"
	"github.com/jhonsferg/corelog/internal/platform/httpserver"
	"github.com/jhonsferg/corelog/internal/project"
	projectpostgres "github.com/jhonsferg/corelog/internal/project/adapter/out/postgres"
	projectport "github.com/jhonsferg/corelog/internal/project/application/port"
	"github.com/jhonsferg/corelog/internal/release"
	releasepostgres "github.com/jhonsferg/corelog/internal/release/adapter/out/postgres"
	"github.com/jhonsferg/corelog/internal/search"
	searchpostgres "github.com/jhonsferg/corelog/internal/search/adapter/out/postgres"
	"github.com/jhonsferg/corelog/internal/sprint"
	sprintpostgres "github.com/jhonsferg/corelog/internal/sprint/adapter/out/postgres"
	"github.com/jhonsferg/corelog/internal/team"
	teampostgres "github.com/jhonsferg/corelog/internal/team/adapter/out/postgres"
	teamport "github.com/jhonsferg/corelog/internal/team/application/port"
	"github.com/jhonsferg/corelog/internal/user"
	userhttp "github.com/jhonsferg/corelog/internal/user/adapter/in/http"
	userpostgres "github.com/jhonsferg/corelog/internal/user/adapter/out/postgres"
	usersms "github.com/jhonsferg/corelog/internal/user/adapter/out/sms"
	userport "github.com/jhonsferg/corelog/internal/user/application/port"
	"github.com/jhonsferg/corelog/internal/workflow"
	workflowpostgres "github.com/jhonsferg/corelog/internal/workflow/adapter/out/postgres"
	"github.com/jhonsferg/corelog/internal/worklog"
	worklogpostgres "github.com/jhonsferg/corelog/internal/worklog/adapter/out/postgres"
)

// buildRouter assembles the HTTP router with every module mounted under /api/v1. It only constructs
// adapters and services; it does not register the global session validator (run() does that) and it
// does not open connections, so it can be called with a nil db to inspect the route table.
func buildRouter(cfg *config.Config, db *sqlx.DB, log *slog.Logger) (chi.Router, error) {
	var userRepo userport.Repository
	var teamRepo teamport.Repository
	var issueDBRepo issueport.Repository
	var projectRepo projectport.Repository
	var projectMemberRepo projectport.MemberRepository
	userRepo = userpostgres.NewUserRepository(db)
	teamRepo = teampostgres.NewTeamRepository(db)
	issueDBRepo = issuepostgres.NewIssueRepository(db)
	projectRepo = projectpostgres.NewProjectRepository(db).WithWorkflowProvisioner(workflowpostgres.NewProvisioner())
	projectMemberRepo = projectpostgres.NewMemberRepository(db)
	projectCatalogRepo := projectpostgres.NewCatalogRepository(db)
	sessionRepo := userpostgres.NewSessionRepository(db)
	issueCatalogRepo := issuepostgres.NewCatalogRepository(db)

	userModule := user.NewModule(userRepo, sessionRepo, database.NewTxManager(db), userpostgres.NewMembershipDetacher(db), cfg.Auth.JWTSecret, cfg.Auth.JWTExpiration)
	legalModule := legal.NewModule(legalpostgres.NewRepository(db), database.NewTxManager(db),
		events.NewPostgresPublisher(func(ctx context.Context) sqlx.ExtContext { return database.Executor(ctx, db) }),
		clock.RealClock{}, cfg.Auth.JWTSecret, cfg.Legal.Enforcement)
	userModule.WithConsents(userConsentGate{legal: legalModule.Service})
	secretCipher, err := twoFactorCipher(cfg, log)
	if err != nil {
		return nil, err
	}
	twoFactorService := user.NewTwoFactorService(userRepo, userpostgres.NewTwoFactorRepository(db), secretCipher,
		database.NewTxManager(db), events.NewPostgresPublisher(func(ctx context.Context) sqlx.ExtContext { return database.Executor(ctx, db) }),
		clock.RealClock{})
	userModule.Handler.WithTwoFactor(twoFactorService)
	userModule.WithPhoneVerification(userpostgres.NewPhoneVerificationRepository(db), usersms.NewNoopSender(log, cfg.Server.Env == "development"))
	orgModule := organization.NewModule(
		organizationpostgres.NewRepository(db),
		database.NewTxManager(db),
		events.NewPostgresPublisher(func(ctx context.Context) sqlx.ExtContext { return database.Executor(ctx, db) }),
		clock.RealClock{},
		cfg.Auth.JWTSecret,
		orgUserEmails{db: db},
		os.Getenv("PUBLIC_URL"),
	).WithCatalogSeeder(issueCatalogRepo).WithConsentGate(invitationConsentGate{legal: legalModule.Service})
	orgs := newOrgDirectory(db)
	teamModule := team.NewModule(teamRepo, orgs, database.NewTxManager(db), cfg.Auth.JWTSecret)
	authorizer := authusecase.NewAuthorizer(authpostgres.NewRoleLookup(db)).WithSchemes(authpostgres.NewSchemeRepository(db))
	projectAuthorizer := projectAuthorizerAdapter{authorizer: authorizer}
	projectModule := project.NewModule(projectRepo, projectMemberRepo, projectCatalogRepo, orgs, cfg.Auth.JWTSecret, projectAuthorizer).
		WithEvents(projectpostgres.NewEventPublisher(db)).
		WithTransactions(database.NewTxManager(db)).
		WithTeamGrants(projectpostgres.NewTeamGrantRepository(db))
	projectModule.Handler.WithPermissions(authusecase.NewPermissionService(authorizer, authpostgres.NewSchemeRepository(db)))
	projectModule.WithClock(clock.RealClock{})

	issueRepo := issueDBRepo
	userDirectory := issuedirectory.NewUserDirectory(db)
	collaborationRepo := issuepostgres.NewCollaborationRepository(db)
	issueLinkRepo := issuepostgres.NewIssueLinkRepository(db)
	watcherRepo := issuepostgres.NewWatcherRepository(db)
	savedFilterRepo := issuepostgres.NewSavedFilterRepository(db)
	attachmentRepo := issuepostgres.NewAttachmentRepository(db)
	attachmentRoot := os.Getenv("ATTACHMENTS_DIR")
	if attachmentRoot == "" {
		attachmentRoot = filepath.Join("data", "attachments")
	}
	fileStorage := issuefilesystem.New(attachmentRoot)
	workflowModule := workflow.NewModule(workflowpostgres.NewRepository(db), clock.RealClock{})
	workflowAdmin := workflow.NewManagementModule(workflowpostgres.NewRepository(db), clock.RealClock{}, workflow.Dependencies{
		Authorizer:  projectAuthorizer,
		OrgRoles:    orgCatalogRoleChecker{svc: orgModule.Service},
		Transaction: database.NewTxManager(db),
		Projects:    projectKeyResolver{projects: projectModule.Service.(projectport.ProjectManager)},
		JWTSecret:   cfg.Auth.JWTSecret,
	})
	notificationModule := notification.NewModule(notificationpostgres.NewNotificationRepository(db), cfg.Auth.JWTSecret)
	issueModule := issue.NewModule(issueRepo, userDirectory, collaborationRepo, issueLinkRepo, watcherRepo, savedFilterRepo, cfg.Auth.JWTSecret, projectAuthorizer, notificationModule.Service, projectMemberRepo).WithTxManager(database.NewTxManager(db))
	searchModule := search.NewModule(searchpostgres.NewSearchIndex(db), cfg.Auth.JWTSecret)
	dashboardModule := dashboard.NewModule(dashboardpostgres.NewDashboardReader(db), cfg.Auth.JWTSecret)
	favoriteModule := favorite.NewModule(favoritepostgres.NewRepository(db), favoritepostgres.NewProjectVisibility(db), cfg.Auth.JWTSecret)
	projectModule.WithRecents(favoriteModule.RecentRecorder)
	noteModule := note.NewModule(notepostgres.NewRepository(db), notepostgres.NewLinkAccess(db), cfg.Auth.JWTSecret)
	wireSearch(searchModule, db, noteModule.Search)
	calendarModule := calendar.NewModule(calendarpostgres.NewCalendarRepository(db), projectAuthorizer, cfg.Auth.JWTSecret)
	sprintModule := sprint.NewModule(sprintpostgres.NewSprintRepository(db), projectAuthorizer, database.NewTxManager(db), clock.RealClock{}, cfg.Auth.JWTSecret)
	worklogModule := worklog.NewModule(worklogpostgres.NewRepository(db), projectAuthorizer, database.NewTxManager(db),
		events.NewPostgresPublisher(func(ctx context.Context) sqlx.ExtContext { return database.Executor(ctx, db) }),
		clock.RealClock{}, cfg.Auth.JWTSecret)
	releaseModule := release.NewModule(release.Dependencies{
		Repository: releasepostgres.NewRepository(db),
		Authorizer: projectAuthorizer,
		Projects:   projectKeyResolver{projects: projectModule.Service.(projectport.ProjectManager)},
		Tx:         database.NewTxManager(db),
		Events:     events.NewPostgresPublisher(func(ctx context.Context) sqlx.ExtContext { return database.Executor(ctx, db) }),
		Clock:      clock.RealClock{},
		JWTSecret:  cfg.Auth.JWTSecret,
	})
	boardRepo := boardpostgres.NewBoardRepository(db)
	boardModule := board.NewModule(board.Deps{
		Repository: boardRepo, Boards: boardRepo, Projects: projectModule.Service, Authorizer: projectAuthorizer,
		Mover:       boardIssueMover{issues: issueModule.Service.(issueport.TransitionService)},
		Transitions: boardCardTransitions{issues: issueModule.Service.(issueport.TransitionService)},
		Editor:      boardCardEditor{issues: issueModule.Service},
		Clock:       clock.RealClock{}, JWTSecret: cfg.Auth.JWTSecret,
	})
	issueModule.WithCatalog(issueCatalogRepo)
	issueModule.WithWorkflowEngine(workflowEngineAdapter{workflows: workflowModule.Service}, issuepostgres.NewTransitionRepository(db))
	issueModule.WithActivityFeed(issuepostgres.NewActivityFeedRepository(db)).WithIssueLabels(issuepostgres.NewIssueLabelRepository(db))
	issueModule.WithVersionLinks(issuepostgres.NewVersionLinkRepository(db))
	issueModule.WithPermissions(issuePermissionChecker{authorizer: authorizer}).
		WithEvents(events.NewPostgresPublisher(func(ctx context.Context) sqlx.ExtContext { return database.Executor(ctx, db) })).
		WithClock(clock.RealClock{})
	catalogModule := issue.NewCatalogModule(issueCatalogRepo, orgCatalogRoleChecker{svc: orgModule.Service}, projectAuthorizer, database.NewTxManager(db), clock.RealClock{}, cfg.Auth.JWTSecret)
	issueModule.Handler.WithAttachments(attachmentRepo, fileStorage)
	issueModule.WithComments(issueusecase.NewCommentService(issueRepo, issuepostgres.NewCommentRepository(db), issuePermissionChecker{authorizer: authorizer}, projectAuthorizer, userDirectory, database.NewTxManager(db), events.NewPostgresPublisher(func(ctx context.Context) sqlx.ExtContext { return database.Executor(ctx, db) }), clock.RealClock{}))
	customFieldModule := issue.NewCustomFieldModule(issuepostgres.NewCustomFieldRepository(db), orgCatalogRoleChecker{svc: orgModule.Service}, projectAuthorizer, database.NewTxManager(db), clock.RealClock{}, cfg.Auth.JWTSecret)
	issueModule.WithCustomFields(customFieldModule.Service, projectBrowseChecker{authorizer: projectAuthorizer})
	issueModule.Handler.WithFieldScope(customFieldModule.Service)
	issueModule.WithOperations(issuepostgres.NewOperationsRepository(db), attachmentRepo, fileStorage)

	avatarStorage := filestorage.NewDiskStorage(dirFromEnv("AVATARS_DIR", filepath.Join("data", "avatars")))
	logoStorage := filestorage.NewDiskStorage(dirFromEnv("LOGOS_DIR", filepath.Join("data", "logos")))
	avatar := userhttp.NewAvatarHandler(userpostgres.NewAvatarRepository(db), avatarStorage, cfg.Auth.JWTSecret)
	logo := organizationhttp.NewLogoHandler(organizationpostgres.NewLogoRepository(db), logoStorage, orgRoleChecker{svc: orgModule.Service}, cfg.Auth.JWTSecret)

	router := httpserver.NewRouter(cfg.Server)
	router.Route("/api/v1", func(r chi.Router) {
		// In block mode, callers with pending legal consents are refused outside the consent endpoints.
		r.Use(legalModule.Handler.Enforce)
		avatar.RegisterRoutes(r)
		logo.RegisterRoutes(r)
		catalogModule.Handler.RegisterCatalogRoutes(r)
		customFieldModule.Handler.RegisterCustomFieldRoutes(r)
		httpserver.Mount(r, userModule.Handler, teamModule.Handler, projectModule.Handler, boardModule.Handler, issueModule.Handler, sprintModule.Handler, notificationModule.Handler, calendarModule.Handler, searchModule.Handler, dashboardModule.Handler, favoriteModule.Handler, noteModule.Handler, orgModule.Handler, workflowAdmin.Handler, releaseModule.Handler, worklogModule.Handler, legalModule.Handler)
	})
	return router, nil
}

// dirFromEnv returns the directory configured in the environment variable or the fallback.
func dirFromEnv(name, fallback string) string {
	if v := os.Getenv(name); v != "" {
		return v
	}
	return fallback
}
