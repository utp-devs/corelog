package testdb_test

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

	"github.com/jhonsferg/corelog/internal/platform/database/testdb"
)

// Mirrors the constants used by testdb: same environment variable, same migration
// directory, same advisory lock key so schema setup stays serialised across packages.
const (
	p02EnvDatabaseURL = "CORELOG_TEST_DATABASE_URL"
	p02MigrationsDir  = "migrations/postgres"
	p02LockKey        = 7401001
	p02Timeout        = 2 * time.Minute

	p02DefaultOrgID = "00000000-0000-4000-8000-000000000002"
	p02AdminID      = "00000000-0000-4000-8000-0000000000a1"
	p02AgentID      = "00000000-0000-4000-8000-0000000000a2"
	p02RequesterID  = "00000000-0000-4000-8000-0000000000a3"
	p02TeamID       = "00000000-0000-4000-8000-0000000000b1"
	p02ProjectID    = "00000000-0000-4000-8000-0000000000c1"
)

type p02Migration struct {
	version int
	name    string
	sql     string
}

// p02Env is an isolated schema with a pool whose search_path points at it.
type p02Env struct {
	admin *sql.DB
	pool  *sql.DB
	files []p02Migration // *.up.sql sorted by version
}

// TestMigrationsP02AllApply checks that the full migration set applies through testdb
// and that the default organization exists after the P2 migrations.
func TestMigrationsP02AllApply(t *testing.T) {
	db := testdb.New(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var count int
	if err := db.QueryRowContext(ctx,
		`SELECT count(*) FROM organizations WHERE id = $1 AND lower(slug) = 'corelog'`,
		p02DefaultOrgID,
	).Scan(&count); err != nil {
		t.Fatalf("lookup default organization: %v", err)
	}
	if count != 1 {
		t.Fatalf("default organization rows = %d, want 1", count)
	}
}

// TestMigrationsP02DataPreservation applies migrations up to 000022, seeds legacy data,
// applies 000023-000025 and checks counts, role mapping and organization scoping.
func TestMigrationsP02DataPreservation(t *testing.T) {
	env := newP02Env(t)
	env.applyRange(t, 1, 22)
	seedP02Legacy(t, env.pool, false)

	before := map[string]int{}
	for _, table := range []string{"users", "teams", "projects", "issues"} {
		before[table] = countP02(t, env.pool, table)
	}
	if before["users"] != 3 || before["teams"] != 1 || before["projects"] != 1 || before["issues"] != 2 {
		t.Fatalf("unexpected seed counts: %v", before)
	}

	env.applyRange(t, 23, 25)

	for table, want := range before {
		if got := countP02(t, env.pool, table); got != want {
			t.Errorf("%s count after migration = %d, want %d", table, got, want)
		}
	}

	// Roles: admin stays admin; agent and requester become user.
	assertP02Value(t, env.pool, `SELECT role FROM users WHERE id = $1`, "admin", "admin role", p02AdminID)
	assertP02Value(t, env.pool, `SELECT role FROM users WHERE id = $1`, "user", "agent role", p02AgentID)
	assertP02Value(t, env.pool, `SELECT role FROM users WHERE id = $1`, "user", "requester role", p02RequesterID)
	assertP02Value(t, env.pool, `SELECT display_name FROM users WHERE id = $1`, "Ann Agent", "display_name backfill", p02AgentID)
	assertP02Value(t, env.pool, `SELECT timezone FROM users WHERE id = $1`, "UTC", "timezone default", p02AgentID)

	// Default organization memberships: admin -> owner, requester -> guest, agent -> member.
	assertP02Value(t, env.pool,
		`SELECT role FROM organization_members WHERE organization_id = $1 AND user_id = $2`,
		"owner", "admin membership", p02DefaultOrgID, p02AdminID)
	assertP02Value(t, env.pool,
		`SELECT role FROM organization_members WHERE organization_id = $1 AND user_id = $2`,
		"guest", "requester membership", p02DefaultOrgID, p02RequesterID)
	assertP02Value(t, env.pool,
		`SELECT role FROM organization_members WHERE organization_id = $1 AND user_id = $2`,
		"member", "agent membership", p02DefaultOrgID, p02AgentID)
	if got := countP02Where(t, env.pool, `organization_members`, `organization_id = $1`, p02DefaultOrgID); got != 3 {
		t.Errorf("default organization members = %d, want 3", got)
	}
	assertP02Value(t, env.pool, `SELECT COALESCE(settings->>'hours_per_day', '') FROM organizations WHERE id = $1`,
		"8", "default settings hours_per_day", p02DefaultOrgID)

	// Team roles: owner becomes maintainer, plain member is unchanged.
	assertP02Value(t, env.pool, `SELECT role FROM team_members WHERE team_id = $1 AND user_id = $2`,
		"maintainer", "team owner mapped to maintainer", p02TeamID, p02AgentID)
	assertP02Value(t, env.pool, `SELECT role FROM team_members WHERE team_id = $1 AND user_id = $2`,
		"member", "team member unchanged", p02TeamID, p02RequesterID)

	// Every team and project belongs to the default organization.
	if got := countP02Where(t, env.pool, `teams`, `organization_id = $1`, p02DefaultOrgID); got != 1 {
		t.Errorf("teams in default organization = %d, want 1", got)
	}
	if got := countP02Where(t, env.pool, `projects`, `organization_id = $1`, p02DefaultOrgID); got != 1 {
		t.Errorf("projects in default organization = %d, want 1", got)
	}
	if got := countP02Where(t, env.pool, `teams`, `organization_id IS NULL`); got != 0 {
		t.Errorf("teams without organization = %d, want 0", got)
	}

	// The new role CHECK rejects legacy values.
	if _, err := env.pool.Exec(`UPDATE users SET role = 'agent' WHERE id = $1`, p02AgentID); err == nil {
		t.Error("users.role accepted legacy value 'agent' after migration")
	}

	// Project keys are unique per organization, not globally.
	if _, err := env.pool.Exec(`
		INSERT INTO organizations (id, slug, name) VALUES ('00000000-0000-4000-8000-0000000000d1', 'other-org', 'Other')`); err != nil {
		t.Fatalf("insert second organization: %v", err)
	}
	if _, err := env.pool.Exec(`
		INSERT INTO projects (id, key, name, lead_id, organization_id)
		VALUES ('00000000-0000-4000-8000-0000000000d2', 'DEMO', 'Same key elsewhere', $1, '00000000-0000-4000-8000-0000000000d1')`,
		p02AdminID); err != nil {
		t.Errorf("same project key in another organization rejected: %v", err)
	}
	if _, err := env.pool.Exec(`
		INSERT INTO projects (id, key, name, lead_id, organization_id)
		VALUES ('00000000-0000-4000-8000-0000000000d3', 'DEMO', 'Duplicate', $1, $2)`,
		p02AdminID, p02DefaultOrgID); err == nil {
		t.Error("duplicate project key inside one organization was accepted")
	}
}

// TestMigrationsP02DownUpCycle applies everything, runs down 25, 24, 23 in reverse and
// then up 23, 24, 25 again on the same schema.
func TestMigrationsP02DownUpCycle(t *testing.T) {
	env := newP02Env(t)
	env.applyRange(t, 1, 25)
	seedP02Legacy(t, env.pool, true)

	for _, version := range []int{25, 24, 23} {
		env.runFile(t, version, ".down.sql")
	}
	env.applyRange(t, 23, 25)

	if got := countP02Where(t, env.pool, `organizations`, `id = $1`, p02DefaultOrgID); got != 1 {
		t.Errorf("default organization rows after up cycle = %d, want 1", got)
	}
	if got := countP02(t, env.pool, "organization_members"); got != 3 {
		t.Errorf("organization members after up cycle = %d, want 3", got)
	}
	if got := countP02Where(t, env.pool, `teams`, `organization_id = $1`, p02DefaultOrgID); got != 1 {
		t.Errorf("teams scoped after up cycle = %d, want 1", got)
	}
}

// newP02Env creates an isolated schema without applying any migration.
func newP02Env(t *testing.T) *p02Env {
	t.Helper()
	dsn := os.Getenv(p02EnvDatabaseURL)
	if dsn == "" {
		t.Skipf("set %s to run PostgreSQL integration tests", p02EnvDatabaseURL)
	}
	base, err := pgx.ParseConfig(dsn)
	if err != nil {
		t.Fatalf("parse %s: %v", p02EnvDatabaseURL, err)
	}

	buf := make([]byte, 8)
	if _, err := rand.Read(buf); err != nil {
		t.Fatalf("schema name: %v", err)
	}
	schema := "p02_" + hex.EncodeToString(buf)

	admin := stdlib.OpenDB(*base.Copy())
	admin.SetMaxOpenConns(2)
	t.Cleanup(func() { _ = admin.Close() })
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), p02Timeout)
		defer cancel()
		if _, err := admin.ExecContext(ctx, `DROP SCHEMA IF EXISTS `+quoteP02(schema)+` CASCADE`); err != nil {
			t.Errorf("drop schema %s: %v", schema, err)
		}
	})

	env := &p02Env{admin: admin, files: loadP02UpFiles(t)}

	withP02Lock(t, admin, func(ctx context.Context, conn *sql.Conn) {
		if _, err := conn.ExecContext(ctx, `CREATE SCHEMA `+quoteP02(schema)); err != nil {
			t.Fatalf("create schema: %v", err)
		}
	})

	cfg := base.Copy()
	if cfg.RuntimeParams == nil {
		cfg.RuntimeParams = make(map[string]string)
	}
	cfg.RuntimeParams["search_path"] = schema
	env.pool = stdlib.OpenDB(*cfg)
	t.Cleanup(func() { _ = env.pool.Close() })
	return env
}

// applyRange applies up migrations with version in [from, to], in order.
func (e *p02Env) applyRange(t *testing.T, from, to int) {
	t.Helper()
	withP02Lock(t, e.admin, func(ctx context.Context, _ *sql.Conn) {
		for _, m := range e.files {
			if m.version < from || m.version > to {
				continue
			}
			if _, err := e.pool.ExecContext(ctx, m.sql); err != nil {
				t.Fatalf("apply %s: %v", m.name, err)
			}
		}
	})
}

// runFile executes the migration file of the given version with the given suffix.
func (e *p02Env) runFile(t *testing.T, version int, suffix string) {
	t.Helper()
	dir := p02MigrationsPath(t)
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read %s: %v", dir, err)
	}
	prefix := fmt.Sprintf("%06d_", version)
	for _, entry := range entries {
		name := entry.Name()
		if !strings.HasPrefix(name, prefix) || !strings.HasSuffix(name, suffix) {
			continue
		}
		raw, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			t.Fatalf("read %s: %v", name, err)
		}
		withP02Lock(t, e.admin, func(ctx context.Context, _ *sql.Conn) {
			if _, err := e.pool.ExecContext(ctx, string(raw)); err != nil {
				t.Fatalf("run %s: %v", name, err)
			}
		})
		return
	}
	t.Fatalf("no %s file for version %d", suffix, version)
}

// withP02Lock holds the testdb migration lock on a dedicated connection while fn runs.
func withP02Lock(t *testing.T, admin *sql.DB, fn func(ctx context.Context, conn *sql.Conn)) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), p02Timeout)
	defer cancel()
	conn, err := admin.Conn(ctx)
	if err != nil {
		t.Fatalf("acquire lock connection: %v", err)
	}
	defer func() { _ = conn.Close() }()
	if _, err := conn.ExecContext(ctx, `SELECT pg_advisory_lock($1)`, p02LockKey); err != nil {
		t.Fatalf("lock: %v", err)
	}
	defer func() {
		_, _ = conn.ExecContext(context.WithoutCancel(ctx), `SELECT pg_advisory_unlock($1)`, p02LockKey)
	}()
	fn(ctx, conn)
}

// seedP02Legacy inserts three users (admin, agent, requester), one team with an owner
// and a member, one project with a member and two tickets, all in the pre-P2 shape.
func seedP02Legacy(t *testing.T, pool *sql.DB, profileColumns bool) {
	t.Helper()
	// Before 000024 the users table has no display_name and accepts the legacy roles.
	// After 000024 display_name is NOT NULL and the role CHECK only accepts admin|user.
	userCols, agentRole, requesterRole := "name, email, password_hash, role, created_at", "agent", "requester"
	if profileColumns {
		userCols = "name, display_name, email, password_hash, role, created_at"
		agentRole, requesterRole = "user", "user"
	}
	// Each row lists name, then display_name when profileColumns is set.
	row := func(id, name, email, role, age string) string {
		values := `('` + id + `', ` + name + `, '` + email + `', 'x', '` + role + `', now() - interval '` + age + `')`
		if profileColumns {
			values = `('` + id + `', ` + name + `, ` + name + `, '` + email + `', 'x', '` + role + `', now() - interval '` + age + `')`
		}
		return values
	}
	// Teams and projects gain organization_id in 000025; the team owner becomes maintainer.
	teamInsert := `INSERT INTO teams (id, name) VALUES ('` + p02TeamID + `', 'Platform')`
	projectInsert := `INSERT INTO projects (id, key, name, lead_id) VALUES ('` + p02ProjectID + `', 'DEMO', 'Demo', '` + p02AdminID + `')`
	ownerRole := "owner"
	if profileColumns {
		teamInsert = `INSERT INTO teams (id, name, organization_id) VALUES ('` + p02TeamID + `', 'Platform', '` + p02DefaultOrgID + `')`
		projectInsert = `INSERT INTO projects (id, key, name, lead_id, organization_id) VALUES ('` + p02ProjectID + `', 'DEMO', 'Demo', '` + p02AdminID + `', '` + p02DefaultOrgID + `')`
		ownerRole = "maintainer"
	}
	stmts := []string{
		`INSERT INTO users (id, ` + userCols + `) VALUES ` +
			row(p02AdminID, "'Ada Admin'", "ada@example.test", "admin", "3 days") + `,` +
			row(p02AgentID, "'Ann Agent'", "ann@example.test", agentRole, "2 days") + `,` +
			row(p02RequesterID, "'Rita Requester'", "rita@example.test", requesterRole, "1 day"),
		teamInsert,
		`INSERT INTO team_members (team_id, user_id, role) VALUES
			('` + p02TeamID + `', '` + p02AgentID + `', '` + ownerRole + `'),
			('` + p02TeamID + `', '` + p02RequesterID + `', 'member')`,
		projectInsert,
		`INSERT INTO project_members (project_id, user_id, role) VALUES ('` + p02ProjectID + `', '` + p02AgentID + `', 'contributor')`,
		`INSERT INTO issues (id, title, description, status, priority, requester_id, project_id) VALUES
			('00000000-0000-4000-8000-0000000000e1', 'First', 'desc', 'open', 'medium', '` + p02RequesterID + `', '` + p02ProjectID + `'),
			('00000000-0000-4000-8000-0000000000e2', 'Second', 'desc', 'open', 'high', '` + p02RequesterID + `', '` + p02ProjectID + `')`,
	}
	for _, stmt := range stmts {
		if _, err := pool.Exec(stmt); err != nil {
			t.Fatalf("seed: %v\n%s", err, stmt)
		}
	}
}

// countP02Where counts rows of table matching where. Table names are fixed literals in this file.
func countP02Where(t *testing.T, pool *sql.DB, table, where string, args ...any) int {
	t.Helper()
	var n int
	query := fmt.Sprintf(`SELECT count(*) FROM %s WHERE %s`, table, where)
	if err := pool.QueryRow(query, args...).Scan(&n); err != nil {
		t.Fatalf("count %s: %v", table, err)
	}
	return n
}

func countP02(t *testing.T, pool *sql.DB, table string) int {
	t.Helper()
	return countP02Where(t, pool, table, "TRUE")
}

// assertP02Value runs query with args and compares the single text result to want.
func assertP02Value(t *testing.T, pool *sql.DB, query, want, what string, args ...any) {
	t.Helper()
	var got string
	if err := pool.QueryRow(query, args...).Scan(&got); err != nil {
		t.Fatalf("%s: query: %v", what, err)
	}
	if got != want {
		t.Errorf("%s = %q, want %q", what, got, want)
	}
}

// loadP02UpFiles reads every *.up.sql file, sorted by the integer prefix.
func loadP02UpFiles(t *testing.T) []p02Migration {
	t.Helper()
	dir := p02MigrationsPath(t)
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read %s: %v", dir, err)
	}
	var out []p02Migration
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".up.sql") {
			continue
		}
		prefix, _, ok := strings.Cut(name, "_")
		if !ok {
			t.Fatalf("migration %s has no numeric prefix", name)
		}
		version, err := strconv.Atoi(prefix)
		if err != nil {
			t.Fatalf("migration %s: parse version: %v", name, err)
		}
		raw, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			t.Fatalf("read %s: %v", name, err)
		}
		out = append(out, p02Migration{version: version, name: name, sql: string(raw)})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].version != out[j].version {
			return out[i].version < out[j].version
		}
		return out[i].name < out[j].name
	})
	return out
}

// p02MigrationsPath walks up from the working directory to find migrations/postgres.
func p02MigrationsPath(t *testing.T) string {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("working directory: %v", err)
	}
	for dir := wd; ; {
		candidate := filepath.Join(dir, filepath.FromSlash(p02MigrationsDir))
		if info, err := os.Stat(candidate); err == nil && info.IsDir() {
			return candidate
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatalf("%s not found above %s", p02MigrationsDir, wd)
		}
		dir = parent
	}
}

func quoteP02(name string) string {
	return `"` + strings.ReplaceAll(name, `"`, `""`) + `"`
}
