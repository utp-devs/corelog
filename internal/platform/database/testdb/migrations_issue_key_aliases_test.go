package testdb_test

import "testing"

// TestMigrationsIssueKeyAliasesRoundTrip applies the key alias migration on legacy data, checks its keys and
// foreign keys, then runs the down and up migrations again.
func TestMigrationsIssueKeyAliasesRoundTrip(t *testing.T) {
	env := newOrgMigEnv(t)
	env.applyRange(t, 1, 28)
	seedOrgMigLegacy(t, env.pool, true)
	for version := 29; version <= 35; version++ {
		env.runFile(t, version, ".up.sql")
	}
	env.runFile(t, 36, ".up.sql")

	assertOrgMigValue(t, env.pool, `SELECT count(*)::text FROM information_schema.tables WHERE table_schema = current_schema() AND table_name = 'issue_key_aliases'`, "1", "alias table created")
	if _, err := env.pool.Exec(`INSERT INTO issue_key_aliases (source_project_id, key_number, issue_id) VALUES ($1, 1, $2)`, orgMigProjectID, "00000000-0000-4000-8000-0000000000ff"); err == nil {
		t.Fatal("alias to a missing issue was accepted")
	}
	if _, err := env.pool.Exec(`INSERT INTO issue_key_aliases (source_project_id, key_number, issue_id) VALUES ($1, 1, $2)`, orgMigProjectID, issueCoreFirstIssue); err != nil {
		t.Fatalf("alias rejected: %v", err)
	}
	if _, err := env.pool.Exec(`INSERT INTO issue_key_aliases (source_project_id, key_number, issue_id) VALUES ($1, 1, $2)`, orgMigProjectID, issueCoreFirstIssue); err == nil {
		t.Fatal("duplicate old key was accepted")
	}
	if _, err := env.pool.Exec(`INSERT INTO issue_key_aliases (source_project_id, key_number, issue_id) VALUES ($1, 0, $2)`, orgMigProjectID, issueCoreFirstIssue); err == nil {
		t.Fatal("non-positive key number was accepted")
	}

	env.runFile(t, 36, ".down.sql")
	assertOrgMigValue(t, env.pool, `SELECT count(*)::text FROM information_schema.tables WHERE table_schema = current_schema() AND table_name = 'issue_key_aliases'`, "0", "alias table removed after down")
	env.runFile(t, 36, ".up.sql")
	assertOrgMigValue(t, env.pool, `SELECT count(*)::text FROM information_schema.tables WHERE table_schema = current_schema() AND table_name = 'issue_key_aliases'`, "1", "alias table back after up again")
}
