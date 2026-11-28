package testdb_test

import "testing"

const (
	integMigUserID    = "00000000-0000-4000-8000-0000000000e1"
	integMigOrgID     = "00000000-0000-4000-8000-0000000000e2"
	integMigProjectID = "00000000-0000-4000-8000-0000000000e3"
)

// TestMigrationsIntegrationsRoundTrip applies the automation, webhook, API token and import migrations, checks the
// constraints and cascades, then runs the down and up migrations again.
func TestMigrationsIntegrationsRoundTrip(t *testing.T) {
	env := newOrgMigEnv(t)
	env.applyRange(t, 1, 55)

	exec := func(query string, args ...any) error {
		_, err := env.pool.Exec(query, args...)
		return err
	}
	must := func(query string, args ...any) {
		t.Helper()
		if err := exec(query, args...); err != nil {
			t.Fatalf("%v\n%s", err, query)
		}
	}

	must(`INSERT INTO users (id, name, email, password_hash, role, display_name, timezone, created_at, updated_at)
		VALUES ($1, 'Integ', 'integ@example.test', 'hash', 'user', 'Integ', 'UTC', now(), now())`, integMigUserID)
	must(`INSERT INTO users (id, name, email, password_hash, role, display_name, timezone, created_at, updated_at)
		VALUES ('00000000-0000-4000-8000-0000000000e6', 'Lead', 'lead@example.test', 'hash', 'user', 'Lead', 'UTC', now(), now())`)
	must(`INSERT INTO organizations (id, name, slug) VALUES ($1, 'Integ', 'integ-org')`, integMigOrgID)
	must(`INSERT INTO projects (id, organization_id, key, name, type, template, lead_id, visibility)
		VALUES ($1, $2, 'INT', 'Integ', 'kanban', 'kanban', '00000000-0000-4000-8000-0000000000e6', 'open')`, integMigProjectID, integMigOrgID)

	// API tokens: scopes are a non-empty subset of read, write, admin and the hash is unique.
	must(`INSERT INTO api_tokens (user_id, name, token_hash, prefix, scopes) VALUES ($1, 't', repeat('a', 64), 'cl_pat_abcd', ARRAY['read'])`, integMigUserID)
	for _, bad := range []string{`ARRAY['root']`, `ARRAY[]::text[]`, `ARRAY['read', 'god']`} {
		if err := exec(`INSERT INTO api_tokens (user_id, name, token_hash, prefix, scopes) VALUES ($1, 'x', repeat('b', 64), 'cl_pat_x', `+bad+`)`, integMigUserID); err == nil {
			t.Errorf("scopes %s accepted", bad)
		}
	}
	if err := exec(`INSERT INTO api_tokens (user_id, name, token_hash, prefix, scopes) VALUES ($1, 'dup', repeat('a', 64), 'cl_pat_abcd', ARRAY['write'])`, integMigUserID); err == nil {
		t.Error("duplicate token hash accepted")
	}

	// Webhooks and deliveries: status check, per-event uniqueness and cascade.
	must(`INSERT INTO webhooks (id, organization_id, name, url, secret_encrypted, events, created_by)
		VALUES ('00000000-0000-4000-8000-0000000000e4', $1, 'w', 'https://example.test', '\x00', ARRAY['issue.created'], $2)`, integMigOrgID, integMigUserID)
	must(`INSERT INTO webhook_deliveries (webhook_id, event_id, event_type, payload) VALUES ('00000000-0000-4000-8000-0000000000e4', 7, 'issue.created', '{}')`)
	if err := exec(`INSERT INTO webhook_deliveries (webhook_id, event_id, event_type, payload) VALUES ('00000000-0000-4000-8000-0000000000e4', 7, 'issue.created', '{}')`); err == nil {
		t.Error("a second delivery of the same event accepted")
	}
	must(`INSERT INTO webhook_deliveries (webhook_id, event_id, event_type, payload) VALUES ('00000000-0000-4000-8000-0000000000e4', NULL, 'ping', '{}')`)
	must(`INSERT INTO webhook_deliveries (webhook_id, event_id, event_type, payload) VALUES ('00000000-0000-4000-8000-0000000000e4', NULL, 'ping', '{}')`)
	if err := exec(`UPDATE webhook_deliveries SET status = 'weird'`); err == nil {
		t.Error("unknown delivery status accepted")
	}

	// Automation rules and runs.
	must(`INSERT INTO automation_rules (id, organization_id, project_id, name, trigger, created_by)
		VALUES ('00000000-0000-4000-8000-0000000000e5', $1, $2, 'r', '{"type":"issue_created"}', $3)`, integMigOrgID, integMigProjectID, integMigUserID)
	if err := exec(`INSERT INTO automation_rules (organization_id, name, trigger) VALUES ($1, 'bad', '[]')`, integMigOrgID); err == nil {
		t.Error("a trigger that is not an object accepted")
	}
	must(`INSERT INTO automation_runs (rule_id, status) VALUES ('00000000-0000-4000-8000-0000000000e5', 'success')`)
	if err := exec(`INSERT INTO automation_runs (rule_id, status) VALUES ('00000000-0000-4000-8000-0000000000e5', 'maybe')`); err == nil {
		t.Error("unknown run status accepted")
	}

	// Import jobs.
	must(`INSERT INTO import_jobs (project_id, user_id) VALUES ($1, $2)`, integMigProjectID, integMigUserID)
	if err := exec(`INSERT INTO import_jobs (project_id, user_id, source) VALUES ($1, $2, 'xml')`, integMigProjectID, integMigUserID); err == nil {
		t.Error("unknown import source accepted")
	}

	// Deleting the user removes tokens and import jobs and keeps rules and webhooks with a null creator.
	must(`DELETE FROM users WHERE id = $1`, integMigUserID)
	assertOrgMigValue(t, env.pool, `SELECT count(*)::text FROM api_tokens`, "0", "tokens cascade on user delete")
	assertOrgMigValue(t, env.pool, `SELECT count(*)::text FROM import_jobs`, "0", "import jobs cascade on user delete")
	assertOrgMigValue(t, env.pool, `SELECT count(*)::text FROM webhooks WHERE created_by IS NULL`, "1", "webhooks survive their creator")
	assertOrgMigValue(t, env.pool, `SELECT count(*)::text FROM automation_rules WHERE created_by IS NULL`, "1", "rules survive their creator")
	must(`DELETE FROM webhooks`)
	assertOrgMigValue(t, env.pool, `SELECT count(*)::text FROM webhook_deliveries`, "0", "deliveries cascade on webhook delete")
	must(`DELETE FROM automation_rules`)
	assertOrgMigValue(t, env.pool, `SELECT count(*)::text FROM automation_runs`, "0", "runs cascade on rule delete")

	// Down and up again.
	for _, version := range []int{55, 54, 53} {
		env.runFile(t, version, ".down.sql")
	}
	for _, table := range []string{"api_tokens", "webhooks", "webhook_deliveries", "automation_rules", "automation_runs", "import_jobs"} {
		if got := countOrgMigWhere(t, env.pool, `information_schema.tables`, `table_name = '`+table+`' AND table_schema = current_schema()`); got != 0 {
			t.Errorf("table %s still exists after the down migrations", table)
		}
	}
	for _, version := range []int{53, 54, 55} {
		env.runFile(t, version, ".up.sql")
	}
	for _, table := range []string{"api_tokens", "webhooks", "webhook_deliveries", "automation_rules", "automation_runs", "import_jobs"} {
		if got := countOrgMigWhere(t, env.pool, `information_schema.tables`, `table_name = '`+table+`' AND table_schema = current_schema()`); got != 1 {
			t.Errorf("table %s missing after the up migrations again", table)
		}
	}
}
