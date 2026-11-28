package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jhonsferg/corelog/internal/platform/config"
	"github.com/jhonsferg/corelog/internal/platform/database/testdb"
)

const (
	// perfStatementBudget is the execution time one statement may take on the generated dataset on an idle machine.
	perfStatementBudget = 200 * time.Millisecond
	// perfRequestBudget is the wall time of a whole request, application code included.
	perfRequestBudget = 600 * time.Millisecond
	// perfSeqScanRows is the size from which a filtered sequential scan is reported.
	perfSeqScanRows = 2000
	// perfReferenceScan is how long the reference scan below takes on an idle machine.
	perfReferenceScan = 10 * time.Millisecond
)

// perfLoadFactor measures how much slower than an idle machine this one is right now (other test packages and other
// processes share the database and the CPU) and scales the time budgets by it, between 1 and 6. A plan problem is
// orders of magnitude slower, so it still fails.
func (f *authzFixture) perfLoadFactor() float64 {
	best := time.Hour
	for i := 0; i < 5; i++ {
		began := time.Now()
		var n int
		if err := f.db.Get(&n, `SELECT count(*) FROM issues WHERE summary LIKE '%no-such-summary%'`); err != nil {
			return 1
		}
		if took := time.Since(began); took < best {
			best = took
		}
	}
	return min(6, max(1, float64(best)/float64(perfReferenceScan)))
}

// perfBigTables are the tables that grow with the data of an organization: a filtered sequential scan over them
// means an access pattern without an index.
var perfBigTables = map[string]bool{
	"issues": true, "activities": true, "comments": true, "worklogs": true, "issue_status_history": true,
	"notifications": true, "audit_events": true, "calendar_events": true,
}

type perfCase struct {
	name   string
	method string
	// path is a target with {big}, {issue}, {dash}, {gadget} and {org} placeholders.
	path  string
	body  any
	actor string
}

// explainNode is the part of an EXPLAIN (FORMAT JSON) node the checks read.
type explainNode struct {
	NodeType string        `json:"Node Type"`
	Relation string        `json:"Relation Name"`
	Rows     float64       `json:"Actual Rows"`
	Removed  float64       `json:"Rows Removed by Filter"`
	Loops    float64       `json:"Actual Loops"`
	Plans    []explainNode `json:"Plans"`
}

type explainDoc struct {
	Plan          explainNode `json:"Plan"`
	ExecutionTime float64     `json:"Execution Time"`
}

// describePlan renders a plan as an indented list of nodes with the rows they produced.
func describePlan(n explainNode, depth int) string {
	line := strings.Repeat("  ", depth) + n.NodeType
	if n.Relation != "" {
		line += " on " + n.Relation
	}
	line += fmt.Sprintf(" rows=%.0f removed=%.0f loops=%.0f", n.Rows, n.Removed, n.Loops)
	for _, c := range n.Plans {
		line += "\n" + describePlan(c, depth+1)
	}
	return line
}

func (n explainNode) walk(visit func(explainNode)) {
	visit(n)
	for _, c := range n.Plans {
		c.walk(visit)
	}
}

// TestPerformanceHotEndpoints generates a dataset of 40 projects and 20,000 issues and, for the hottest read
// endpoints, checks that the statement count does not depend on the amount of data, that no statement scans a big
// table sequentially for a selective filter and that statements and requests stay inside a generous time budget.
// The plans are printed (go test -v) so regressions are easy to read.
func TestPerformanceHotEndpoints(t *testing.T) {
	if testing.Short() {
		t.Skip("performance dataset skipped in short mode")
	}
	if raceEnabled {
		t.Skip("time budgets are not meaningful under the race detector")
	}
	f := newAuthzFixture(t)
	data := f.buildPerfDataset(t, defaultPerfScale)

	rec := testdb.NewRecorder(t, f.db)
	cfg := &config.Config{Auth: config.AuthConfig{JWTSecret: authzSecret, JWTExpiration: time.Hour}}
	router, err := buildRouter(cfg, rec.DB(), slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatal(err)
	}
	f.router = router
	load := f.perfLoadFactor()
	statementBudget := time.Duration(float64(perfStatementBudget) * load)
	requestBudget := time.Duration(float64(perfRequestBudget) * load)
	t.Logf("perf: load factor %.1f, statement budget %s, request budget %s", load, statementBudget, requestBudget)

	from, to := time.Now().AddDate(0, 0, -60).Format("2006-01-02"), time.Now().AddDate(0, 0, 60).Format("2006-01-02")
	rangeQuery := "from=" + from + "&to=" + to
	cases := []perfCase{
		{name: "issue list with filters", method: http.MethodGet, actor: actLead,
			path: "/issues?project_id={big}&status=in_progress&priority=high&q=lorem&page_size=50"},
		{name: "board view", method: http.MethodGet, actor: actLead,
			path: "/projects/{big}/board"},
		{name: "backlog", method: http.MethodGet, actor: actLead,
			path: "/projects/{big}/backlog"},
		{name: "coreql text search", method: http.MethodPost, actor: actLead,
			path: "/search/issues", body: map[string]any{"jql": `text ~ "token77" AND status != "Hecho"`, "page_size": 50}},
		{name: "global search", method: http.MethodGet, actor: actLead, path: "/search?q=token777"},
		{name: "issue activity feed", method: http.MethodGet, actor: actLead,
			path: "/issues/{issue}/activity"},
		{name: "dashboard gadget statistics", method: http.MethodGet, actor: actLead, path: "/dashboards/{dash}/gadgets/{gadget}/data"},
		{name: "cumulative flow report", method: http.MethodGet, actor: actLead,
			path: "/projects/{big}/reports/cumulative-flow?" + rangeQuery},
		{name: "control chart report", method: http.MethodGet, actor: actLead,
			path: "/projects/{big}/reports/control?" + rangeQuery},
		{name: "roadmap", method: http.MethodGet, actor: actLead,
			path: "/projects/{big}/roadmap?" + rangeQuery},
		{name: "calendar window", method: http.MethodGet, actor: actLead, path: "/calendar?" + rangeQuery},
		{name: "notifications list", method: http.MethodGet, actor: actLead, path: "/notifications"},
		{name: "audit list", method: http.MethodGet, actor: actOrgOwner, path: "/organizations/{org}/audit?limit=50"},
	}

	replacer := strings.NewReplacer("{big}", data.Big, "{issue}", data.BigIssue,
		"{dash}", data.Dashboard, "{gadget}", data.Gadget, "{org}", f.orgA)
	run := func(c perfCase, path, actor string) (int, time.Duration, []testdb.Statement) {
		rec.Reset()
		began := time.Now()
		status, body := f.call(c.method, replacer.Replace(path), f.tokens[actor], c.body)
		elapsed := time.Since(began)
		if status != http.StatusOK {
			t.Fatalf("%s: %s %s -> %d %s", c.name, c.method, path, status, truncate(body, 200))
		}
		return status, elapsed, rec.Statements()
	}

	var summary []string
	growRound := 0
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			run(c, c.path, c.actor) // warm the caches of the database and the application
			_, elapsed, statements := run(c, c.path, c.actor)
			if elapsed > requestBudget {
				t.Errorf("request took %s, budget %s", elapsed, requestBudget)
			}
			worst := time.Duration(0)
			seqScans := 0
			for i, s := range statements {
				q := strings.TrimSpace(s.Query)
				if !strings.HasPrefix(strings.ToUpper(q), "SELECT") && !strings.HasPrefix(strings.ToUpper(q), "WITH") {
					continue
				}
				docs, exec, err := bestPlan(f, q, s.Args, statementBudget)
				if err != nil {
					t.Errorf("statement %d: %v\n%s", i, err, q)
					continue
				}
				if exec > worst {
					worst = exec
				}
				if exec > statementBudget {
					t.Errorf("statement %d took %s, budget %s:\n%s", i, exec.Round(time.Millisecond), statementBudget, q)
				}
				docs[0].Plan.walk(func(n explainNode) {
					if n.NodeType != "Seq Scan" || !perfBigTables[n.Relation] {
						return
					}
					scanned := (n.Rows + n.Removed) * max(n.Loops, 1)
					if scanned >= perfSeqScanRows && n.Removed > n.Rows {
						seqScans++
						t.Errorf("statement %d scans %s sequentially (%.0f rows read, %.0f kept):\n%s", i, n.Relation, scanned, n.Rows*max(n.Loops, 1), q)
					}
				})
				if os.Getenv("PERF_PLANS") != "" {
					t.Logf("statement %d (%s) %s\n%s", i, exec.Round(time.Microsecond), firstLine(q), describePlan(docs[0].Plan, 1))
				}
			}
			// The same call after the data of the caller has grown must cost the same number of statements.
			growRound++
			f.growPerfDataset(t, data, growRound)
			f.exec(`VACUUM (ANALYZE) issues, activities, comments, notifications, calendar_events, audit_events`)
			if _, _, grown := run(c, c.path, c.actor); len(grown) != len(statements) {
				t.Errorf("statement count depends on the amount of data: %d before, %d after adding rows", len(statements), len(grown))
			}
			summary = append(summary, fmt.Sprintf("%-28s statements=%-3d request=%-8s slowest=%-8s seqscans=%d", c.name, len(statements), elapsed.Round(time.Millisecond), worst.Round(time.Millisecond), seqScans))
		})
	}
	t.Logf("performance summary at %d issues across %d projects:\n  %s", defaultPerfScale.Issues, defaultPerfScale.Projects, strings.Join(summary, "\n  "))
}

func firstLine(q string) string {
	line, _, _ := strings.Cut(strings.TrimSpace(q), "\n")
	return line
}

// bestPlan runs EXPLAIN ANALYZE up to three times and returns the fastest execution: a busy machine must not turn
// a good plan into a failure.
func bestPlan(f *authzFixture, query string, args []any, budget time.Duration) ([]explainDoc, time.Duration, error) {
	var best []explainDoc
	var bestTime time.Duration
	for attempt := 0; attempt < 3; attempt++ {
		var raw string
		if err := f.db.QueryRowContext(context.Background(), "EXPLAIN (ANALYZE, BUFFERS, FORMAT JSON) "+query, args...).Scan(&raw); err != nil {
			return nil, 0, fmt.Errorf("EXPLAIN failed: %w", err)
		}
		var docs []explainDoc
		if err := json.Unmarshal([]byte(raw), &docs); err != nil || len(docs) == 0 {
			return nil, 0, fmt.Errorf("unreadable plan: %v", err)
		}
		took := time.Duration(docs[0].ExecutionTime * float64(time.Millisecond))
		if attempt == 0 || took < bestTime {
			best, bestTime = docs, took
		}
		if bestTime <= budget {
			break
		}
	}
	return best, bestTime, nil
}
