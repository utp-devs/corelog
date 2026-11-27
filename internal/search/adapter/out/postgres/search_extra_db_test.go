package postgres_test

import (
	"context"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/jmoiron/sqlx"

	"github.com/jhonsferg/corelog/internal/platform/apperror"
	"github.com/jhonsferg/corelog/internal/platform/clock"
	"github.com/jhonsferg/corelog/internal/search/adapter/out/postgres"
	"github.com/jhonsferg/corelog/internal/search/application/port"
	"github.com/jhonsferg/corelog/internal/search/application/usecase"
	"github.com/jhonsferg/corelog/internal/search/domain"
	"github.com/jhonsferg/corelog/internal/search/domain/coreql"
)

// TestSearchInjectionAgainstDatabase runs hostile values through every kind of clause and checks that the
// database never executes them: no error, no rows, and the tables are intact.
func TestSearchInjectionAgainstDatabase(t *testing.T) {
	e := newSearchEnv(t)
	var before int
	if err := e.db.Get(&before, `SELECT count(*) FROM issues`); err != nil {
		t.Fatal(err)
	}
	payloads := []string{
		`x'; DROP TABLE issues; --`, `x" OR "1"="1`, `x') UNION SELECT password_hash FROM users --`,
		`x$$; DELETE FROM users; $$`, `x\'); SELECT pg_sleep(10); --`, "x\n/* c */ OR 1=1", `%`, `_`, `\`, `x\`,
	}
	templates := []string{
		"project = %s", "type = %s", "status = %s", "priority = %s", "resolution = %s", "assignee = %s", "reporter = %s",
		"watcher = %s", "labels = %s", "component = %s", "sprint = %s", "summary = %s", "summary ~ %s", "description ~ %s",
		"comment ~ %s", "text ~ %s", "cf[Cliente] = %s", "cf[Cliente] ~ %s", "cf[Entorno] = %s", "cf[Horas] = %s", "key = %s",
		"fixVersion = %s", "assignee IN (membersOf(%s))", "epic = %s",
	}
	for _, tpl := range templates {
		for _, p := range payloads {
			quoted := `"` + strings.NewReplacer(`\`, `\\`, `"`, `\"`).Replace(p) + `"`
			jql := strings.ReplaceAll(tpl, "%s", quoted)
			start := time.Now()
			out, err := e.jql.Search(e.ctx, port.SearchInput{Viewer: e.viewer(e.alice), Query: jql, PageSize: 10})
			if err != nil {
				// Values that are not valid for the field (a number, a date) are rejected before the database.
				if apperror.KindOf(err) != apperror.KindValidation {
					t.Fatalf("%q: %v", jql, err)
				}
				continue
			}
			if time.Since(start) > 5*time.Second {
				t.Fatalf("%q ran a sleep", jql)
			}
			// "%" and "_" are literals, so they match no summary of the fixture.
			if len(out.Page.Items) > 0 && !strings.HasPrefix(tpl, "summary") && !strings.Contains(tpl, "~") {
				t.Fatalf("%q matched %d issues", jql, len(out.Page.Items))
			}
		}
	}
	var after int
	if err := e.db.Get(&after, `SELECT count(*) FROM issues`); err != nil || after != before {
		t.Fatalf("issues table changed: %d -> %d (%v)", before, after, err)
	}
	if err := e.db.Get(&after, `SELECT count(*) FROM users`); err != nil || after == 0 {
		t.Fatalf("users table changed: %v", err)
	}
	// Wildcards are literal in substring matches.
	if got := e.keys(e.alice, `summary ~ "%"`); len(got) != 0 {
		t.Fatalf("%% must be literal: %v", got)
	}
}

func TestSearchStatementTimeout(t *testing.T) {
	e := newSearchEnv(t)
	fast := postgres.NewIssueSearchRepository(e.db).WithTimeout(50 * time.Millisecond)
	err := fast.SleepForTest(e.ctx, 2)
	if apperror.CodeOf(err) != domain.CodeQueryTimeout || apperror.KindOf(err) != apperror.KindValidation {
		t.Fatalf("timeout must map to %s, got %v", domain.CodeQueryTimeout, err)
	}
	// A query that finishes in time is untouched, and the timeout does not outlive its transaction.
	if err := postgres.NewIssueSearchRepository(e.db).WithTimeout(5*time.Second).SleepForTest(e.ctx, 0); err != nil {
		t.Fatal(err)
	}
	var setting string
	if err := e.db.Get(&setting, `SHOW statement_timeout`); err != nil || setting != "0" {
		t.Fatalf("statement_timeout leaked out of the transaction: %q %v", setting, err)
	}
	// Cancelling the request is not reported as a timeout.
	ctx, cancel := context.WithCancel(e.ctx)
	cancel()
	if err := postgres.NewIssueSearchRepository(e.db).SleepForTest(ctx, 0); err == nil || apperror.CodeOf(err) == domain.CodeQueryTimeout {
		t.Fatalf("cancelled request: %v", err)
	}
}

// queryCounter counts the statements a pool sends.
type queryCounter struct{ n atomic.Int64 }

func (c *queryCounter) TraceQueryStart(ctx context.Context, _ *pgx.Conn, _ pgx.TraceQueryStartData) context.Context {
	c.n.Add(1)
	return ctx
}
func (c *queryCounter) TraceQueryEnd(context.Context, *pgx.Conn, pgx.TraceQueryEndData) {}

// countingDB opens a second pool on the schema of db that counts statements.
func countingDB(t *testing.T, db *sqlx.DB) (*sqlx.DB, *queryCounter) {
	t.Helper()
	var schema string
	if err := db.Get(&schema, `SELECT current_schema()`); err != nil {
		t.Fatal(err)
	}
	cfg, err := pgx.ParseConfig(os.Getenv("CORELOG_TEST_DATABASE_URL"))
	if err != nil {
		t.Fatal(err)
	}
	counter := &queryCounter{}
	cfg.Tracer = counter
	cfg.RuntimeParams["search_path"] = schema
	pool := sqlx.NewDb(stdlib.OpenDB(*cfg), "pgx")
	pool.SetMaxOpenConns(2)
	t.Cleanup(func() { _ = pool.Close() })
	return pool, counter
}

// TestSearchStatementCounts proves the endpoints use a constant number of statements: a page of one issue and
// a page of five cost the same, so there is no per-row query.
func TestSearchStatementCounts(t *testing.T) {
	e := newSearchEnv(t)
	pool, counter := countingDB(t, e.db)
	repo := postgres.NewIssueSearchRepository(pool)
	svc := usecase.NewJQLService(repo, repo, repo, clock.NewFixed(testNow)).WithSuggestions(repo)
	statements := func(in port.SearchInput) int64 {
		counter.n.Store(0)
		in.Viewer = e.viewer(e.alice)
		if _, err := svc.Search(e.ctx, in); err != nil {
			t.Fatal(err)
		}
		return counter.n.Load()
	}
	one := statements(port.SearchInput{Query: "key = PUB-1", WithTotal: true})
	five := statements(port.SearchInput{Query: "project = PUB", WithTotal: true})
	if one != five {
		t.Fatalf("statements depend on the page size: %d vs %d", one, five)
	}
	if one > 10 {
		t.Fatalf("a search should need few statements, got %d", one)
	}
	noTotal := statements(port.SearchInput{Query: "project = PUB"})
	if noTotal != five-1 {
		t.Fatalf("the count statement must be skipped without total: %d vs %d", noTotal, five)
	}
	t.Logf("search statements: %d (with total), %d (without)", five, noTotal)

	counter.n.Store(0)
	if _, err := svc.Suggest(e.ctx, port.SuggestInput{Viewer: e.viewer(e.alice), Field: "assignee", Prefix: "a"}); err != nil {
		t.Fatal(err)
	}
	if n := counter.n.Load(); n > 3 {
		t.Fatalf("suggest used %d statements", n)
	}
}

// TestSearchUsesIndexesAndStaysFast seeds 20000 issues and checks that substring and full-text predicates use
// their indexes and that the statement stays within the budget.
func TestSearchUsesIndexesAndStaysFast(t *testing.T) {
	e := newSearchEnv(t)
	e.exec(`INSERT INTO issues (id, project_id, summary, description, status, priority, reporter_id, issue_type, key_number, rank,
			issue_type_id, priority_id, created_at, updated_at)
		SELECT gen_random_uuid(), $1, 'Incidencia ' || g || ' ' || md5(g::text), 'descripcion de prueba numero ' || g, 'open', 'medium', $2, 'task', 1000 + g, 'r',
			(SELECT id FROM issue_types WHERE organization_id = $3 AND legacy_key = 'task'),
			(SELECT id FROM priorities WHERE organization_id = $3 AND legacy_key = 'medium'),
			now() - (g || ' minutes')::interval, now() - (g || ' minutes')::interval
		FROM generate_series(1, 20000) g`, e.pub, e.alice, testOrg)
	e.exec(`ANALYZE issues`)

	explain := func(jql string) (string, time.Duration) {
		built := mustBuildFor(t, e, jql)
		rows, err := e.db.Query(`EXPLAIN (ANALYZE, FORMAT TEXT) `+built.SelectSQL, built.SelectArgs...)
		if err != nil {
			t.Fatalf("explain %q: %v", jql, err)
		}
		defer rows.Close()
		var sb strings.Builder
		for rows.Next() {
			var line string
			_ = rows.Scan(&line)
			sb.WriteString(line + "\n")
		}
		start := time.Now()
		if _, err := e.jql.Search(e.ctx, port.SearchInput{Viewer: e.viewer(e.alice), Query: jql, PageSize: 20, WithTotal: true}); err != nil {
			t.Fatal(err)
		}
		return sb.String(), time.Since(start)
	}

	var hasTrgm bool
	if err := e.db.Get(&hasTrgm, `SELECT EXISTS (SELECT 1 FROM pg_indexes WHERE schemaname = current_schema() AND indexname = 'idx_issues_summary_trgm')`); err != nil {
		t.Fatal(err)
	}
	if hasTrgm {
		plan, took := explain(`summary ~ "md5zzzz"`)
		if strings.Contains(plan, "Seq Scan on issues") {
			t.Fatalf("summary ~ must not scan the whole table:\n%s", plan)
		}
		t.Logf("summary ~ over 20k issues: %v", took)
	} else {
		t.Log("pg_trgm is not available here: trigram index skipped")
	}
	plan, _ := explain(`key = PUB-1005 AND project = PUB`)
	if strings.Contains(plan, "Seq Scan on issues") && !strings.Contains(plan, "Filter") {
		t.Fatalf("unexpected plan:\n%s", plan)
	}
	for _, jql := range []string{`text ~ "descripcion numero 777"`, `project = PUB AND status = "Por hacer" ORDER BY updated DESC`, `assignee = currentUser() OR labels = bug`, ""} {
		_, took := explain(jql)
		t.Logf("%-60q %v", jql, took)
		if took > 3*time.Second {
			t.Fatalf("%q took %v over 20k issues", jql, took)
		}
	}
	// The full-text index is usable for the weighted document.
	var plan2 []string
	tx, err := e.db.Beginx()
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.Exec(`SET LOCAL enable_seqscan = off`); err != nil {
		t.Fatal(err)
	}
	if hasTrgm {
		var trgm []string
		if err := tx.Select(&trgm, `EXPLAIN SELECT id FROM issues WHERE summary ILIKE '%md5zzzz%'`); err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(strings.Join(trgm, "\n"), "idx_issues_summary_trgm") {
			t.Fatalf("trigram index unusable:\n%s", strings.Join(trgm, "\n"))
		}
	}
	if err := tx.Select(&plan2, `EXPLAIN SELECT id FROM issues WHERE search_vector @@ websearch_to_tsquery('spanish', 'descripcion')`); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(strings.Join(plan2, "\n"), "idx_issues_search_vector") {
		t.Fatalf("search_vector index unused:\n%s", strings.Join(plan2, "\n"))
	}
}

func mustBuildFor(t *testing.T, e *searchEnv, jql string) *postgres.BuiltQuery {
	t.Helper()
	p, err := e.jqlPrepare(jql)
	if err != nil {
		t.Fatal(err)
	}
	return p
}

// jqlPrepare builds the SQL of a query as alice, using the same catalog the service would.
func (e *searchEnv) jqlPrepare(jql string) (*postgres.BuiltQuery, error) {
	fields, err := e.repo.CustomFields(e.ctx, e.viewer(e.alice))
	if err != nil {
		return nil, err
	}
	custom := map[string]port.CustomFieldDef{}
	for _, f := range fields {
		custom[port.CustomRef("cf["+f.Name+"]")] = f
	}
	q, err := coreql.Parse(jql)
	if err != nil {
		return nil, err
	}
	return postgres.BuildIssueQuery(port.IssueQuery{Viewer: e.viewer(e.alice), Query: q, Custom: custom, Now: testNow, Page: 1, PageSize: 20}, postgres.VersionTables{})
}

func TestSearchVersionsAgainstRealTables(t *testing.T) {
	e := newSearchEnv(t)
	var v10, v20 uuid.UUID
	for name, dst := range map[string]*uuid.UUID{"1.0": &v10, "2.0": &v20} {
		*dst = uuid.New()
		e.exec(`INSERT INTO versions (id, project_id, name) VALUES ($1, $2, $3)`, *dst, e.pub, name)
	}
	e.exec(`UPDATE versions SET status = 'released', released_at = now() WHERE id = $1`, v10)
	var i1, i2 uuid.UUID
	if err := e.db.Get(&i1, `SELECT i.id FROM issues i JOIN projects p ON p.id = i.project_id WHERE p.key = 'PUB' AND i.key_number = 1`); err != nil {
		t.Fatal(err)
	}
	if err := e.db.Get(&i2, `SELECT i.id FROM issues i JOIN projects p ON p.id = i.project_id WHERE p.key = 'PUB' AND i.key_number = 2`); err != nil {
		t.Fatal(err)
	}
	e.exec(`INSERT INTO issue_versions (issue_id, version_id, kind) VALUES ($1, $2, 'fix'), ($3, $4, 'fix'), ($1, $4, 'affects')`, i1, v10, i2, v20)
	cases := []struct {
		jql  string
		want []string
	}{
		{`fixVersion = "1.0"`, []string{"PUB-1"}},
		{`fixVersion IN ("1.0", "2.0")`, []string{"PUB-1", "PUB-2"}},
		{`affectedVersion = "2.0"`, []string{"PUB-1"}},
		{`fixVersion IN (releasedVersions())`, []string{"PUB-1"}},
		{`fixVersion IN (unreleasedVersions())`, []string{"PUB-2"}},
		{`fixVersion IS EMPTY AND project = PUB`, []string{"PUB-3", "PUB-4", "PUB-5"}},
		{`fixVersion != "1.0" AND project = PUB`, []string{"PUB-2", "PUB-3", "PUB-4", "PUB-5"}},
	}
	for _, tc := range cases {
		if got := e.keys(e.alice, tc.jql); !sameSet(got, tc.want) {
			t.Errorf("%s: got %v, want %v", tc.jql, got, tc.want)
		}
	}
	// Without the version port the fields are reported as unavailable, not as an empty result.
	plain := postgres.NewIssueSearchRepository(e.db)
	svc := usecase.NewJQLService(plain, plain, plain, clock.NewFixed(testNow))
	_, err := svc.Search(e.ctx, port.SearchInput{Viewer: e.viewer(e.alice), Query: `fixVersion = "1.0"`})
	if apperror.CodeOf(err) != domain.CodeFieldUnavailable {
		t.Fatalf("unavailable versions: %v", err)
	}
	if vals, _ := plain.SuggestValues(e.ctx, e.viewer(e.alice), "fixversion", "", 5); len(vals) != 0 {
		t.Fatalf("version suggestions without the port: %v", vals)
	}
	vals, err := e.repo.SuggestValues(e.ctx, e.viewer(e.alice), "fixversion", "1", 5)
	if err != nil || len(vals) != 1 || vals[0].Value != "1.0" {
		t.Fatalf("version suggestions: %v %v", vals, err)
	}
}

func TestSearchTimezoneAndCustomFieldCatalog(t *testing.T) {
	e := newSearchEnv(t)
	tz, err := e.repo.Timezone(e.ctx, e.alice)
	if err != nil || tz == "" {
		t.Fatalf("timezone: %q %v", tz, err)
	}
	if tz, _ := e.repo.Timezone(e.ctx, uuid.New()); tz != "UTC" {
		t.Fatalf("unknown user falls back to UTC, got %q", tz)
	}
	fields, err := e.repo.CustomFields(e.ctx, e.viewer(e.alice))
	if err != nil || len(fields) != 4 {
		t.Fatalf("fields: %v %v", fields, err)
	}
	// People of another organization see none of them; the platform administrator sees them all.
	if other, _ := e.repo.CustomFields(e.ctx, e.viewer(e.dave)); len(other) != 0 {
		t.Fatalf("custom fields leaked across organizations: %v", other)
	}
	if all, _ := e.repo.CustomFields(e.ctx, e.viewer(e.admin)); len(all) != 4 {
		t.Fatalf("admin custom fields: %v", all)
	}
	// Same name in two organizations: one entry with both ids, and a query matches either.
	e.exec(`INSERT INTO custom_fields (organization_id, name, field_type) VALUES ($1, 'cliente', 'text')`, e.org2)
	fields, _ = e.repo.CustomFields(e.ctx, e.viewer(e.admin))
	merged := 0
	for _, f := range fields {
		if strings.EqualFold(f.Name, "cliente") {
			merged = len(f.IDs)
		}
	}
	if merged != 2 {
		t.Fatalf("expected one merged field with two ids, got %v", fields)
	}
	if got := e.keys(e.admin, `cf[Cliente] = Acme`); !sameSet(got, []string{"PUB-1"}) {
		t.Fatalf("merged field query: %v", got)
	}
}
