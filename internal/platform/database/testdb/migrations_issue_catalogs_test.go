package testdb_test

import (
	"strconv"
	"testing"
)

const (
	catalogIssueCritical = "00000000-0000-4000-8000-0000000000d1"
	catalogIssueDone     = "00000000-0000-4000-8000-0000000000d2"
	catalogIssueOpen     = "00000000-0000-4000-8000-0000000000d3"
	catalogIssueLowSub   = "00000000-0000-4000-8000-0000000000d4"
)

// TestMigrationsIssueCatalogsBackfillAndRoundTrip seeds legacy issues before the catalog migration, checks that
// every issue gets a catalog item that mirrors its legacy enum values, and runs a down and up cycle.
func TestMigrationsIssueCatalogsBackfillAndRoundTrip(t *testing.T) {
	env := newOrgMigEnv(t)
	env.applyRange(t, 1, 28)
	seedOrgMigLegacy(t, env.pool, true)
	env.runFile(t, 29, ".up.sql")
	env.runFile(t, 30, ".up.sql")

	legacy := []struct {
		id, status, priority, issueType string
	}{
		{catalogIssueCritical, "closed", "critical", "epic"},
		{catalogIssueDone, "resolved", "high", "story"},
		{catalogIssueOpen, "open", "medium", "bug"},
		{catalogIssueLowSub, "in_progress", "low", "subtask"},
		{"00000000-0000-4000-8000-0000000000d5", "open", "low", "task"},
	}
	for i, row := range legacy {
		if _, err := env.pool.Exec(`
			INSERT INTO issues (id, summary, description, status, priority, issue_type, reporter_id, project_id,
			                    key_number, created_at, updated_at, closed_at)
			VALUES ($1, 'Legacy', 'd', $2, $3, $4, $5, $6, $7, now() - interval '3 days', now() - interval '1 day',
			        CASE WHEN $8::boolean THEN now() - interval '2 days' END)`,
			row.id, row.status, row.priority, row.issueType, orgMigRequesterID, orgMigProjectID, 100+i, row.status == "closed"); err != nil {
			t.Fatalf("seed legacy issue %s: %v", row.id, err)
		}
	}

	env.runFile(t, 31, ".up.sql")
	assertIssueCatalogMapping(t, env)

	env.runFile(t, 31, ".down.sql")
	if got := countOrgMigWhere(t, env.pool, `information_schema.tables`,
		`table_name = 'issue_types' AND table_schema = current_schema()`); got != 0 {
		t.Fatalf("issue_types still exists after the down migration")
	}
	assertOrgMigValue(t, env.pool, `SELECT issue_type FROM issues WHERE id = $1`, "epic",
		"legacy enum kept after down", catalogIssueCritical)

	env.runFile(t, 31, ".up.sql")
	assertIssueCatalogMapping(t, env)
}

// assertIssueCatalogMapping checks that every issue points at the catalog items that carry its legacy values, and
// that done issues have the default resolution and a resolution time.
func assertIssueCatalogMapping(t *testing.T, env *orgMigEnv) {
	t.Helper()
	total := countOrgMig(t, env.pool, "issues")
	if total == 0 {
		t.Fatal("no issues to check")
	}
	assertOrgMigValue(t, env.pool, `
		SELECT count(*)::text FROM issues i
		JOIN issue_types it ON it.id = i.issue_type_id
		WHERE it.legacy_key = i.issue_type`,
		strconv.Itoa(total), "issues with a matching issue type")
	assertOrgMigValue(t, env.pool, `
		SELECT count(*)::text FROM issues i
		JOIN priorities pr ON pr.id = i.priority_id
		WHERE pr.legacy_key = i.priority`,
		strconv.Itoa(total), "issues with a matching priority")
	assertOrgMigValue(t, env.pool, `
		SELECT pr.name FROM issues i JOIN priorities pr ON pr.id = i.priority_id WHERE i.id = $1`,
		"Máxima", "critical issue mapped to the highest priority", catalogIssueCritical)
	assertOrgMigValue(t, env.pool, `
		SELECT count(*)::text FROM issues
		WHERE id IN ($1, $2) AND resolution_id IS NOT NULL AND resolved_at IS NOT NULL`,
		"2", "done legacy issues with a resolution", catalogIssueCritical, catalogIssueDone)
	assertOrgMigValue(t, env.pool, `
		SELECT count(*)::text FROM issues
		WHERE status NOT IN ('resolved', 'closed') AND (resolution_id IS NOT NULL OR resolved_at IS NOT NULL)`,
		"0", "open issues without a resolution")
}
