// Package jobs runs named periodic jobs in-process. Several API replicas may run the same scheduler: one of them
// becomes the leader through a PostgreSQL advisory lock and runs every job, the others wait and take over when the
// leader stops. Every run is recorded in scheduler_runs.
package jobs

import (
	"context"
	"database/sql/driver"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"sort"
	"sync"
	"time"

	"github.com/jmoiron/sqlx"

	"github.com/jhonsferg/corelog/internal/platform/clock"
)

// Func is the work executed by a job.
type Func func(ctx context.Context) error

type job struct {
	name     string
	schedule Schedule
	fn       Func
}

// JobInfo describes a registered job.
type JobInfo struct {
	Name     string
	Schedule string
}

// Scheduler runs registered jobs on their schedules.
type Scheduler struct {
	db          *sqlx.DB
	logger      *slog.Logger
	clock       clock.Clock
	leaderRetry time.Duration
	leaderCheck time.Duration

	mu      sync.Mutex
	jobs    []job
	started bool
	cancel  context.CancelFunc
	wg      sync.WaitGroup
	leader  bool
}

// NewScheduler creates a scheduler. A nil db runs jobs without leader election or persisted runs, which is useful
// in tests. A nil logger uses slog.Default().
func NewScheduler(db *sqlx.DB, logger *slog.Logger) *Scheduler {
	if logger == nil {
		logger = slog.Default()
	}
	return &Scheduler{db: db, logger: logger, clock: clock.RealClock{}, leaderRetry: 5 * time.Second, leaderCheck: 2 * time.Second}
}

// WithClock replaces the time source used to compute schedules.
func (s *Scheduler) WithClock(c clock.Clock) *Scheduler {
	s.clock = c
	return s
}

// WithLeaderTiming sets how often a waiting replica tries to become leader and how often the leader checks its
// lock connection.
func (s *Scheduler) WithLeaderTiming(retry, check time.Duration) *Scheduler {
	if retry > 0 {
		s.leaderRetry = retry
	}
	if check > 0 {
		s.leaderCheck = check
	}
	return s
}

// Enabled reports whether background jobs should run. JOBS_ENABLED=false
// disables them; any other value, or no value, enables them.
func Enabled() bool {
	return os.Getenv("JOBS_ENABLED") != "false"
}

// Every registers a job that runs at a fixed interval. Jobs registered after Start are ignored.
func (s *Scheduler) Every(name string, interval time.Duration, fn func(ctx context.Context) error) {
	if interval <= 0 {
		s.logger.Error("job interval must be positive; not registered", slog.String("job", name))
		return
	}
	s.add(name, Interval(interval), fn)
}

// Register adds a job with a schedule text ("every 10s", "daily at 03:00 America/Bogota", "cron */5 * * * *").
func (s *Scheduler) Register(name, spec string, fn func(ctx context.Context) error) error {
	schedule, err := ParseSchedule(spec)
	if err != nil {
		return err
	}
	if name == "" || fn == nil {
		return errors.New("jobs: a job needs a name and a function")
	}
	s.add(name, schedule, fn)
	return nil
}

func (s *Scheduler) add(name string, schedule Schedule, fn Func) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.started {
		s.logger.Warn("job registered after scheduler start; ignored", slog.String("job", name))
		return
	}
	s.jobs = append(s.jobs, job{name: name, schedule: schedule, fn: fn})
}

// Jobs lists the registered jobs by name.
func (s *Scheduler) Jobs() []JobInfo {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]JobInfo, 0, len(s.jobs))
	for _, j := range s.jobs {
		out = append(out, JobInfo{Name: j.name, Schedule: j.schedule.String()})
	}
	sort.Slice(out, func(a, b int) bool { return out[a].Name < out[b].Name })
	return out
}

// IsLeader reports whether this replica currently runs the jobs.
func (s *Scheduler) IsLeader() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.leader
}

// Start begins supervising the jobs. Calling Start again is a no-op.
func (s *Scheduler) Start(ctx context.Context) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.started {
		return
	}
	s.started = true
	runCtx, cancel := context.WithCancel(ctx)
	s.cancel = cancel
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		if s.db == nil {
			s.setLeader(true)
			s.runJobs(runCtx, nil)
			return
		}
		s.registerJobs(runCtx)
		s.superviseLeadership(runCtx)
	}()
}

// Stop cancels the jobs, releases the leadership and waits for running goroutines to finish.
func (s *Scheduler) Stop() {
	s.mu.Lock()
	cancel := s.cancel
	s.mu.Unlock()
	if cancel != nil {
		cancel()
	}
	s.wg.Wait()
}

func (s *Scheduler) setLeader(v bool) {
	s.mu.Lock()
	s.leader = v
	s.mu.Unlock()
}

// leaderLockQuery keys the lock by schema so that isolated test schemas do not compete.
const leaderLockQuery = `SELECT pg_try_advisory_lock(hashtext('corelog-scheduler:' || current_schema()))`
const leaderUnlockQuery = `SELECT pg_advisory_unlock(hashtext('corelog-scheduler:' || current_schema()))`

func (s *Scheduler) superviseLeadership(ctx context.Context) {
	for ctx.Err() == nil {
		conn, err := s.db.Connx(ctx)
		if err == nil {
			var locked bool
			if err = conn.QueryRowContext(ctx, leaderLockQuery).Scan(&locked); err == nil && locked {
				s.lead(ctx, conn)
				continue
			}
			_ = conn.Close()
		}
		if err != nil && ctx.Err() == nil {
			s.logger.Error("scheduler leader election failed", slog.String("error", err.Error()))
		}
		select {
		case <-ctx.Done():
		case <-time.After(s.leaderRetry):
		}
	}
}

// lead runs the jobs while the lock connection stays healthy, then releases the lock.
func (s *Scheduler) lead(ctx context.Context, conn *sqlx.Conn) {
	s.logger.Info("scheduler leader elected")
	s.setLeader(true)
	leadCtx, cancel := context.WithCancel(ctx)
	store := &runStore{db: s.db}
	store.abandonRunning(leadCtx, s.logger)

	done := make(chan struct{})
	go func() {
		defer close(done)
		s.runJobs(leadCtx, store)
	}()

	ticker := time.NewTicker(s.leaderCheck)
	lost := false
loop:
	for {
		select {
		case <-leadCtx.Done():
			break loop
		case <-done:
			break loop
		case <-ticker.C:
			if err := conn.PingContext(leadCtx); err != nil && leadCtx.Err() == nil {
				s.logger.Error("scheduler leader connection lost", slog.String("error", err.Error()))
				lost = true
				break loop
			}
		}
	}
	ticker.Stop()
	cancel()
	<-done
	s.setLeader(false)
	s.release(conn, lost)
}

func (s *Scheduler) release(conn *sqlx.Conn, lost bool) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if !lost {
		var unlocked bool
		if err := conn.QueryRowContext(ctx, leaderUnlockQuery).Scan(&unlocked); err == nil && unlocked {
			_ = conn.Close()
			return
		}
	}
	// The lock could not be released explicitly: drop the physical connection, which ends the session.
	_ = conn.Raw(func(any) error { return driver.ErrBadConn })
	_ = conn.Close()
}

func (s *Scheduler) registerJobs(ctx context.Context) {
	store := &runStore{db: s.db}
	s.mu.Lock()
	jobs := append([]job(nil), s.jobs...)
	s.mu.Unlock()
	for _, j := range jobs {
		if err := store.registerJob(ctx, j.name, j.schedule.String()); err != nil && ctx.Err() == nil {
			s.logger.Error("job registration failed", slog.String("job", j.name), slog.String("error", err.Error()))
		}
	}
}

// runJobs runs one loop per job until ctx is done and waits for them.
func (s *Scheduler) runJobs(ctx context.Context, store *runStore) {
	s.mu.Lock()
	jobs := append([]job(nil), s.jobs...)
	s.mu.Unlock()
	var wg sync.WaitGroup
	for _, j := range jobs {
		wg.Add(1)
		go func(j job) {
			defer wg.Done()
			s.loop(ctx, j, store)
		}(j)
	}
	wg.Wait()
}

func (s *Scheduler) loop(ctx context.Context, j job, store *runStore) {
	for {
		now := s.clock.Now()
		wait := j.schedule.Next(now).Sub(now)
		if wait < 0 {
			wait = 0
		}
		timer := time.NewTimer(wait)
		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case <-timer.C:
			s.run(ctx, j, store)
		}
	}
}

// run executes one job with panic recovery, records the run and logs one line for it.
func (s *Scheduler) run(ctx context.Context, j job, store *runStore) {
	start := s.clock.Now()
	var runID int64
	if store != nil {
		id, err := store.begin(ctx, j.name, start)
		if err != nil && ctx.Err() == nil {
			s.logger.Error("job run not recorded", slog.String("job", j.name), slog.String("error", err.Error()))
		}
		runID = id
	}
	began := time.Now()
	err := safeCall(ctx, j.fn)
	elapsed := time.Since(began)
	if store != nil && runID != 0 {
		store.finish(runID, s.clock.Now(), elapsed, err, s.logger)
	}
	attrs := []any{slog.String("job", j.name), slog.Duration("duration", elapsed)}
	if err != nil {
		s.logger.Error("job run finished with error", append(attrs, slog.String("error", err.Error()))...)
		return
	}
	s.logger.Debug("job run finished", attrs...)
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
