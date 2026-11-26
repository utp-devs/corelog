// Package testdb provides isolated PostgreSQL schemas for integration tests.
//
// Tests call New, which skips when CORELOG_TEST_DATABASE_URL is unset. Otherwise
// it creates a unique schema, applies every migration under migrations/postgres
// in version order, and returns a pool whose connections all use that schema as
// search_path. The schema is dropped when the test finishes.
package testdb

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/jmoiron/sqlx"
)

const (
	envDatabaseURL = "CORELOG_TEST_DATABASE_URL"
	migrationsDir  = "migrations/postgres"
	// migrationLockKey serialises schema setup across test packages running in
	// parallel processes, so concurrent CREATE EXTENSION IF NOT EXISTS calls do
	// not race on pg_extension.
	migrationLockKey = 7401001
	setupTimeout     = 2 * time.Minute
)

// New returns a *sqlx.DB bound to a fresh schema with all migrations applied.
// The test is skipped when CORELOG_TEST_DATABASE_URL is not set and fails when
// the schema cannot be prepared.
func New(t testing.TB) *sqlx.DB {
	t.Helper()
	dsn := os.Getenv(envDatabaseURL)
	if dsn == "" {
		t.Skipf("set %s to run PostgreSQL integration tests", envDatabaseURL)
	}

	base, err := pgx.ParseConfig(dsn)
	if err != nil {
		t.Fatalf("testdb: parse %s: %v", envDatabaseURL, err)
	}
	migrations, err := loadMigrations()
	if err != nil {
		t.Fatalf("testdb: load migrations: %v", err)
	}

	schema, err := newSchemaName()
	if err != nil {
		t.Fatalf("testdb: schema name: %v", err)
	}

	admin := stdlib.OpenDB(*base.Copy())
	admin.SetMaxOpenConns(2)
	t.Cleanup(func() { _ = admin.Close() })

	ctx, cancel := context.WithTimeout(context.Background(), setupTimeout)
	defer cancel()

	// Registered before any setup so the schema is dropped even when setup fails.
	// Cleanups run last-in first-out: pool closed, then schema dropped, then admin closed.
	var pool *sql.DB
	t.Cleanup(func() {
		dropCtx, dropCancel := context.WithTimeout(context.Background(), setupTimeout)
		defer dropCancel()
		if _, err := admin.ExecContext(dropCtx, `DROP SCHEMA IF EXISTS `+quoteIdent(schema)+` CASCADE`); err != nil {
			t.Errorf("testdb: drop schema %s: %v", schema, err)
		}
	})

	if err := prepareSchema(ctx, admin, schema); err != nil {
		t.Fatalf("testdb: %v", err)
	}

	cfg := base.Copy()
	if cfg.RuntimeParams == nil {
		cfg.RuntimeParams = make(map[string]string)
	}
	cfg.RuntimeParams["search_path"] = schema
	pool = stdlib.OpenDB(*cfg)
	pool.SetMaxOpenConns(4)
	t.Cleanup(func() { _ = pool.Close() })

	if err := applyMigrations(ctx, admin, pool, migrations); err != nil {
		t.Fatalf("testdb: %v", err)
	}
	return sqlx.NewDb(pool, "pgx")
}

// prepareSchema creates the schema while holding a session advisory lock.
func prepareSchema(ctx context.Context, admin *sql.DB, schema string) error {
	conn, err := admin.Conn(ctx)
	if err != nil {
		return fmt.Errorf("acquire setup connection: %w", err)
	}
	defer func() { _ = conn.Close() }()

	if _, err := conn.ExecContext(ctx, `SELECT pg_advisory_lock($1)`, migrationLockKey); err != nil {
		return fmt.Errorf("lock setup: %w", err)
	}
	defer func() {
		_, _ = conn.ExecContext(context.WithoutCancel(ctx), `SELECT pg_advisory_unlock($1)`, migrationLockKey)
	}()

	if _, err := conn.ExecContext(ctx, `CREATE SCHEMA `+quoteIdent(schema)); err != nil {
		return fmt.Errorf("create schema %s: %w", schema, err)
	}
	return nil
}

// applyMigrations runs each up migration in order on the test pool. Extensions
// created by migrations land in the test schema and are dropped with it.
func applyMigrations(ctx context.Context, admin *sql.DB, pool *sql.DB, migrations []migration) error {
	conn, err := admin.Conn(ctx)
	if err != nil {
		return fmt.Errorf("acquire migration lock connection: %w", err)
	}
	defer func() { _ = conn.Close() }()
	if _, err := conn.ExecContext(ctx, `SELECT pg_advisory_lock($1)`, migrationLockKey); err != nil {
		return fmt.Errorf("lock migrations: %w", err)
	}
	defer func() {
		_, _ = conn.ExecContext(context.WithoutCancel(ctx), `SELECT pg_advisory_unlock($1)`, migrationLockKey)
	}()

	for _, m := range migrations {
		if _, err := pool.ExecContext(ctx, m.sql); err != nil {
			return fmt.Errorf("apply migration %s: %w", m.file, err)
		}
	}
	return nil
}

type migration struct {
	version int
	file    string
	sql     string
}

// loadMigrations reads every *.up.sql file and sorts by the integer prefix, so
// "0010_workflows" sorts between "000009_..." and "000011_...".
func loadMigrations() ([]migration, error) {
	dir, err := findMigrationsDir()
	if err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", dir, err)
	}
	var out []migration
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".up.sql") {
			continue
		}
		prefix, _, ok := strings.Cut(name, "_")
		if !ok {
			return nil, fmt.Errorf("migration %s has no numeric prefix", name)
		}
		version, err := strconv.Atoi(prefix)
		if err != nil {
			return nil, fmt.Errorf("migration %s: parse version: %w", name, err)
		}
		raw, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			return nil, fmt.Errorf("read migration %s: %w", name, err)
		}
		out = append(out, migration{version: version, file: name, sql: string(raw)})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].version != out[j].version {
			return out[i].version < out[j].version
		}
		return out[i].file < out[j].file
	})
	if len(out) == 0 {
		return nil, fmt.Errorf("no up migrations found in %s", dir)
	}
	return out, nil
}

// findMigrationsDir walks up from the working directory until migrations/postgres exists.
func findMigrationsDir() (string, error) {
	wd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("working directory: %w", err)
	}
	for dir := wd; ; {
		candidate := filepath.Join(dir, filepath.FromSlash(migrationsDir))
		if info, err := os.Stat(candidate); err == nil && info.IsDir() {
			return candidate, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("%s not found above %s", migrationsDir, wd)
		}
		dir = parent
	}
}

func newSchemaName() (string, error) {
	buf := make([]byte, 8)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return "test_" + hex.EncodeToString(buf), nil
}

// quoteIdent quotes an identifier. Schema names are generated here, so this is a
// defensive measure rather than a user input path.
func quoteIdent(name string) string {
	return `"` + strings.ReplaceAll(name, `"`, `""`) + `"`
}
