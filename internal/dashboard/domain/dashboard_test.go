package domain

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/jhonsferg/corelog/internal/platform/apperror"
)

func TestNormalizeConfigMatrix(t *testing.T) {
	id := uuid.NewString()
	type tc struct {
		name   string
		gadget string
		config string
		want   string // canonical JSON when valid
		field  string // field of the error when invalid
	}
	cases := []tc{
		{"assigned defaults", GadgetAssignedToMe, ``, `{"limit":10,"include_done":false}`, ""},
		{"assigned null", GadgetAssignedToMe, `null`, `{"limit":10,"include_done":false}`, ""},
		{"assigned custom", GadgetAssignedToMe, `{"limit":25,"include_done":true}`, `{"limit":25,"include_done":true}`, ""},
		{"assigned limit high", GadgetAssignedToMe, `{"limit":51}`, "", "limit"},
		{"assigned limit negative", GadgetAssignedToMe, `{"limit":-1}`, "", "limit"},
		{"assigned unknown key", GadgetAssignedToMe, `{"colour":"red"}`, "", ""},
		{"assigned wrong type", GadgetAssignedToMe, `{"limit":"ten"}`, "", ""},
		{"assigned trailing", GadgetAssignedToMe, `{} {}`, "", ""},
		{"created defaults", GadgetCreatedByMe, `{}`, `{"limit":10,"include_done":false}`, ""},
		{"created bad limit", GadgetCreatedByMe, `{"limit":0,"x":1}`, "", ""},

		{"filter by query", GadgetFilterResults, `{"query":"  assignee = currentUser() "}`, `{"filter_id":null,"query":"assignee = currentUser()","columns":["key","summary","status","assignee","updated"],"limit":10}`, ""},
		{"filter by id", GadgetFilterResults, `{"filter_id":"` + id + `","columns":["key","due"],"limit":5}`, `{"filter_id":"` + id + `","query":"","columns":["key","due"],"limit":5}`, ""},
		{"filter none", GadgetFilterResults, `{}`, "", "filter_id"},
		{"filter both", GadgetFilterResults, `{"filter_id":"` + id + `","query":"x"}`, "", "filter_id"},
		{"filter bad id", GadgetFilterResults, `{"filter_id":"nope"}`, "", ""},
		{"filter long query", GadgetFilterResults, `{"query":"` + strings.Repeat("a", 2001) + `"}`, "", "query"},
		{"filter bad column", GadgetFilterResults, `{"query":"x","columns":["password"]}`, "", "columns"},
		{"filter empty columns", GadgetFilterResults, `{"query":"x","columns":[]}`, "", "columns"},
		{"filter repeated column", GadgetFilterResults, `{"query":"x","columns":["key","key"]}`, "", "columns"},
		{"filter limit", GadgetFilterResults, `{"query":"x","limit":100}`, "", "limit"},

		{"stats defaults", GadgetIssueStatistics, `{}`, `{"project_id":null,"group_by":"status","include_done":false,"limit":10}`, ""},
		{"stats group label", GadgetIssueStatistics, `{"group_by":"label","limit":20}`, `{"project_id":null,"group_by":"label","include_done":false,"limit":20}`, ""},
		{"stats bad group", GadgetIssueStatistics, `{"group_by":"sprint"}`, "", "group_by"},
		{"stats bad limit", GadgetIssueStatistics, `{"limit":21}`, "", "limit"},
		{"stats bad project", GadgetIssueStatistics, `{"project_id":"x"}`, "", ""},
		{"pie defaults", GadgetPie, `{"group_by":"priority"}`, `{"project_id":null,"group_by":"priority","include_done":false,"limit":10}`, ""},
		{"pie bad group", GadgetPie, `{"group_by":"nope"}`, "", "group_by"},

		{"two dim defaults", GadgetTwoDimensional, `{}`, `{"project_id":null,"x_axis":"status","y_axis":"assignee","include_done":false,"limit":10}`, ""},
		{"two dim same axes", GadgetTwoDimensional, `{"x_axis":"status","y_axis":"status"}`, "", "y_axis"},
		{"two dim bad axis", GadgetTwoDimensional, `{"x_axis":"mood"}`, "", "x_axis"},
		{"two dim bad limit", GadgetTwoDimensional, `{"limit":-3}`, "", "limit"},

		{"cvr defaults", GadgetCreatedVsResolved, `{}`, `{"project_id":null,"days":30}`, ""},
		{"cvr 7", GadgetCreatedVsResolved, `{"days":7}`, `{"project_id":null,"days":7}`, ""},
		{"cvr 90", GadgetCreatedVsResolved, `{"days":90}`, `{"project_id":null,"days":90}`, ""},
		{"cvr too few", GadgetCreatedVsResolved, `{"days":6}`, "", "days"},
		{"cvr too many", GadgetCreatedVsResolved, `{"days":91}`, "", "days"},

		{"burndown empty", GadgetSprintBurndown, `{}`, `{"project_id":null,"sprint_id":null}`, ""},
		{"burndown sprint", GadgetSprintBurndown, `{"sprint_id":"` + id + `"}`, `{"project_id":null,"sprint_id":"` + id + `"}`, ""},
		{"burndown bad id", GadgetSprintBurndown, `{"sprint_id":"x"}`, "", ""},

		{"activity defaults", GadgetActivityStream, `{}`, `{"project_id":null,"limit":10}`, ""},
		{"activity limit", GadgetActivityStream, `{"limit":51}`, "", "limit"},

		{"recent defaults", GadgetRecentProjects, `{}`, `{"limit":10}`, ""},
		{"recent limit", GadgetRecentProjects, `{"limit":-1}`, "", "limit"},

		{"favorites defaults", GadgetFavorites, `{}`, `{"types":[],"limit":10}`, ""},
		{"favorites types", GadgetFavorites, `{"types":["project","filter","project"]}`, `{"types":["filter","project"],"limit":10}`, ""},
		{"favorites bad type", GadgetFavorites, `{"types":["team"]}`, "", "types"},

		{"text", GadgetText, `{"markdown":"# Hola"}`, `{"markdown":"# Hola"}`, ""},
		{"text empty", GadgetText, `{}`, `{"markdown":""}`, ""},
		{"text long", GadgetText, `{"markdown":"` + strings.Repeat("a", 10001) + `"}`, "", "markdown"},

		{"upcoming defaults", GadgetCalendarUpcoming, `{}`, `{"days":14,"limit":10}`, ""},
		{"upcoming 60", GadgetCalendarUpcoming, `{"days":60,"limit":50}`, `{"days":60,"limit":50}`, ""},
		{"upcoming days", GadgetCalendarUpcoming, `{"days":61}`, "", "days"},
		{"upcoming limit", GadgetCalendarUpcoming, `{"limit":51}`, "", "limit"},

		{"unknown type", "weather", `{}`, "", "type"},
		{"too big", GadgetText, `{"markdown":"` + strings.Repeat("a", 17000) + `"}`, "", ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := NormalizeConfig(c.gadget, json.RawMessage(c.config))
			if c.want != "" {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				if string(got) != c.want {
					t.Fatalf("got  %s\nwant %s", got, c.want)
				}
				// Normalizing a canonical config is a fixed point.
				again, err := NormalizeConfig(c.gadget, got)
				if err != nil || string(again) != c.want {
					t.Fatalf("not idempotent: %s %v", again, err)
				}
				return
			}
			if err == nil {
				t.Fatalf("accepted: %s", got)
			}
			if apperror.KindOf(err) != apperror.KindValidation || apperror.CodeOf(err) != CodeInvalidGadget {
				t.Fatalf("error kind/code: %v", err)
			}
			if c.field != "" {
				details := apperror.DetailsOf(err)
				if len(details) == 0 || details[0].Field != c.field {
					t.Fatalf("details = %+v, want field %q", details, c.field)
				}
			}
		})
	}
}

func TestEveryCatalogTypeHasTitleAndDefaults(t *testing.T) {
	for _, g := range GadgetTypes {
		if !KnownGadget(g) || DefaultTitle(g) == "" {
			t.Errorf("%s: incomplete catalog entry", g)
		}
		if g == GadgetFilterResults {
			continue
		}
		if _, err := NormalizeConfig(g, nil); err != nil {
			t.Errorf("%s: empty config rejected: %v", g, err)
		}
	}
	if KnownGadget("nope") || DefaultTitle("nope") != "" {
		t.Error("unknown type recognised")
	}
}

func TestNormalizeGadgets(t *testing.T) {
	keep := uuid.New()
	existing := map[uuid.UUID]bool{keep: true}
	two := 2
	zero := 0
	in := []GadgetInput{
		{Type: GadgetText, Title: "  Nota ", Config: json.RawMessage(`{"markdown":"a"}`), ColumnIndex: 1},
		{ID: &keep, Type: GadgetAssignedToMe, ColumnIndex: 0, Position: &two},
		{Type: GadgetFavorites, ColumnIndex: 0, Position: &zero},
		{Type: GadgetRecentProjects, ColumnIndex: 1},
	}
	got, err := NormalizeGadgets(Layout2, in, existing)
	if err != nil {
		t.Fatal(err)
	}
	order := []string{}
	for _, g := range got {
		order = append(order, g.Type)
		if g.Title == "" {
			t.Errorf("%s has no title", g.Type)
		}
	}
	if strings.Join(order, ",") != "favorites,assigned_to_me,text,recent_projects" {
		t.Fatalf("order = %v", order)
	}
	if got[0].Position != 0 || got[1].Position != 1 || got[2].Position != 0 || got[3].Position != 1 {
		t.Fatalf("positions not consecutive per column: %+v", got)
	}
	if got[1].ID != keep || got[2].Title != "Nota" {
		t.Fatalf("id or title lost: %+v", got)
	}

	bad := map[string][]GadgetInput{
		"unknown type":   {{Type: "weather"}},
		"bad column":     {{Type: GadgetText, ColumnIndex: 2}},
		"negative col":   {{Type: GadgetText, ColumnIndex: -1}},
		"unknown id":     {{ID: ptr(uuid.New()), Type: GadgetText}},
		"duplicate id":   {{ID: &keep, Type: GadgetText}, {ID: &keep, Type: GadgetText}},
		"bad config":     {{Type: GadgetAssignedToMe, Config: json.RawMessage(`{"limit":99}`)}},
		"long title":     {{Type: GadgetText, Title: strings.Repeat("t", 121)}},
		"too many items": make([]GadgetInput, MaxGadgets+1),
	}
	for name, list := range bad {
		if _, err := NormalizeGadgets(Layout2, list, existing); apperror.KindOf(err) != apperror.KindValidation {
			t.Errorf("%s: err = %v", name, err)
		}
	}
	_, err = NormalizeGadgets(Layout2, []GadgetInput{{Type: GadgetAssignedToMe, Config: json.RawMessage(`{"limit":99}`)}}, nil)
	if d := apperror.DetailsOf(err); len(d) != 1 || d[0].Field != "gadgets[0].config.limit" {
		t.Fatalf("error path = %+v", d)
	}
	// A single column layout rejects the second column.
	if _, err := NormalizeGadgets(Layout1, []GadgetInput{{Type: GadgetText, ColumnIndex: 1}}, nil); err == nil {
		t.Fatal("column 1 accepted in a one column layout")
	}
	// Exactly the maximum is fine.
	full := make([]GadgetInput, MaxGadgets)
	for i := range full {
		full[i] = GadgetInput{Type: GadgetText}
	}
	if got, err := NormalizeGadgets(Layout3, full, nil); err != nil || len(got) != MaxGadgets {
		t.Fatalf("max gadgets: %v", err)
	}
}

func ptr[T any](v T) *T { return &v }

func TestLayoutsAndAccess(t *testing.T) {
	for l, cols := range map[Layout]int{Layout1: 1, Layout2: 2, Layout3: 3, Layout21: 2, Layout12: 2} {
		if !l.Valid() || l.Columns() != cols {
			t.Errorf("%s: columns = %d", l, l.Columns())
		}
	}
	if Layout("4").Valid() {
		t.Error("layout 4 valid")
	}
	if l, err := ParseLayout(""); err != nil || l != Layout2 {
		t.Errorf("default layout: %v %v", l, err)
	}
	if _, err := ParseLayout("5"); err == nil {
		t.Error("bad layout accepted")
	}
	cases := []struct {
		owner  bool
		shared Permission
		access Access
		edit   bool
		manage bool
		name   string
	}{
		{true, "", AccessOwner, true, true, "owner"},
		{true, PermissionView, AccessOwner, true, true, "owner"},
		{false, PermissionEdit, AccessEdit, true, false, "edit"},
		{false, PermissionView, AccessView, false, false, "view"},
		{false, "", AccessNone, false, false, "none"},
	}
	for _, c := range cases {
		a := ResolveAccess(c.owner, c.shared)
		if a != c.access || a.CanEdit() != c.edit || a.CanManage() != c.manage || a.String() != c.name {
			t.Errorf("%+v -> %v", c, a)
		}
	}
}

func TestNormalizeNameAndDescription(t *testing.T) {
	if n, err := NormalizeName("  Panel  "); err != nil || n != "Panel" {
		t.Fatalf("name: %q %v", n, err)
	}
	for _, bad := range []string{"", "   ", strings.Repeat("n", 121)} {
		if _, err := NormalizeName(bad); err == nil {
			t.Errorf("name %q accepted", bad)
		}
	}
	if _, err := NormalizeDescription(strings.Repeat("d", 501)); err == nil {
		t.Error("long description accepted")
	}
	if d, _ := NormalizeDescription(" ok "); d != "ok" {
		t.Errorf("description = %q", d)
	}
}

func TestNormalizeShares(t *testing.T) {
	org, owner, user, team := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	got, err := NormalizeShares([]Share{
		{Type: ShareUser, TargetID: user},
		{Type: ShareTeam, TargetID: team, Permission: PermissionEdit},
		{Type: ShareOrganization, TargetID: uuid.New()},
	}, org, owner)
	if err != nil || got[0].Permission != PermissionView || got[2].TargetID != org {
		t.Fatalf("shares: %+v %v", got, err)
	}
	bad := map[string][]Share{
		"type":       {{Type: "group", TargetID: user}},
		"permission": {{Type: ShareUser, TargetID: user, Permission: "admin"}},
		"target":     {{Type: ShareUser}},
		"owner":      {{Type: ShareUser, TargetID: owner}},
		"duplicate":  {{Type: ShareUser, TargetID: user}, {Type: ShareUser, TargetID: user, Permission: PermissionEdit}},
		"two orgs":   {{Type: ShareOrganization, TargetID: uuid.New()}, {Type: ShareOrganization, TargetID: uuid.New()}},
		"too many":   make([]Share, MaxShares+1),
	}
	for name, list := range bad {
		if _, err := NormalizeShares(list, org, owner); apperror.CodeOf(err) != CodeInvalidShare {
			t.Errorf("%s: err = %v", name, err)
		}
	}
}
