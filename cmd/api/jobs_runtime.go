package main

import (
	"context"
	"log/slog"

	"github.com/jmoiron/sqlx"

	"github.com/jhonsferg/corelog/internal/platform/config"
	"github.com/jhonsferg/corelog/internal/platform/jobs"
)

// startJobs runs the job scheduler (one leader across API instances) until ctx is done. Jobs come from the
// registrars added with registerJobs. JOBS_ENABLED=false turns every job off.
func startJobs(ctx context.Context, cfg *config.Config, db *sqlx.DB, log *slog.Logger) {
	if !jobs.Enabled() {
		log.Info("background jobs disabled (JOBS_ENABLED=false)")
		return
	}
	scheduler := jobs.NewScheduler(db, log)
	deps := jobDeps{Cfg: cfg, DB: db, Log: log}
	for _, register := range jobRegistrars {
		register(scheduler, deps)
	}
	scheduler.Start(ctx)
	<-ctx.Done()
	scheduler.Stop()
}
