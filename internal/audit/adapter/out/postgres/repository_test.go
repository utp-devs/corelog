package postgres_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"

	"github.com/jhonsferg/corelog/internal/audit/adapter/out/postgres"
	"github.com/jhonsferg/corelog/internal/audit/application/port"
	"github.com/jhonsferg/corelog/internal/audit/application/usecase"
	"github.com/jhonsferg/corelog/internal/audit/domain"
	"github.com/jhonsferg/corelog/internal/platform/clock"
	"github.com/jhonsferg/corelog/internal/platform/database/testdb"
	"github.com/jhonsferg/corelog/internal/platform/events"
)

func entryAt(org uuid.UUID, actor uuid.UUID, action string, at time.Time, source *int64) domain.Entry {
	return domain.Entry{
		OrganizationID: &org,
		ActorID:        &actor,
		Action:         action,
		EntityType:     "member",
		Summary:        "resumen " + action,
		Before:         map[string]any{"role": "viewer"},
		After:          map[string]any{"role": "admin"},
		IP:             "192.168.0.10",
		UserAgent:      "test-agent",
		SourceEventID:  source,
		CreatedAt:      at,
	}
}

func i64(v int64) *int64 { return &v }

func TestRepositorySaveRoundTripAndDuplicateSourceIgnored(t *testing.T) {
	db := testdb.New(t)
	repo := postgres.NewRepository(db)
	ctx := context.Background()
	org, actor := uuid.New(), uuid.New()
	at := time.Date(2026, 2, 3, 4, 5, 6, 0, time.UTC)

	if err := repo.Save(ctx, entryAt(org, actor, "member.role_changed", at, i64(11))); err != nil {
		t.Fatalf("Save: %v", err)
	}
	// Same source event delivered again: no error, no second row.
	if err := repo.Save(ctx, entryAt(org, actor, "member.role_changed", at, i64(11))); err != nil {
		t.Fatalf("duplicate Save: %v", err)
	}
	if n := countRows(t, db); n != 1 {
		t.Fatalf("rows = %d, want 1", n)
	}

	page, err := repo.List(ctx, port.ListFilter{OrganizationID: &org})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(page.Entries) != 1 {
		t.Fatalf("entries = %d, want 1", len(page.Entries))
	}
	got := page.Entries[0]
	if got.Summary != "resumen member.role_changed" || got.IP != "192.168.0.10" || got.UserAgent != "test-agent" {
		t.Fatalf("round trip lost fields: %+v", got)
	}
	if got.Before["role"] != "viewer" || got.After["role"] != "admin" {
		t.Fatalf("before/after = %v / %v", got.Before, got.After)
	}
	if got.SourceEventID == nil || *got.SourceEventID != 11 || !got.CreatedAt.Equal(at) {
		t.Fatalf("source/created_at = %v / %v", got.SourceEventID, got.CreatedAt)
	}
}

func TestRepositoryListFiltersAndCursorPaging(t *testing.T) {
	db := testdb.New(t)
	repo := postgres.NewRepository(db)
	ctx := context.Background()
	orgA, orgB := uuid.New(), uuid.New()
	ana, luis := uuid.New(), uuid.New()
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

	// Five entries for org A (alternating actors and actions), one for org B.
	for i := 0; i < 5; i++ {
		actor, action := ana, "project.created"
		if i%2 == 1 {
			actor, action = luis, "auth.login"
		}
		if err := repo.Save(ctx, entryAt(orgA, actor, action, base.AddDate(0, 0, i), nil)); err != nil {
			t.Fatalf("Save %d: %v", i, err)
		}
	}
	if err := repo.Save(ctx, entryAt(orgB, ana, "project.created", base, nil)); err != nil {
		t.Fatalf("Save org B: %v", err)
	}

	t.Run("org isolation", func(t *testing.T) {
		page, err := repo.List(ctx, port.ListFilter{OrganizationID: &orgB})
		if err != nil {
			t.Fatal(err)
		}
		if len(page.Entries) != 1 {
			t.Fatalf("org B entries = %d, want 1", len(page.Entries))
		}
	})

	t.Run("actor and action filters", func(t *testing.T) {
		page, err := repo.List(ctx, port.ListFilter{OrganizationID: &orgA, ActorID: &luis, Action: "auth.login"})
		if err != nil {
			t.Fatal(err)
		}
		if len(page.Entries) != 2 {
			t.Fatalf("entries = %d, want 2", len(page.Entries))
		}
		for _, e := range page.Entries {
			if e.Action != "auth.login" || *e.ActorID != luis {
				t.Fatalf("filter leaked %+v", e)
			}
		}
	})

	t.Run("date range", func(t *testing.T) {
		since := base.AddDate(0, 0, 1)
		until := base.AddDate(0, 0, 3)
		page, err := repo.List(ctx, port.ListFilter{OrganizationID: &orgA, Since: &since, Until: &until})
		if err != nil {
			t.Fatal(err)
		}
		if len(page.Entries) != 2 {
			t.Fatalf("entries in [day1, day3) = %d, want 2", len(page.Entries))
		}
	})

	t.Run("cursor paging newest first", func(t *testing.T) {
		var seen []int64
		filter := port.ListFilter{OrganizationID: &orgA, Limit: 2}
		for pages := 0; pages < 10; pages++ {
			page, err := repo.List(ctx, filter)
			if err != nil {
				t.Fatal(err)
			}
			for _, e := range page.Entries {
				seen = append(seen, e.ID)
			}
			if page.NextCursor == 0 {
				break
			}
			filter.Cursor = page.NextCursor
		}
		if len(seen) != 5 {
			t.Fatalf("paged ids = %v, want 5 entries", seen)
		}
		for i := 1; i < len(seen); i++ {
			if seen[i] >= seen[i-1] {
				t.Fatalf("ids not strictly descending: %v", seen)
			}
		}
	})
}

func TestRepositoryUpdateAndDeleteAreRejected(t *testing.T) {
	db := testdb.New(t)
	repo := postgres.NewRepository(db)
	ctx := context.Background()
	if err := repo.Save(ctx, entryAt(uuid.New(), uuid.New(), "auth.login", time.Now().UTC(), nil)); err != nil {
		t.Fatal(err)
	}

	if _, err := db.ExecContext(ctx, `UPDATE audit_events SET summary = 'changed'`); err == nil || !strings.Contains(err.Error(), "append-only") {
		t.Fatalf("UPDATE error = %v, want append-only rejection", err)
	}
	if _, err := db.ExecContext(ctx, `DELETE FROM audit_events`); err == nil || !strings.Contains(err.Error(), "append-only") {
		t.Fatalf("DELETE error = %v, want append-only rejection", err)
	}
	if _, err := db.ExecContext(ctx, `TRUNCATE audit_events`); err == nil || !strings.Contains(err.Error(), "append-only") {
		t.Fatalf("TRUNCATE error = %v, want append-only rejection", err)
	}
	if n := countRows(t, db); n != 1 {
		t.Fatalf("rows = %d after rejected statements, want 1", n)
	}
}

func TestRepositoryPurgeOlderThanDeletesOnlyOldRows(t *testing.T) {
	db := testdb.New(t)
	repo := postgres.NewRepository(db)
	ctx := context.Background()
	org, actor := uuid.New(), uuid.New()
	now := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)

	for _, at := range []time.Time{now.AddDate(-2, 0, 0), now.AddDate(0, -13, 0), now.AddDate(0, 0, -10), now} {
		if err := repo.Save(ctx, entryAt(org, actor, "auth.login", at, nil)); err != nil {
			t.Fatal(err)
		}
	}

	deleted, err := repo.PurgeOlderThan(ctx, now.AddDate(0, 0, -365))
	if err != nil {
		t.Fatalf("PurgeOlderThan: %v", err)
	}
	if deleted != 2 {
		t.Fatalf("deleted = %d, want 2", deleted)
	}
	if n := countRows(t, db); n != 2 {
		t.Fatalf("rows after purge = %d, want 2", n)
	}
	// The retention setting is transaction-local: ordinary deletes stay blocked.
	if _, err := db.ExecContext(ctx, `DELETE FROM audit_events`); err == nil {
		t.Fatal("DELETE allowed after purge transaction ended")
	}
}

func TestRetentionJobThroughRepository(t *testing.T) {
	db := testdb.New(t)
	repo := postgres.NewRepository(db)
	ctx := context.Background()
	now := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	if err := repo.Save(ctx, entryAt(uuid.New(), uuid.New(), "auth.login", now.AddDate(-1, -1, 0), nil)); err != nil {
		t.Fatal(err)
	}
	if err := repo.Save(ctx, entryAt(uuid.New(), uuid.New(), "auth.login", now, nil)); err != nil {
		t.Fatal(err)
	}
	job := usecase.NewRetentionJob(repo, 365, clock.NewFixed(now))
	if err := job(ctx); err != nil {
		t.Fatalf("retention job: %v", err)
	}
	if n := countRows(t, db); n != 1 {
		t.Fatalf("rows = %d, want 1", n)
	}
}

// TestDispatcherStoresAuditEntries publishes events inside a transaction, runs
// the real dispatcher with the audit consumer and checks the stored entries.
func TestDispatcherStoresAuditEntries(t *testing.T) {
	db := testdb.New(t)
	repo := postgres.NewRepository(db)
	ctx := context.Background()
	org, actor := uuid.New(), uuid.New()

	tx, err := db.BeginTxx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	pub := events.NewPostgresPublisher(func(context.Context) sqlx.ExtContext { return tx })
	for _, e := range []events.Event{
		{OrgID: &org, ActorID: &actor, Type: "org.created", EntityType: "organization",
			Payload: map[string]any{"actor_name": "Ana", "name": "Acme", "ip": "10.0.0.5", "user_agent": "ua"}},
		{OrgID: &org, ActorID: &actor, Type: "board.column_resized", EntityType: "board", Payload: map[string]any{}},
		{OrgID: &org, ActorID: &actor, Type: "member.role_changed", EntityType: "member",
			Payload: map[string]any{"member_name": "Luis", "role": "admin",
				"before": map[string]any{"role": "viewer"}, "after": map[string]any{"role": "admin"}}},
	} {
		if err := pub.Publish(ctx, e); err != nil {
			t.Fatalf("publish %s: %v", e.Type, err)
		}
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}

	consumer := usecase.NewConsumer(repo, clock.RealClock{})
	d, err := events.NewDispatcher(db, []events.Consumer{consumer}, events.Options{})
	if err != nil {
		t.Fatal(err)
	}
	if err := d.RunOnce(ctx); err != nil {
		t.Fatalf("RunOnce: %v", err)
	}
	// A second run must not duplicate entries.
	if err := d.RunOnce(ctx); err != nil {
		t.Fatalf("second RunOnce: %v", err)
	}

	page, err := repo.List(ctx, port.ListFilter{OrganizationID: &org})
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Entries) != 2 {
		t.Fatalf("audit entries = %d, want 2 (board.column_resized is not audited)", len(page.Entries))
	}
	byAction := map[string]domain.Entry{}
	for _, e := range page.Entries {
		byAction[e.Action] = e
	}
	created, ok := byAction["org.created"]
	if !ok || created.Summary != "Ana creó la organización \"Acme\"" || created.IP != "10.0.0.0" || created.UserAgent != "ua" {
		t.Fatalf("org.created entry = %+v", created)
	}
	changed, ok := byAction["member.role_changed"]
	if !ok || changed.Before["role"] != "viewer" || changed.After["role"] != "admin" || changed.Summary != "Un usuario cambió el rol de Luis a admin" {
		t.Fatalf("member.role_changed entry = %+v", changed)
	}
}

func countRows(t *testing.T, db *sqlx.DB) int {
	t.Helper()
	var n int
	if err := db.QueryRowContext(context.Background(), `SELECT count(*) FROM audit_events`).Scan(&n); err != nil {
		t.Fatalf("count: %v", err)
	}
	return n
}
