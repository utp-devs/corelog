package postgres

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/jhonsferg/corelog/internal/dashboard/application/port"
	"github.com/jhonsferg/corelog/internal/dashboard/domain"
)

type issueSpec struct {
	project  uuid.UUID
	summary  string
	status   string // legacy: open, in_progress, closed
	assignee *uuid.UUID
	created  time.Time
	resolved *time.Time
	points   *float64
	sprint   *uuid.UUID
}

func (f *fixture) issue(s issueSpec) uuid.UUID {
	f.t.Helper()
	id := uuid.New()
	if s.created.IsZero() {
		s.created = time.Date(2026, 11, 20, 10, 0, 0, 0, time.UTC)
	}
	f.exec(`INSERT INTO issues (id, project_id, summary, description, status, priority, reporter_id, assignee_id, key_number, issue_type_id, priority_id,
			created_at, updated_at, resolved_at, story_points, sprint_id)
		SELECT $1, $2, $3, '', $4, 'medium', $5, $6, (SELECT COALESCE(max(key_number), 0) + 1 FROM issues WHERE project_id = $2),
		       (SELECT it.id FROM issue_types it WHERE it.organization_id = p.organization_id AND it.is_default),
		       (SELECT pr.id FROM priorities pr WHERE pr.organization_id = p.organization_id AND pr.is_default), $7, $7, $8, $9, $10
		FROM projects p WHERE p.id = $2`, id, s.project, s.summary, s.status, f.owner, s.assignee, s.created, s.resolved, s.points, s.sprint)
	return id
}

func groupCounts(groups []port.StatGroup) map[string]int {
	out := map[string]int{}
	for _, g := range groups {
		out[g.Label] = g.Count
	}
	return out
}

func TestStatisticsRespectViewerVisibilityPostgres(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	me := f.owner
	f.issue(issueSpec{project: f.open, summary: "a", status: "open", assignee: &me})
	f.issue(issueSpec{project: f.open, summary: "b", status: "open"})
	f.issue(issueSpec{project: f.open, summary: "c", status: "in_progress", assignee: &me})
	f.issue(issueSpec{project: f.open, summary: "d", status: "closed"})
	f.issue(issueSpec{project: f.secret, summary: "secreta 1", status: "open"})
	f.issue(issueSpec{project: f.secret, summary: "secreta 2", status: "open"})
	f.exec(`UPDATE issues SET deleted_at = now() WHERE summary = 'c'`)
	cfg := domain.StatsConfig{GroupBy: "status", Limit: 10}

	// The owner is a member of both projects and sees every live, open issue.
	got, err := f.data.Statistics(ctx, v(f.owner), cfg)
	if err != nil || got.Total != 4 || groupCounts(got.Groups)["Por hacer"] != 4 || got.GroupBy != "status" {
		t.Fatalf("owner: %+v %v", got, err)
	}
	cfg.IncludeDone = true
	if got, _ := f.data.Statistics(ctx, v(f.owner), cfg); got.Total != 5 || groupCounts(got.Groups)["Hecho"] != 1 {
		t.Fatalf("include done: %+v", got)
	}
	cfg.IncludeDone = false

	// A plain member sees the open project only: the private one never leaks through a shared dashboard.
	got, _ = f.data.Statistics(ctx, v(f.member), cfg)
	if got.Total != 2 || groupCounts(got.Groups)["Por hacer"] != 2 {
		t.Fatalf("member: %+v", got)
	}
	// The organization admin sees everything; the other organization nothing; a team grant is enough.
	if got, _ := f.data.Statistics(ctx, v(f.admin), cfg); got.Total != 4 {
		t.Fatalf("admin: %+v", got)
	}
	if got, _ := f.data.Statistics(ctx, v(f.other), cfg); got.Total != 0 || len(got.Groups) != 0 {
		t.Fatalf("outsider: %+v", got)
	}
	// Asking for a project the viewer cannot browse returns nothing, not an error.
	cfg.ProjectID = &f.secret
	if got, err := f.data.Statistics(ctx, v(f.member), cfg); err != nil || got.Total != 0 {
		t.Fatalf("private project as member: %+v %v", got, err)
	}
	if got, _ := f.data.Statistics(ctx, v(f.owner), cfg); got.Total != 2 {
		t.Fatalf("private project as lead: %+v", got)
	}
	// The platform administrator is the support view over every project.
	if got, _ := f.data.Statistics(ctx, port.Viewer{UserID: f.other, PlatformAdmin: true}, cfg); got.Total != 2 {
		t.Fatalf("platform admin: %+v", got)
	}
	f.exec(`INSERT INTO project_team_roles (project_id, team_id, role) VALUES ($1, $2, 'viewer')`, f.secret, f.team)
	if got, _ := f.data.Statistics(ctx, v(f.teamer), cfg); got.Total != 2 {
		t.Fatalf("team grant: %+v", got)
	}
}

func TestStatisticsGroupingsPostgres(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	a, b := f.owner, f.member
	f.issue(issueSpec{project: f.open, summary: "1", status: "open", assignee: &a})
	f.issue(issueSpec{project: f.open, summary: "2", status: "open", assignee: &a})
	f.issue(issueSpec{project: f.open, summary: "3", status: "in_progress", assignee: &b})
	f.issue(issueSpec{project: f.open, summary: "4", status: "in_progress"})
	f.issue(issueSpec{project: f.secret, summary: "5", status: "open", assignee: &b})
	f.exec(`INSERT INTO labels (id, project_id, name) VALUES ($1, $3, 'back'), ($2, $3, 'front')`, uuid.New(), uuid.New(), f.open)
	f.exec(`INSERT INTO issue_labels (issue_id, label_id) SELECT i.id, l.id FROM issues i, labels l WHERE i.summary IN ('1', '2') AND l.name = 'back'`)
	f.exec(`INSERT INTO issue_labels (issue_id, label_id) SELECT i.id, l.id FROM issues i, labels l WHERE i.summary = '1' AND l.name = 'front'`)

	cases := map[string]map[string]int{
		"status":   {"Por hacer": 3, "En curso": 2},
		"assignee": {"ana": 2, "beto": 2, "Sin asignar": 1},
		"priority": {},
		"type":     {},
		"label":    {"back": 2, "front": 1, "Sin etiqueta": 3},
		"project":  {"OPN": 4, "SEC": 1},
	}
	for field, want := range cases {
		got, err := f.data.Statistics(ctx, v(f.owner), domain.StatsConfig{GroupBy: field, Limit: 10})
		if err != nil {
			t.Fatalf("%s: %v", field, err)
		}
		counts := groupCounts(got.Groups)
		if got.Total != 5 {
			t.Errorf("%s: total = %d", field, got.Total)
		}
		if len(want) == 0 {
			if len(counts) != 1 {
				t.Errorf("%s: groups = %v", field, counts)
			}
			continue
		}
		for k, n := range want {
			if counts[k] != n {
				t.Errorf("%s: %q = %d, want %d (all %v)", field, k, counts[k], n, counts)
			}
		}
	}
	// The limit moves the rest into "other"; the groups come busiest first.
	got, _ := f.data.Statistics(ctx, v(f.owner), domain.StatsConfig{GroupBy: "assignee", Limit: 1})
	if len(got.Groups) != 1 || got.Groups[0].Count != 2 || got.Other != 3 {
		t.Fatalf("limit: %+v", got)
	}

	// Two dimensions.
	m, err := f.data.TwoDimensional(ctx, v(f.owner), domain.TwoDimensionalConfig{XAxis: "status", YAxis: "assignee", Limit: 10})
	if err != nil || m.Total != 5 || len(m.XKeys) != 2 || len(m.YKeys) != 3 {
		t.Fatalf("matrix: %+v %v", m, err)
	}
	cell := map[[2]string]int{}
	for _, c := range m.Cells {
		cell[[2]string{c.X, c.Y}] = c.Count
	}
	if cell[[2]string{"Por hacer", f.owner.String()}] != 2 || cell[[2]string{"En curso", "unassigned"}] != 1 || cell[[2]string{"En curso", f.member.String()}] != 1 {
		t.Fatalf("cells: %+v", m.Cells)
	}
	m, _ = f.data.TwoDimensional(ctx, v(f.owner), domain.TwoDimensionalConfig{XAxis: "project", YAxis: "label", Limit: 2})
	if len(m.XKeys) != 2 || len(m.YKeys) != 2 {
		t.Fatalf("axis limit: %+v", m)
	}
	if m, _ := f.data.TwoDimensional(ctx, v(f.member), domain.TwoDimensionalConfig{XAxis: "status", YAxis: "assignee", Limit: 10}); m.Total != 4 {
		t.Fatalf("matrix isolation: total %d", m.Total)
	}
}

func TestCreatedVsResolvedPostgres(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	today := time.Date(2026, 11, 27, 15, 0, 0, 0, time.UTC)
	day := func(d int) time.Time { return time.Date(2026, 11, d, 9, 0, 0, 0, time.UTC) }
	res := day(26)
	f.issue(issueSpec{project: f.open, summary: "a", status: "closed", created: day(25), resolved: &res})
	f.issue(issueSpec{project: f.open, summary: "b", status: "open", created: day(26)})
	f.issue(issueSpec{project: f.open, summary: "c", status: "open", created: day(26)})
	f.issue(issueSpec{project: f.open, summary: "old", status: "open", created: time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)})
	f.issue(issueSpec{project: f.secret, summary: "s", status: "open", created: day(27)})

	got, err := f.data.CreatedVsResolved(ctx, v(f.owner), domain.CreatedVsResolvedConfig{Days: 7}, today)
	if err != nil || len(got.Series) != 7 || got.Series[0].Date != "2026-11-21" || got.Series[6].Date != "2026-11-27" {
		t.Fatalf("series: %+v %v", got, err)
	}
	by := map[string]port.DayPoint{}
	for _, p := range got.Series {
		by[p.Date] = p
	}
	if by["2026-11-25"].Created != 1 || by["2026-11-26"].Created != 2 || by["2026-11-26"].Resolved != 1 || by["2026-11-27"].Created != 1 {
		t.Fatalf("points: %+v", got.Series)
	}
	if got.TotalCreated != 4 || got.TotalResolved != 1 || got.CumulativeOpened != 3 {
		t.Fatalf("totals: %+v", got)
	}
	// The member does not see the secret project's issue; the project filter narrows.
	if got, _ := f.data.CreatedVsResolved(ctx, v(f.member), domain.CreatedVsResolvedConfig{Days: 7}, today); got.TotalCreated != 3 {
		t.Fatalf("member: %+v", got)
	}
	if got, _ := f.data.CreatedVsResolved(ctx, v(f.owner), domain.CreatedVsResolvedConfig{Days: 7, ProjectID: &f.secret}, today); got.TotalCreated != 1 {
		t.Fatalf("project filter: %+v", got)
	}
	if got, _ := f.data.CreatedVsResolved(ctx, v(f.other), domain.CreatedVsResolvedConfig{Days: 90}, today); got.TotalCreated != 0 || len(got.Series) != 90 {
		t.Fatalf("outsider: %+v", got)
	}
}

func TestBurndownPostgres(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	today := time.Date(2026, 11, 12, 12, 0, 0, 0, time.UTC)
	sprint, hidden := uuid.New(), uuid.New()
	f.exec(`INSERT INTO sprints (id, project_id, name, status, start_date, end_date) VALUES ($1, $2, 'Sprint 5', 'active', '2026-11-09', '2026-11-13')`, sprint, f.open)
	f.exec(`INSERT INTO sprints (id, project_id, name, status, start_date, end_date) VALUES ($1, $2, 'Secreto', 'active', '2026-11-09', '2026-11-20')`, hidden, f.secret)
	p := func(x float64) *float64 { return &x }
	r1, r2 := time.Date(2026, 11, 10, 9, 0, 0, 0, time.UTC), time.Date(2026, 11, 12, 9, 0, 0, 0, time.UTC)
	f.issue(issueSpec{project: f.open, summary: "a", status: "closed", resolved: &r1, points: p(3), sprint: &sprint})
	f.issue(issueSpec{project: f.open, summary: "b", status: "closed", resolved: &r2, points: p(2), sprint: &sprint})
	f.issue(issueSpec{project: f.open, summary: "c", status: "open", points: p(5), sprint: &sprint})

	got, err := f.data.Burndown(ctx, v(f.owner), domain.BurndownConfig{}, today)
	if err != nil || got.Sprint == nil || got.Sprint.Name != "Sprint 5" || got.Total != 10 || len(got.Points) != 5 {
		t.Fatalf("burndown: %+v %v", got, err)
	}
	remaining := func(i int) float64 {
		if got.Points[i].Remaining == nil {
			return -1
		}
		return *got.Points[i].Remaining
	}
	if remaining(0) != 10 || remaining(1) != 7 || remaining(2) != 7 || remaining(3) != 5 || remaining(4) != -1 {
		t.Fatalf("remaining: %+v", got.Points)
	}
	if got.Points[0].Ideal != 10 || got.Points[4].Ideal != 0 || got.Points[2].Ideal != 5 {
		t.Fatalf("ideal: %+v", got.Points)
	}
	// Explicit sprint ids and project filters.
	if got, _ := f.data.Burndown(ctx, v(f.owner), domain.BurndownConfig{SprintID: &hidden}, today); got.Sprint == nil || got.Sprint.Name != "Secreto" {
		t.Fatalf("explicit sprint: %+v", got)
	}
	// A member cannot reach the private project's sprint, by id or by default.
	if got, _ := f.data.Burndown(ctx, v(f.member), domain.BurndownConfig{SprintID: &hidden}, today); got.Sprint != nil || len(got.Points) != 0 {
		t.Fatalf("private sprint leaked: %+v", got)
	}
	if got, _ := f.data.Burndown(ctx, v(f.member), domain.BurndownConfig{}, today); got.Sprint == nil || got.Sprint.Name != "Sprint 5" {
		t.Fatalf("member default sprint: %+v", got)
	}
	if got, _ := f.data.Burndown(ctx, v(f.other), domain.BurndownConfig{}, today); got.Sprint != nil {
		t.Fatalf("outsider: %+v", got)
	}
	// Without estimates the burndown counts issues.
	f.exec(`UPDATE issues SET story_points = NULL WHERE sprint_id = $1`, sprint)
	if got, _ := f.data.Burndown(ctx, v(f.owner), domain.BurndownConfig{ProjectID: &f.open}, today); got.Total != 3 || *got.Points[3].Remaining != 1 {
		t.Fatalf("unit burndown: %+v", got)
	}
}

func TestActivityRecentAndFavoritesPostgres(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	visibleIssue := f.issue(issueSpec{project: f.open, summary: "abierta", status: "open"})
	secretIssue := f.issue(issueSpec{project: f.secret, summary: "secreta", status: "open"})
	for i, id := range []uuid.UUID{visibleIssue, secretIssue, visibleIssue} {
		f.exec(`INSERT INTO activities (id, issue_id, actor_id, action, field_name, old_value, new_value, created_at) VALUES ($1, $2, $3, 'updated', 'summary', 'x', 'y', $4)`,
			uuid.New(), id, f.owner, time.Date(2026, 11, 20, 10, i, 0, 0, time.UTC))
	}
	act, err := f.data.Activity(ctx, v(f.owner), domain.ActivityConfig{Limit: 10})
	if err != nil || len(act) != 3 || act[0].IssueKey != "OPN-1" && act[0].IssueKey != "SEC-1" || act[0].Actor != "ana" {
		t.Fatalf("owner activity: %+v %v", act, err)
	}
	if !act[0].OccurredAt.After(act[1].OccurredAt) {
		t.Fatalf("not newest first: %+v", act)
	}
	if act, _ := f.data.Activity(ctx, v(f.member), domain.ActivityConfig{Limit: 10}); len(act) != 2 {
		t.Fatalf("member activity: %d", len(act))
	}
	if act, _ := f.data.Activity(ctx, v(f.owner), domain.ActivityConfig{Limit: 1, ProjectID: &f.secret}); len(act) != 1 || act[0].IssueKey != "SEC-1" {
		t.Fatalf("project filter: %+v", act)
	}
	if act, _ := f.data.Activity(ctx, v(f.other), domain.ActivityConfig{Limit: 10}); len(act) != 0 {
		t.Fatalf("outsider activity: %d", len(act))
	}

	// Recent projects: only the ones still browsable.
	f.exec(`INSERT INTO recent_items (user_id, entity_type, entity_id, viewed_at) VALUES ($1, 'project', $2, '2026-11-20 10:00:00+00'), ($1, 'project', $3, '2026-11-21 10:00:00+00'), ($1, 'issue', $4, now())`,
		f.member, f.open, f.secret, visibleIssue)
	recent, err := f.data.RecentProjects(ctx, v(f.member), 10)
	if err != nil || len(recent) != 1 || recent[0].Key != "OPN" {
		t.Fatalf("recent: %+v %v", recent, err)
	}
	f.exec(`INSERT INTO recent_items (user_id, entity_type, entity_id, viewed_at) VALUES ($1, 'project', $2, now())`, f.owner, f.secret)
	f.exec(`INSERT INTO recent_items (user_id, entity_type, entity_id, viewed_at) VALUES ($1, 'project', $2, '2026-01-01 00:00:00+00')`, f.owner, f.open)
	if recent, _ := f.data.RecentProjects(ctx, v(f.owner), 10); len(recent) != 2 || recent[0].Key != "SEC" {
		t.Fatalf("owner recent: %+v", recent)
	}
	if recent, _ := f.data.RecentProjects(ctx, v(f.owner), 1); len(recent) != 1 {
		t.Fatalf("recent limit: %+v", recent)
	}

	// Favorites: names resolved, hidden entities dropped, types filtered.
	filter, mine, board := uuid.New(), f.create(f.member, "Mío"), uuid.New()
	f.exec(`INSERT INTO filters (id, owner_id, organization_id, name) VALUES ($1, $2, $3, 'Mi filtro')`, filter, f.owner, f.org)
	f.exec(`INSERT INTO boards (id, project_id, name, type) VALUES ($1, $2, 'Tablero', 'kanban')`, board, f.secret)
	for i, fav := range []struct {
		kind string
		id   uuid.UUID
	}{{"project", f.open}, {"project", f.secret}, {"issue", visibleIssue}, {"issue", secretIssue}, {"filter", filter}, {"board", board}, {"dashboard", mine.ID}} {
		f.exec(`INSERT INTO favorites (user_id, entity_type, entity_id, created_at) VALUES ($1, $2, $3, $4)`, f.member, fav.kind, fav.id, time.Date(2026, 11, 1, 0, i, 0, 0, time.UTC))
	}
	favs, err := f.data.Favorites(ctx, v(f.member), domain.FavoritesConfig{Limit: 20})
	if err != nil {
		t.Fatal(err)
	}
	names := map[string]string{}
	for _, fav := range favs {
		names[fav.Type+":"+fav.Name] = fav.Link
	}
	// The member sees: the open project, the open issue and their own dashboard. Not the secret project, secret
	// issue, board of the secret project, nor the filter of somebody else that is not shared with them.
	if len(favs) != 3 || favs[0].Type != "dashboard" || names["project:Abierto"] == "" || names["issue:OPN-1 abierta"] == "" || names["dashboard:Mío"] == "" {
		t.Fatalf("member favorites: %+v", favs)
	}
	if favs, _ := f.data.Favorites(ctx, v(f.member), domain.FavoritesConfig{Types: []string{"project"}, Limit: 20}); len(favs) != 1 || favs[0].Type != "project" {
		t.Fatalf("type filter: %+v", favs)
	}
	f.exec(`INSERT INTO favorites (user_id, entity_type, entity_id) VALUES ($1, 'filter', $2), ($1, 'board', $3), ($1, 'dashboard', $4)`, f.owner, filter, board, mine.ID)
	if favs, _ := f.data.Favorites(ctx, v(f.owner), domain.FavoritesConfig{Limit: 20}); len(favs) != 2 {
		t.Fatalf("owner favorites (own filter and board; dashboard of somebody else is hidden): %+v", favs)
	}
	if favs, _ := f.data.Favorites(ctx, v(f.member), domain.FavoritesConfig{Limit: 2}); len(favs) != 2 {
		t.Fatalf("favorites limit: %+v", favs)
	}
}

func TestGadgetQueriesStatementsDoNotGrowPostgres(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	run := func() int32 {
		f.counter.Store(0)
		viewer := v(f.owner)
		if _, err := f.data.Statistics(ctx, viewer, domain.StatsConfig{GroupBy: "label", Limit: 10}); err != nil {
			t.Fatal(err)
		}
		if _, err := f.data.TwoDimensional(ctx, viewer, domain.TwoDimensionalConfig{XAxis: "status", YAxis: "assignee", Limit: 10}); err != nil {
			t.Fatal(err)
		}
		if _, err := f.data.CreatedVsResolved(ctx, viewer, domain.CreatedVsResolvedConfig{Days: 30}, time.Now()); err != nil {
			t.Fatal(err)
		}
		if _, err := f.data.Activity(ctx, viewer, domain.ActivityConfig{Limit: 10}); err != nil {
			t.Fatal(err)
		}
		if _, err := f.data.Favorites(ctx, viewer, domain.FavoritesConfig{Limit: 10}); err != nil {
			t.Fatal(err)
		}
		return f.counter.Load()
	}
	a := f.owner
	f.issue(issueSpec{project: f.open, summary: "x", status: "open", assignee: &a})
	small := run()
	for i := 0; i < 40; i++ {
		f.issue(issueSpec{project: f.open, summary: "y", status: "open", assignee: &a})
	}
	if large := run(); small != large || small != 7 {
		t.Fatalf("statements: %d with 1 issue, %d with 41 (want 7)", small, large)
	}
}
