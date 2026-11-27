package testdb_test

import "testing"

// TestMigrationsWorkflowCategorySyncRoundTrip applies the category sync migration, checks that the sync trigger
// honours the transaction-local flag, then runs the down and up migrations again.
func TestMigrationsWorkflowCategorySyncRoundTrip(t *testing.T) {
	env := newOrgMigEnv(t)
	env.applyRange(t, 1, 38)
	const honours = `SELECT (prosrc LIKE '%corelog.keep_status_id%')::text FROM pg_proc
		WHERE proname = 'corelog_sync_issue_status' AND pronamespace::regnamespace::text = current_schema()`
	assertOrgMigValue(t, env.pool, honours, "true", "trigger honours the category sync flag after up")

	env.runFile(t, 38, ".down.sql")
	assertOrgMigValue(t, env.pool, honours, "false", "trigger restored by the down migration")

	env.runFile(t, 38, ".up.sql")
	assertOrgMigValue(t, env.pool, honours, "true", "trigger honours the category sync flag after up again")
}
