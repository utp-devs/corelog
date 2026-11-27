package testdb_test

import "testing"

// TestMigrationsVersionsComponentsRoundTrip applies the versions migration over an existing project with a
// component, checks the constraints and the component defaults, then runs down and up again.
func TestMigrationsVersionsComponentsRoundTrip(t *testing.T) {
	env := newOrgMigEnv(t)
	env.applyRange(t, 1, 40)
	const (
		org       = "00000000-0000-4000-8000-000000000002"
		user      = "00000000-0000-4000-8000-0000000000a2"
		project   = "00000000-0000-4000-8000-0000000000b2"
		component = "00000000-0000-4000-8000-0000000000e1"
		version   = "00000000-0000-4000-8000-0000000000f1"
	)
	exec := func(q string, args ...any) {
		t.Helper()
		if _, err := env.pool.Exec(q, args...); err != nil {
			t.Fatalf("%v\n%s", err, q)
		}
	}
	exec(`INSERT INTO users (id, name, display_name, email, password_hash, role) VALUES ($1, 'v', 'v', 'v@example.test', 'x', 'user')`, user)
	exec(`INSERT INTO projects (id, organization_id, key, name, type, template, lead_id, visibility) VALUES ($1, $2, 'VER', 'Ver', 'scrum', 'scrum', $3, 'private')`, project, org, user)
	exec(`SELECT corelog_ensure_workflow($1)`, project)
	exec(`INSERT INTO components (id, project_id, name) VALUES ($1, $2, 'Pagos')`, component, project)

	env.runFile(t, 41, ".up.sql")
	assertOrgMigValue(t, env.pool, `SELECT count(*)::text FROM information_schema.tables WHERE table_schema = current_schema() AND table_name IN ('versions', 'issue_versions')`,
		"2", "version tables created")
	assertOrgMigValue(t, env.pool, `SELECT default_assignee_policy || '/' || (lead_id IS NULL)::text FROM components WHERE id = $1`,
		"none/true", "existing components default to no policy and no lead", component)

	exec(`INSERT INTO versions (id, project_id, name) VALUES ($1, $2, '1.0')`, version, project)
	if _, err := env.pool.Exec(`INSERT INTO versions (project_id, name) VALUES ($1, '1.0')`, project); err == nil {
		t.Error("duplicate version name accepted")
	}
	exec(`INSERT INTO versions (id, project_id, name) VALUES (gen_random_uuid(), $1, 'Alpha')`, project)
	if _, err := env.pool.Exec(`INSERT INTO versions (project_id, name) VALUES ($1, 'ALPHA')`, project); err == nil {
		t.Error("duplicate version name (case-insensitive) accepted")
	}
	if _, err := env.pool.Exec(`INSERT INTO versions (project_id, name, status) VALUES ($1, '2.0', 'frozen')`, project); err == nil {
		t.Error("unknown version status accepted")
	}
	if _, err := env.pool.Exec(`UPDATE components SET default_assignee_policy = 'random' WHERE id = $1`, component); err == nil {
		t.Error("unknown assignee policy accepted")
	}
	exec(`UPDATE components SET lead_id = $2, default_assignee_policy = 'component_lead' WHERE id = $1`, component, user)
	if _, err := env.pool.Exec(`INSERT INTO issue_versions (issue_id, version_id, kind) VALUES ('00000000-0000-4000-8000-0000000000ff', $1, 'fix')`, version); err == nil {
		t.Error("link to a missing issue accepted")
	}

	env.runFile(t, 41, ".down.sql")
	assertOrgMigValue(t, env.pool, `SELECT count(*)::text FROM information_schema.tables WHERE table_schema = current_schema() AND table_name IN ('versions', 'issue_versions')`,
		"0", "version tables removed by the down")
	assertOrgMigValue(t, env.pool, `SELECT count(*)::text FROM information_schema.columns WHERE table_schema = current_schema() AND table_name = 'components' AND column_name IN ('lead_id', 'default_assignee_policy')`,
		"0", "component columns removed by the down")
	assertOrgMigValue(t, env.pool, `SELECT count(*)::text FROM components WHERE id = $1`, "1", "components survive the down", component)

	env.runFile(t, 41, ".up.sql")
	assertOrgMigValue(t, env.pool, `SELECT count(*)::text FROM information_schema.tables WHERE table_schema = current_schema() AND table_name IN ('versions', 'issue_versions')`,
		"2", "version tables back after the second up")
}
