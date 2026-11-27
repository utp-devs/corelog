package main

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"

	"github.com/jhonsferg/corelog/internal/platform/database/testdb"
)

// projectRows indexes the projects table of a dataset by key. Each value maps a column name to the
// value of that row.
func projectRows(t *testing.T, ds *dataset) map[string]map[string]any {
	t.Helper()
	keys := columnValues(t, ds, "projects", "key")
	out := make(map[string]map[string]any, len(keys))
	for _, col := range ds.byName["projects"].cols {
		values := columnValues(t, ds, "projects", col)
		for i, k := range keys {
			if out[k.(string)] == nil {
				out[k.(string)] = map[string]any{}
			}
			out[k.(string)][col] = values[i]
		}
	}
	return out
}

func TestE2EProjectTemplatesAndAppearance(t *testing.T) {
	rows := projectRows(t, buildE2E(testHash))
	want := map[string]struct{ template, typ, visibility, color, icon string }{
		"SCRUM": {"scrum", "scrum", "open", "palette-3", "pi pi-sync"},
		"KAN":   {"kanban", "kanban", "open", "palette-5", "pi pi-table"},
		"SEC":   {"kanban", "kanban", "private", "palette-9", "pi pi-table"},
		"EMP":   {"task_tracking", "kanban", "open", "palette-11", "pi pi-check-square"},
	}
	if len(rows) != len(want) {
		t.Fatalf("e2e projects = %d, want %d", len(rows), len(want))
	}
	for key, w := range want {
		r, ok := rows[key]
		if !ok {
			t.Fatalf("project %s missing", key)
		}
		if r["template"] != w.template || r["type"] != w.typ || r["visibility"] != w.visibility ||
			r["color"] != w.color || r["icon"] != w.icon {
			t.Errorf("project %s = template %v type %v visibility %v color %v icon %v, want %+v",
				key, r["template"], r["type"], r["visibility"], r["color"], r["icon"], w)
		}
		if r["default_assignee_policy"] != "unassigned" {
			t.Errorf("project %s default_assignee_policy = %v", key, r["default_assignee_policy"])
		}
	}
}

// TestSeededIssueCountersFollowIssues checks that next_issue_number is one more than the number of
// seeded issues of each project, for every scenario that creates projects.
func TestSeededIssueCountersFollowIssues(t *testing.T) {
	for name, ds := range map[string]*dataset{
		"e2e":  buildE2E(testHash),
		"demo": buildDemo(testHash),
	} {
		counts := map[uuid.UUID]int{}
		for _, id := range columnValues(t, ds, "issues", "project_id") {
			counts[id.(uuid.UUID)]++
		}
		ids := columnValues(t, ds, "projects", "id")
		counters := columnValues(t, ds, "projects", "next_issue_number")
		for i, id := range ids {
			want := counts[id.(uuid.UUID)] + 1
			if counters[i] != want {
				t.Errorf("%s: next_issue_number of project %d = %v, want %d", name, i, counters[i], want)
			}
		}
	}
}

func TestE2ETeamGrantsAndPersonalState(t *testing.T) {
	ds := buildE2E(testHash)

	grantTeams := columnValues(t, ds, "project_team_roles", "team_id")
	grantRoles := columnValues(t, ds, "project_team_roles", "role")
	teamIDs := map[uuid.UUID]bool{}
	for _, id := range columnValues(t, ds, "teams", "id") {
		teamIDs[id.(uuid.UUID)] = true
	}
	if len(grantTeams) != 2 {
		t.Fatalf("team grants = %d, want 2", len(grantTeams))
	}
	wantGrants := map[string]string{
		idFor("team", "acme", "Design").String():   "viewer",
		idFor("team", "acme", "Platform").String(): "contributor",
	}
	for i, id := range grantTeams {
		u := id.(uuid.UUID)
		if !teamIDs[u] {
			t.Errorf("team grant %d references unknown team %s", i, u)
		}
		if wantGrants[u.String()] != grantRoles[i] {
			t.Errorf("team grant %d role = %v, want %v", i, grantRoles[i], wantGrants[u.String()])
		}
	}

	member := userID("member@corelog.test")
	if favs := columnValues(t, ds, "favorites", "user_id"); len(favs) != 1 || favs[0] != member {
		t.Errorf("favorites = %v, want exactly one for the e2e member", favs)
	}
	if rec := columnValues(t, ds, "recent_items", "user_id"); len(rec) != 1 || rec[0] != member {
		t.Errorf("recent items = %v, want exactly one for the e2e member", rec)
	}
}

func TestDemoProjectsHaveTemplates(t *testing.T) {
	rows := projectRows(t, buildDemo(testHash))
	wantTemplates := map[string]string{"CORE": "kanban", "OPS": "kanban", "SEC": "scrum", "WEB": "scrum"}
	for key, template := range wantTemplates {
		r, ok := rows[key]
		if !ok {
			t.Fatalf("demo project %s missing", key)
		}
		if r["template"] != template {
			t.Errorf("demo project %s template = %v, want %s", key, r["template"], template)
		}
		if r["icon"] == "" || r["color"] == "" {
			t.Errorf("demo project %s has no appearance: icon %v color %v", key, r["icon"], r["color"])
		}
	}
}

func TestE2EProjectsPersistWithGrantsAndFavorites(t *testing.T) {
	db := testdb.New(t)
	if err := runScenario(t, db, "e2e", testOptions(t)); err != nil {
		t.Fatalf("e2e seed: %v", err)
	}

	var grants []struct {
		Project string `db:"key"`
		Team    string `db:"name"`
		Role    string `db:"role"`
	}
	if err := db.Select(&grants, `
		SELECT p.key, t.name, ptr.role
		FROM project_team_roles ptr
		JOIN projects p ON p.id = ptr.project_id
		JOIN teams t ON t.id = ptr.team_id
		ORDER BY p.key`); err != nil {
		t.Fatalf("select team grants: %v", err)
	}
	want := []string{"KAN/Platform/contributor", "SCRUM/Design/viewer"}
	if len(grants) != len(want) {
		t.Fatalf("persisted team grants = %d, want %d", len(grants), len(want))
	}
	for i, g := range grants {
		if got := g.Project + "/" + g.Team + "/" + g.Role; got != want[i] {
			t.Errorf("grant %d = %s, want %s", i, got, want[i])
		}
	}

	var counters []struct {
		Key     string `db:"key"`
		Counter int    `db:"next_issue_number"`
		Issues  int    `db:"issues"`
	}
	if err := db.Select(&counters, `
		SELECT p.key, p.next_issue_number, count(i.id) AS issues
		FROM projects p LEFT JOIN issues i ON i.project_id = p.id
		GROUP BY p.id, p.key, p.next_issue_number ORDER BY p.key`); err != nil {
		t.Fatalf("select counters: %v", err)
	}
	for _, c := range counters {
		if c.Counter != c.Issues+1 {
			t.Errorf("project %s next_issue_number = %d, want %d", c.Key, c.Counter, c.Issues+1)
		}
	}

	var favorites, recents int
	if err := db.Get(&favorites, `SELECT count(*) FROM favorites`); err != nil {
		t.Fatal(err)
	}
	if err := db.Get(&recents, `SELECT count(*) FROM recent_items`); err != nil {
		t.Fatal(err)
	}
	if favorites != 1 || recents != 1 {
		t.Errorf("favorites = %d, recents = %d, want 1 and 1", favorites, recents)
	}
}

func TestGuardRealAccountsRefusesHandMadeUsers(t *testing.T) {
	db := testdb.New(t)
	ctx := context.Background()
	t.Setenv("SEED_ALLOW_DATA_LOSS", "")
	if _, err := db.ExecContext(ctx, `INSERT INTO users (id, name, email, password_hash, role) VALUES (gen_random_uuid(), 'Real', 'someone@example.com', 'x', 'user')`); err != nil {
		t.Skipf("cannot insert a user in this schema: %v", err)
	}
	t.Cleanup(func() { _, _ = db.ExecContext(ctx, `DELETE FROM users WHERE email = 'someone@example.com'`) })
	if err := guardRealAccounts(ctx, db); !errors.Is(err, errRealAccounts) {
		t.Fatalf("guardRealAccounts = %v, want errRealAccounts", err)
	}
	t.Setenv("SEED_ALLOW_DATA_LOSS", "yes")
	if err := guardRealAccounts(ctx, db); err != nil {
		t.Fatalf("with opt-in: %v", err)
	}
}
