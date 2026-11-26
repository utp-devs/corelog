package main

import (
	"reflect"
	"testing"
)

const testHash = "$2a$10$fixed-hash-placeholder-for-generator-tests"

// columnValues returns every value of one column of a table, in row order.
func columnValues(t *testing.T, ds *dataset, tableName, column string) []any {
	t.Helper()
	tbl, ok := ds.byName[tableName]
	if !ok {
		t.Fatalf("table %s not in dataset", tableName)
	}
	idx := -1
	for i, c := range tbl.cols {
		if c == column {
			idx = i
		}
	}
	if idx < 0 {
		t.Fatalf("column %s.%s not found", tableName, column)
	}
	out := make([]any, 0, len(tbl.rows))
	for _, r := range tbl.rows {
		out = append(out, r[idx])
	}
	return out
}

func rowCount(ds *dataset, tableName string) int {
	if tbl, ok := ds.byName[tableName]; ok {
		return len(tbl.rows)
	}
	return 0
}

func TestBuildersAreDeterministic(t *testing.T) {
	builders := map[string]func() *dataset{
		"demo":  func() *dataset { return buildDemo(testHash) },
		"e2e":   func() *dataset { return buildE2E(testHash) },
		"scale": func() *dataset { return buildScale(testHash, 2500) },
	}
	for name, build := range builders {
		first, second := build(), build()
		if !reflect.DeepEqual(first, second) {
			t.Errorf("%s: two runs produced different data", name)
		}
	}
}

func TestEmailsAreUniqueInEveryScenario(t *testing.T) {
	datasets := map[string]*dataset{
		"demo":  buildDemo(testHash),
		"e2e":   buildE2E(testHash),
		"scale": buildScale(testHash, 3000),
	}
	for name, ds := range datasets {
		seen := map[any]bool{}
		for _, email := range columnValues(t, ds, "users", "email") {
			if seen[email] {
				t.Errorf("%s: duplicate email %v", name, email)
			}
			seen[email] = true
		}
		ids := map[any]bool{}
		for _, id := range columnValues(t, ds, "users", "id") {
			if ids[id] {
				t.Errorf("%s: duplicate user id %v", name, id)
			}
			ids[id] = true
		}
	}
}

func TestDemoShape(t *testing.T) {
	ds := buildDemo(testHash)
	if got := rowCount(ds, "users"); got < 20 {
		t.Errorf("demo users = %d, want >= 20", got)
	}
	if got := rowCount(ds, "issues"); got < 100 {
		t.Errorf("demo issues = %d, want >= 100", got)
	}
	for _, name := range []string{"teams", "projects", "comments", "activities", "sprints", "notifications", "saved_filters", "issue_links"} {
		if rowCount(ds, name) == 0 {
			t.Errorf("demo has no rows in %s", name)
		}
	}
}

func TestE2EAccounts(t *testing.T) {
	ds := buildE2E(testHash)
	want := map[string]string{
		"admin@corelog.local":     "admin",
		"agent@corelog.local":     "user",
		"requester@corelog.local": "user",
		"ana.torres@corelog.test": "user",
		"owner@corelog.test":      "user",
		"orgadmin@corelog.test":   "user",
		"lead@corelog.test":       "user",
		"member@corelog.test":     "user",
		"viewer@corelog.test":     "user",
		"guest@corelog.test":      "user",
		"newbie@corelog.test":     "user",
	}
	emails := columnValues(t, ds, "users", "email")
	roles := columnValues(t, ds, "users", "role")
	got := map[string]string{}
	for i, e := range emails {
		got[e.(string)] = roles[i].(string)
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("e2e accounts = %v, want %v", got, want)
	}
	if n := rowCount(ds, "issues"); n != len(e2eIssues) || n < 10 {
		t.Errorf("e2e issues = %d", n)
	}
}

func TestScaleRowCounts(t *testing.T) {
	const n = 2500
	ds := buildScale(testHash, n)
	if got := rowCount(ds, "issues"); got != n {
		t.Errorf("scale issues = %d, want %d", got, n)
	}
	if got := rowCount(ds, "users"); got != 21+n/1000 {
		t.Errorf("scale users = %d, want %d", got, 21+n/1000)
	}
	if got := rowCount(ds, "activities"); got < n {
		t.Errorf("scale activities = %d, want >= %d", got, n)
	}
}

func TestIssueParentsPrecedeChildren(t *testing.T) {
	ds := buildDemo(testHash)
	seen := map[any]bool{}
	ids := columnValues(t, ds, "issues", "id")
	parents := columnValues(t, ds, "issues", "parent_id")
	for i, id := range ids {
		if p := parents[i]; p != nil && !seen[p] {
			t.Errorf("issue %v references parent %v that is not inserted before it", id, p)
		}
		seen[id] = true
	}
}

func TestRegistryKnowsAllScenarios(t *testing.T) {
	for _, name := range []string{"demo", "e2e", "scale"} {
		if _, err := lookupScenario(name); err != nil {
			t.Errorf("scenario %s not registered: %v", name, err)
		}
	}
	if _, err := lookupScenario("nope"); err == nil {
		t.Error("unknown scenario accepted")
	}
}
