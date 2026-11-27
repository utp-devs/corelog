package postgres

import (
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/jhonsferg/corelog/internal/search/application/port"
	"github.com/jhonsferg/corelog/internal/search/domain"
	"github.com/jhonsferg/corelog/internal/search/domain/coreql"
)

var builderNow = time.Date(2026, 11, 27, 15, 30, 0, 0, time.UTC)

var builderViewer = port.Viewer{UserID: uuid.MustParse("11111111-1111-4111-8111-111111111111"), Timezone: "UTC"}

var builderField = uuid.MustParse("22222222-2222-4222-8222-222222222222")

func builderCustom() map[string]port.CustomFieldDef {
	m := map[string]port.CustomFieldDef{}
	for kind, name := range map[string]string{
		"text": "ctext", "number": "cnum", "date": "cdate", "datetime": "cdt", "select": "csel", "multiselect": "cmulti",
		"checkbox": "cbox", "user": "cuser", "labels": "clabels",
	} {
		def := port.CustomFieldDef{Name: name, Kind: kind, IDs: []uuid.UUID{builderField}}
		m["cf["+name+"]"] = def
		m[name] = def
	}
	return m
}

func catalogFor(custom map[string]port.CustomFieldDef) coreql.FieldCatalog {
	var defs []coreql.FieldDef
	for ref, def := range custom {
		if !strings.HasPrefix(ref, "cf[") {
			continue
		}
		fd := coreql.FieldDef{Name: ref, Type: coreql.TypeText, Sortable: true}
		switch def.Kind {
		case "number":
			fd.Type = coreql.TypeNumber
		case "date":
			fd.Type = coreql.TypeDate
		case "datetime":
			fd.Type = coreql.TypeDateTime
		case "select", "multiselect":
			fd.Type = coreql.TypeEnum
		case "checkbox":
			fd.Type = coreql.TypeBool
		case "user":
			fd.Type = coreql.TypeUser
		case "labels":
			fd.Type = coreql.TypeLabels
		}
		defs = append(defs, fd)
		bare := fd
		bare.Name = strings.TrimSuffix(strings.TrimPrefix(ref, "cf["), "]")
		defs = append(defs, bare)
	}
	return coreql.CompositeCatalog{Top: coreql.SystemCatalog(), Base: coreql.NewMapCatalog(defs...)}
}

// buildFor parses, validates and builds a query. It reports a validation failure as an error.
func buildFor(jql string, versions VersionResolver, mut func(*port.IssueQuery)) (*BuiltQuery, error) {
	q, err := coreql.Parse(jql)
	if err != nil {
		return nil, err
	}
	custom := builderCustom()
	if errs := coreql.Validate(q, catalogFor(custom), nil); len(errs) > 0 {
		return nil, fmt.Errorf("validate: %s", errs[0].Message)
	}
	in := port.IssueQuery{Viewer: builderViewer, Query: q, Custom: custom, Now: builderNow, Page: 1, PageSize: 20}
	if mut != nil {
		mut(&in)
	}
	return BuildIssueQuery(in, versions)
}

func mustBuild(t *testing.T, jql string) *BuiltQuery {
	t.Helper()
	b, err := buildFor(jql, nil, nil)
	if err != nil {
		t.Fatalf("%q: %v", jql, err)
	}
	return b
}

var placeholder = regexp.MustCompile(`\$(\d+)::`)

// maxPlaceholder returns the highest parameter number of a statement.
func maxPlaceholder(sql string) int {
	maxN := 0
	for _, m := range placeholder.FindAllStringSubmatch(sql, -1) {
		n, _ := strconv.Atoi(m[1])
		if n > maxN {
			maxN = n
		}
	}
	return maxN
}

func TestBuilderGoldenShapes(t *testing.T) {
	cases := []struct {
		name string
		jql  string
		sql  []string // fragments the statement must contain
		args []any    // arguments after the viewer, admin flag and project scope
	}{
		{"project", "project = CORE", []string{"lower(p.key) = lower($3::text)", "p.id::text = lower($3::text)"}, []any{"CORE"}},
		{"key", "key = core-12", []string{"lower(p.key) = lower($3::text) AND i.key_number = $4::int"}, []any{"core", 12}},
		{"key contains", `key ~ "COR"`, []string{"(p.key || '-' || i.key_number::text) ILIKE $3::text ESCAPE"}, []any{"%COR%"}},
		{"type", "type = Error", []string{"FROM issue_types it WHERE it.id = i.issue_type_id", "it.legacy_key = lower("}, []any{"Error"}},
		{"status", `status = "In Progress"`, []string{"FROM statuses st WHERE st.id = i.status_id AND (lower(st.name) = lower($3::text))"}, []any{"In Progress"}},
		{"status category", "statusCategory = done", []string{"corelog_issue_category(i.status_id, i.status) = $3::text"}, []any{"done"}},
		{"status category alias", `statusCategory = "To Do"`, []string{"corelog_issue_category"}, []any{"todo"}},
		{"priority", "priority = High", []string{"FROM priorities pr WHERE pr.id = i.priority_id"}, []any{"High"}},
		{"resolution unresolved", "resolution = Unresolved", []string{"i.resolution_id IS NULL"}, nil},
		{"assignee current user", "assignee = currentUser()", []string{"i.assignee_id = $3::uuid"}, []any{builderViewer.UserID}},
		{"assignee by text", `assignee = "a@b.c"`, []string{"i.assignee_id IN (SELECT u.id FROM users u WHERE lower(u.email) = lower($3::text)"}, []any{"a@b.c"}},
		{"assignee team", `assignee IN (membersOf("Backend"))`, []string{"JOIN teams t ON t.id = tm.team_id", "t.organization_id = p.organization_id"}, []any{"Backend"}},
		{"assignee empty", "assignee IS EMPTY", []string{"i.assignee_id IS NULL"}, nil},
		{"assignee not empty", "assignee IS NOT EMPTY", []string{"NOT COALESCE(i.assignee_id IS NULL, FALSE)"}, nil},
		{"watcher", "watcher = currentUser()", []string{"FROM watchers w WHERE w.issue_id = i.id AND w.user_id = $3::uuid"}, []any{builderViewer.UserID}},
		{"labels", "labels = bug", []string{"FROM issue_labels il JOIN labels l ON l.id = il.label_id"}, []any{"bug"}},
		{"labels not in", "labels NOT IN (a, b)", []string{"NOT COALESCE((EXISTS", "OR EXISTS"}, []any{"a", "b"}},
		{"component", "component = Web", []string{"FROM issue_components ic JOIN components c"}, []any{"Web"}},
		{"sprint open", "sprint IN (openSprints())", []string{"FROM sprints s WHERE s.status = $3::text"}, []any{"active"}},
		{"sprint name", `sprint = "S1"`, []string{"lower(s.name) = lower($3::text)"}, []any{"S1"}},
		{"epic", "epic = CORE-1", []string{"i.parent_id IN (SELECT e.id", "SELECT c.id FROM issues c WHERE c.deleted_at IS NULL AND c.parent_id IN"}, []any{"CORE", 1, "CORE", 1}},
		{"parent", "parent = CORE-1", []string{"i.parent_id IN (SELECT e.id"}, []any{"CORE", 1}},
		{"summary contains", `summary ~ "a_b%"`, []string{"i.summary ILIKE $3::text ESCAPE"}, []any{`%a\_b\%%`}},
		{"summary equals", `summary = "x"`, []string{"lower(i.summary) = lower($3::text)"}, []any{"x"}},
		{"comment", `comment ~ "x"`, []string{"to_tsvector('spanish', c.body) @@ websearch_to_tsquery('spanish', $3::text)", "c.body ILIKE"}, []any{"x", "%x%"}},
		{"text", `text ~ "x"`, []string{"i.search_vector @@ (websearch_to_tsquery('spanish', $3::text) || websearch_to_tsquery('simple', $3::text))"}, []any{"x", "%x%"}},
		{"due date", `due >= "2026-01-31"`, []string{"i.due_date >= $3::text::date"}, []any{"2026-01-31"}},
		{"created relative", "created >= -7d", []string{"i.created_at >= $3::timestamptz"}, []any{builderNow.AddDate(0, 0, -7)}},
		{"created month", "created < 1M", []string{"i.created_at < $3::timestamptz"}, []any{builderNow.AddDate(0, 1, 0)}},
		{"created day literal", `created = "2026-03-04"`, []string{"i.created_at >= $3::timestamptz AND i.created_at < $4::timestamptz"}, []any{time.Date(2026, 3, 4, 0, 0, 0, 0, time.UTC), time.Date(2026, 3, 5, 0, 0, 0, 0, time.UTC)}},
		{"created gt day", `created > "2026-03-04"`, []string{"i.created_at >= $3::timestamptz"}, []any{time.Date(2026, 3, 5, 0, 0, 0, 0, time.UTC)}},
		{"created le day", `created <= "2026-03-04"`, []string{"i.created_at < $3::timestamptz"}, []any{time.Date(2026, 3, 5, 0, 0, 0, 0, time.UTC)}},
		{"created minute", `created >= "2026-03-04 10:15"`, []string{"i.created_at >= $3::timestamptz"}, []any{time.Date(2026, 3, 4, 10, 15, 0, 0, time.UTC)}},
		{"start of day", "updated >= startOfDay()", []string{"i.updated_at >= $3::timestamptz"}, []any{time.Date(2026, 11, 27, 0, 0, 0, 0, time.UTC)}},
		{"start of day offset", "updated >= startOfDay(-1d)", []string{"i.updated_at >= $3::timestamptz"}, []any{time.Date(2026, 11, 26, 0, 0, 0, 0, time.UTC)}},
		{"start of week", "updated >= startOfWeek()", nil, []any{time.Date(2026, 11, 23, 0, 0, 0, 0, time.UTC)}},
		{"end of week", "updated <= endOfWeek()", nil, []any{time.Date(2026, 11, 29, 23, 59, 59, 999999000, time.UTC)}},
		{"start of month", "updated >= startOfMonth()", nil, []any{time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)}},
		{"end of month", "updated <= endOfMonth()", nil, []any{time.Date(2026, 11, 30, 23, 59, 59, 999999000, time.UTC)}},
		{"end of day", "updated <= endOfDay()", nil, []any{time.Date(2026, 11, 27, 23, 59, 59, 999999000, time.UTC)}},
		{"now", "updated <= now()", nil, []any{builderNow}},
		{"votes", "votes >= 3", []string{"i.vote_count >= $3::text::numeric"}, []any{"3"}},
		{"story points decimal", "storyPoints > 2.5", []string{"i.story_points > $3::text::numeric"}, []any{"2.5"}},
		{"estimate", "originalEstimate <= 3600", []string{"i.original_estimate_seconds <= $3::text::numeric"}, []any{"3600"}},
		{"flagged", "flagged = true", []string{"i.flagged = $3::boolean"}, []any{true}},
		{"custom text", "cf[ctext] = Acme", []string{"lower((i.custom ->> $3::text)) = lower($4::text)"}, []any{builderField.String(), "Acme"}},
		{"custom bare", "ctext ~ ac", []string{"(i.custom ->> $3::text) ILIKE $4::text ESCAPE"}, []any{builderField.String(), "%ac%"}},
		{"custom number", "cf[cnum] >= 3", []string{"jsonb_typeof(i.custom -> $3::text) = 'number'", ">= $4::text::numeric"}, []any{builderField.String(), "3"}},
		{"custom date", `cf[cdate] < "2026-01-01"`, []string{"(i.custom ->> $3::text) < $4::text"}, []any{builderField.String(), "2026-01-01"}},
		{"custom datetime", `cf[cdt] >= -1d`, []string{"(i.custom ->> $3::text) >= $4::text"}, []any{builderField.String(), "2026-11-26T15:30:00Z"}},
		{"custom select", `cf[csel] = "Prod"`, []string{"FROM custom_fields cfo, jsonb_array_elements(cfo.options) o WHERE cfo.id = $3::text::uuid"}, []any{builderField.String(), "Prod"}},
		{"custom multiselect", `cf[cmulti] = "Prod"`, []string{"jsonb_array_elements_text(CASE WHEN jsonb_typeof(i.custom -> $3::text) = 'array'"}, nil},
		{"custom checkbox", "cf[cbox] = true", []string{"COALESCE((i.custom ->> $3::text), 'false') = $4::text"}, []any{builderField.String(), "true"}},
		{"custom user", "cf[cuser] = currentUser()", []string{"(i.custom ->> $3::text) = $4::uuid::text"}, nil},
		{"custom labels", "cf[clabels] = x", []string{"lower(e.v) = lower($4::text)"}, nil},
		{"custom empty", "cf[ctext] IS EMPTY", []string{"IN ('null'::jsonb, '[]'::jsonb, '\"\"'::jsonb)"}, nil},
		{"order", "ORDER BY updated DESC, summary", []string{"ORDER BY i.updated_at DESC NULLS LAST, lower(i.summary) ASC NULLS LAST, i.id ASC"}, nil},
		{"order default", "", []string{"ORDER BY i.created_at DESC NULLS LAST, i.id ASC"}, nil},
		{"order dedupe", "ORDER BY created, created DESC", []string{"ORDER BY i.created_at ASC NULLS LAST, i.id ASC"}, nil},
		{"order key", "ORDER BY key DESC", []string{"p.key DESC NULLS LAST, i.key_number DESC NULLS LAST, i.id ASC"}, nil},
		{"order custom", "ORDER BY cf[cnum]", []string{"ORDER BY (CASE WHEN jsonb_typeof(i.custom -> $3::text) = 'number'"}, nil},
		{"not", "NOT labels = a", []string{"NOT COALESCE(EXISTS"}, nil},
		{"and or", "labels = a AND labels = b OR labels = c", []string{" AND ", " OR "}, nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			b := mustBuild(t, tc.jql)
			for _, frag := range tc.sql {
				if !strings.Contains(b.SelectSQL, frag) {
					t.Errorf("SQL lacks %q:\n%s", frag, b.SelectSQL)
				}
			}
			if tc.args != nil {
				// Arguments: viewer and admin flag first, then the user values; limit and offset last.
				got := b.SelectArgs[2 : len(b.SelectArgs)-2]
				if tc.name == "custom text" || strings.HasPrefix(tc.name, "custom") {
					got = b.SelectArgs[2 : len(b.SelectArgs)-2]
				}
				if fmt.Sprint(got) != fmt.Sprint(tc.args) {
					t.Errorf("args = %v, want %v", got, tc.args)
				}
			}
			if maxPlaceholder(b.SelectSQL) != len(b.SelectArgs) {
				t.Errorf("placeholders %d vs args %d", maxPlaceholder(b.SelectSQL), len(b.SelectArgs))
			}
			if maxPlaceholder(b.CountSQL) != len(b.CountArgs) {
				t.Errorf("count placeholders %d vs args %d", maxPlaceholder(b.CountSQL), len(b.CountArgs))
			}
			if strings.Contains(b.CountSQL, "ORDER BY") || strings.Contains(b.CountSQL, "LIMIT") {
				t.Errorf("count statement must not sort or page: %s", b.CountSQL)
			}
		})
	}
}

func TestBuilderVisibilityIsOutsideUserTree(t *testing.T) {
	b := mustBuild(t, "project = A OR NOT project = B OR labels IS EMPTY")
	vis := strings.Index(b.SelectSQL, "vom.user_id")
	user := strings.Index(b.SelectSQL, "lower(p.key) = lower($")
	if vis < 0 || user < 0 || vis > user {
		t.Fatalf("visibility must precede the user expression:\n%s", b.SelectSQL)
	}
	// The user expression is one parenthesized AND operand: the visibility test is a sibling, not inside.
	if !strings.Contains(b.SelectSQL, "p.visibility <> 'private')))) AND (") {
		t.Fatalf("user expression must be wrapped after the visibility test:\n%s", b.SelectSQL)
	}
	if !strings.Contains(b.SelectSQL, "i.deleted_at IS NULL") || !strings.Contains(b.SelectSQL, "p.deleted_at IS NULL") {
		t.Fatal("soft-deleted issues and projects must be excluded")
	}
	scoped, err := buildFor("labels = a OR project = B", nil, func(q *port.IssueQuery) { id := uuid.New(); q.ProjectID = &id })
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(scoped.SelectSQL, "AND p.id = $3::uuid AND (") {
		t.Fatalf("project scope must sit outside the user expression:\n%s", scoped.SelectSQL)
	}
}

func TestBuilderPagingAndCaps(t *testing.T) {
	b, err := buildFor("", nil, func(q *port.IssueQuery) { q.Page, q.PageSize = 3, 25 })
	if err != nil {
		t.Fatal(err)
	}
	n := len(b.SelectArgs)
	if b.SelectArgs[n-2] != 25 || b.SelectArgs[n-1] != 50 {
		t.Fatalf("limit/offset = %v %v", b.SelectArgs[n-2], b.SelectArgs[n-1])
	}
	b, _ = buildFor("", nil, func(q *port.IssueQuery) { q.PageSize = 5000 })
	if b.SelectArgs[len(b.SelectArgs)-2] != 100 {
		t.Fatal("page size must be capped at 100")
	}
	b, _ = buildFor("", nil, func(q *port.IssueQuery) { q.Page, q.PageSize = 0, 0 })
	if b.SelectArgs[len(b.SelectArgs)-1] != 0 {
		t.Fatal("page defaults to 1")
	}
	if _, err = buildFor("", nil, func(q *port.IssueQuery) { q.Page, q.PageSize = 101, 100 }); !errors.Is(err, domain.ErrPageOutOfRange) {
		t.Fatalf("offset cap: %v", err)
	}

	// Clause, value and depth caps.
	many := strings.TrimSuffix(strings.Repeat("labels = a AND ", 101), " AND ")
	if _, err := buildFor(many, nil, nil); !isBuildCode(err, domain.CodeQueryTooComplex) {
		t.Fatalf("clause cap: %v", err)
	}
	values := "labels IN (" + strings.TrimSuffix(strings.Repeat("a, ", 501), ", ") + ")"
	if _, err := buildFor(values, nil, nil); !isBuildCode(err, domain.CodeQueryTooComplex) {
		t.Fatalf("value cap: %v", err)
	}
	deep := strings.Repeat("NOT ", 60) + "labels = a"
	if _, err := buildFor(deep, nil, nil); !isBuildCode(err, domain.CodeQueryTooComplex) {
		t.Fatalf("depth cap: %v", err)
	}
	long := strings.TrimSuffix(strings.Repeat("labels = a OR ", 100), " OR ")
	if _, err := buildFor(long, nil, nil); err != nil {
		t.Fatalf("a flat chain of 100 clauses is allowed: %v", err)
	}
}

func isBuildCode(err error, code string) bool {
	var be *port.BuildError
	return errors.As(err, &be) && be.Code == code
}

func TestBuilderErrors(t *testing.T) {
	cases := []struct {
		jql, code string
	}{
		{"timeSpent > 1", domain.CodeFieldUnavailable},
		{"timeSpent IS EMPTY", domain.CodeFieldUnavailable},
		{"fixVersion = 1.0", domain.CodeFieldUnavailable},
		{"fixVersion IS EMPTY", domain.CodeFieldUnavailable},
		{"project = currentProject()", domain.CodeScopeRequired},
		{"statusCategory = nope", domain.CodeInvalidQuery},
		{"ORDER BY labels", ""},
	}
	for _, tc := range cases {
		_, err := buildFor(tc.jql, nil, nil)
		if tc.code == "" {
			if err == nil {
				t.Errorf("%q: expected an error", tc.jql)
			}
			continue
		}
		if !isBuildCode(err, tc.code) {
			t.Errorf("%q: %v, want code %s", tc.jql, err, tc.code)
		}
	}
	// A date value that slips past validation is rejected too.
	bad := &coreql.Query{Expr: &coreql.Compare{Field: coreql.Field{Name: "due"}, Op: coreql.OpLt, Value: &coreql.Word{Text: "x"}}}
	if _, err := BuildIssueQuery(port.IssueQuery{Viewer: builderViewer, Query: bad}, nil); !isBuildCode(err, domain.CodeInvalidQuery) {
		t.Fatalf("bad date: %v", err)
	}
	// An unknown field that slips past validation is still rejected by the builder.
	q, _ := coreql.Parse("nosuch = 1")
	if _, err := BuildIssueQuery(port.IssueQuery{Viewer: builderViewer, Query: q}, nil); !isBuildCode(err, domain.CodeUnknownField) {
		t.Fatalf("unknown field: %v", err)
	}
	// So is an order by a field that cannot be sorted.
	q, _ = coreql.Parse("ORDER BY labels")
	if _, err := BuildIssueQuery(port.IssueQuery{Viewer: builderViewer, Query: q}, nil); !isBuildCode(err, domain.CodeInvalidQuery) {
		t.Fatalf("order by labels: %v", err)
	}
	// A nil query is an empty query.
	if _, err := BuildIssueQuery(port.IssueQuery{Viewer: builderViewer}, nil); err != nil {
		t.Fatal(err)
	}
}

type fakeVersions struct {
	err   error
	calls []port.VersionSpec
}

func (f *fakeVersions) Condition(kind port.VersionKind, spec port.VersionSpec) (SQLFragment, error) {
	f.calls = append(f.calls, spec)
	if f.err != nil {
		return SQLFragment{}, f.err
	}
	if spec.Empty {
		return SQLFragment{SQL: "NOT EXISTS (SELECT 1 FROM fake_versions fv WHERE fv.issue_id = i.id AND fv.kind = '" + string(kind) + "')"}, nil
	}
	if spec.Function != "" {
		return SQLFragment{SQL: "EXISTS (SELECT 1 FROM fake_versions fv WHERE fv.issue_id = i.id AND fv.state = ?)", Args: []string{spec.Function}}, nil
	}
	return SQLFragment{SQL: "EXISTS (SELECT 1 FROM fake_versions fv WHERE fv.issue_id = i.id AND fv.kind = '" + string(kind) + "' AND fv.name = ?)", Args: []string{spec.Name}}, nil
}

func TestBuilderVersionsThroughPort(t *testing.T) {
	fake := &fakeVersions{}
	b, err := buildFor(`fixVersion = "1.0" AND affectedVersion IN (a, b) AND fixVersion IN (releasedVersions()) AND fixVersion IS EMPTY`, fake, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(b.SelectSQL, "fv.kind = 'fix' AND fv.name = $3::text") || !strings.Contains(b.SelectSQL, "fv.kind = 'affects' AND fv.name = $4::text") {
		t.Fatalf("fragments not bound:\n%s", b.SelectSQL)
	}
	if fmt.Sprint(b.SelectArgs[2:6]) != "[1.0 a b releasedversions]" {
		t.Fatalf("args: %v", b.SelectArgs)
	}
	if len(fake.calls) != 5 {
		t.Fatalf("calls: %+v", fake.calls)
	}
	// An unavailable resolver is reported as an unavailable field.
	unavailable := &fakeVersions{err: ErrVersionsUnavailable}
	for _, jql := range []string{"fixVersion = 1.0", "fixVersion IS EMPTY"} {
		if _, err := buildFor(jql, unavailable, nil); !isBuildCode(err, domain.CodeFieldUnavailable) {
			t.Fatalf("%q: %v", jql, err)
		}
	}
	other := &fakeVersions{err: errors.New("boom")}
	if _, err := buildFor("fixVersion = 1.0", other, nil); !isBuildCode(err, domain.CodeInvalidQuery) {
		t.Fatalf("other resolver error: %v", err)
	}
	// A fragment whose placeholders do not match its arguments is rejected.
	bad := badFragments{}
	if _, err := buildFor("fixVersion = 1.0", bad, nil); !isBuildCode(err, domain.CodeInvalidQuery) {
		t.Fatalf("bad fragment: %v", err)
	}
}

type badFragments struct{}

func (badFragments) Condition(port.VersionKind, port.VersionSpec) (SQLFragment, error) {
	return SQLFragment{SQL: "x = ? AND y = ?", Args: []string{"one"}}, nil
}

func TestVersionTablesFragments(t *testing.T) {
	v := VersionTables{}
	f, err := v.Condition(port.VersionFix, port.VersionSpec{Name: "1.0"})
	if err != nil || strings.Count(f.SQL, "?") != len(f.Args) || !strings.Contains(f.SQL, "iv.kind = 'fix'") {
		t.Fatalf("%v %+v", err, f)
	}
	f, _ = v.Condition(port.VersionAffects, port.VersionSpec{Function: "releasedversions"})
	if !strings.Contains(f.SQL, "v.status = 'released'") || !strings.Contains(f.SQL, "'affects'") {
		t.Fatalf("%+v", f)
	}
	f, _ = v.Condition(port.VersionFix, port.VersionSpec{Function: "unreleasedversions"})
	if !strings.Contains(f.SQL, "v.status = 'unreleased'") {
		t.Fatalf("%+v", f)
	}
	f, _ = v.Condition(port.VersionFix, port.VersionSpec{Empty: true})
	if !strings.HasPrefix(f.SQL, "NOT EXISTS") {
		t.Fatalf("%+v", f)
	}
	if _, err := v.Condition(port.VersionFix, port.VersionSpec{Function: "now"}); err == nil {
		t.Fatal("unknown function must fail")
	}
}

func TestBuilderTimeZones(t *testing.T) {
	loc, err := time.LoadLocation("America/Bogota")
	if err != nil {
		t.Skip("tzdata unavailable")
	}
	mut := func(q *port.IssueQuery) { q.Viewer.Timezone = "America/Bogota" }
	b, err := buildFor(`created = "2026-03-04"`, nil, mut)
	if err != nil {
		t.Fatal(err)
	}
	want := time.Date(2026, 3, 4, 0, 0, 0, 0, loc).UTC()
	if got := b.SelectArgs[2].(time.Time); !got.Equal(want) {
		t.Fatalf("day start in viewer zone = %v, want %v", got, want)
	}
	// Now is 15:30 UTC = 10:30 in Bogota: the local day started at 05:00 UTC.
	b, _ = buildFor("created >= startOfDay()", nil, mut)
	if got := b.SelectArgs[2].(time.Time); !got.Equal(time.Date(2026, 11, 27, 5, 0, 0, 0, time.UTC)) {
		t.Fatalf("start of local day = %v", got)
	}
	b, _ = buildFor("due = startOfDay()", nil, mut)
	if b.SelectArgs[2] != "2026-11-27" {
		t.Fatalf("date column compares the local date: %v", b.SelectArgs[2])
	}
}

// hostile values that would break out of a quoted literal, a comment or an identifier.
var hostileValues = []string{
	`zq9x'; DROP TABLE issues; --`,
	`zq9x" OR "1"="1`,
	`zq9x') UNION SELECT password_hash FROM users --`,
	`zq9x$$; DELETE FROM users; $$`,
	`zq9x\'); SELECT pg_sleep(10); --`,
	"zq9x\n/* comment */ OR 1=1",
	`zq9x)) OR TRUE --`,
	`zq9x'||(SELECT 1)||'`,
	`zq9x; SET ROLE postgres`,
}

// TestBuilderNeverConcatenatesValues proves the injection guard: for every field and operator the SQL text is
// identical whatever the value, and every hostile value travels only as a bind parameter.
func TestBuilderNeverConcatenatesValues(t *testing.T) {
	templates := []string{
		"project = %s", "key = %s", "key ~ %s", "type = %s", "status = %s", "statusCategory = %s", "priority = %s",
		"resolution = %s", "assignee = %s", "reporter = %s", "watcher = %s", "labels = %s", "labels IN (%s, x)",
		"component = %s", "sprint = %s", "epic = %s", "parent = %s", "summary = %s", "summary ~ %s", "summary !~ %s",
		"description ~ %s", "comment ~ %s", "text ~ %s", "cf[ctext] = %s", "cf[ctext] ~ %s", "cf[csel] = %s",
		"cf[cmulti] = %s", "cf[clabels] = %s", "cf[cuser] = %s", "assignee IN (membersOf(%s))",
		"NOT labels = %s", "(labels = %s OR labels = %s)", "labels = %s ORDER BY updated",
	}
	for _, tpl := range templates {
		var baseSQL string
		for i, payload := range append([]string{"plain"}, hostileValues...) {
			quoted := `"` + strings.NewReplacer(`\`, `\\`, `"`, `\"`).Replace(payload) + `"`
			jql := strings.ReplaceAll(tpl, "%s", quoted)
			q, err := coreql.Parse(jql)
			if err != nil {
				t.Fatalf("%q: %v", jql, err)
			}
			custom := builderCustom()
			if len(coreql.Validate(q, catalogFor(custom), nil)) > 0 {
				break
			}
			b, err := BuildIssueQuery(port.IssueQuery{Viewer: builderViewer, Query: q, Custom: custom, Now: builderNow, Page: 1, PageSize: 10}, nil)
			if err != nil {
				break
			}
			if i == 0 {
				baseSQL = b.SelectSQL
				continue
			}
			if b.SelectSQL != baseSQL {
				t.Fatalf("%q: SQL depends on the value:\n%s\nvs\n%s", tpl, b.SelectSQL, baseSQL)
			}
			if strings.Contains(b.SelectSQL, "zq9x") || strings.Contains(b.CountSQL, "zq9x") {
				t.Fatalf("%q: value text leaked into SQL: %s", tpl, b.SelectSQL)
			}
			found := false
			for _, a := range b.SelectArgs {
				escaped := strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(payload)
				if s, ok := a.(string); ok && (strings.Contains(s, payload) || strings.Contains(s, escaped)) {
					found = true
				}
			}
			if !found && !strings.HasPrefix(tpl, "key") && !strings.HasPrefix(tpl, "epic") && !strings.HasPrefix(tpl, "parent") {
				t.Fatalf("%q: payload %q is not bound as a parameter: %v", tpl, payload, b.SelectArgs)
			}
		}
	}
}

// TestBuilderCustomNamesAreNeverSQL checks that a hostile custom field name never reaches the statement.
func TestBuilderCustomNamesAreNeverSQL(t *testing.T) {
	name := `x'; DROP TABLE issues; --`
	id := uuid.New()
	custom := map[string]port.CustomFieldDef{port.CustomRef("cf[" + name + "]"): {Name: name, Kind: "text", IDs: []uuid.UUID{id}}}
	q, err := coreql.Parse(`cf[` + name + `] = x ORDER BY cf[` + name + `]`)
	if err != nil {
		t.Fatal(err)
	}
	b, err := BuildIssueQuery(port.IssueQuery{Viewer: builderViewer, Query: q, Custom: custom, Now: builderNow}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(b.SelectSQL, "DROP") || strings.Contains(b.SelectSQL, "x'") {
		t.Fatalf("custom field name leaked:\n%s", b.SelectSQL)
	}
	if !strings.Contains(fmt.Sprint(b.SelectArgs), id.String()) {
		t.Fatalf("field id must be bound: %v", b.SelectArgs)
	}
}

// FuzzBuildIssueQuery builds whatever parses and validates: no panic, balanced parameters, no value text in SQL.
func FuzzBuildIssueQuery(f *testing.F) {
	for _, s := range []string{
		"project = CORE", `labels = "a'b" OR NOT assignee = currentUser()`, "created >= -7d ORDER BY updated DESC",
		`text ~ "x"`, "cf[ctext] = 1", `assignee IN (membersOf("t"))`, "((a = 1))", "key = A-1 AND epic = A-2",
	} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, jql string) {
		q, err := coreql.Parse(jql)
		if err != nil {
			return
		}
		custom := builderCustom()
		if len(coreql.Validate(q, catalogFor(custom), nil)) > 0 {
			return
		}
		b, err := BuildIssueQuery(port.IssueQuery{Viewer: builderViewer, Query: q, Custom: custom, Now: builderNow, Page: 1, PageSize: 10}, nil)
		if err != nil {
			return
		}
		if maxPlaceholder(b.SelectSQL) != len(b.SelectArgs) || maxPlaceholder(b.CountSQL) != len(b.CountArgs) {
			t.Fatalf("unbalanced parameters for %q", jql)
		}
		for _, a := range b.SelectArgs {
			if s, ok := a.(string); ok && len(s) > 3 && strings.Contains(b.SelectSQL, s) {
				t.Fatalf("value %q appears in the SQL of %q", s, jql)
			}
		}
	})
}
