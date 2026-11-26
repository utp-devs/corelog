package main

import (
	"context"
	"errors"
	"io"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"

	"github.com/jhonsferg/corelog/internal/platform/database/testdb"
)

// testOptions returns options with a random throwaway password. Nothing here
// hard-codes a credential.
func testOptions(t *testing.T) Options {
	t.Helper()
	return Options{Password: uuid.NewString(), Out: io.Discard}
}

func countRows(t *testing.T, db *sqlx.DB, table string) int {
	t.Helper()
	var n int
	if err := db.Get(&n, "SELECT count(*) FROM "+table); err != nil {
		t.Fatalf("count %s: %v", table, err)
	}
	return n
}

func runScenario(t *testing.T, db *sqlx.DB, name string, opts Options) error {
	t.Helper()
	sc, err := lookupScenario(name)
	if err != nil {
		t.Fatal(err)
	}
	return Execute(context.Background(), db, sc, opts)
}

func TestSeedRefusesNonEmptyDatabaseUntilForceReset(t *testing.T) {
	db := testdb.New(t)
	opts := testOptions(t)

	if err := runScenario(t, db, "e2e", opts); err != nil {
		t.Fatalf("first e2e seed: %v", err)
	}
	if got := countRows(t, db, "users"); got != 4 {
		t.Fatalf("users after first seed = %d, want 4", got)
	}

	err := runScenario(t, db, "e2e", opts)
	if !errors.Is(err, errNotEmpty) {
		t.Fatalf("second seed error = %v, want errNotEmpty", err)
	}
	if got := countRows(t, db, "users"); got != 4 {
		t.Fatalf("refused seed changed users: %d", got)
	}

	opts.ForceReset = true
	if err := runScenario(t, db, "e2e", opts); err != nil {
		t.Fatalf("force-reset seed: %v", err)
	}
	if got := countRows(t, db, "users"); got != 4 {
		t.Errorf("users after force-reset = %d, want 4", got)
	}
	if got := countRows(t, db, "issues"); got != len(e2eIssues) {
		t.Errorf("issues after force-reset = %d, want %d", got, len(e2eIssues))
	}
	if got := countRows(t, db, "teams"); got != 1 {
		t.Errorf("teams after force-reset = %d, want 1", got)
	}
}

func TestDemoScenarioProducesUsableData(t *testing.T) {
	db := testdb.New(t)
	if err := runScenario(t, db, "demo", testOptions(t)); err != nil {
		t.Fatalf("demo seed: %v", err)
	}
	if got := countRows(t, db, "users"); got < 20 {
		t.Errorf("users = %d, want >= 20", got)
	}
	if got := countRows(t, db, "issues"); got < 100 {
		t.Errorf("issues = %d, want >= 100", got)
	}
	for _, table := range []string{"projects", "comments", "notifications", "sprints", "saved_filters"} {
		if countRows(t, db, table) == 0 {
			t.Errorf("no rows in %s", table)
		}
	}
}

func TestScaleScenarioInsertsRequestedIssues(t *testing.T) {
	db := testdb.New(t)
	opts := testOptions(t)
	opts.Issues = 1500 // more than one insert chunk
	if err := runScenario(t, db, "scale", opts); err != nil {
		t.Fatalf("scale seed: %v", err)
	}
	if got := countRows(t, db, "issues"); got != 1500 {
		t.Errorf("issues = %d, want 1500", got)
	}
}

func TestScaleRejectsNonPositiveIssues(t *testing.T) {
	db := testdb.New(t)
	opts := testOptions(t)
	opts.Issues = 0
	if err := runScenario(t, db, "scale", opts); err == nil {
		t.Fatal("scale with 0 issues accepted")
	}
}

// TestLargeScaleRun seeds the default 50,000 issues and checks the time budget.
// It is opt-in because it takes a while: CORELOG_SEED_LARGE=1.
func TestLargeScaleRun(t *testing.T) {
	if os.Getenv("CORELOG_SEED_LARGE") != "1" {
		t.Skip("set CORELOG_SEED_LARGE=1 to run the 50,000 issue scale check")
	}
	db := testdb.New(t)
	opts := testOptions(t)
	opts.Issues = defaultIssues
	start := time.Now()
	if err := runScenario(t, db, "scale", opts); err != nil {
		t.Fatalf("scale seed: %v", err)
	}
	elapsed := time.Since(start)
	t.Logf("scale %d issues in %s", defaultIssues, elapsed)
	if elapsed > 2*time.Minute {
		t.Errorf("scale seed took %s, budget is 2m", elapsed)
	}
	if got := countRows(t, db, "issues"); got != defaultIssues {
		t.Errorf("issues = %d, want %d", got, defaultIssues)
	}
}
