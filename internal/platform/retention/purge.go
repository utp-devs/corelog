// Package retention permanently removes records that were soft-deleted long ago.
package retention

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jmoiron/sqlx"

	"github.com/jhonsferg/corelog/internal/platform/clock"
)

// foreignKeyViolation is the SQLSTATE of a delete blocked by a referencing row.
const foreignKeyViolation = "23503"

// Result counts what a purge run found and did.
type Result struct {
	// IssuesFound and ProjectsFound are the soft-deleted rows older than the retention.
	IssuesFound   int
	ProjectsFound int
	// IssuesPurged and ProjectsPurged are the rows removed (always 0 in a dry run).
	IssuesPurged   int
	ProjectsPurged int
	// Blocked rows are kept because other rows still reference them.
	Blocked int
}

// Purger deletes issues and projects that were soft-deleted before the retention period.
type Purger struct {
	db        *sqlx.DB
	clock     clock.Clock
	retention time.Duration
	dryRun    bool
	batch     int
}

// NewPurger returns a purger. With dryRun it only counts. A zero retention means 30 days.
func NewPurger(db *sqlx.DB, clk clock.Clock, retention time.Duration, dryRun bool) *Purger {
	if retention <= 0 {
		retention = 30 * 24 * time.Hour
	}
	return &Purger{db: db, clock: clk, retention: retention, dryRun: dryRun, batch: 200}
}

// Run purges one batch of issues and then of projects. Every row is deleted on its own: a row that other tables
// still reference is counted as blocked and left alone. A project is only removed once it has no issues left.
func (p *Purger) Run(ctx context.Context) (Result, error) {
	cutoff := p.clock.Now().Add(-p.retention)
	var res Result

	var issueIDs []string
	if err := p.db.SelectContext(ctx, &issueIDs, `
		SELECT id::text FROM issues WHERE deleted_at IS NOT NULL AND deleted_at < $1 ORDER BY deleted_at LIMIT $2`, cutoff, p.batch); err != nil {
		return res, err
	}
	res.IssuesFound = len(issueIDs)
	if !p.dryRun {
		for _, id := range issueIDs {
			purged, err := p.deleteOne(ctx, `DELETE FROM issues WHERE id = $1::uuid AND deleted_at IS NOT NULL AND deleted_at < $2`, id, cutoff)
			if err != nil {
				return res, err
			}
			if purged {
				res.IssuesPurged++
			} else {
				res.Blocked++
			}
		}
	}

	var projectIDs []string
	if err := p.db.SelectContext(ctx, &projectIDs, `
		SELECT p.id::text FROM projects p
		WHERE p.deleted_at IS NOT NULL AND p.deleted_at < $1
		  AND NOT EXISTS (SELECT 1 FROM issues i WHERE i.project_id = p.id)
		ORDER BY p.deleted_at LIMIT $2`, cutoff, p.batch); err != nil {
		return res, err
	}
	res.ProjectsFound = len(projectIDs)
	if !p.dryRun {
		for _, id := range projectIDs {
			purged, err := p.deleteOne(ctx, `DELETE FROM projects WHERE id = $1::uuid AND deleted_at IS NOT NULL AND deleted_at < $2`, id, cutoff)
			if err != nil {
				return res, err
			}
			if purged {
				res.ProjectsPurged++
			} else {
				res.Blocked++
			}
		}
	}
	return res, nil
}

// deleteOne removes one row and reports false when a foreign key keeps it (or it changed meanwhile).
func (p *Purger) deleteOne(ctx context.Context, query, id string, cutoff time.Time) (bool, error) {
	result, err := p.db.ExecContext(ctx, query, id, cutoff)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == foreignKeyViolation {
			return false, nil
		}
		return false, err
	}
	n, _ := result.RowsAffected()
	return n > 0, nil
}
