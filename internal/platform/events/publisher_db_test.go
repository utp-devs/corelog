package events

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"

	"github.com/jhonsferg/corelog/internal/platform/database/testdb"
)

func countEvents(t *testing.T, db *sqlx.DB) int {
	t.Helper()
	var n int
	if err := db.QueryRowContext(context.Background(), `SELECT count(*) FROM domain_events`).Scan(&n); err != nil {
		t.Fatalf("count events: %v", err)
	}
	return n
}

func TestPublisherRowFollowsTransactionOutcome(t *testing.T) {
	db := testdb.New(t)
	ctx := context.Background()
	orgID, actorID := uuid.New(), uuid.New()

	tx, err := db.Beginx()
	if err != nil {
		t.Fatal(err)
	}
	pub := NewPostgresPublisher(func(context.Context) sqlx.ExtContext { return tx })
	if err := pub.Publish(ctx, Event{
		OrgID:      &orgID,
		ActorID:    &actorID,
		Type:       "sprint.started",
		EntityType: "sprint",
		Payload:    map[string]any{"name": "Sprint 1"},
	}); err != nil {
		t.Fatalf("publish in tx: %v", err)
	}
	if err := tx.Rollback(); err != nil {
		t.Fatal(err)
	}
	if got := countEvents(t, db); got != 0 {
		t.Fatalf("rolled back transaction left %d events", got)
	}

	tx, err = db.Beginx()
	if err != nil {
		t.Fatal(err)
	}
	pub = NewPostgresPublisher(func(context.Context) sqlx.ExtContext { return tx })
	if err := pub.Publish(ctx, Event{Type: "project.created", EntityType: "project"}); err != nil {
		t.Fatalf("publish in tx: %v", err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
	if got := countEvents(t, db); got != 1 {
		t.Fatalf("committed transaction has %d events, want 1", got)
	}
}
