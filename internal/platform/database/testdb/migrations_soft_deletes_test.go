package testdb_test

import "testing"

var softDeleteTables = []string{
	"api_tokens", "webhooks", "automation_rules", "attachments", "issue_links", "saved_filters", "custom_fields",
	"filters", "dashboards", "labels", "components", "versions", "boards", "sprints", "teams", "issue_types",
	"priorities", "resolutions",
}

// TestMigrationsSoftDeletesRoundTrip applies the logical deletion migration, takes it down and up again, and
// checks the columns and the partial unique indexes after every step.
func TestMigrationsSoftDeletesRoundTrip(t *testing.T) {
	env := newOrgMigEnv(t)
	env.applyRange(t, 1, 56)

	columns := func() int {
		n := 0
		for _, table := range softDeleteTables {
			n += countOrgMigWhere(t, env.pool, `information_schema.columns`,
				`table_schema = current_schema() AND table_name = $1 AND column_name = 'deleted_at'`, table)
		}
		return n
	}
	// Unique indexes that exclude deleted rows: every one of them must be partial on deleted_at.
	liveUnique := func() int {
		return countOrgMigWhere(t, env.pool, `pg_index x JOIN pg_class c ON c.oid = x.indexrelid`,
			`c.relnamespace = current_schema()::regnamespace AND x.indisunique AND x.indpred IS NOT NULL
			 AND pg_get_expr(x.indpred, x.indrelid) LIKE '%deleted_at IS NULL%'`)
	}
	inlineUnique := func() int {
		return countOrgMigWhere(t, env.pool, `pg_constraint`,
			`contype = 'u' AND conrelid IN (SELECT c.oid FROM pg_class c JOIN pg_namespace n ON n.oid = c.relnamespace
			 WHERE n.nspname = current_schema() AND c.relname IN ('labels', 'components', 'issue_links', 'saved_filters', 'teams'))`)
	}

	if got := columns(); got != 0 {
		t.Fatalf("deleted_at columns before up = %d, want 0", got)
	}
	baseLive := liveUnique()
	beforeInline := inlineUnique()
	if beforeInline != 5 {
		t.Fatalf("inline unique constraints before up = %d, want 5", beforeInline)
	}

	env.runFile(t, 58, ".up.sql")
	if got := columns(); got != len(softDeleteTables) {
		t.Fatalf("deleted_at columns after up = %d, want %d", got, len(softDeleteTables))
	}
	if got := inlineUnique(); got != 0 {
		t.Fatalf("inline unique constraints after up = %d, want 0", got)
	}
	upLive := liveUnique()
	if upLive < baseLive+20 {
		t.Fatalf("partial unique indexes after up = %d, want at least 20", upLive)
	}

	env.runFile(t, 58, ".down.sql")
	if got := columns(); got != 0 {
		t.Fatalf("deleted_at columns after down = %d, want 0", got)
	}
	if got := inlineUnique(); got != beforeInline {
		t.Fatalf("inline unique constraints after down = %d, want %d", got, beforeInline)
	}
	if got := liveUnique(); got != baseLive {
		t.Fatalf("partial unique indexes after down = %d, want %d", got, baseLive)
	}

	env.runFile(t, 58, ".up.sql")
	if got := columns(); got != len(softDeleteTables) {
		t.Fatalf("deleted_at columns after up again = %d, want %d", got, len(softDeleteTables))
	}
	if got := liveUnique(); got != upLive {
		t.Fatalf("partial unique indexes after up again = %d, want %d", got, upLive)
	}
}
