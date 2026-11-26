// Package jobs runs named periodic jobs in-process. Each run is guarded by a
// PostgreSQL advisory lock so that several API replicas do not duplicate work.
package jobs

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"sync"
	"time"

	"github.com/jmoiron/sqlx"
)

// Func is the work executed by a job.
type Func func(ctx context.Context) error

type job struct {
	name     string
	interval time.Duration
	fn       Func
}

// Scheduler runs registered jobs on fixed intervals.
type Scheduler struct {
	db     *sqlx.DB
	logger *slog.Logger

	mu      sync.Mutex
	jobs    []job
	started bool
	cancel  context.CancelFunc
	wg      sync.WaitGroup
}

// NewScheduler creates a scheduler. A nil db runs jobs without advisory locking,
// which is useful in tests. A nil logger uses slog.Default().
func NewScheduler(db *sqlx.DB, logger *slog.Logger) *Scheduler {
	if logger == nil {
		logger = slog.Default()
	}
	return &Scheduler{db: db, logger: logger}
}

// Enabled reports whether background jobs should run. JOBS_ENABLED=false
// disables them; any other value, or no value, enables them.
func Enabled() bool {
	return os.Getenv("JOBS_ENABLED") != "false"
}

// Every registers a job. Jobs registered after Start are ignored.
func (s *Scheduler) Every(name string, interval time.Duration, fn func(ctx context.Context) error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.started {
		s.logger.Warn("job registered after scheduler start; ignored", slog.String("job", name))
		return
	}
	s.jobs = append(s.jobs, job{name: name, interval: interval, fn: fn})
}

// Start launches one goroutine per registered job. Calling Start again is a no-op.
func (s *Scheduler) Start(ctx context.Context) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.started {
		return
	}
	s.started = true
	runCtx, cancel := context.WithCancel(ctx)
	s.cancel = cancel
	for _, j := range s.jobs {
		if j.interval <= 0 {
			s.logger.Error("job interval must be positive; not started", slog.String("job", j.name))
			continue
		}
		s.wg.Add(1)
		go s.loop(runCtx, j)
	}
}

// Stop cancels the jobs and waits for running goroutines to finish.
func (s *Scheduler) Stop() {
	s.mu.Lock()
	cancel := s.cancel
	s.mu.Unlock()
	if cancel != nil {
		cancel()
	}
	s.wg.Wait()
}

func (s *Scheduler) loop(ctx context.Context, j job) {
	defer s.wg.Done()
	ticker := time.NewTicker(j.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.tick(ctx, j)
		}
	}
}

func (s *Scheduler) tick(ctx context.Context, j job) {
	if s.db == nil {
		s.run(ctx, j)
		return
	}
	conn, err := s.db.Connx(ctx)
	if err != nil {
		if ctx.Err() == nil {
			s.logger.Error("job lock connection failed", slog.String("job", j.name), slog.String("error", err.Error()))
		}
		return
	}
	defer func() { _ = conn.Close() }()

	key := "job:" + j.name
	var locked bool
	if err := conn.QueryRowContext(ctx, `SELECT pg_try_advisory_lock(hashtext($1))`, key).Scan(&locked); err != nil {
		if ctx.Err() == nil {
			s.logger.Error("job lock failed", slog.String("job", j.name), slog.String("error", err.Error()))
		}
		return
	}
	if !locked {
		s.logger.Debug("job already running elsewhere", slog.String("job", j.name))
		return
	}
	defer func() {
		unlockCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 10*time.Second)
		defer cancel()
		if _, err := conn.ExecContext(unlockCtx, `SELECT pg_advisory_unlock(hashtext($1))`, key); err != nil {
			s.logger.Error("job unlock failed", slog.String("job", j.name), slog.String("error", err.Error()))
		}
	}()
	s.run(ctx, j)
}

// run executes one job with panic recovery and logs one line for the run.
func (s *Scheduler) run(ctx context.Context, j job) {
	start := time.Now()
	err := safeCall(ctx, j.fn)
	attrs := []any{slog.String("job", j.name), slog.Duration("duration", time.Since(start))}
	if err != nil {
		s.logger.Error("job run finished with error", append(attrs, slog.String("error", err.Error()))...)
		return
	}
	s.logger.Info("job run finished", attrs...)
}

func safeCall(ctx context.Context, fn Func) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("jobs: panic: %v", r)
		}
	}()
	if fn == nil {
		return errors.New("jobs: nil job function")
	}
	return fn(ctx)
}
