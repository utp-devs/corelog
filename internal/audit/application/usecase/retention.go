package usecase

import (
	"context"
	"fmt"

	"github.com/jhonsferg/corelog/internal/audit/application/port"
	"github.com/jhonsferg/corelog/internal/platform/clock"
)

// DefaultRetentionDays is used when the configured value is not positive.
const DefaultRetentionDays = 365

// RetentionJob deletes audit entries older than the retention window. It is
// run periodically by the jobs scheduler.
type RetentionJob func(ctx context.Context) error

// NewRetentionJob returns a job that purges entries created more than
// retentionDays days before the clock's current time. A non-positive value
// selects DefaultRetentionDays.
func NewRetentionJob(repo port.Repository, retentionDays int, clk clock.Clock) RetentionJob {
	if retentionDays <= 0 {
		retentionDays = DefaultRetentionDays
	}
	return func(ctx context.Context) error {
		cutoff := clk.Now().AddDate(0, 0, -retentionDays)
		if _, err := repo.PurgeOlderThan(ctx, cutoff); err != nil {
			return fmt.Errorf("audit: retention purge: %w", err)
		}
		return nil
	}
}
