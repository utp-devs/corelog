package retention

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/jhonsferg/corelog/internal/platform/clock"
)

func (f *fixture) lifecycle(dryRun bool, dir string) *Lifecycle {
	return NewLifecycle(f.db, clock.NewFixed(now), Options{DryRun: dryRun, AttachmentsDir: dir, DeleteOrphans: true})
}

func ago(days int) time.Time { return now.AddDate(0, 0, -days) }

func (f *fixture) label(project uuid.UUID, name string, deletedDaysAgo int) {
	f.exec(`INSERT INTO labels (project_id, name, deleted_at) VALUES ($1, $2, $3)`, project, name, nullableTime(deletedDaysAgo))
}

func nullableTime(daysAgo int) any {
	if daysAgo == 0 {
		return nil
	}
	return ago(daysAgo)
}

func (f *fixture) attachment(issue uuid.UUID, deletedDaysAgo int, dir string) uuid.UUID {
	key := uuid.New()
	f.exec(`INSERT INTO attachments (issue_id, uploader_id, file_name, content_type, size_bytes, storage_key, deleted_at) VALUES ($1, $2, 'a.txt', 'text/plain', 3, $3, $4)`,
		issue, f.user, key, nullableTime(deletedDaysAgo))
	if dir != "" {
		if err := os.WriteFile(filepath.Join(dir, key.String()), []byte("abc"), 0o600); err != nil {
			f.t.Fatal(err)
		}
	}
	return key
}

func (f *fixture) removed(rep Report, name string) int {
	for _, c := range rep.Counts {
		if c.Name == name {
			return c.Removed
		}
	}
	f.t.Fatalf("no count %s in %+v", name, rep.Counts)
	return 0
}

func TestPurgeSoftDeletedRemovesOldRowsPerTable(t *testing.T) {
	f := newFixture(t)
	project := f.project("LIV", 0)
	f.label(project, "old", 40)
	f.label(project, "recent", 5)
	f.label(project, "live", 0)
	f.exec(`INSERT INTO notes (owner_id, title, deleted_at) VALUES ($1, 'old', $2), ($1, 'live', NULL), ($1, 'recent', $3)`, f.user, ago(31), ago(2))
	f.exec(`INSERT INTO teams (name, organization_id, deleted_at) VALUES ('old', $1, $2), ('live', $1, NULL)`, orgID, ago(60))
	f.exec(`INSERT INTO sprints (project_id, name, deleted_at) VALUES ($1, 'old', $2)`, project, ago(60))

	rep, err := f.lifecycle(false, "").PurgeSoftDeleted(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if f.removed(rep, "labels") != 1 || f.removed(rep, "notes") != 1 || f.removed(rep, "teams") != 1 || f.removed(rep, "sprints") != 1 {
		t.Fatalf("unexpected report %+v", rep)
	}
	if f.count("labels") != 2 || f.count("notes") != 2 || f.count("teams") != 1 {
		t.Fatal("rows that are live or recent must stay")
	}
	if !strings.Contains(rep.Summary(), "labels=1") {
		t.Fatalf("summary %q", rep.Summary())
	}
	// Idempotent: a second run finds nothing.
	rep, err = f.lifecycle(false, "").PurgeSoftDeleted(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if found, removed := rep.Total(); found != 0 || removed != 0 {
		t.Fatalf("second run found %d removed %d", found, removed)
	}
	if rep.Summary() != "nothing to remove" {
		t.Fatalf("summary %q", rep.Summary())
	}
}

func TestPurgeSoftDeletedKeepsReferencedRows(t *testing.T) {
	f := newFixture(t)
	project := f.project("LIV", 0)
	issue := f.issue(project, 1, 0)
	// A deleted priority that a live issue still uses.
	var usedPriority uuid.UUID
	if err := f.db.Get(&usedPriority, `SELECT priority_id FROM issues WHERE id = $1`, issue); err != nil {
		t.Fatal(err)
	}
	f.exec(`UPDATE priorities SET deleted_at = $2 WHERE id = $1`, usedPriority, ago(90))
	f.exec(`INSERT INTO priorities (organization_id, name, deleted_at) VALUES ($1, 'spare', $2)`, orgID, ago(90))
	// A deleted sprint that an issue still points to, and one nobody uses.
	var used, spare uuid.UUID
	f.exec(`INSERT INTO sprints (id, project_id, name, deleted_at) VALUES ($1, $2, 'used', $3), ($4, $2, 'spare', $3)`, uuid.New(), project, ago(90), uuid.New())
	if err := f.db.Get(&used, `SELECT id FROM sprints WHERE name = 'used'`); err != nil {
		t.Fatal(err)
	}
	if err := f.db.Get(&spare, `SELECT id FROM sprints WHERE name = 'spare'`); err != nil {
		t.Fatal(err)
	}
	f.exec(`UPDATE issues SET sprint_id = $2 WHERE id = $1`, issue, used)
	// A deleted board that a sprint still references.
	f.exec(`INSERT INTO boards (project_id, name, type, deleted_at) VALUES ($1, 'b', 'scrum', $2)`, project, ago(90))
	f.exec(`UPDATE sprints SET board_id = (SELECT id FROM boards WHERE name = 'b') WHERE id = $1`, spare)

	rep, err := f.lifecycle(false, "").PurgeSoftDeleted(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if f.removed(rep, "priorities") != 1 || f.removed(rep, "sprints") != 1 {
		t.Fatalf("unexpected report %+v", rep)
	}
	var n int
	_ = f.db.Get(&n, `SELECT count(*) FROM priorities WHERE id = $1`, usedPriority)
	if n != 1 {
		t.Fatal("the priority used by an issue was removed")
	}
	_ = f.db.Get(&n, `SELECT count(*) FROM sprints WHERE id = $1`, used)
	if n != 1 {
		t.Fatal("the sprint used by an issue was removed")
	}
	// Children go first, so the board is removed in the same run once its sprint is gone.
	if f.removed(rep, "boards") != 1 {
		t.Fatalf("the board must go with its sprint: %+v", rep)
	}
}

func TestPurgeSoftDeletedRemovesAttachmentFiles(t *testing.T) {
	f := newFixture(t)
	dir := t.TempDir()
	project := f.project("LIV", 0)
	issue := f.issue(project, 1, 0)
	gone := f.attachment(issue, 40, dir)
	missing := f.attachment(issue, 40, "")
	kept := f.attachment(issue, 3, dir)
	live := f.attachment(issue, 0, dir)

	rep, err := f.lifecycle(false, dir).PurgeSoftDeleted(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if f.removed(rep, "attachments") != 2 || f.removed(rep, "attachment_files") != 1 {
		t.Fatalf("unexpected report %+v", rep)
	}
	if _, err := os.Stat(filepath.Join(dir, gone.String())); !os.IsNotExist(err) {
		t.Fatal("the file of a purged attachment must be removed")
	}
	if _, err := os.Stat(filepath.Join(dir, kept.String())); err != nil {
		t.Fatal("a recent soft-deleted attachment keeps its file")
	}
	if _, err := os.Stat(filepath.Join(dir, live.String())); err != nil {
		t.Fatal("a live attachment keeps its file")
	}
	_ = missing
	if f.count("attachments") != 2 {
		t.Fatalf("attachments left: %d", f.count("attachments"))
	}
}

func TestDryRunDeletesNothing(t *testing.T) {
	f := newFixture(t)
	dir := t.TempDir()
	project := f.project("LIV", 0)
	issue := f.issue(project, 1, 0)
	f.label(project, "old", 40)
	key := f.attachment(issue, 40, dir)
	f.exec(`INSERT INTO login_fingerprints (user_id, fingerprint, first_seen_at, last_seen_at) VALUES ($1, repeat('a', 64), $2, $2)`, f.user, ago(200))
	orphan := uuid.New().String()
	if err := os.WriteFile(filepath.Join(dir, orphan), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(filepath.Join(dir, orphan), ago(3), ago(3)); err != nil {
		t.Fatal(err)
	}
	life := f.lifecycle(true, dir)
	soft, err := life.PurgeSoftDeleted(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	pruned, err := life.Prune(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	orphans, err := life.CheckOrphanFiles(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if found, removed := soft.Total(); found < 2 || removed != 0 {
		t.Fatalf("soft-deleted dry run: found %d removed %d", found, removed)
	}
	if found, removed := pruned.Total(); found != 1 || removed != 0 {
		t.Fatalf("prune dry run: found %d removed %d", found, removed)
	}
	if found, removed := orphans.Total(); found != 1 || removed != 0 {
		t.Fatalf("orphan dry run: found %d removed %d", found, removed)
	}
	if f.count("labels") != 1 || f.count("attachments") != 1 || f.count("login_fingerprints") != 1 {
		t.Fatal("a dry run deleted rows")
	}
	for _, name := range []string{key.String(), orphan} {
		if _, err := os.Stat(filepath.Join(dir, name)); err != nil {
			t.Fatalf("a dry run deleted file %s", name)
		}
	}
	if !strings.HasPrefix(soft.Summary(), "dry-run ") {
		t.Fatalf("summary %q", soft.Summary())
	}
}

func TestPruneRemovesStaleBookkeepingRows(t *testing.T) {
	f := newFixture(t)
	f.exec(`INSERT INTO login_fingerprints (user_id, fingerprint, first_seen_at, last_seen_at) VALUES
		($1, repeat('a', 64), $2, $2), ($1, repeat('b', 64), $3, $3)`, f.user, ago(100), ago(10))
	other := uuid.New()
	f.exec(`INSERT INTO users (id, name, display_name, email, password_hash, role) VALUES ($1, 'o', 'o', 'o@example.test', 'x', 'user')`, other)
	f.exec(`INSERT INTO password_reset_tokens (user_id, token_hash, expires_at, used_at) VALUES
		($1, 'old', $2, NULL), ($3, 'used', $4, $5), ($3, 'live', $6, NULL)`, other, now.Add(-48*time.Hour), f.user, now.Add(48*time.Hour), now.Add(-48*time.Hour), now.Add(time.Hour))
	f.exec(`INSERT INTO user_sessions (id, user_id, created_at, last_seen_at, expires_at, revoked_at) VALUES
		($1, $5, $2, $2, $3, NULL), ($4, $5, $2, $2, $6, NULL), ($7, $5, $2, $2, $6, $8)`,
		uuid.New(), ago(60), ago(20), uuid.New(), f.user, now.Add(24*time.Hour), uuid.New(), ago(20))
	f.exec(`INSERT INTO organization_invitations (organization_id, email, role, token_hash, expires_at, accepted_at, revoked_at) VALUES
		($1, 'a@example.test', 'member', 'h1', $2, NULL, NULL),
		($1, 'b@example.test', 'member', 'h2', $3, $2, NULL),
		($1, 'c@example.test', 'member', 'h3', $3, NULL, $2),
		($1, 'd@example.test', 'member', 'h4', $3, NULL, NULL)`, orgID, ago(40), now.Add(72*time.Hour))
	f.exec(`INSERT INTO notifications (user_id, kind, message, read_at) VALUES
		($1, 'k', 'old read', $2), ($1, 'k', 'recent read', $3), ($1, 'k', 'old unread', NULL)`, f.user, ago(200), ago(10))

	rep, err := f.lifecycle(false, "").Prune(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]int{"login_fingerprints": 1, "password_reset_tokens": 2, "user_sessions": 2, "organization_invitations": 2, "read_notifications": 1}
	for name, n := range want {
		if got := f.removed(rep, name); got != n {
			t.Errorf("%s removed %d, want %d", name, got, n)
		}
	}
	if f.count("notifications") != 2 || f.count("organization_invitations") != 2 {
		t.Fatal("unread notifications and accepted or pending invitations must stay")
	}
	rep, _ = f.lifecycle(false, "").Prune(context.Background())
	if _, removed := rep.Total(); removed != 0 {
		t.Fatal("second run must be a no-op")
	}
}

func TestOrphanFilesAreReportedAndDeletedOnlyOnRequest(t *testing.T) {
	f := newFixture(t)
	dir := t.TempDir()
	project := f.project("LIV", 0)
	issue := f.issue(project, 1, 0)
	referenced := f.attachment(issue, 0, dir)
	old := func(name string, age time.Duration) {
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, []byte("x"), 0o600); err != nil {
			t.Fatal(err)
		}
		if err := os.Chtimes(path, now.Add(-age), now.Add(-age)); err != nil {
			t.Fatal(err)
		}
	}
	orphan, fresh := uuid.New().String(), uuid.New().String()
	old(orphan, 48*time.Hour)
	old(fresh, time.Hour)
	old("notes.txt", 100*time.Hour)
	old(referenced.String(), 100*time.Hour)

	report := NewLifecycle(f.db, clock.NewFixed(now), Options{AttachmentsDir: dir})
	rep, err := report.CheckOrphanFiles(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if found, removed := rep.Total(); found != 1 || removed != 0 {
		t.Fatalf("report only: found %d removed %d", found, removed)
	}
	rep, err = f.lifecycle(false, dir).CheckOrphanFiles(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if found, removed := rep.Total(); found != 1 || removed != 1 {
		t.Fatalf("delete: found %d removed %d", found, removed)
	}
	for name, exists := range map[string]bool{orphan: false, fresh: true, "notes.txt": true, referenced.String(): true} {
		_, err := os.Stat(filepath.Join(dir, name))
		if (err == nil) != exists {
			t.Errorf("file %s exists=%v, want %v", name, err == nil, exists)
		}
	}
}

func TestLifecycleJobRunsOnlyOnceAtATime(t *testing.T) {
	f := newFixture(t)
	project := f.project("LIV", 0)
	f.label(project, "old", 40)
	conn, err := f.db.Connx(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	var got bool
	if err := conn.GetContext(context.Background(), &got, `SELECT pg_try_advisory_lock($1)`, lockSoftDeleted); err != nil || !got {
		t.Fatalf("lock not taken: %v", err)
	}
	rep, err := f.lifecycle(false, "").PurgeSoftDeleted(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !rep.Skipped || f.count("labels") != 1 {
		t.Fatalf("a concurrent run must be skipped without deleting: %+v", rep)
	}
	if _, err := conn.ExecContext(context.Background(), `SELECT pg_advisory_unlock($1)`, lockSoftDeleted); err != nil {
		t.Fatal(err)
	}
	rep, err = f.lifecycle(false, "").PurgeSoftDeleted(context.Background())
	if err != nil || rep.Skipped || f.count("labels") != 0 {
		t.Fatalf("after release the job must run: %+v %v", rep, err)
	}
}
