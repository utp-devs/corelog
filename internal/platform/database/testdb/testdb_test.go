package testdb

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestNewAppliesAllMigrations(t *testing.T) {
	db := New(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	for _, table := range []string{"users", "workflows", "domain_events", "event_cursors"} {
		var found *string
		if err := db.QueryRowContext(ctx, `SELECT to_regclass($1)::text`, table).Scan(&found); err != nil {
			t.Fatalf("lookup %s: %v", table, err)
		}
		if found == nil {
			t.Fatalf("table %s was not created by migrations", table)
		}
	}
}

func TestNewUsesSchemaOnEveryConnection(t *testing.T) {
	db := New(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var schema string
	if err := db.QueryRowContext(ctx, `SELECT current_schema()`).Scan(&schema); err != nil {
		t.Fatalf("current_schema: %v", err)
	}
	if !strings.HasPrefix(schema, "test_") {
		t.Fatalf("unexpected schema %q", schema)
	}

	// Hold several pooled connections at once so each one is opened and checked.
	conns := make([]interface{ Close() error }, 0, 4)
	defer func() {
		for _, c := range conns {
			_ = c.Close()
		}
	}()
	for i := 0; i < 4; i++ {
		conn, err := db.Connx(ctx)
		if err != nil {
			t.Fatalf("acquire connection %d: %v", i, err)
		}
		conns = append(conns, conn)
		var got string
		if err := conn.QueryRowContext(ctx, `SELECT current_schema()`).Scan(&got); err != nil {
			t.Fatalf("connection %d current_schema: %v", i, err)
		}
		if got != schema {
			t.Fatalf("connection %d uses schema %q, want %q", i, got, schema)
		}
	}
}

// Two parallel subtests create the same table name. They wait for each other
// before reading, so a shared schema would fail or show the other subtest's row.
func TestParallelSchemasAreIsolated(t *testing.T) {
	var barrier sync.WaitGroup
	barrier.Add(2)
	t.Run("a", func(t *testing.T) { checkIsolation(t, "a", &barrier) })
	t.Run("b", func(t *testing.T) { checkIsolation(t, "b", &barrier) })
}

func checkIsolation(t *testing.T, who string, barrier *sync.WaitGroup) {
	t.Parallel()
	db := New(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if _, err := db.ExecContext(ctx, `CREATE TABLE isolation_probe (owner TEXT NOT NULL)`); err != nil {
		t.Fatalf("create probe: %v", err)
	}
	if _, err := db.ExecContext(ctx, `INSERT INTO isolation_probe (owner) VALUES ($1)`, who); err != nil {
		t.Fatalf("insert probe: %v", err)
	}
	barrier.Done()
	barrier.Wait()

	var count int
	if err := db.QueryRowContext(ctx, `SELECT count(*) FROM isolation_probe`).Scan(&count); err != nil {
		t.Fatalf("count probe: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected only this test's row, found %d rows", count)
	}
}
