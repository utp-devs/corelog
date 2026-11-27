package postgres_test

import (
	"errors"
	"sort"
	"strings"
	"testing"

	"github.com/jhonsferg/corelog/internal/platform/apperror"
	"github.com/jhonsferg/corelog/internal/search/application/port"
	"github.com/jhonsferg/corelog/internal/search/domain"
)

func sameSet(a, b []string) bool {
	x, y := append([]string(nil), a...), append([]string(nil), b...)
	sort.Strings(x)
	sort.Strings(y)
	return strings.Join(x, ",") == strings.Join(y, ",")
}

// TestSearchQueriesBehavior runs representative queries as alice (who sees the public and the private
// project of her organization) and checks the keys that come back.
func TestSearchQueriesBehavior(t *testing.T) {
	e := newSearchEnv(t)
	cases := []struct {
		name string
		jql  string
		want []string
	}{
		{"empty returns all browsable", "", []string{"PUB-1", "PUB-2", "PUB-3", "PUB-4", "PUB-5", "PRIV-1"}},
		{"project by key", "project = PUB", []string{"PUB-1", "PUB-2", "PUB-3", "PUB-4", "PUB-5"}},
		{"project by name case-insensitive", `project = "privado"`, []string{"PRIV-1"}},
		{"project in", "project IN (PUB, PRIV)", []string{"PUB-1", "PUB-2", "PUB-3", "PUB-4", "PUB-5", "PRIV-1"}},
		{"key equals", "key = PUB-3", []string{"PUB-3"}},
		{"key in", "key IN (PUB-1, PRIV-1, PUB-99)", []string{"PUB-1", "PRIV-1"}},
		{"type by name", "type = Error", []string{"PUB-1", "PUB-4"}},
		{"type by legacy key", "issuetype = bug", []string{"PUB-1", "PUB-4"}},
		{"type not", "type != Error AND project = PUB", []string{"PUB-2", "PUB-3", "PUB-5"}},
		{"status name", `status = "En curso"`, []string{"PUB-2", "PUB-5"}},
		{"status in", `status IN ("Por hacer", "Hecho") AND project = PUB`, []string{"PUB-1", "PUB-3", "PUB-4"}},
		{"status category", "statusCategory = done", []string{"PUB-3"}},
		{"status category alias", `statusCategory = "In Progress"`, []string{"PUB-2", "PUB-5"}},
		{"priority", "priority = Alta", []string{"PUB-1", "PUB-5"}},
		{"priority in", "priority IN (Alta, Máxima)", []string{"PUB-1", "PUB-4", "PUB-5"}},
		{"resolution empty", "resolution IS EMPTY AND project = PUB", []string{"PUB-1", "PUB-2", "PUB-4", "PUB-5"}},
		{"resolution unresolved", "resolution = Unresolved AND project = PUB", []string{"PUB-1", "PUB-2", "PUB-4", "PUB-5"}},
		{"resolution value", "resolution = Hecho", []string{"PUB-3"}},
		{"assignee current user", "assignee = currentUser()", []string{"PUB-1", "PUB-4", "PRIV-1"}},
		{"assignee by name", "assignee = bob", []string{"PUB-2"}},
		{"assignee by email", `assignee = "bob@example.test"`, []string{"PUB-2"}},
		{"assignee empty", "assignee IS EMPTY AND project = PUB", []string{"PUB-3", "PUB-5"}},
		{"assignee not includes unassigned", "assignee != alice AND project = PUB", []string{"PUB-2", "PUB-3", "PUB-5"}},
		{"assignee members of team", `assignee IN (membersOf("Backend")) AND project = PUB`, []string{"PUB-1", "PUB-2", "PUB-4"}},
		{"assignee members of other team", `assignee IN (membersOf("Diseno"))`, nil},
		{"reporter", "reporter = carol", []string{"PUB-4"}},
		{"creator alias", "creator = bob", []string{"PUB-1"}},
		{"labels", "labels = bug", []string{"PUB-1", "PUB-4"}},
		{"labels in", "labels IN (perf, docs)", []string{"PUB-2", "PUB-3"}},
		{"labels not in", "labels NOT IN (bug) AND project = PUB", []string{"PUB-2", "PUB-3", "PUB-5"}},
		{"labels empty", "labels IS EMPTY AND project = PUB", []string{"PUB-5"}},
		{"labels both", "labels = bug AND labels = mobile", []string{"PUB-1"}},
		{"component", "component = Frontend", []string{"PUB-1"}},
		{"sprint by name", `sprint = "Sprint 1"`, []string{"PUB-1"}},
		{"open sprints", "sprint IN (openSprints())", []string{"PUB-1"}},
		{"closed sprints", "sprint IN (closedSprints())", []string{"PUB-3"}},
		{"sprint empty", "sprint IS EMPTY AND project = PUB", []string{"PUB-2", "PUB-4", "PUB-5"}},
		{"epic children", "epic = PUB-5", []string{"PUB-2", "PUB-4"}},
		{"parent children", "parent = PUB-5", []string{"PUB-2", "PUB-4"}},
		{"parent empty", "parent IS EMPTY AND project = PUB", []string{"PUB-1", "PUB-3", "PUB-5"}},
		{"summary contains", `summary ~ "login"`, []string{"PUB-1"}},
		{"summary contains accent", `summary ~ "móvil"`, []string{"PUB-1"}},
		{"summary not contains", `summary !~ "pago" AND project = PUB`, []string{"PUB-1", "PUB-2", "PUB-3"}},
		{"summary equals", `summary = "documentar la api"`, []string{"PUB-3"}},
		{"description contains", `description ~ "stripe"`, []string{"PUB-4"}},
		{"comment contains", `comment ~ "despliegue"`, []string{"PUB-1"}},
		{"text searches summary", `text ~ "rendimiento"`, []string{"PUB-2"}},
		{"text searches description", `text ~ "android"`, []string{"PUB-1"}},
		{"text searches comments", `text ~ "despliegue"`, []string{"PUB-1"}},
		{"text stems spanish", `text ~ "documentando"`, []string{"PUB-3"}},
		{"text partial word", `text ~ "rendim"`, []string{"PUB-2"}},
		{"due before", `due < "2026-11-28"`, []string{"PUB-2"}},
		{"due equals", `due = "2026-11-29"`, []string{"PUB-1"}},
		{"due relative overdue", "due < startOfDay()", []string{"PUB-2"}},
		{"due end of week", "due <= endOfWeek()", []string{"PUB-1", "PUB-2"}},
		{"due empty", "due IS EMPTY AND project = PUB", []string{"PUB-3", "PUB-4", "PUB-5"}},
		{"created relative", "created >= -5d", []string{"PUB-2", "PUB-4", "PRIV-1"}},
		{"created range", `created >= -15d AND created < -2d AND project = PUB`, []string{"PUB-1", "PUB-2"}},
		{"created function", "created >= startOfWeek() AND project = PUB", []string{"PUB-2", "PUB-4"}},
		{"created day literal", `created = "2026-11-17"`, []string{"PUB-1"}},
		{"created start of month", "created >= startOfMonth()", []string{"PUB-1", "PUB-2", "PUB-3", "PUB-4", "PRIV-1"}},
		{"resolved", "resolved >= -2d", []string{"PUB-3"}},
		{"votes", "votes >= 3", []string{"PUB-4"}},
		{"story points", "storyPoints > 2", []string{"PUB-1", "PUB-2"}},
		{"story points equals", "storyPoints = 1", []string{"PUB-3"}},
		{"flagged", "flagged = true", []string{"PUB-1"}},
		{"not flagged", "flagged = false AND project = PUB", []string{"PUB-2", "PUB-3", "PUB-4", "PUB-5"}},
		{"custom text by cf", `cf[Cliente] = acme`, []string{"PUB-1"}},
		{"custom text bare name", `Cliente = Globex`, []string{"PUB-2"}},
		{"custom text contains", `cf[cliente] ~ "lob"`, []string{"PUB-2"}},
		{"custom number", `cf[Horas] > 10`, []string{"PUB-2"}},
		{"custom number range", `cf[Horas] >= 8 AND cf[Horas] <= 20`, []string{"PUB-1", "PUB-2"}},
		{"custom select label", `cf[Entorno] = "Producción"`, []string{"PUB-1"}},
		{"custom select id", `cf[Entorno] = opt-prod`, []string{"PUB-1"}},
		{"custom checkbox", `cf[Urgente] = true`, []string{"PUB-1"}},
		{"custom empty", `cf[Cliente] IS EMPTY AND project = PUB`, []string{"PUB-3", "PUB-4", "PUB-5"}},
		{"custom by field id", `cf[` + "FIELDID" + `] = Acme`, []string{"PUB-1"}},
		{"and or precedence", "labels = perf OR labels = docs AND project = PUB", []string{"PUB-2", "PUB-3"}},
		{"parentheses", "(labels = perf OR labels = bug) AND assignee = currentUser()", []string{"PUB-1", "PUB-4"}},
		{"not", "NOT labels = bug AND project = PUB", []string{"PUB-2", "PUB-3", "PUB-5"}},
		{"not group", "NOT (assignee = alice OR assignee = bob) AND project = PUB", []string{"PUB-3", "PUB-5"}},
		{"fix version unknown returns none", "fixVersion = 1.0", nil},
		{"fix version empty", "fixVersion IS EMPTY AND project = PUB", []string{"PUB-1", "PUB-2", "PUB-3", "PUB-4", "PUB-5"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			jql := strings.ReplaceAll(tc.jql, "FIELDID", e.fieldClient.String())
			got := e.keys(e.alice, jql)
			if !sameSet(got, tc.want) {
				t.Fatalf("%s\n got %v\nwant %v", jql, got, tc.want)
			}
		})
	}
}

func TestSearchOrderAndPagination(t *testing.T) {
	e := newSearchEnv(t)
	order := func(jql string) []string { return e.keys(e.alice, jql) }

	if got := order("project = PUB ORDER BY created ASC"); strings.Join(got, ",") != "PUB-5,PUB-3,PUB-1,PUB-2,PUB-4" {
		t.Fatalf("created asc: %v", got)
	}
	if got := order("project = PUB"); strings.Join(got, ",") != "PUB-4,PUB-2,PUB-1,PUB-3,PUB-5" {
		t.Fatalf("default order is created desc: %v", got)
	}
	if got := order("project = PUB ORDER BY priority ASC, key ASC"); got[0] != "PUB-4" {
		t.Fatalf("priority asc first is the highest: %v", got)
	}
	if got := order("project = PUB ORDER BY key DESC"); strings.Join(got, ",") != "PUB-5,PUB-4,PUB-3,PUB-2,PUB-1" {
		t.Fatalf("key desc: %v", got)
	}
	if got := order("project = PUB ORDER BY storyPoints DESC"); strings.Join(got[:3], ",") != "PUB-1,PUB-2,PUB-3" || !sameSet(got[3:], []string{"PUB-4", "PUB-5"}) {
		t.Fatalf("story points desc with nulls last: %v", got)
	}
	if got := order("project = PUB ORDER BY assignee ASC, key ASC"); got[0] != "PUB-1" {
		t.Fatalf("assignee asc: %v", got)
	}
	if got := order("project = PUB ORDER BY cf[Horas] DESC"); got[0] != "PUB-2" {
		t.Fatalf("custom number order: %v", got)
	}

	// Override replaces the ORDER BY of the query and warns.
	out, err := e.jql.Search(e.ctx, port.SearchInput{Viewer: e.viewer(e.alice), Query: "project = PUB ORDER BY created ASC", Order: "key DESC", PageSize: 100})
	if err != nil || out.Page.Items[0].Key != "PUB-5" || len(out.Warnings) != 1 {
		t.Fatalf("override: %v %v %v", err, out.Page.Items[0].Key, out.Warnings)
	}

	// Pages are stable and the total is exact.
	var all []string
	for page := 1; page <= 3; page++ {
		out, err := e.jql.Search(e.ctx, port.SearchInput{Viewer: e.viewer(e.alice), Query: "project = PUB ORDER BY key", Page: page, PageSize: 2, WithTotal: true})
		if err != nil {
			t.Fatal(err)
		}
		if out.Page.Total != 5 || out.PageNumber != page || out.PageSize != 2 {
			t.Fatalf("page %d meta: %+v", page, out)
		}
		for _, it := range out.Page.Items {
			all = append(all, it.Key)
		}
	}
	if strings.Join(all, ",") != "PUB-1,PUB-2,PUB-3,PUB-4,PUB-5" {
		t.Fatalf("paged keys: %v", all)
	}
	// The total is only computed on request.
	out, err = e.jql.Search(e.ctx, port.SearchInput{Viewer: e.viewer(e.alice), Query: "", PageSize: 2})
	if err != nil || out.Page.HasTotal {
		t.Fatalf("total must be optional: %v %+v", err, out.Page)
	}
	// Page size is clamped with a warning; deep pages are rejected.
	out, err = e.jql.Search(e.ctx, port.SearchInput{Viewer: e.viewer(e.alice), PageSize: 1000})
	if err != nil || out.PageSize != 100 || len(out.Warnings) == 0 {
		t.Fatalf("clamp: %v %+v", err, out)
	}
	if _, err = e.jql.Search(e.ctx, port.SearchInput{Viewer: e.viewer(e.alice), Page: 1000, PageSize: 100}); apperror.CodeOf(err) != domain.CodePageOutOfRange {
		t.Fatalf("deep page: %v", err)
	}
}

func TestSearchRecordShape(t *testing.T) {
	e := newSearchEnv(t)
	out, err := e.jql.Search(e.ctx, port.SearchInput{Viewer: e.viewer(e.alice), Query: "key = PUB-1"})
	if err != nil || len(out.Page.Items) != 1 {
		t.Fatalf("%v %+v", err, out)
	}
	it := out.Page.Items[0]
	if it.Key != "PUB-1" || it.ProjectKey != "PUB" || it.Status == nil || it.Status.Name != "Por hacer" || it.Status.Category != "todo" ||
		it.IssueType == nil || it.IssueType.Name != "Error" || it.Priority == nil || it.Priority.Name != "Alta" ||
		it.AssigneeName == nil || *it.AssigneeName != "alice" || it.ReporterName != "bob" ||
		len(it.Labels) != 2 || it.Labels[0] != "bug" || it.DueDate == nil || *it.DueDate != "2026-11-29" ||
		it.StoryPoints == nil || *it.StoryPoints != 5 || !it.Flagged || it.Custom[e.fieldClient.String()] != "Acme" {
		t.Fatalf("record: %+v", it)
	}
}

// TestSearchVisibility proves that the browse rule is applied outside the query: no query shape reaches an
// issue the caller cannot browse, and deleted issues never appear.
func TestSearchVisibility(t *testing.T) {
	e := newSearchEnv(t)
	hostile := []string{
		"", "project = PRIV", `text ~ "secreto"`, "key = PRIV-1", "project = PRIV OR project = PUB",
		"NOT project = PUB", "NOT (project = PUB AND labels = bug)", "labels IS EMPTY", "assignee = alice OR assignee IS EMPTY",
		`summary ~ "login"`, "key = OTH-1", "project = OTH", "assignee = currentUser() OR assignee IS EMPTY",
	}
	visible := map[string]map[string]bool{
		"bob":      {"PUB-1": true, "PUB-2": true, "PUB-3": true, "PUB-4": true, "PUB-5": true},
		"carol":    {}, // guest without project role: nothing
		"dave":     {"OTH-1": true},
		"outsider": {},
	}
	users := map[string]func() port.Viewer{
		"bob": func() port.Viewer { return e.viewer(e.bob) }, "carol": func() port.Viewer { return e.viewer(e.carol) },
		"dave": func() port.Viewer { return e.viewer(e.dave) }, "outsider": func() port.Viewer { return e.viewer(e.outsider) },
	}
	for name, mk := range users {
		for _, jql := range hostile {
			out, err := e.jql.Search(e.ctx, port.SearchInput{Viewer: mk(), Query: jql, PageSize: 100, WithTotal: true})
			if err != nil {
				t.Fatalf("%s %q: %v", name, jql, err)
			}
			for _, it := range out.Page.Items {
				if !visible[name][it.Key] {
					t.Fatalf("%s saw %s with %q", name, it.Key, jql)
				}
			}
			if out.Page.Total != len(out.Page.Items) {
				t.Fatalf("%s total leaks: %d vs %d (%q)", name, out.Page.Total, len(out.Page.Items), jql)
			}
		}
	}
	// Owner of the organization sees the private project (lead on every project); alice too as a member.
	if got := e.keys(e.owner, "project = PRIV"); !sameSet(got, []string{"PRIV-1"}) {
		t.Fatalf("org owner: %v", got)
	}
	if got := e.keys(e.alice, "text ~ secreto"); !sameSet(got, []string{"PRIV-1"}) {
		t.Fatalf("project member: %v", got)
	}
	// The platform administrator sees every live issue; soft-deleted ones stay hidden.
	got := e.keys(e.admin, "")
	if len(got) != 7 {
		t.Fatalf("admin should see 7 live issues, got %v", got)
	}
	for _, k := range got {
		if k == "PUB-6" || k == "PRIV-2" {
			t.Fatalf("deleted issue %s returned", k)
		}
	}
	// A team grant gives access to a private project.
	e.exec(`INSERT INTO project_team_roles (project_id, team_id, role) VALUES ($1, $2, 'viewer')`, e.priv, e.teamBackend)
	if got := e.keys(e.bob, "project = PRIV"); !sameSet(got, []string{"PRIV-1"}) {
		t.Fatalf("team grant: %v", got)
	}
	// A deleted project and a deleted organization membership hide everything.
	e.exec(`UPDATE projects SET deleted_at = now() WHERE id = $1`, e.priv)
	if got := e.keys(e.alice, "project = PRIV"); len(got) != 0 {
		t.Fatalf("deleted project: %v", got)
	}
	e.exec(`DELETE FROM organization_members WHERE user_id = $1`, e.bob)
	if got := e.keys(e.bob, ""); len(got) != 0 {
		t.Fatalf("removed member keeps access: %v", got)
	}
}

func TestSearchScopeAndCurrentProject(t *testing.T) {
	e := newSearchEnv(t)
	out, err := e.jql.Search(e.ctx, port.SearchInput{Viewer: e.viewer(e.alice), Query: "project = currentProject() ORDER BY key", ProjectID: &e.priv, PageSize: 10})
	if err != nil || len(out.Page.Items) != 1 || out.Page.Items[0].Key != "PRIV-1" {
		t.Fatalf("currentProject: %v %+v", err, out.Page.Items)
	}
	// The scope is outside the query: an OR cannot reach other projects.
	out, err = e.jql.Search(e.ctx, port.SearchInput{Viewer: e.viewer(e.alice), Query: "project = PUB OR project = PRIV", ProjectID: &e.priv, PageSize: 10})
	if err != nil || len(out.Page.Items) != 1 {
		t.Fatalf("scope escaped: %v %+v", err, out.Page.Items)
	}
	_, err = e.jql.Search(e.ctx, port.SearchInput{Viewer: e.viewer(e.alice), Query: "project = currentProject()"})
	if apperror.CodeOf(err) != domain.CodeScopeRequired {
		t.Fatalf("scope required: %v", err)
	}
}

func TestSearchQueryErrors(t *testing.T) {
	e := newSearchEnv(t)
	cases := []struct {
		jql, code string
	}{
		{"project =", domain.CodeSyntaxError},
		{"status WAS \"x\"", domain.CodeOperatorUnavailable},
		{"status CHANGED", domain.CodeOperatorUnavailable},
		{"projct = PUB", domain.CodeUnknownField},
		{"timeSpent > 1", domain.CodeFieldUnavailable},
		{"statusCategory = whatever", domain.CodeInvalidQuery},
		{"due < \"nope\"", domain.CodeInvalidQuery},
		{"labels = bug ORDER BY labels", domain.CodeInvalidQuery},
		{strings.Repeat("a", 4001), domain.CodeQueryTooLong},
	}
	for _, tc := range cases {
		_, err := e.jql.Search(e.ctx, port.SearchInput{Viewer: e.viewer(e.alice), Query: tc.jql})
		if apperror.CodeOf(err) != tc.code || apperror.KindOf(err) != apperror.KindValidation {
			t.Errorf("%q: code %q kind %v (%v), want %s", tc.jql, apperror.CodeOf(err), apperror.KindOf(err), err, tc.code)
		}
	}
	var qe *domain.QueryError
	_, err := e.jql.Search(e.ctx, port.SearchInput{Viewer: e.viewer(e.alice), Query: `projct = PUB`})
	if !errors.As(err, &qe) || len(qe.Problems) != 1 || qe.Problems[0].Pos != 0 || qe.Problems[0].Len != 6 || len(qe.Problems[0].Suggestions) == 0 {
		t.Fatalf("problem details: %v", err)
	}
	// Positions are in characters, not bytes.
	_, err = e.jql.Search(e.ctx, port.SearchInput{Viewer: e.viewer(e.alice), Query: `summary ~ "á" AND projct = PUB`})
	if !errors.As(err, &qe) || qe.Problems[0].Pos != 18 {
		t.Fatalf("rune positions: %v %+v", err, qe)
	}
}

func TestValidateReportsEveryProblem(t *testing.T) {
	e := newSearchEnv(t)
	rep, err := e.jql.Validate(e.ctx, e.viewer(e.alice), "projct = PUB AND stat = x", nil)
	if err != nil || rep.Valid || len(rep.Problems) != 2 {
		t.Fatalf("%v %+v", err, rep)
	}
	rep, err = e.jql.Validate(e.ctx, e.viewer(e.alice), "project=PUB and status = Done order by created", nil)
	if err != nil || !rep.Valid || rep.Normalized == "" {
		t.Fatalf("%v %+v", err, rep)
	}
	rep, err = e.jql.Validate(e.ctx, e.viewer(e.alice), "project = ", nil)
	if err != nil || rep.Valid || rep.Problems[0].Code != domain.CodeSyntaxError {
		t.Fatalf("%v %+v", err, rep)
	}
}

func TestSuggest(t *testing.T) {
	e := newSearchEnv(t)
	out, err := e.jql.Suggest(e.ctx, port.SuggestInput{Viewer: e.viewer(e.alice), Field: "assignee", Prefix: "b"})
	if err != nil || len(out.Values) != 1 || out.Values[0].Label != "bob" {
		t.Fatalf("users: %v %+v", err, out.Values)
	}
	// People of other organizations never appear.
	out, _ = e.jql.Suggest(e.ctx, port.SuggestInput{Viewer: e.viewer(e.alice), Field: "assignee", Prefix: "dave"})
	if len(out.Values) != 0 {
		t.Fatalf("leaked another organization: %+v", out.Values)
	}
	out, _ = e.jql.Suggest(e.ctx, port.SuggestInput{Viewer: e.viewer(e.alice), Field: "status", Prefix: "En"})
	if len(out.Values) == 0 {
		t.Fatalf("statuses: %+v", out.Values)
	}
	out, _ = e.jql.Suggest(e.ctx, port.SuggestInput{Viewer: e.viewer(e.alice), Field: "labels", Prefix: "b"})
	if len(out.Values) != 1 || out.Values[0].Value != "bug" {
		t.Fatalf("labels: %+v", out.Values)
	}
	out, _ = e.jql.Suggest(e.ctx, port.SuggestInput{Viewer: e.viewer(e.bob), Field: "project", Prefix: ""})
	if len(out.Values) != 1 || out.Values[0].Value != "PUB" {
		t.Fatalf("bob must not see the private project: %+v", out.Values)
	}
	out, _ = e.jql.Suggest(e.ctx, port.SuggestInput{Viewer: e.viewer(e.alice), Field: "key", Prefix: "PRIV"})
	if len(out.Values) != 1 {
		t.Fatalf("keys: %+v", out.Values)
	}
	out, _ = e.jql.Suggest(e.ctx, port.SuggestInput{Viewer: e.viewer(e.bob), Field: "key", Prefix: "PRIV"})
	if len(out.Values) != 0 {
		t.Fatalf("keys leak: %+v", out.Values)
	}
	out, _ = e.jql.Suggest(e.ctx, port.SuggestInput{Viewer: e.viewer(e.alice), Field: "statusCategory"})
	if len(out.Values) != 3 {
		t.Fatalf("categories: %+v", out.Values)
	}
	if _, err := e.jql.Suggest(e.ctx, port.SuggestInput{Viewer: e.viewer(e.alice), Field: "nope"}); apperror.CodeOf(err) != domain.CodeUnknownField {
		t.Fatalf("unknown field: %v", err)
	}
	out, err = e.jql.Suggest(e.ctx, port.SuggestInput{Viewer: e.viewer(e.alice), Query: "assignee = cur", Cursor: -1})
	if err != nil || len(out.Completions) == 0 || out.Completions[0].Label != "currentUser()" {
		t.Fatalf("completions: %v %+v", err, out.Completions)
	}
	out, _ = e.jql.Suggest(e.ctx, port.SuggestInput{Viewer: e.viewer(e.alice), Query: "Cli", Cursor: -1})
	found := false
	for _, c := range out.Completions {
		if c.Label == "cf[Cliente]" {
			found = true
		}
	}
	if !found {
		// Custom fields are offered as cf[Name] only when the prefix matches that spelling.
		out, _ = e.jql.Suggest(e.ctx, port.SuggestInput{Viewer: e.viewer(e.alice), Query: "cf", Cursor: -1})
		for _, c := range out.Completions {
			if c.Label == "cf[Cliente]" {
				found = true
			}
		}
	}
	if !found {
		t.Fatalf("custom field completion missing: %+v", out.Completions)
	}
}
