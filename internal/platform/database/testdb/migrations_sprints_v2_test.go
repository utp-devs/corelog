package testdb_test

import "testing"

// TestMigrationsSprintsV2RoundTrip applies the sprint history migration over existing sprints, checks the
// best-effort backfill, then runs the down and up migrations again.
func TestMigrationsSprintsV2RoundTrip(t *testing.T) {
	env := newOrgMigEnv(t)
	env.applyRange(t, 1, 38)
	const (
		org     = "00000000-0000-4000-8000-000000000002"
		user    = "00000000-0000-4000-8000-0000000000a1"
		project = "00000000-0000-4000-8000-0000000000b1"
		closed  = "00000000-0000-4000-8000-0000000000c1"
		active  = "00000000-0000-4000-8000-0000000000c2"
		planned = "00000000-0000-4000-8000-0000000000c3"
	)
	exec := func(q string, args ...any) {
		t.Helper()
		if _, err := env.pool.Exec(q, args...); err != nil {
			t.Fatalf("%v\n%s", err, q)
		}
	}
	exec(`INSERT INTO users (id, name, display_name, email, password_hash, role) VALUES ($1, 'u', 'u', 'u@example.test', 'x', 'user')`, user)
	exec(`INSERT INTO projects (id, organization_id, key, name, type, template, lead_id, visibility) VALUES ($1, $2, 'MIG', 'Mig', 'scrum', 'scrum', $3, 'private')`, project, org, user)
	exec(`SELECT corelog_ensure_workflow($1)`, project)
	exec(`INSERT INTO sprints (id, project_id, name, status, start_date, end_date, closed_at) VALUES ($1, $2, 'Viejo', 'closed', CURRENT_DATE - 14, CURRENT_DATE - 1, now() - interval '1 day')`, closed, project)
	exec(`INSERT INTO sprints (id, project_id, name, status, start_date, end_date) VALUES ($1, $2, 'Activo', 'active', CURRENT_DATE - 2, CURRENT_DATE + 10)`, active, project)
	exec(`INSERT INTO sprints (id, project_id, name, status) VALUES ($1, $2, 'Plan', 'planned')`, planned, project)
	insertIssue := func(n int, status, sprint string, points float64) {
		exec(`INSERT INTO issues (id, project_id, summary, description, status, priority, reporter_id, issue_type, key_number, story_points, sprint_id, issue_type_id, priority_id)
			VALUES (gen_random_uuid(), $1, 'i', '', $2, 'medium', $3, 'task', $4, $5, $6,
			        (SELECT id FROM issue_types WHERE organization_id = $7 AND legacy_key = 'task'),
			        (SELECT id FROM priorities WHERE organization_id = $7 AND legacy_key = 'medium'))`,
			project, status, user, n, points, sprint, org)
	}
	insertIssue(1, "closed", closed, 5)
	insertIssue(2, "open", closed, 3)
	insertIssue(3, "in_progress", active, 2)
	insertIssue(4, "open", planned, 8)

	env.runFile(t, 40, ".up.sql")
	count := func(q string) string {
		var v string
		if err := env.pool.QueryRow(q).Scan(&v); err != nil {
			t.Fatalf("%v\n%s", err, q)
		}
		return v
	}
	if got := count(`SELECT committed_points::text || '/' || completed_points::text FROM sprints WHERE id = '` + closed + `'`); got != "8.0/5.0" {
		t.Fatalf("closed sprint points = %s, want 8.0/5.0", got)
	}
	if got := count(`SELECT count(*)::text FROM sprint_issue_events WHERE sprint_id = '` + closed + `'`); got != "4" {
		t.Fatalf("closed sprint events = %s, want 2 added + 2 verdicts", got)
	}
	if got := count(`SELECT count(*)::text FROM sprint_issue_events WHERE sprint_id = '` + closed + `' AND event = 'completed'`); got != "1" {
		t.Fatalf("completed events = %s", got)
	}
	if got := count(`SELECT committed_points::text || '/' || (completed_points IS NULL)::text || '/' || (started_at IS NOT NULL)::text FROM sprints WHERE id = '` + active + `'`); got != "2.0/true/true" {
		t.Fatalf("active sprint = %s", got)
	}
	if got := count(`SELECT count(*)::text FROM sprint_issue_events WHERE sprint_id = '` + planned + `'`); got != "0" {
		t.Fatalf("planned sprint events = %s", got)
	}

	// The trigger records a story point change only while the sprint is active.
	exec(`UPDATE issues SET story_points = 6 WHERE sprint_id = $1`, active)
	exec(`UPDATE issues SET story_points = 9 WHERE sprint_id = $1`, planned)
	if got := count(`SELECT count(*)::text FROM sprint_issue_events WHERE event = 'points_changed'`); got != "1" {
		t.Fatalf("points_changed events = %s, want 1", got)
	}

	env.runFile(t, 40, ".down.sql")
	if got := count(`SELECT count(*)::text FROM information_schema.tables WHERE table_schema = current_schema() AND table_name = 'sprint_issue_events'`); got != "0" {
		t.Fatalf("events table survived the down migration")
	}
	if got := count(`SELECT count(*)::text FROM information_schema.columns WHERE table_schema = current_schema() AND table_name = 'sprints' AND column_name IN ('started_at', 'completed_at', 'committed_points', 'completed_points', 'board_id')`); got != "0" {
		t.Fatalf("columns survived the down migration: %s", got)
	}

	env.runFile(t, 40, ".up.sql")
	if got := count(`SELECT count(*)::text FROM sprint_issue_events`); got == "0" {
		t.Fatal("backfill must run again after the second up")
	}
}
