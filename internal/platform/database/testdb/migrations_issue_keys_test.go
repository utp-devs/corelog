package testdb_test

import "testing"

const (
	issueKeyProjectB = "00000000-0000-4000-8000-0000000000c2"
	issueKeyOldest   = "00000000-0000-4000-8000-0000000000f3"
	issueKeyMiddle   = "00000000-0000-4000-8000-0000000000f2"
	issueKeyNewest   = "00000000-0000-4000-8000-0000000000f1"
	issueKeyOther    = "00000000-0000-4000-8000-0000000000f4"
)

// TestMigrationsIssueKeysBackfillAndRoundTrip applies everything before 000029 on legacy rows, then checks the
// per-project numbering by creation time, the project counters, the renamed columns, and a down/up cycle.
func TestMigrationsIssueKeysBackfillAndRoundTrip(t *testing.T) {
	env := newOrgMigEnv(t)
	env.applyRange(t, 1, 28)
	seedOrgMigLegacy(t, env.pool, true)

	// Project DEMO already holds two issues created at the same instant (tie broken by id). Add three more with
	// explicit, out-of-id-order creation times, plus one issue in another project.
	stmts := []string{
		`INSERT INTO projects (id, key, name, lead_id, organization_id) VALUES ('` + issueKeyProjectB + `', 'OTHR', 'Other', '` + orgMigAdminID + `', '` + orgMigDefaultOrgID + `')`,
		`INSERT INTO issues (id, title, description, status, priority, requester_id, project_id, created_at) VALUES
			('` + issueKeyNewest + `', 'Newest', 'd', 'open', 'low', '` + orgMigRequesterID + `', '` + orgMigProjectID + `', now() + interval '2 days'),
			('` + issueKeyMiddle + `', 'Middle', 'd', 'open', 'low', '` + orgMigRequesterID + `', '` + orgMigProjectID + `', now() + interval '1 day'),
			('` + issueKeyOldest + `', 'Oldest', 'd', 'open', 'low', '` + orgMigRequesterID + `', '` + orgMigProjectID + `', now() - interval '5 days'),
			('` + issueKeyOther + `', 'Elsewhere', 'd', 'open', 'low', '` + orgMigRequesterID + `', '` + issueKeyProjectB + `', now())`,
	}
	for _, stmt := range stmts {
		if _, err := env.pool.Exec(stmt); err != nil {
			t.Fatalf("seed: %v\n%s", err, stmt)
		}
	}

	env.runFile(t, 29, ".up.sql")

	number := func(id string) string {
		var n string
		if err := env.pool.QueryRow(`SELECT key_number::text FROM issues WHERE id = $1`, id).Scan(&n); err != nil {
			t.Fatalf("key_number of %s: %v", id, err)
		}
		return n
	}
	// Legacy rows e1 and e2 share created_at: id order decides. Oldest comes first, then e1, e2, Middle, Newest.
	want := map[string]string{
		issueKeyOldest:                         "1",
		"00000000-0000-4000-8000-0000000000e1": "2",
		"00000000-0000-4000-8000-0000000000e2": "3",
		issueKeyMiddle:                         "4",
		issueKeyNewest:                         "5",
		issueKeyOther:                          "1",
	}
	for id, n := range want {
		if got := number(id); got != n {
			t.Errorf("key_number of %s = %s, want %s", id, got, n)
		}
	}
	assertOrgMigValue(t, env.pool, `SELECT next_issue_number::text FROM projects WHERE id = $1`, "6", "DEMO counter", orgMigProjectID)
	assertOrgMigValue(t, env.pool, `SELECT next_issue_number::text FROM projects WHERE id = $1`, "2", "OTHR counter", issueKeyProjectB)
	assertOrgMigValue(t, env.pool, `SELECT summary FROM issues WHERE id = $1`, "Oldest", "title renamed to summary", issueKeyOldest)
	assertOrgMigValue(t, env.pool, `SELECT reporter_id::text FROM issues WHERE id = $1`, orgMigRequesterID, "requester_id renamed to reporter_id", issueKeyOldest)

	// The number is unique per project and required.
	if _, err := env.pool.Exec(`UPDATE issues SET key_number = 1 WHERE id = $1`, issueKeyNewest); err == nil {
		t.Error("duplicate (project_id, key_number) was accepted")
	}
	if _, err := env.pool.Exec(`UPDATE issues SET key_number = NULL WHERE id = $1`, issueKeyNewest); err == nil {
		t.Error("NULL key_number was accepted")
	}

	// Down restores the old names and drops the number; up again renumbers identically.
	env.runFile(t, 29, ".down.sql")
	assertOrgMigValue(t, env.pool, `SELECT title FROM issues WHERE id = $1`, "Oldest", "summary renamed back", issueKeyOldest)
	assertOrgMigValue(t, env.pool, `SELECT requester_id::text FROM issues WHERE id = $1`, orgMigRequesterID, "reporter_id renamed back", issueKeyOldest)
	if got := countOrgMigWhere(t, env.pool, `information_schema.columns`, `table_name = 'issues' AND column_name = 'key_number' AND table_schema = current_schema()`); got != 0 {
		t.Errorf("key_number still present after down")
	}
	env.runFile(t, 29, ".up.sql")
	for id, n := range want {
		if got := number(id); got != n {
			t.Errorf("after up/down/up key_number of %s = %s, want %s", id, got, n)
		}
	}

	// Running the up migration again on an already migrated schema changes nothing.
	env.runFile(t, 29, ".up.sql")
	assertOrgMigValue(t, env.pool, `SELECT next_issue_number::text FROM projects WHERE id = $1`, "6", "DEMO counter after re-run", orgMigProjectID)
}
