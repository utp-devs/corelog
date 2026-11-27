package testdb_test

import "testing"

const issueCoreFirstIssue = "00000000-0000-4000-8000-0000000000e1"

// TestMigrationsIssueCoreRoundTrip applies the schema up to the issue core migration on legacy data, checks the
// new columns, constraints and the date conversion, then runs the down and up migrations again.
func TestMigrationsIssueCoreRoundTrip(t *testing.T) {
	env := newOrgMigEnv(t)
	env.applyRange(t, 1, 28)
	seedOrgMigLegacy(t, env.pool, true)
	for version := 29; version <= 33; version++ {
		env.runFile(t, version, ".up.sql")
	}
	if _, err := env.pool.Exec(`UPDATE issues SET due_date = '2026-05-01 23:30:00+00', story_points = 12.5 WHERE id = $1`, issueCoreFirstIssue); err != nil {
		t.Fatalf("prepare legacy issue: %v", err)
	}

	env.runFile(t, 34, ".up.sql")
	assertOrgMigValue(t, env.pool, `SELECT due_date::text FROM issues WHERE id = $1`, "2026-05-01", "due date converted to a date", issueCoreFirstIssue)
	assertOrgMigValue(t, env.pool, `SELECT story_points::text FROM issues WHERE id = $1`, "12.5", "story points kept", issueCoreFirstIssue)
	assertOrgMigValue(t, env.pool, `SELECT flagged::text || vote_count::text FROM issues WHERE id = $1`, "false0", "defaults for the new columns", issueCoreFirstIssue)
	assertOrgMigValue(t, env.pool, `SELECT count(*)::text FROM issues WHERE deleted_at IS NOT NULL`, "0", "no issue starts deleted")
	assertOrgMigValue(t, env.pool, `SELECT data_type FROM information_schema.columns WHERE table_name = 'issues' AND column_name = 'due_date' AND table_schema = current_schema()`, "date", "due_date type")
	assertOrgMigValue(t, env.pool, `SELECT count(*)::text FROM information_schema.columns WHERE table_schema = current_schema() AND table_name = 'activities' AND column_name = 'batch_id'`, "1", "activity batch column")

	if _, err := env.pool.Exec(`UPDATE issues SET start_date = '2026-06-01', due_date = '2026-05-01' WHERE id = $1`, issueCoreFirstIssue); err == nil {
		t.Fatal("start after due was accepted")
	}
	if _, err := env.pool.Exec(`UPDATE issues SET story_points = 999.6 WHERE id = $1`, issueCoreFirstIssue); err == nil {
		t.Fatal("story points above 999.5 were accepted")
	}
	if _, err := env.pool.Exec(`UPDATE issues SET original_estimate_seconds = -1 WHERE id = $1`, issueCoreFirstIssue); err == nil {
		t.Fatal("negative estimate was accepted")
	}
	if _, err := env.pool.Exec(`UPDATE issues SET story_points = 999.5, original_estimate_seconds = 3600, deleted_at = now() WHERE id = $1`, issueCoreFirstIssue); err != nil {
		t.Fatalf("valid values rejected: %v", err)
	}

	env.runFile(t, 34, ".down.sql")
	assertOrgMigValue(t, env.pool, `SELECT data_type FROM information_schema.columns WHERE table_name = 'issues' AND column_name = 'due_date' AND table_schema = current_schema()`, "timestamp with time zone", "due_date type after down")
	assertOrgMigValue(t, env.pool, `SELECT count(*)::text FROM information_schema.columns WHERE table_schema = current_schema() AND table_name = 'issues' AND column_name IN ('deleted_at','flagged','vote_count','start_date','original_estimate_seconds','remaining_estimate_seconds')`, "0", "new issue columns removed")
	assertOrgMigValue(t, env.pool, `SELECT count(*)::text FROM information_schema.columns WHERE table_schema = current_schema() AND table_name = 'activities' AND column_name = 'batch_id'`, "0", "activity batch column removed")
	assertOrgMigValue(t, env.pool, `SELECT story_points::text FROM issues WHERE id = $1`, "999.5", "story points survive the down migration", issueCoreFirstIssue)

	env.runFile(t, 34, ".up.sql")
	assertOrgMigValue(t, env.pool, `SELECT data_type FROM information_schema.columns WHERE table_name = 'issues' AND column_name = 'due_date' AND table_schema = current_schema()`, "date", "due_date type after up again")
}
