package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"

	authpostgres "github.com/jhonsferg/corelog/internal/authorization/adapter/out/postgres"
	authusecase "github.com/jhonsferg/corelog/internal/authorization/application/usecase"
	"github.com/jhonsferg/corelog/internal/notification"
	notificationemail "github.com/jhonsferg/corelog/internal/notification/adapter/out/email"
	notificationpostgres "github.com/jhonsferg/corelog/internal/notification/adapter/out/postgres"
	notificationport "github.com/jhonsferg/corelog/internal/notification/application/port"
	notificationusecase "github.com/jhonsferg/corelog/internal/notification/application/usecase"
	orgport "github.com/jhonsferg/corelog/internal/organization/application/port"
	"github.com/jhonsferg/corelog/internal/platform/clock"
	"github.com/jhonsferg/corelog/internal/platform/config"
	"github.com/jhonsferg/corelog/internal/platform/database"
	"github.com/jhonsferg/corelog/internal/platform/jobs"
	"github.com/jhonsferg/corelog/internal/platform/retention"
	"github.com/jhonsferg/corelog/internal/search"
	searchpostgres "github.com/jhonsferg/corelog/internal/search/adapter/out/postgres"
	searchport "github.com/jhonsferg/corelog/internal/search/application/port"
)

// newNotificationEngine builds the scheme engine over the database. Recipients are checked against the browse
// permission of the project.
func newNotificationEngine(cfg *config.Config, db *sqlx.DB, authorizer *authusecase.Authorizer) *notificationusecase.Engine {
	return notificationusecase.NewEngine(notificationusecase.EngineDeps{
		Schemes:  notificationpostgres.NewSchemeRepository(db),
		Facts:    notificationpostgres.NewFactsRepository(db),
		Notes:    notificationpostgres.NewNotificationRepository(db),
		Outbox:   notificationpostgres.NewOutboxRepository(db),
		Delivery: notificationpostgres.NewDeliveryRepository(db),
		Browser:  mentionBrowser{perms: issuePermissionChecker{authorizer: authorizer}},
		Render:   notificationemail.NewRenderer(),
		Tx:       database.NewTxManager(db),
		Clock:    clock.RealClock{},
		BaseURL:  cfg.Security.AppPublicURL,
	})
}

// wireNotifications connects the scheme engine to the notification service and enables the scheme and email
// outbox endpoints.
func wireNotifications(cfg *config.Config, db *sqlx.DB, m *notification.Module, authorizer *authusecase.Authorizer, projects projectAuthorizerAdapter, orgs orgport.Service) {
	m.Notifications.WithEngine(newNotificationEngine(cfg, db, authorizer))
	clk := clock.RealClock{}
	m.Handler.
		WithSchemes(notificationusecase.NewSchemeService(notificationpostgres.NewSchemeRepository(db), projects, orgRoleChecker{svc: orgs}, clk)).
		WithOutboxAdmin(notificationusecase.NewOutboxAdminService(notificationpostgres.NewOutboxRepository(db), clk))
}

// newSchemeConsumer is the event consumer that notifies according to the schemes.
func newSchemeConsumer(cfg *config.Config, db *sqlx.DB) *notificationusecase.SchemeConsumer {
	authorizer := authusecase.NewAuthorizer(authpostgres.NewRoleLookup(db)).WithSchemes(authpostgres.NewSchemeRepository(db))
	return notificationusecase.NewSchemeConsumer(newNotificationEngine(cfg, db, authorizer))
}

// jobDeps are the dependencies a job registrar may use.
type jobDeps struct {
	Cfg *config.Config
	DB  *sqlx.DB
	Log *slog.Logger
}

// jobRegistrar adds jobs to the scheduler before it starts. Other modules register theirs from an init function
// with registerJobs.
type jobRegistrar func(s *jobs.Scheduler, deps jobDeps)

var jobRegistrars []jobRegistrar

// registerJobs adds a registrar. It must be called before the scheduler starts (from init).
func registerJobs(r jobRegistrar) { jobRegistrars = append(jobRegistrars, r) }

func init() { registerJobs(registerNotificationJobs) }

func mustRegister(s *jobs.Scheduler, log *slog.Logger, name, spec string, fn func(ctx context.Context) error) {
	if err := s.Register(name, spec, fn); err != nil {
		log.Error("job not registered", "job", name, "error", err)
	}
}

func envOr(name, fallback string) string {
	if v := strings.TrimSpace(os.Getenv(name)); v != "" {
		return v
	}
	return fallback
}

// registerNotificationJobs adds the email outbox sender, the daily digest, the filter subscription sender, the
// bookkeeping cleanup and the soft-delete purge.
func registerNotificationJobs(s *jobs.Scheduler, d jobDeps) {
	clk := clock.RealClock{}
	outbox := notificationpostgres.NewOutboxRepository(d.DB)
	delivery := notificationpostgres.NewDeliveryRepository(d.DB)
	facts := notificationpostgres.NewFactsRepository(d.DB)
	notes := notificationpostgres.NewNotificationRepository(d.DB)
	render := notificationemail.NewRenderer()
	baseURL := d.Cfg.Security.AppPublicURL

	worker := notificationusecase.NewOutboxWorker(outbox, newMailer(d.Cfg, d.Log), clk, d.Log, 25)
	mustRegister(s, d.Log, "email-outbox", "every 10s", func(ctx context.Context) error {
		_, _, err := worker.Run(ctx)
		return err
	})

	digest := notificationusecase.NewDigestJob(delivery, facts, outbox, render, notes, database.NewTxManager(d.DB), clk, baseURL)
	mustRegister(s, d.Log, "email-digest", envOr("EMAIL_DIGEST_SCHEDULE", "daily at 08:00 UTC"), func(ctx context.Context) error {
		_, err := digest.Run(ctx)
		return err
	})

	subs := notificationusecase.NewFilterSubscriptionJob(newFilterSubscriptions(d.DB), facts, notes, outbox, render, clk, baseURL)
	mustRegister(s, d.Log, "filter-subscriptions", "every 1m", func(ctx context.Context) error {
		_, err := subs.Run(ctx)
		return err
	})

	mustRegister(s, d.Log, "notification-cleanup", "daily at 03:30 UTC", func(ctx context.Context) error {
		_, err := delivery.PurgeDeliveries(ctx, clk.Now().Add(-30*24*time.Hour))
		if err != nil {
			return err
		}
		_, err = jobs.PurgeRuns(ctx, d.DB, clk.Now().Add(-14*24*time.Hour))
		return err
	})

	registerRetentionJobs(s, d, clk)
}

// registerRetentionJobs adds the data lifecycle jobs when PURGE_ENABLED is true. Each one reports what it found
// or removed as the result of its run.
func registerRetentionJobs(s *jobs.Scheduler, d jobDeps, clk clock.Clock) {
	cfg := d.Cfg.Retention
	if !cfg.Enabled {
		return
	}
	day := 24 * time.Hour
	attachmentsDir := os.Getenv("ATTACHMENTS_DIR")
	if attachmentsDir == "" {
		attachmentsDir = filepath.Join("data", "attachments")
	}
	purger := retention.NewPurger(d.DB, clk, time.Duration(cfg.SoftDeletedDays)*day, cfg.DryRun)
	mustRegister(s, d.Log, "soft-delete-purge", "daily at 04:00 UTC", func(ctx context.Context) error {
		res, err := purger.Run(ctx)
		jobs.ReportResult(ctx, fmt.Sprintf("dry_run=%t issues_found=%d issues_purged=%d projects_found=%d projects_purged=%d blocked=%d",
			cfg.DryRun, res.IssuesFound, res.IssuesPurged, res.ProjectsFound, res.ProjectsPurged, res.Blocked))
		d.Log.Info("soft-delete purge", "dry_run", cfg.DryRun, "issues_found", res.IssuesFound, "issues_purged", res.IssuesPurged,
			"projects_found", res.ProjectsFound, "projects_purged", res.ProjectsPurged, "blocked", res.Blocked)
		return err
	})

	life := retention.NewLifecycle(d.DB, clk, retention.Options{
		SoftDeletedAfter:       time.Duration(cfg.SoftDeletedDays) * day,
		ReadNotificationsAfter: time.Duration(cfg.ReadNotificationDays) * day,
		FingerprintsAfter:      time.Duration(cfg.FingerprintDays) * day,
		DryRun:                 cfg.DryRun,
		AttachmentsDir:         attachmentsDir,
		DeleteOrphans:          cfg.DeleteOrphans,
	})
	lifecycleJob := func(name string, run func(ctx context.Context) (retention.Report, error)) func(ctx context.Context) error {
		return func(ctx context.Context) error {
			rep, err := run(ctx)
			jobs.ReportResult(ctx, rep.Summary())
			d.Log.Info(name, "dry_run", rep.DryRun, "skipped", rep.Skipped, "summary", rep.Summary())
			return err
		}
	}
	mustRegister(s, d.Log, "retention-soft-deleted", "daily at 04:15 UTC", lifecycleJob("retention soft-deleted", life.PurgeSoftDeleted))
	mustRegister(s, d.Log, "retention-housekeeping", "daily at 04:30 UTC", lifecycleJob("retention housekeeping", life.Prune))
	mustRegister(s, d.Log, "retention-orphan-files", "daily at 04:45 UTC", lifecycleJob("retention orphan files", life.CheckOrphanFiles))
}

// filterSubscriptions adapts the saved filter module to the subscription job.
type filterSubscriptions struct {
	db      *sqlx.DB
	filters searchport.FilterService
}

var _ notificationport.FilterSubscriptions = filterSubscriptions{}

func newFilterSubscriptions(db *sqlx.DB) filterSubscriptions {
	clk := clock.RealClock{}
	m := search.NewModule(searchpostgres.NewSearchIndex(db), "")
	repo := searchpostgres.NewIssueSearchRepository(db).WithVersions(searchpostgres.VersionTables{})
	m.WithIssueSearch(search.IssueSearchDeps{Searcher: repo, Custom: repo, Viewers: repo, Suggest: repo, Clock: clk}).
		WithFilters(searchpostgres.NewFilterRepository(db), clk)
	return filterSubscriptions{db: db, filters: m.Filters}
}

func (f filterSubscriptions) Due(ctx context.Context) ([]notificationport.DueFilter, error) {
	due, err := f.filters.DueSubscriptions(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]notificationport.DueFilter, 0, len(due))
	for _, item := range due {
		out = append(out, notificationport.DueFilter{
			SubscriptionID: item.Subscription.ID, UserID: item.Subscription.UserID, FilterID: item.Filter.ID,
			FilterName: item.Filter.Name, LastSentAt: item.Subscription.LastSentAt,
		})
	}
	return out, nil
}

func (f filterSubscriptions) Run(ctx context.Context, due notificationport.DueFilter, limit int) ([]notificationport.FilterRow, int, error) {
	var viewer struct {
		Role     string `db:"role"`
		Timezone string `db:"timezone"`
	}
	if err := f.db.GetContext(ctx, &viewer, `SELECT role, timezone FROM users WHERE id = $1`, due.UserID); err != nil {
		return nil, 0, err
	}
	out, err := f.filters.Run(ctx, searchport.Viewer{UserID: due.UserID, PlatformAdmin: viewer.Role == "admin", Timezone: viewer.Timezone}, due.FilterID, 1, limit, true)
	if err != nil {
		return nil, 0, err
	}
	rows := make([]notificationport.FilterRow, 0, len(out.Page.Items))
	for _, item := range out.Page.Items {
		status := item.LegacyStatus
		if item.Status != nil {
			status = item.Status.Name
		}
		rows = append(rows, notificationport.FilterRow{Key: item.Key, Summary: item.Summary, Status: status})
	}
	return rows, out.Page.Total, nil
}

func (f filterSubscriptions) MarkDelivered(ctx context.Context, id uuid.UUID) error {
	return f.filters.MarkDelivered(ctx, id)
}
