package testdb_test

import "testing"

const notifMigUserID = "00000000-0000-4000-8000-0000000000f2"

// TestMigrationsNotificationSchemesOutboxRoundTrip applies the notification schemes and email outbox migration
// over existing preferences, checks the constraints, then runs its down and up again.
func TestMigrationsNotificationSchemesOutboxRoundTrip(t *testing.T) {
	env := newOrgMigEnv(t)
	env.applyRange(t, 1, 51)

	exec := func(query string, args ...any) error {
		_, err := env.pool.Exec(query, args...)
		return err
	}
	if err := exec(`INSERT INTO users (id, name, email, password_hash, role, display_name, timezone, created_at, updated_at)
		VALUES ($1, 'Notif', 'notif@example.test', 'hash', 'user', 'Notif', 'UTC', now(), now())`, notifMigUserID); err != nil {
		t.Fatalf("insert user: %v", err)
	}
	if err := exec(`INSERT INTO notification_preferences (user_id, in_app) VALUES ($1, false)`, notifMigUserID); err != nil {
		t.Fatalf("insert preference: %v", err)
	}

	env.runFile(t, 52, ".up.sql")
	assertOrgMigValue(t, env.pool, `SELECT email_mode || '/' || notify_own_actions::text || '/' || in_app::text FROM notification_preferences`, "immediate/false/false", "existing preferences keep in_app and get the new defaults")
	if err := exec(`UPDATE notification_preferences SET email_mode = 'weekly'`); err == nil {
		t.Error("unknown email mode accepted")
	}

	var orgID, projectID string
	if err := env.pool.QueryRow(`SELECT id::text FROM organizations LIMIT 1`).Scan(&orgID); err != nil {
		t.Fatalf("organization: %v", err)
	}
	projectID = "00000000-0000-4000-8000-0000000000f3"
	if err := exec(`INSERT INTO projects (id, organization_id, key, name, type, template, lead_id, visibility) VALUES ($1, $2, 'MIG', 'Mig', 'kanban', 'scrum', $3, 'open')`, projectID, orgID, notifMigUserID); err != nil {
		t.Fatalf("insert project: %v", err)
	}
	if err := exec(`INSERT INTO notification_schemes (organization_id, project_id, rules) VALUES ($1, $2, '[]')`, orgID, projectID); err != nil {
		t.Fatalf("project scheme: %v", err)
	}
	if err := exec(`INSERT INTO notification_schemes (organization_id, project_id, rules) VALUES ($1, $2, '[]')`, orgID, projectID); err == nil {
		t.Error("two schemes for one project accepted")
	}
	if err := exec(`INSERT INTO notification_schemes (organization_id, project_id, rules) VALUES ($1, NULL, '[]')`, orgID); err != nil {
		t.Fatalf("organization scheme: %v", err)
	}
	if err := exec(`INSERT INTO notification_schemes (organization_id, project_id, rules) VALUES ($1, NULL, '[]')`, orgID); err == nil {
		t.Error("two organization defaults accepted")
	}

	insertMail := `INSERT INTO email_outbox (to_email, kind, subject, body_text, idempotency_key, status) VALUES ('a@example.test', 'x', 's', 't', $1, $2)`
	if err := exec(insertMail, "k1", "pending"); err != nil {
		t.Fatalf("outbox: %v", err)
	}
	if err := exec(insertMail, "k1", "pending"); err == nil {
		t.Error("duplicate idempotency key accepted")
	}
	if err := exec(insertMail, nil, "pending"); err != nil {
		t.Fatalf("outbox without key: %v", err)
	}
	if err := exec(insertMail, nil, "pending"); err != nil {
		t.Fatalf("a second message without key: %v", err)
	}
	if err := exec(insertMail, "k2", "lost"); err == nil {
		t.Error("unknown outbox status accepted")
	}
	if err := exec(`INSERT INTO scheduler_runs (job, started_at, status) VALUES ('j', now(), 'weird')`); err == nil {
		t.Error("unknown run status accepted")
	}

	if err := exec(`INSERT INTO notification_digest_items (user_id, event) VALUES ($1, 'issue_created')`, notifMigUserID); err != nil {
		t.Fatalf("digest item: %v", err)
	}
	if err := exec(`INSERT INTO notification_deliveries (dedup_key, user_id) VALUES ('evt:1', $1)`, notifMigUserID); err != nil {
		t.Fatalf("delivery: %v", err)
	}
	if err := exec(`DELETE FROM projects WHERE id = $1`, projectID); err != nil {
		t.Fatalf("delete project: %v", err)
	}
	assertOrgMigValue(t, env.pool, `SELECT count(*)::text FROM notification_schemes`, "1", "the project scheme goes with its project")
	if err := exec(`DELETE FROM users WHERE id = $1`, notifMigUserID); err != nil {
		t.Fatalf("delete user: %v", err)
	}
	assertOrgMigValue(t, env.pool, `SELECT (SELECT count(*) FROM notification_digest_items)::text || '/' || (SELECT count(*) FROM notification_deliveries)::text`, "0/0", "per-user bookkeeping cascades")

	const tables = `SELECT count(*)::text FROM information_schema.tables WHERE table_schema = current_schema() AND table_name IN
		('notification_schemes', 'email_outbox', 'notification_digest_items', 'notification_deliveries', 'scheduler_jobs', 'scheduler_runs')`
	assertOrgMigValue(t, env.pool, tables, "6", "tables after the up")

	env.runFile(t, 52, ".down.sql")
	assertOrgMigValue(t, env.pool, tables, "0", "tables removed by the down")
	assertOrgMigValue(t, env.pool, `SELECT count(*)::text FROM information_schema.columns WHERE table_schema = current_schema() AND table_name = 'notification_preferences' AND column_name IN ('email_mode', 'notify_own_actions')`, "0", "preference columns removed by the down")

	env.runFile(t, 52, ".up.sql")
	assertOrgMigValue(t, env.pool, tables, "6", "tables after the up again")
}
