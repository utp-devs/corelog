package testdb_test

import "testing"

const (
	dcOrg     = "00000000-0000-4000-8000-000000000002"
	dcUser    = "00000000-0000-4000-8000-0000000000e1"
	dcProject = "00000000-0000-4000-8000-0000000000e2"
)

// TestMigrationsDashboardsCalendarRoundTrip applies the dashboards and calendar migrations, checks the constraints and
// the cascades, then runs the down and up migrations again.
func TestMigrationsDashboardsCalendarRoundTrip(t *testing.T) {
	env := newOrgMigEnv(t)
	env.applyRange(t, 1, 46)
	exec := func(q string, args ...any) error {
		_, err := env.pool.Exec(q, args...)
		return err
	}
	must := func(q string, args ...any) {
		t.Helper()
		if err := exec(q, args...); err != nil {
			t.Fatalf("%v\n%s", err, q)
		}
	}
	refuse := func(what, q string, args ...any) {
		t.Helper()
		if err := exec(q, args...); err == nil {
			t.Errorf("%s was accepted", what)
		}
	}
	must(`INSERT INTO users (id, name, display_name, email, password_hash, role) VALUES ($1, 'dc', 'dc', 'dc@example.test', 'x', 'user')`, dcUser)
	must(`INSERT INTO organization_members (organization_id, user_id, role) VALUES ($1, $2, 'member')`, dcOrg, dcUser)
	must(`INSERT INTO projects (id, organization_id, key, name, type, template, lead_id, visibility) VALUES ($1, $2, 'DCM', 'Paneles', 'scrum', 'scrum', $3, 'open')`, dcProject, dcOrg, dcUser)

	for round := 0; round < 2; round++ {
		env.runFile(t, 48, ".up.sql")
		env.runFile(t, 49, ".up.sql")
		assertDashboardSchema(t, env, exec, must, refuse)
		assertCalendarSchema(t, env, exec, must, refuse)
		env.runFile(t, 49, ".down.sql")
		assertOrgMigValue(t, env.pool, `SELECT count(*)::text FROM information_schema.tables WHERE table_schema = current_schema() AND table_name IN ('calendar_events', 'calendar_event_attendees', 'calendar_feed_tokens')`, "0", "calendar tables removed")
		env.runFile(t, 48, ".down.sql")
		assertOrgMigValue(t, env.pool, `SELECT count(*)::text FROM information_schema.tables WHERE table_schema = current_schema() AND table_name IN ('dashboards', 'dashboard_gadgets', 'dashboard_shares', 'roadmap_epic_ranks')`, "0", "dashboard tables removed")
		assertOrgMigValue(t, env.pool, `SELECT count(*)::text FROM pg_indexes WHERE schemaname = current_schema() AND indexname = 'idx_activities_created_at'`, "0", "activity index removed")
		// The tables of other migrations are untouched.
		assertOrgMigValue(t, env.pool, `SELECT count(*)::text FROM projects WHERE id = $1`, "1", "project survives the down", dcProject)
	}
	// A final up leaves the schema in place for the idempotent re-run.
	env.runFile(t, 48, ".up.sql")
	env.runFile(t, 49, ".up.sql")
	env.runFile(t, 48, ".up.sql")
	env.runFile(t, 49, ".up.sql")
}

func assertDashboardSchema(t *testing.T, env *orgMigEnv, exec func(string, ...any) error, must func(string, ...any), refuse func(string, string, ...any)) {
	t.Helper()
	must(`INSERT INTO dashboards (id, owner_id, organization_id, name, is_default) VALUES ('00000000-0000-4000-8000-0000000000f1', $1, $2, 'Panel', true)`, dcUser, dcOrg)
	assertOrgMigValue(t, env.pool, `SELECT layout ->> 'template' FROM dashboards WHERE name = 'Panel'`, "2", "default layout")
	refuse("a blank name", `INSERT INTO dashboards (owner_id, organization_id, name) VALUES ($1, $2, '   ')`, dcUser, dcOrg)
	refuse("a duplicate name of the same owner", `INSERT INTO dashboards (owner_id, organization_id, name) VALUES ($1, $2, 'PANEL')`, dcUser, dcOrg)
	refuse("a second default of the owner", `INSERT INTO dashboards (owner_id, organization_id, name, is_default) VALUES ($1, $2, 'Otro', true)`, dcUser, dcOrg)
	refuse("a non-object layout", `INSERT INTO dashboards (owner_id, organization_id, name, layout) VALUES ($1, $2, 'Raro', '[]')`, dcUser, dcOrg)
	refuse("a long description", `INSERT INTO dashboards (owner_id, organization_id, name, description) VALUES ($1, $2, 'Largo', repeat('x', 501))`, dcUser, dcOrg)

	must(`INSERT INTO dashboard_gadgets (dashboard_id, type, title, config, position, column_index) VALUES ('00000000-0000-4000-8000-0000000000f1', 'text', 'Nota', '{"markdown": "a"}', 0, 2)`)
	refuse("a fourth column", `INSERT INTO dashboard_gadgets (dashboard_id, type, column_index) VALUES ('00000000-0000-4000-8000-0000000000f1', 'text', 3)`)
	refuse("a negative position", `INSERT INTO dashboard_gadgets (dashboard_id, type, position) VALUES ('00000000-0000-4000-8000-0000000000f1', 'text', -1)`)
	refuse("an array config", `INSERT INTO dashboard_gadgets (dashboard_id, type, config) VALUES ('00000000-0000-4000-8000-0000000000f1', 'text', '[]')`)
	refuse("a gadget of a missing dashboard", `INSERT INTO dashboard_gadgets (dashboard_id, type) VALUES ('00000000-0000-4000-8000-0000000000ff', 'text')`)

	must(`INSERT INTO dashboard_shares (dashboard_id, share_type, target_id) VALUES ('00000000-0000-4000-8000-0000000000f1', 'user', $1)`, dcUser)
	assertOrgMigValue(t, env.pool, `SELECT permission FROM dashboard_shares`, "view", "default permission")
	refuse("an unknown share type", `INSERT INTO dashboard_shares (dashboard_id, share_type, target_id) VALUES ('00000000-0000-4000-8000-0000000000f1', 'group', $1)`, dcUser)
	refuse("an unknown permission", `INSERT INTO dashboard_shares (dashboard_id, share_type, target_id, permission) VALUES ('00000000-0000-4000-8000-0000000000f1', 'team', $1, 'admin')`, dcUser)
	refuse("a repeated share", `INSERT INTO dashboard_shares (dashboard_id, share_type, target_id) VALUES ('00000000-0000-4000-8000-0000000000f1', 'user', $1)`, dcUser)

	must(`INSERT INTO issues (id, project_id, summary, description, status, priority, reporter_id, issue_type, key_number, rank, issue_type_id, priority_id)
		SELECT '00000000-0000-4000-8000-0000000000f2', p.id, 'Epica', '', 'open', 'medium', $1, 'epic', 1, 'm',
		       (SELECT id FROM issue_types WHERE organization_id = p.organization_id AND legacy_key = 'epic'),
		       (SELECT id FROM priorities WHERE organization_id = p.organization_id AND legacy_key = 'medium')
		FROM projects p WHERE p.id = $2`, dcUser, dcProject)
	must(`INSERT INTO roadmap_epic_ranks (project_id, issue_id, position) VALUES ($1, '00000000-0000-4000-8000-0000000000f2', 0)`, dcProject)
	refuse("a negative rank", `INSERT INTO roadmap_epic_ranks (project_id, issue_id, position) VALUES ($1, '00000000-0000-4000-8000-0000000000f2', -1)`, dcProject)

	// Deleting the dashboard removes its gadgets and shares; deleting the issue removes its rank.
	must(`DELETE FROM dashboards WHERE id = '00000000-0000-4000-8000-0000000000f1'`)
	assertOrgMigValue(t, env.pool, `SELECT (SELECT count(*) FROM dashboard_gadgets) + (SELECT count(*) FROM dashboard_shares)::int || ''`, "0", "dashboard cascade")
	must(`DELETE FROM issues WHERE id = '00000000-0000-4000-8000-0000000000f2'`)
	assertOrgMigValue(t, env.pool, `SELECT count(*)::text FROM roadmap_epic_ranks`, "0", "rank cascade")
	assertOrgMigValue(t, env.pool, `SELECT count(*)::text FROM pg_indexes WHERE schemaname = current_schema() AND indexname = 'idx_activities_created_at'`, "1", "activity index")
}

func assertCalendarSchema(t *testing.T, env *orgMigEnv, exec func(string, ...any) error, must func(string, ...any), refuse func(string, string, ...any)) {
	t.Helper()
	const event = "00000000-0000-4000-8000-0000000000a1"
	must(`INSERT INTO calendar_events (id, owner_id, organization_id, title, starts_at, ends_at) VALUES ($1, $2, $3, 'Reunión', '2026-12-07T09:00:00Z', '2026-12-07T10:00:00Z')`, event, dcUser, dcOrg)
	assertOrgMigValue(t, env.pool, `SELECT visibility || '/' || color || '/' || timezone || '/' || recurrence_exceptions::text FROM calendar_events`, "private/blue/UTC/[]", "defaults")
	refuse("a blank title", `INSERT INTO calendar_events (owner_id, organization_id, title, starts_at, ends_at) VALUES ($1, $2, ' ', now(), now())`, dcUser, dcOrg)
	refuse("an end before the start", `INSERT INTO calendar_events (owner_id, organization_id, title, starts_at, ends_at) VALUES ($1, $2, 'x', now(), now() - interval '1 hour')`, dcUser, dcOrg)
	refuse("an unknown visibility", `INSERT INTO calendar_events (owner_id, organization_id, title, starts_at, ends_at, visibility) VALUES ($1, $2, 'x', now(), now(), 'team')`, dcUser, dcOrg)
	refuse("a project event without project", `INSERT INTO calendar_events (owner_id, organization_id, title, starts_at, ends_at, visibility) VALUES ($1, $2, 'x', now(), now(), 'project')`, dcUser, dcOrg)
	refuse("an unknown color", `INSERT INTO calendar_events (owner_id, organization_id, title, starts_at, ends_at, color) VALUES ($1, $2, 'x', now(), now(), 'beige')`, dcUser, dcOrg)
	refuse("non-array exceptions", `INSERT INTO calendar_events (owner_id, organization_id, title, starts_at, ends_at, recurrence_exceptions) VALUES ($1, $2, 'x', now(), now(), '{}')`, dcUser, dcOrg)
	must(`INSERT INTO calendar_events (owner_id, organization_id, project_id, title, starts_at, ends_at, visibility, recurrence_rule) VALUES ($1, $2, $3, 'Del proyecto', now(), now(), 'project', 'FREQ=DAILY')`, dcUser, dcOrg, dcProject)

	must(`INSERT INTO calendar_event_attendees (event_id, user_id) VALUES ($1, $2)`, event, dcUser)
	assertOrgMigValue(t, env.pool, `SELECT response FROM calendar_event_attendees`, "needs_action", "default response")
	refuse("an unknown response", `UPDATE calendar_event_attendees SET response = 'maybe'`)
	refuse("a repeated attendee", `INSERT INTO calendar_event_attendees (event_id, user_id) VALUES ($1, $2)`, event, dcUser)

	must(`INSERT INTO calendar_feed_tokens (user_id, token_hash) VALUES ($1, 'abc')`, dcUser)
	refuse("a second token of the user", `INSERT INTO calendar_feed_tokens (user_id, token_hash) VALUES ($1, 'def')`, dcUser)

	// Deleting the project removes its events; deleting the event removes its guests; deleting the user removes the token.
	must(`DELETE FROM calendar_events WHERE id = $1`, event)
	assertOrgMigValue(t, env.pool, `SELECT count(*)::text FROM calendar_event_attendees`, "0", "attendee cascade")
	must(`UPDATE calendar_events SET deleted_at = now() WHERE title = 'Del proyecto'`)
	assertOrgMigValue(t, env.pool, `SELECT count(*)::text FROM calendar_events WHERE deleted_at IS NULL`, "0", "soft delete")
	// The token hash is unique even across users.
	other := "00000000-0000-4000-8000-0000000000e9"
	must(`INSERT INTO users (id, name, display_name, email, password_hash, role) VALUES ($1, 'dd', 'dd', 'dd@example.test', 'x', 'user') ON CONFLICT DO NOTHING`, other)
	refuse("a token hash shared by two users", `INSERT INTO calendar_feed_tokens (user_id, token_hash) VALUES ($1, 'abc')`, other)
	must(`DELETE FROM users WHERE id = $1`, other)
	must(`DELETE FROM calendar_feed_tokens WHERE user_id = $1`, dcUser)
	must(`DELETE FROM calendar_events`)
}
