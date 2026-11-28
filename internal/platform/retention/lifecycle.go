package retention

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jmoiron/sqlx"

	"github.com/jhonsferg/corelog/internal/platform/clock"
)

// Advisory lock keys: one per job, so two API instances never run the same lifecycle job at the same time even
// when the scheduler leadership briefly overlaps.
const (
	lockSoftDeleted int64 = 0x636c726574_01
	lockHousekeep   int64 = 0x636c726574_02
	lockOrphans     int64 = 0x636c726574_03
)

const (
	// orphanGrace is how old a file must be before it can count as an orphan (an upload writes the file first).
	orphanGrace = 24 * time.Hour
	// sessionGrace is how long an expired or revoked session row is kept.
	sessionGrace = 7 * 24 * time.Hour
	// tokenGrace is how long an expired or used reset or verification token is kept.
	tokenGrace = 24 * time.Hour
	// maxBatches bounds the work of one run per table.
	maxBatches = 50
)

// Options configures the lifecycle jobs.
type Options struct {
	// SoftDeletedAfter is how long logically deleted rows are kept. Zero means 30 days.
	SoftDeletedAfter time.Duration
	// ReadNotificationsAfter is how long read notifications are kept. Zero means 180 days.
	ReadNotificationsAfter time.Duration
	// FingerprintsAfter is how long a sign-in fingerprint is kept after its last use. Zero means 90 days.
	FingerprintsAfter time.Duration
	// DryRun counts what would go away and deletes nothing.
	DryRun bool
	// AttachmentsDir is the attachment file store. Empty disables the file removal and the orphan check.
	AttachmentsDir string
	// DeleteOrphans lets the orphan check delete the files it finds. Otherwise it only reports them.
	DeleteOrphans bool
	// BatchSize is the number of rows one delete statement removes. Zero means 500.
	BatchSize int
}

func (o Options) withDefaults() Options {
	day := 24 * time.Hour
	if o.SoftDeletedAfter <= 0 {
		o.SoftDeletedAfter = 30 * day
	}
	if o.ReadNotificationsAfter <= 0 {
		o.ReadNotificationsAfter = 180 * day
	}
	if o.FingerprintsAfter <= 0 {
		o.FingerprintsAfter = 90 * day
	}
	if o.BatchSize <= 0 {
		o.BatchSize = 500
	}
	return o
}

// Count is what one lifecycle step found and removed.
type Count struct {
	Name string
	// Found is the number of eligible rows (or files).
	Found int
	// Removed is the number actually deleted (always 0 in a dry run).
	Removed int
	// Blocked is the number kept because other rows still reference them.
	Blocked int
}

// Report is the outcome of one lifecycle job run.
type Report struct {
	DryRun bool
	// Skipped is true when another instance already holds the job lock.
	Skipped bool
	Counts  []Count
}

// Total returns the sum of found and removed rows.
func (r Report) Total() (found, removed int) {
	for _, c := range r.Counts {
		found += c.Found
		removed += c.Removed
	}
	return found, removed
}

// Summary is a one-line description for the jobs console, for example "dry-run labels=2 sprints=1".
func (r Report) Summary() string {
	if r.Skipped {
		return "skipped: another instance is running this job"
	}
	var parts []string
	for _, c := range r.Counts {
		n := c.Removed
		if r.DryRun {
			n = c.Found
		}
		if n == 0 && c.Blocked == 0 {
			continue
		}
		part := fmt.Sprintf("%s=%d", c.Name, n)
		if c.Blocked > 0 {
			part += fmt.Sprintf("(blocked %d)", c.Blocked)
		}
		parts = append(parts, part)
	}
	if len(parts) == 0 {
		parts = append(parts, "nothing to remove")
	}
	if r.DryRun {
		parts = append([]string{"dry-run"}, parts...)
	}
	return strings.Join(parts, " ")
}

// Lifecycle removes old data: logically deleted rows, stale bookkeeping rows and orphan attachment files.
type Lifecycle struct {
	db    *sqlx.DB
	clock clock.Clock
	opts  Options
}

// NewLifecycle returns the lifecycle jobs.
func NewLifecycle(db *sqlx.DB, clk clock.Clock, opts Options) *Lifecycle {
	return &Lifecycle{db: db, clock: clk, opts: opts.withDefaults()}
}

// softTable is a table with logical deletion. keep is a condition (alias t is the table) that, when true, means
// the row must stay because live data still points to it.
type softTable struct {
	name string
	keep string
}

// softTables is ordered children first. A row that live data still points to is kept.
var softTables = []softTable{
	{name: "issue_links"},
	{name: "automation_rules"},
	{name: "webhooks"},
	{name: "api_tokens"},
	{name: "saved_filters"},
	{name: "filters"},
	{name: "dashboards"},
	{name: "custom_fields"},
	{name: "labels"},
	{name: "components"},
	{name: "versions"},
	{name: "sprints", keep: `EXISTS (SELECT 1 FROM issues i WHERE i.sprint_id = t.id)`},
	{name: "boards", keep: `EXISTS (SELECT 1 FROM sprints s WHERE s.board_id = t.id)`},
	{name: "teams"},
	{name: "issue_types", keep: `EXISTS (SELECT 1 FROM issues i WHERE i.issue_type_id = t.id)`},
	{name: "priorities", keep: `EXISTS (SELECT 1 FROM issues i WHERE i.priority_id = t.id)`},
	{name: "resolutions", keep: `EXISTS (SELECT 1 FROM issues i WHERE i.resolution_id = t.id)`},
	{name: "notes"},
	{name: "organizations", keep: `EXISTS (SELECT 1 FROM projects p WHERE p.organization_id = t.id)`},
}

// PurgeSoftDeleted removes the logically deleted rows older than the retention window from every soft-delete
// table, and the stored file of every removed attachment. Issues and projects are handled by Purger.
func (l *Lifecycle) PurgeSoftDeleted(ctx context.Context) (Report, error) {
	return l.locked(ctx, lockSoftDeleted, func(ctx context.Context, rep *Report) error {
		cutoff := l.clock.Now().Add(-l.opts.SoftDeletedAfter)
		attachments, err := l.purgeAttachments(ctx, cutoff)
		rep.Counts = append(rep.Counts, attachments...)
		if err != nil {
			return err
		}
		for _, table := range softTables {
			count, err := l.purgeTable(ctx, table, cutoff)
			rep.Counts = append(rep.Counts, count)
			if err != nil {
				return err
			}
		}
		return nil
	})
}

func (t softTable) where() string {
	cond := `t.deleted_at IS NOT NULL AND t.deleted_at < $1`
	if t.keep != "" {
		cond += ` AND NOT ` + t.keep
	}
	return cond
}

func (l *Lifecycle) purgeTable(ctx context.Context, table softTable, cutoff time.Time) (Count, error) {
	count := Count{Name: table.name}
	if err := l.db.GetContext(ctx, &count.Found, `SELECT count(*) FROM `+table.name+` t WHERE `+table.where(), cutoff); err != nil {
		return count, fmt.Errorf("retention: count %s: %w", table.name, err)
	}
	if l.opts.DryRun || count.Found == 0 {
		return count, nil
	}
	query := `DELETE FROM ` + table.name + ` WHERE id IN (SELECT t.id FROM ` + table.name + ` t WHERE ` + table.where() +
		` ORDER BY t.deleted_at LIMIT $2)`
	for i := 0; i < maxBatches; i++ {
		res, err := l.db.ExecContext(ctx, query, cutoff, l.opts.BatchSize)
		if err != nil {
			if isForeignKeyViolation(err) {
				// A reference appeared after the count: keep the rest for a later run.
				count.Blocked = count.Found - count.Removed
				return count, nil
			}
			return count, fmt.Errorf("retention: purge %s: %w", table.name, err)
		}
		n, _ := res.RowsAffected()
		count.Removed += int(n)
		if int(n) < l.opts.BatchSize {
			break
		}
	}
	return count, nil
}

// purgeAttachments deletes the attachment rows and then their files. The row goes first: a file that cannot be
// removed becomes an orphan that the orphan check reports, never a row that points to a missing file.
func (l *Lifecycle) purgeAttachments(ctx context.Context, cutoff time.Time) ([]Count, error) {
	rows := Count{Name: "attachments"}
	files := Count{Name: "attachment_files"}
	out := func() []Count { return []Count{rows, files} }
	where := `t.deleted_at IS NOT NULL AND t.deleted_at < $1`
	if err := l.db.GetContext(ctx, &rows.Found, `SELECT count(*) FROM attachments t WHERE `+where, cutoff); err != nil {
		return out(), fmt.Errorf("retention: count attachments: %w", err)
	}
	files.Found = rows.Found
	if l.opts.DryRun || rows.Found == 0 {
		return out(), nil
	}
	query := `DELETE FROM attachments WHERE id IN (SELECT t.id FROM attachments t WHERE ` + where + ` ORDER BY t.deleted_at LIMIT $2) RETURNING storage_key::text`
	for i := 0; i < maxBatches; i++ {
		var keys []string
		if err := l.db.SelectContext(ctx, &keys, query, cutoff, l.opts.BatchSize); err != nil {
			return out(), fmt.Errorf("retention: purge attachments: %w", err)
		}
		rows.Removed += len(keys)
		for _, key := range keys {
			if l.removeFile(key) {
				files.Removed++
			}
		}
		if len(keys) < l.opts.BatchSize {
			break
		}
	}
	return out(), nil
}

// removeFile deletes one stored file. A missing file is not an error. It reports whether a file was removed.
func (l *Lifecycle) removeFile(key string) bool {
	if l.opts.AttachmentsDir == "" {
		return false
	}
	if _, err := uuid.Parse(key); err != nil {
		return false
	}
	return os.Remove(filepath.Join(l.opts.AttachmentsDir, key)) == nil
}

// pruneRule removes bookkeeping rows. where uses alias t and $1 as the cutoff.
type pruneRule struct {
	name  string
	table string
	where string
	age   func(o Options) time.Duration
}

var pruneRules = []pruneRule{
	{"login_fingerprints", "login_fingerprints", `t.last_seen_at < $1`, func(o Options) time.Duration { return o.FingerprintsAfter }},
	{"password_reset_tokens", "password_reset_tokens", `t.expires_at < $1 OR (t.used_at IS NOT NULL AND t.used_at < $1)`, func(Options) time.Duration { return tokenGrace }},
	{"email_verification_tokens", "email_verification_tokens", `t.expires_at < $1 OR (t.used_at IS NOT NULL AND t.used_at < $1)`, func(Options) time.Duration { return tokenGrace }},
	{"user_sessions", "user_sessions", `t.expires_at < $1 OR (t.revoked_at IS NOT NULL AND t.revoked_at < $1)`, func(Options) time.Duration { return sessionGrace }},
	// Accepted invitations stay as history; revoked ones and ones that expired unanswered go away.
	{"organization_invitations", "organization_invitations", `(t.revoked_at IS NOT NULL AND t.revoked_at < $1) OR (t.accepted_at IS NULL AND t.expires_at < $1)`, func(o Options) time.Duration { return o.SoftDeletedAfter }},
	{"read_notifications", "notifications", `t.read_at IS NOT NULL AND t.read_at < $1`, func(o Options) time.Duration { return o.ReadNotificationsAfter }},
}

// Prune removes the stale bookkeeping rows: sign-in fingerprints, used or expired reset and verification tokens,
// expired or revoked sessions, expired or revoked invitations and old read notifications.
func (l *Lifecycle) Prune(ctx context.Context) (Report, error) {
	return l.locked(ctx, lockHousekeep, func(ctx context.Context, rep *Report) error {
		now := l.clock.Now()
		for _, rule := range pruneRules {
			cutoff := now.Add(-rule.age(l.opts))
			count := Count{Name: rule.name}
			where := `(` + rule.where + `)`
			if err := l.db.GetContext(ctx, &count.Found, `SELECT count(*) FROM `+rule.table+` t WHERE `+where, cutoff); err != nil {
				return fmt.Errorf("retention: count %s: %w", rule.name, err)
			}
			if !l.opts.DryRun && count.Found > 0 {
				query := `DELETE FROM ` + rule.table + ` WHERE ctid IN (SELECT t.ctid FROM ` + rule.table + ` t WHERE ` + where + ` LIMIT $2)`
				for i := 0; i < maxBatches; i++ {
					res, err := l.db.ExecContext(ctx, query, cutoff, l.opts.BatchSize)
					if err != nil {
						return fmt.Errorf("retention: prune %s: %w", rule.name, err)
					}
					n, _ := res.RowsAffected()
					count.Removed += int(n)
					if int(n) < l.opts.BatchSize {
						break
					}
				}
			}
			rep.Counts = append(rep.Counts, count)
		}
		return nil
	})
}

// CheckOrphanFiles finds the stored files that no attachment row points to and that are older than 24 hours. It
// deletes them only when DeleteOrphans is set and the run is not a dry run. Files with other names are ignored.
func (l *Lifecycle) CheckOrphanFiles(ctx context.Context) (Report, error) {
	return l.locked(ctx, lockOrphans, func(ctx context.Context, rep *Report) error {
		count := Count{Name: "orphan_files"}
		defer func() { rep.Counts = append(rep.Counts, count) }()
		if l.opts.AttachmentsDir == "" {
			return nil
		}
		entries, err := os.ReadDir(l.opts.AttachmentsDir)
		if err != nil {
			if os.IsNotExist(err) {
				return nil
			}
			return fmt.Errorf("retention: read attachments dir: %w", err)
		}
		limit := l.clock.Now().Add(-orphanGrace)
		var candidates []string
		for _, entry := range entries {
			if entry.IsDir() {
				continue
			}
			if _, err := uuid.Parse(entry.Name()); err != nil {
				continue
			}
			info, err := entry.Info()
			if err != nil || !info.ModTime().Before(limit) {
				continue
			}
			candidates = append(candidates, entry.Name())
		}
		for start := 0; start < len(candidates); start += l.opts.BatchSize {
			end := min(start+l.opts.BatchSize, len(candidates))
			chunk := candidates[start:end]
			var known []string
			if err := l.db.SelectContext(ctx, &known, `SELECT storage_key::text FROM attachments WHERE storage_key::text = ANY($1::text[])`, chunk); err != nil {
				return fmt.Errorf("retention: look up attachment keys: %w", err)
			}
			referenced := make(map[string]struct{}, len(known))
			for _, k := range known {
				referenced[k] = struct{}{}
			}
			for _, name := range chunk {
				if _, ok := referenced[name]; ok {
					continue
				}
				count.Found++
				if l.opts.DeleteOrphans && !l.opts.DryRun && l.removeFile(name) {
					count.Removed++
				}
			}
		}
		return nil
	})
}

// locked runs fn while holding a session level advisory lock. When another instance holds it the report is marked
// as skipped and fn does not run.
func (l *Lifecycle) locked(ctx context.Context, key int64, fn func(ctx context.Context, rep *Report) error) (Report, error) {
	rep := Report{DryRun: l.opts.DryRun}
	conn, err := l.db.Connx(ctx)
	if err != nil {
		return rep, err
	}
	defer conn.Close()
	var got bool
	if err := conn.GetContext(ctx, &got, `SELECT pg_try_advisory_lock($1)`, key); err != nil {
		return rep, err
	}
	if !got {
		rep.Skipped = true
		return rep, nil
	}
	defer func() {
		// The context may already be cancelled: unlock on a fresh one.
		unlockCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_, _ = conn.ExecContext(unlockCtx, `SELECT pg_advisory_unlock($1)`, key)
	}()
	return rep, fn(ctx, &rep)
}

func isForeignKeyViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == foreignKeyViolation
}
