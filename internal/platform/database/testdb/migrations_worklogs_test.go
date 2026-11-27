package testdb_test

import "testing"

// TestMigrationsWorklogsRoundTrip applies the worklog migration over an issue, checks the constraints and the
// aggregate column, then runs the down and up migration again.
func TestMigrationsWorklogsRoundTrip(t *testing.T) {
	env := newOrgMigEnv(t)
	env.applyRange(t, 1, 42)
	const (
		org     = "00000000-0000-4000-8000-000000000002"
		author  = "00000000-0000-4000-8000-0000000000c1"
		project = "00000000-0000-4000-8000-0000000000c2"
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
	exec(`INSERT INTO users (id, name, display_name, email, password_hash, role) VALUES ($1, 'wl', 'wl', 'wl@example.test', 'x', 'user')`, author)
	exec(`INSERT INTO organization_members (organization_id, user_id, role) VALUES ($1, $2, 'member')`, org, author)
	exec(`INSERT INTO projects (id, organization_id, key, name, type, template, lead_id, visibility) VALUES ($1, $2, 'WLG', 'Worklogs', 'scrum', 'scrum', $3, 'open')`, project, org, author)
	exec(`INSERT INTO issues (id, project_id, summary, description, status, priority, reporter_id, issue_type, key_number, rank, issue_type_id, priority_id)
		SELECT '00000000-0000-4000-8000-0000000000c3', p.id, 'Tiempo', '', 'open', 'medium', $1, 'task', 1, 'm',
		       (SELECT id FROM issue_types WHERE organization_id = p.organization_id AND legacy_key = 'task'),
		       (SELECT id FROM priorities WHERE organization_id = p.organization_id AND legacy_key = 'medium')
		FROM projects p WHERE p.id = $2`, author, project)

	env.runFile(t, 43, ".up.sql")

	if got := scalar(`SELECT count(*)::text FROM issues WHERE time_spent_seconds = 0`); got != "1" {
		t.Fatalf("aggregate default = %s", got)
	}
	insert := `INSERT INTO worklogs (issue_id, author_id, started_at, time_spent_seconds, description, remaining_mode)
		VALUES ('00000000-0000-4000-8000-0000000000c3', $1, now(), $2, '', $3)`
	exec(insert, author, 3600, "auto")
	for _, bad := range []struct {
		seconds int64
		mode    string
	}{{0, "auto"}, {259201, "auto"}, {60, "bogus"}} {
		if _, err := env.pool.Exec(insert, author, bad.seconds, bad.mode); err == nil {
			t.Fatalf("invalid worklog accepted: %+v", bad)
		}
	}
	for _, idx := range []string{"idx_worklogs_issue_id", "idx_worklogs_author_started"} {
		if got := scalar(`SELECT count(*)::text FROM pg_indexes WHERE schemaname = current_schema() AND indexname = '` + idx + `'`); got != "1" {
			t.Fatalf("index %s missing", idx)
		}
	}

	env.runFile(t, 43, ".down.sql")
	if got := scalar(`SELECT count(*)::text FROM information_schema.tables WHERE table_schema = current_schema() AND table_name = 'worklogs'`); got != "0" {
		t.Fatalf("worklogs table survived the down migration")
	}
	if got := scalar(`SELECT count(*)::text FROM information_schema.columns WHERE table_schema = current_schema() AND table_name = 'issues' AND column_name = 'time_spent_seconds'`); got != "0" {
		t.Fatalf("aggregate column survived the down migration")
	}

	env.runFile(t, 43, ".up.sql")
	exec(insert, author, 60, "leave")
}
