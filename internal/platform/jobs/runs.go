package jobs

import (
	"context"
	"log/slog"
	"time"

	"github.com/jmoiron/sqlx"
)

const maxErrorLength = 500

// runStore persists the job registry and the run history.
type runStore struct {
	db *sqlx.DB
}

func (s *runStore) registerJob(ctx context.Context, name, schedule string) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO scheduler_jobs (name, schedule, registered_at, last_seen_at) VALUES ($1, $2, now(), now())
		ON CONFLICT (name) DO UPDATE SET schedule = EXCLUDED.schedule, last_seen_at = now()`, name, schedule)
	return err
}

// abandonRunning closes the runs a previous leader left open when it stopped abruptly.
func (s *runStore) abandonRunning(ctx context.Context, logger *slog.Logger) {
	if _, err := s.db.ExecContext(ctx, `
		UPDATE scheduler_runs SET status = 'abandoned', finished_at = now() WHERE status = 'running'`); err != nil && ctx.Err() == nil {
		logger.Error("scheduler runs cleanup failed", slog.String("error", err.Error()))
	}
}

func (s *runStore) begin(ctx context.Context, job string, at time.Time) (int64, error) {
	var id int64
	err := s.db.GetContext(ctx, &id, `INSERT INTO scheduler_runs (job, started_at, status) VALUES ($1, $2, 'running') RETURNING id`, job, at)
	return id, err
}

func (s *runStore) finish(id int64, at time.Time, elapsed time.Duration, runErr error, logger *slog.Logger) {
	status, message := "ok", ""
	if runErr != nil {
		status, message = "error", runErr.Error()
		if len(message) > maxErrorLength {
			message = message[:maxErrorLength]
		}
	}
	// A cancelled run context must not lose the outcome.
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if _, err := s.db.ExecContext(ctx, `
		UPDATE scheduler_runs SET status = $2, error = $3, finished_at = $4, duration_ms = $5 WHERE id = $1`,
		id, status, message, at, elapsed.Milliseconds()); err != nil {
		logger.Error("job run outcome not recorded", slog.String("error", err.Error()))
	}
}

// PurgeRuns deletes run history older than the cutoff and returns how many rows went away.
func PurgeRuns(ctx context.Context, db *sqlx.DB, cutoff time.Time) (int64, error) {
	res, err := db.ExecContext(ctx, `DELETE FROM scheduler_runs WHERE started_at < $1 AND status <> 'running'`, cutoff)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}
