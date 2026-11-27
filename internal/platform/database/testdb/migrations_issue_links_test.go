package testdb_test

import "testing"

const issueLinksSecondIssue = "00000000-0000-4000-8000-0000000000e2"

// TestMigrationsIssueLinksVotesRoundTrip applies the link and vote migration on legacy data, checks the new kinds,
// the self link rule, the vote primary key and counter backfill, then runs the down and up migrations again.
func TestMigrationsIssueLinksVotesRoundTrip(t *testing.T) {
	env := newOrgMigEnv(t)
	env.applyRange(t, 1, 28)
	seedOrgMigLegacy(t, env.pool, true)
	for version := 29; version <= 35; version++ {
		env.runFile(t, version, ".up.sql")
	}

	assertOrgMigValue(t, env.pool, `SELECT count(*)::text FROM information_schema.tables WHERE table_schema = current_schema() AND table_name = 'issue_votes'`, "1", "votes table created")
	assertOrgMigValue(t, env.pool, `SELECT vote_count::text FROM issues WHERE id = $1`, "0", "vote counter starts at zero", issueCoreFirstIssue)

	if _, err := env.pool.Exec(`INSERT INTO issue_links (source_issue_id, target_issue_id, kind, created_by) VALUES ($1, $1, 'relates', $2)`, issueCoreFirstIssue, orgMigAdminID); err == nil {
		t.Fatal("self link was accepted")
	}
	if _, err := env.pool.Exec(`INSERT INTO issue_links (source_issue_id, target_issue_id, kind, created_by) VALUES ($1, $2, 'bogus', $3)`, issueCoreFirstIssue, issueLinksSecondIssue, orgMigAdminID); err == nil {
		t.Fatal("unknown link kind was accepted")
	}
	for _, kind := range []string{"blocks", "relates", "duplicates", "clones", "causes", "implements"} {
		if _, err := env.pool.Exec(`INSERT INTO issue_links (source_issue_id, target_issue_id, kind, created_by) VALUES ($1, $2, $3, $4)`, issueCoreFirstIssue, issueLinksSecondIssue, kind, orgMigAdminID); err != nil {
			t.Fatalf("kind %s rejected: %v", kind, err)
		}
		if _, err := env.pool.Exec(`DELETE FROM issue_links WHERE source_issue_id = $1 AND target_issue_id = $2 AND kind = $3`, issueCoreFirstIssue, issueLinksSecondIssue, kind); err != nil {
			t.Fatalf("cleanup link %s: %v", kind, err)
		}
	}

	if _, err := env.pool.Exec(`INSERT INTO issue_votes (issue_id, user_id) VALUES ($1, $2)`, issueCoreFirstIssue, orgMigAdminID); err != nil {
		t.Fatalf("vote rejected: %v", err)
	}
	if _, err := env.pool.Exec(`INSERT INTO issue_votes (issue_id, user_id) VALUES ($1, $2)`, issueCoreFirstIssue, orgMigAdminID); err == nil {
		t.Fatal("duplicate vote was accepted")
	}
	if _, err := env.pool.Exec(`DELETE FROM issue_votes WHERE issue_id = $1`, issueCoreFirstIssue); err != nil {
		t.Fatalf("cleanup votes: %v", err)
	}

	env.runFile(t, 35, ".down.sql")
	assertOrgMigValue(t, env.pool, `SELECT count(*)::text FROM information_schema.tables WHERE table_schema = current_schema() AND table_name = 'issue_votes'`, "0", "votes table removed after down")
	if _, err := env.pool.Exec(`INSERT INTO issue_links (source_issue_id, target_issue_id, kind, created_by) VALUES ($1, $2, 'clones', $3)`, issueCoreFirstIssue, issueLinksSecondIssue, orgMigAdminID); err == nil {
		t.Fatal("clones link accepted after down")
	}

	env.runFile(t, 35, ".up.sql")
	assertOrgMigValue(t, env.pool, `SELECT count(*)::text FROM information_schema.tables WHERE table_schema = current_schema() AND table_name = 'issue_votes'`, "1", "votes table back after up again")
	if _, err := env.pool.Exec(`INSERT INTO issue_links (source_issue_id, target_issue_id, kind, created_by) VALUES ($1, $2, 'implements', $3)`, issueCoreFirstIssue, issueLinksSecondIssue, orgMigAdminID); err != nil {
		t.Fatalf("implements link rejected after up again: %v", err)
	}
}
