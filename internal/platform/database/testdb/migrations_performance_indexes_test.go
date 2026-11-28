package testdb_test

import "testing"

// TestMigrationsPerformanceIndexesRoundTrip applies every migration, checks the performance indexes, then runs the
// down and up migrations again.
func TestMigrationsPerformanceIndexesRoundTrip(t *testing.T) {
	env := newOrgMigEnv(t)
	env.applyRange(t, 1, 56)

	indexCount := func() int {
		return countOrgMigWhere(t, env.pool, `pg_indexes`,
			`schemaname = current_schema() AND indexname IN ('idx_issues_start_date_live', 'idx_issues_description_trgm')`)
	}
	// The trigram index needs pg_trgm; the start date index never does.
	wantWithTrgm := countOrgMigWhere(t, env.pool, `pg_extension`, `extname = 'pg_trgm'`)*1 + 1
	if got := indexCount(); got != wantWithTrgm {
		t.Fatalf("indexes after up = %d, want %d", got, wantWithTrgm)
	}

	env.runFile(t, 56, ".down.sql")
	if got := indexCount(); got != 0 {
		t.Fatalf("indexes after down = %d, want 0", got)
	}
	env.runFile(t, 56, ".up.sql")
	if got := indexCount(); got != wantWithTrgm {
		t.Fatalf("indexes after up again = %d, want %d", got, wantWithTrgm)
	}
	// Running the up migration twice is harmless.
	env.runFile(t, 56, ".up.sql")
	if got := indexCount(); got != wantWithTrgm {
		t.Fatalf("indexes after a second up = %d, want %d", got, wantWithTrgm)
	}
}
