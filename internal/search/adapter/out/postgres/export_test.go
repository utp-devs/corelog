package postgres

import (
	"context"

	"github.com/jmoiron/sqlx"
)

// SleepForTest runs a slow statement inside the read transaction of the repository, so tests can prove the
// statement timeout and its mapping without a slow user query.
func (r *IssueSearchRepository) SleepForTest(ctx context.Context, seconds float64) error {
	return r.readTx(ctx, func(tx *sqlx.Tx) error {
		_, err := tx.ExecContext(ctx, `SELECT pg_sleep($1)`, seconds)
		return err
	})
}
