package testdb_test

import "testing"

// TestMigrationsSearchRoundTrip applies the search migration over existing saved filters, checks the conversion
// to CoreQL and the new objects, then runs the down and up migrations again.
func TestMigrationsSearchRoundTrip(t *testing.T) {
	env := newOrgMigEnv(t)
	env.applyRange(t, 1, 41)
	const (
		org     = "00000000-0000-4000-8000-000000000002"
		owner   = "00000000-0000-4000-8000-0000000000a1"
		loner   = "00000000-0000-4000-8000-0000000000a2"
		f1      = "00000000-0000-4000-8000-0000000000f1"
		f2      = "00000000-0000-4000-8000-0000000000f2"
		f3      = "00000000-0000-4000-8000-0000000000f3"
		project = "00000000-0000-4000-8000-0000000000b1"
	)
	exec := func(q string, args ...any) {
		t.Helper()
		if _, err := env.pool.Exec(q, args...); err != nil {
			t.Fatalf("%v\n%s", err, q)
		}
	}
	scalar := func(q string) string {
		t.Helper()
		var v string
		if err := env.pool.QueryRow(q).Scan(&v); err != nil {
			t.Fatalf("%v\n%s", err, q)
		}
		return v
	}
	exec(`INSERT INTO users (id, name, display_name, email, password_hash, role) VALUES ($1, 'o', 'o', 'o@example.test', 'x', 'user'), ($2, 'l', 'l', 'l@example.test', 'x', 'user')`, owner, loner)
	exec(`INSERT INTO organization_members (organization_id, user_id, role) VALUES ($1, $2, 'member')`, org, owner)
	exec(`INSERT INTO saved_filters (id, owner_id, name, filters) VALUES
		($1, $4, 'Abiertas altas', '{"status":"open","priority":"high","issue_type":"bug"}'),
		($2, $4, 'Texto y fechas', '{"query":"say \"hola\"","due_from":"2026-01-01T00:00:00Z","due_to":"2026-02-01T00:00:00Z"}'),
		($3, $5, 'Sin organizacion', '{"status":"closed"}')`, f1, f2, f3, owner, loner)
	exec(`INSERT INTO projects (id, organization_id, key, name, type, template, lead_id, visibility) VALUES ($1, $2, 'SRCH', 'Search', 'scrum', 'scrum', $3, 'open')`, project, org, owner)
	exec(`INSERT INTO issues (id, project_id, summary, description, status, priority, reporter_id, issue_type, key_number, rank, issue_type_id, priority_id)
		SELECT gen_random_uuid(), p.id, 'Búsqueda de ejemplo', 'texto largo de descripción', 'open', 'medium', $1, 'task', 9999, 'm',
		       (SELECT id FROM issue_types WHERE organization_id = p.organization_id AND legacy_key = 'task'),
		       (SELECT id FROM priorities WHERE organization_id = p.organization_id AND legacy_key = 'medium')
		FROM projects p WHERE p.id = $2`, owner, project)

	env.runFile(t, 42, ".up.sql")

	if got := scalar(`SELECT query_text FROM filters WHERE id = '` + f1 + `'`); got != `statusCategory = todo AND priority = "Alta" AND type = "Error"` {
		t.Fatalf("converted filter 1 = %q", got)
	}
	if got := scalar(`SELECT query_text FROM filters WHERE id = '` + f2 + `'`); got != `text ~ "say \"hola\"" AND due >= "2026-01-01" AND due <= "2026-02-01"` {
		t.Fatalf("converted filter 2 = %q", got)
	}
	if got := scalar(`SELECT count(*)::text FROM filters WHERE id = '` + f3 + `'`); got != "0" {
		t.Fatal("a filter whose owner has no organization is not converted")
	}
	if got := scalar(`SELECT organization_id::text FROM filters WHERE id = '` + f1 + `'`); got != org {
		t.Fatalf("filter organization = %s", got)
	}
	if got := scalar(`SELECT count(*)::text FROM saved_filters`); got != "3" {
		t.Fatal("the legacy table must be left in place")
	}
	// The generated document is populated and indexed.
	if got := scalar(`SELECT count(*)::text FROM issues WHERE search_vector @@ websearch_to_tsquery('spanish', 'busqueda')`); got != "1" {
		t.Fatalf("search_vector backfill = %s", got)
	}
	for _, idx := range []string{"idx_issues_search_vector", "idx_comments_fts", "uq_filters_owner_name", "idx_filter_shares_target", "idx_filter_subscriptions_user"} {
		if got := scalar(`SELECT count(*)::text FROM pg_indexes WHERE schemaname = current_schema() AND indexname = '` + idx + `'`); got != "1" {
			t.Fatalf("index %s missing", idx)
		}
	}
	// Constraints: share types and permissions, subscriptions are one per user and filter.
	if _, err := env.pool.Exec(`INSERT INTO filter_shares (filter_id, share_type, target_id, permission) VALUES ($1, 'galaxy', $2, 'view')`, f1, owner); err == nil {
		t.Fatal("unknown share type accepted")
	}
	if _, err := env.pool.Exec(`INSERT INTO filter_shares (filter_id, share_type, target_id, permission) VALUES ($1, 'user', $2, 'own')`, f1, owner); err == nil {
		t.Fatal("unknown permission accepted")
	}
	exec(`INSERT INTO filter_subscriptions (filter_id, user_id, schedule) VALUES ($1, $2, '{"frequency":"daily"}')`, f1, owner)
	if _, err := env.pool.Exec(`INSERT INTO filter_subscriptions (filter_id, user_id, schedule) VALUES ($1, $2, '{}')`, f1, owner); err == nil {
		t.Fatal("duplicate subscription accepted")
	}
	const temp = "00000000-0000-4000-8000-0000000000a3"
	exec(`INSERT INTO users (id, name, display_name, email, password_hash, role) VALUES ($1, 't', 't', 't@example.test', 'x', 'user')`, temp)
	exec(`INSERT INTO filters (id, owner_id, organization_id, name) VALUES ('00000000-0000-4000-8000-0000000000f9', $1, $2, 'Temporal')`, temp, org)
	exec(`INSERT INTO filter_subscriptions (filter_id, user_id, schedule) VALUES ('00000000-0000-4000-8000-0000000000f9', $1, '{}')`, temp)
	exec(`DELETE FROM users WHERE id = $1`, temp)
	if got := scalar(`SELECT count(*)::text FROM filters WHERE name = 'Temporal'`) + scalar(`SELECT count(*)::text FROM filter_subscriptions WHERE user_id = '`+temp+`'`); got != "00" {
		t.Fatalf("filters and subscriptions must cascade with the owner, got %s", got)
	}

	env.runFile(t, 42, ".down.sql")
	for _, table := range []string{"filters", "filter_shares", "filter_subscriptions"} {
		if got := scalar(`SELECT count(*)::text FROM information_schema.tables WHERE table_schema = current_schema() AND table_name = '` + table + `'`); got != "0" {
			t.Fatalf("table %s survived the down migration", table)
		}
	}
	if got := scalar(`SELECT count(*)::text FROM information_schema.columns WHERE table_schema = current_schema() AND table_name = 'issues' AND column_name = 'search_vector'`); got != "0" {
		t.Fatal("search_vector survived the down migration")
	}

	env.runFile(t, 42, ".up.sql")
	if got := scalar(`SELECT count(*)::text FROM issues WHERE search_vector IS NOT NULL`); got == "0" {
		t.Fatal("search_vector must be rebuilt by the second up")
	}
	// The legacy filters are converted again from the table that was left in place.
	if got := scalar(`SELECT count(*)::text FROM filters`); got != "2" {
		t.Fatalf("filters after the second up = %s", got)
	}
}
