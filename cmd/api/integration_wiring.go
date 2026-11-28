package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"strconv"
	"sync/atomic"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"

	"github.com/jhonsferg/corelog/internal/apitoken"
	apitokenpostgres "github.com/jhonsferg/corelog/internal/apitoken/adapter/out/postgres"
	"github.com/jhonsferg/corelog/internal/automation"
	automationpostgres "github.com/jhonsferg/corelog/internal/automation/adapter/out/postgres"
	automationusecase "github.com/jhonsferg/corelog/internal/automation/application/usecase"
	"github.com/jhonsferg/corelog/internal/importexport"
	importpostgres "github.com/jhonsferg/corelog/internal/importexport/adapter/out/postgres"
	importusecase "github.com/jhonsferg/corelog/internal/importexport/application/usecase"
	issueport "github.com/jhonsferg/corelog/internal/issue/application/port"
	notificationport "github.com/jhonsferg/corelog/internal/notification/application/port"
	orgport "github.com/jhonsferg/corelog/internal/organization/application/port"
	"github.com/jhonsferg/corelog/internal/platform/clock"
	"github.com/jhonsferg/corelog/internal/platform/config"
	"github.com/jhonsferg/corelog/internal/platform/crypto"
	"github.com/jhonsferg/corelog/internal/platform/database"
	"github.com/jhonsferg/corelog/internal/platform/events"
	"github.com/jhonsferg/corelog/internal/platform/filestorage"
	"github.com/jhonsferg/corelog/internal/platform/jobs"
	"github.com/jhonsferg/corelog/internal/platform/ratelimit"
	searchport "github.com/jhonsferg/corelog/internal/search/application/port"
	"github.com/jhonsferg/corelog/internal/webhook"
	"github.com/jhonsferg/corelog/internal/webhook/adapter/out/httpsender"
	webhookpostgres "github.com/jhonsferg/corelog/internal/webhook/adapter/out/postgres"
	webhookport "github.com/jhonsferg/corelog/internal/webhook/application/port"
	webhookusecase "github.com/jhonsferg/corelog/internal/webhook/application/usecase"
)

// integrationDeps are the pieces of the main wiring the integration modules build on.
type integrationDeps struct {
	cfg               *config.Config
	db                *sqlx.DB
	log               *slog.Logger
	issues            issueport.Service
	transitions       issueport.TransitionService
	comments          issueport.CommentService
	search            searchport.IssueSearchService
	projectAuthorizer projectAuthorizerAdapter
	orgs              orgport.Service
	notifications     notificationport.Service
}

// integrations groups the automation, webhook, API token and import/export modules.
type integrations struct {
	tokens       *apitoken.Module
	webhooks     *webhook.Module
	automation   *automation.Module
	importExport *importexport.Module
	log          *slog.Logger
}

// currentIntegrations is the instance the runtime loops use; buildRouter sets it.
var currentIntegrations atomic.Pointer[integrations]

// webhookCipher returns the AES-GCM cipher for stored webhook secrets. APP_ENCRYPTION_KEY is a base64 32 byte key;
// without it a key is derived from the JWT secret and a warning is logged.
func webhookCipher(cfg *config.Config, log *slog.Logger) (*crypto.Cipher, error) {
	var key []byte
	if raw := os.Getenv("APP_ENCRYPTION_KEY"); raw != "" {
		parsed, err := crypto.ParseKey(raw)
		if err != nil {
			return nil, fmt.Errorf("APP_ENCRYPTION_KEY must be 32 bytes encoded as base64: %w", err)
		}
		key = parsed
	} else {
		derived, err := crypto.DeriveKey([]byte(cfg.Auth.JWTSecret), "corelog webhook secret encryption")
		if err != nil {
			return nil, err
		}
		key = derived
		log.Warn("APP_ENCRYPTION_KEY is not set: webhook secrets use a key derived from JWT_SECRET")
	}
	return crypto.NewCipher(key)
}

// wireIntegrations builds the modules and installs bearer token authentication.
func wireIntegrations(d integrationDeps) (*integrations, error) {
	cfg, db := d.cfg, d.db
	clk := clock.RealClock{}
	tx := database.NewTxManager(db)
	publisher := events.NewPostgresPublisher(func(ctx context.Context) sqlx.ExtContext { return database.Executor(ctx, db) })

	// API tokens.
	var limiter *ratelimit.Limiter
	if cfg.LimitsEnabled() {
		perMin := 600
		if v, err := strconv.Atoi(os.Getenv("API_TOKEN_RATE_LIMIT_PER_MIN")); err == nil && v > 0 {
			perMin = v
		}
		limiter = ratelimit.New(ratelimit.PerMinute(perMin), clk)
	}
	tokens := apitoken.NewModule(apitokenpostgres.NewRepository(db), tx, publisher, clk, limiterOrNil(limiter), cfg.Auth.JWTSecret)
	tokens.RegisterAuthentication()

	// Search bridge shared by webhooks filters, automation conditions and the CSV export.
	bridge := searchBridge{db: db, search: d.search}

	// Webhooks.
	cipher, err := webhookCipher(cfg, d.log)
	if err != nil {
		return nil, err
	}
	allowPrivate := os.Getenv("WEBHOOK_ALLOW_PRIVATE") == "true" && !cfg.Server.IsProduction()
	if os.Getenv("WEBHOOK_ALLOW_PRIVATE") == "true" && cfg.Server.IsProduction() {
		d.log.Warn("WEBHOOK_ALLOW_PRIVATE is ignored in production")
	}
	sender := httpsender.New(httpsender.Options{AllowPrivate: allowPrivate})
	webhooks := webhook.NewModule(webhookusecase.Deps{
		Repo: webhookpostgres.NewRepository(db), Access: auditOrgAccess{svc: d.orgs}, Box: webhook.CipherBox{Cipher: cipher},
		Sender: sender, Describer: webhookpostgres.NewDescriber(db), Matcher: webhookMatcher{bridge: bridge}, Tx: tx,
		Publisher: publisher, Clock: clk, AllowPrivate: allowPrivate,
	}, cfg.Auth.JWTSecret)

	// Automation.
	data := automationData{db: db}
	rules := automation.NewModule(automationusecase.Deps{
		Repo: automationpostgres.NewRepository(db), Auth: d.projectAuthorizer, Issues: data,
		Actions: automationActions{db: db, data: data, issues: d.issues, transitions: d.transitions, comments: d.comments},
		Matcher: bridge, Directory: data, Notifier: automationNotifier{svc: d.notifications}, Poster: webhookPoster{sender: sender},
		Publisher: publisher, Clock: clk,
	}, cfg.Auth.JWTSecret)

	// Import and export.
	importExport := importexport.NewModule(importusecase.Deps{
		Repo: importpostgres.NewRepository(db), Files: filestorage.NewDiskStorage(dirFromEnv("IMPORTS_DIR", "data/imports")),
		Importer: rowImporter{db: db, issues: d.issues, transitions: d.transitions}, Auth: d.projectAuthorizer,
		Searcher: exportSearcher{bridge: bridge}, Reader: importpostgres.NewReader(db), Publisher: publisher, Clock: clk,
	}, cfg.Auth.JWTSecret)

	return &integrations{tokens: tokens, webhooks: webhooks, automation: rules, importExport: importExport, log: d.log}, nil
}

// limiterOrNil avoids a typed nil inside the Limiter interface.
func limiterOrNil(l *ratelimit.Limiter) interface {
	Allow(key string) (bool, time.Duration)
} {
	if l == nil {
		return nil
	}
	return l
}

// register mounts the HTTP routes of the integration modules.
func (i *integrations) register(r chi.Router) {
	i.tokens.Handler.RegisterRoutes(r)
	i.webhooks.Handler.RegisterRoutes(r)
	i.automation.Handler.RegisterRoutes(r)
	i.importExport.Handler.RegisterRoutes(r)
}

// webhookMatcher adapts the search bridge to the webhook filter port.
type webhookMatcher struct{ bridge searchBridge }

func (m webhookMatcher) Matches(ctx context.Context, projectID, issueID uuid.UUID, query string) (bool, error) {
	return m.bridge.Matches(ctx, projectID, issueID, query)
}

func (m webhookMatcher) Validate(ctx context.Context, query string) error {
	return m.bridge.Validate(ctx, query)
}

// webhookPoster sends the body of a send_webhook automation action with the same SSRF protections.
type webhookPoster struct{ sender *httpsender.Sender }

func (p webhookPoster) Post(ctx context.Context, url string, body []byte) error {
	res := p.sender.Send(ctx, webhookport.SendRequest{URL: url, Body: body, Headers: map[string]string{"Content-Type": "application/json", "User-Agent": "CoreLog-Automation/1"}})
	if res.Err != nil {
		return fmt.Errorf("no se pudo enviar el webhook: %w", res.Err)
	}
	if res.Status < http.StatusOK || res.Status >= http.StatusMultipleChoices {
		return fmt.Errorf("el destino respondio %d", res.Status)
	}
	return nil
}

// startIntegrationRuntime runs the consumers of the events outbox (webhook capture, automation), the delivery
// worker, the import runner and the scheduled automation runner until ctx is done.
func startIntegrationRuntime(ctx context.Context, db *sqlx.DB, log *slog.Logger) error {
	in := currentIntegrations.Load()
	if in == nil {
		return nil
	}
	dispatcher, err := events.NewDispatcher(db, []events.Consumer{in.webhooks.Service.Consumer(), in.automation.Engine.Consumer()}, events.Options{Logger: log})
	if err != nil {
		return err
	}
	if jobs.Enabled() {
		go in.webhooks.Service.Run(ctx, 5*time.Second)
		go in.importExport.Service.Run(ctx, 3*time.Second)
		go runScheduledRules(ctx, in, log)
	}
	return dispatcher.Run(ctx, time.Second)
}

// runScheduledRules triggers the scheduled automation rules once a minute.
func runScheduledRules(ctx context.Context, in *integrations, log *slog.Logger) {
	t := time.NewTicker(time.Minute)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case now := <-t.C:
			if err := in.automation.Engine.RunScheduled(ctx, now.UTC()); err != nil {
				log.Error("scheduled automation run failed", "error", err)
			}
		}
	}
}
