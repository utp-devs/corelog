package postgres_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/jhonsferg/corelog/internal/audit/adapter/out/postgres"
	"github.com/jhonsferg/corelog/internal/audit/application/port"
	"github.com/jhonsferg/corelog/internal/platform/database/testdb"
)

func TestRepositoryKeepsRequestIDAndJoinsActorName(t *testing.T) {
	db := testdb.New(t)
	repo := postgres.NewRepository(db)
	ctx := context.Background()

	var actor uuid.UUID
	if err := db.GetContext(ctx, &actor,
		`INSERT INTO users (name, display_name, email, password_hash, role) VALUES ('Ana Prueba', 'Ana Prueba', 'ana-audit@example.com', 'x', 'user') RETURNING id`); err != nil {
		t.Fatalf("insert user: %v", err)
	}
	org := uuid.New()
	e := entryAt(org, actor, "auth.login", time.Now().UTC(), i64(77))
	e.RequestID = "req-abc"
	if err := repo.Save(ctx, e); err != nil {
		t.Fatal(err)
	}
	page, err := repo.List(ctx, port.ListFilter{OrganizationID: &org})
	if err != nil || len(page.Entries) != 1 {
		t.Fatalf("list: %v %d", err, len(page.Entries))
	}
	got := page.Entries[0]
	if got.RequestID != "req-abc" || got.ActorName != "Ana Prueba" {
		t.Fatalf("request id %q actor %q", got.RequestID, got.ActorName)
	}
	name, err := repo.ActorName(ctx, actor)
	if err != nil || name != "Ana Prueba" {
		t.Fatalf("ActorName = %q, %v", name, err)
	}
	if name, err := repo.ActorName(ctx, uuid.New()); err != nil || name != "" {
		t.Fatalf("unknown actor = %q, %v", name, err)
	}
}
