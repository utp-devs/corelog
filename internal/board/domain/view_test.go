package domain

import (
	"testing"

	"github.com/google/uuid"

	"github.com/jhonsferg/corelog/internal/platform/apperror"
)

func ip(n int) *int { return &n }

func TestEvaluateWIP(t *testing.T) {
	for _, tc := range []struct {
		name     string
		count    int
		min, max *int
		want     WIPState
	}{
		{"no limits", 9, nil, nil, WIPOk},
		{"below min", 1, ip(2), ip(5), WIPBelowMin},
		{"at min", 2, ip(2), ip(5), WIPOk},
		{"under max", 4, nil, ip(5), WIPOk},
		{"at max", 5, ip(2), ip(5), WIPAtMax},
		{"over max", 6, nil, ip(5), WIPOverMax},
		{"empty column with min", 0, ip(1), nil, WIPBelowMin},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := EvaluateWIP(tc.count, tc.min, tc.max); got != tc.want {
				t.Fatalf("EvaluateWIP(%d)=%s, want %s", tc.count, got, tc.want)
			}
		})
	}
}

func TestValidateColumns(t *testing.T) {
	a, b, c := uuid.New(), uuid.New(), uuid.New()
	workflow := map[uuid.UUID]bool{a: true, b: true, c: true}
	for _, tc := range []struct {
		name    string
		specs   []ColumnSpec
		wantErr bool
		field   string
	}{
		{"valid with an unmapped status", []ColumnSpec{{Name: "A", StatusIDs: []uuid.UUID{a}}, {Name: "B", StatusIDs: []uuid.UUID{b}}}, false, ""},
		{"empty column is allowed", []ColumnSpec{{Name: "A"}}, false, ""},
		{"no columns", nil, true, "columns"},
		{"status mapped twice", []ColumnSpec{{Name: "A", StatusIDs: []uuid.UUID{a}}, {Name: "B", StatusIDs: []uuid.UUID{a}}}, true, "columns[1].status_ids"},
		{"status outside the workflow", []ColumnSpec{{Name: "A", StatusIDs: []uuid.UUID{uuid.New()}}}, true, "columns[0].status_ids"},
		{"blank name", []ColumnSpec{{Name: "  "}}, true, "columns[0].name"},
		{"duplicate name", []ColumnSpec{{Name: "A"}, {Name: "a"}}, true, "columns[1].name"},
		{"min above max", []ColumnSpec{{Name: "A", MinIssues: ip(5), MaxIssues: ip(2)}}, true, "columns[0].min_issues"},
		{"max zero", []ColumnSpec{{Name: "A", MaxIssues: ip(0)}}, true, "columns[0].max_issues"},
		{"unknown color", []ColumnSpec{{Name: "A", Color: "chartreuse"}}, true, "columns[0].color"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateColumns(tc.specs, workflow)
			if (err != nil) != tc.wantErr {
				t.Fatalf("err=%v, wantErr=%v", err, tc.wantErr)
			}
			if !tc.wantErr {
				return
			}
			if apperror.KindOf(err) != apperror.KindValidation || apperror.CodeOf(err) != "board.invalid_columns" {
				t.Fatalf("error kind/code = %s/%s", apperror.KindOf(err), apperror.CodeOf(err))
			}
			found := false
			for _, d := range apperror.DetailsOf(err) {
				found = found || d.Field == tc.field
			}
			if !found {
				t.Fatalf("details %+v miss field %s", apperror.DetailsOf(err), tc.field)
			}
		})
	}
	if got := UnmappedStatuses([]uuid.UUID{a, b, c}, []Column{{StatusIDs: []uuid.UUID{b}}}); len(got) != 2 || got[0] != a || got[1] != c {
		t.Fatalf("unmapped = %v, want [a c]", got)
	}
}

func TestValidateSettings(t *testing.T) {
	ok := Settings{Name: " Sprint ", SwimlaneMode: SwimlanesEpic, Estimation: EstimationTime, CardFields: []string{"key", "key", "labels"}}
	if err := ValidateSettings(&ok); err != nil || ok.Name != "Sprint" || len(ok.CardFields) != 2 {
		t.Fatalf("valid settings: %v %+v", err, ok)
	}
	for _, bad := range []Settings{
		{Name: "", SwimlaneMode: SwimlanesNone, Estimation: EstimationTime},
		{Name: "x", SwimlaneMode: "diagonal", Estimation: EstimationTime},
		{Name: "x", SwimlaneMode: SwimlanesNone, Estimation: "hours"},
		{Name: "x", SwimlaneMode: SwimlanesNone, Estimation: EstimationTime, CardFields: []string{"nope"}},
		{Name: "x", SwimlaneMode: SwimlanesNone, Estimation: EstimationTime, DoneRetentionDays: ip(-1)},
	} {
		if err := ValidateSettings(&bad); apperror.CodeOf(err) != "board.invalid_settings" {
			t.Errorf("settings %+v: err=%v", bad, err)
		}
	}
}

func TestQueryGrammar(t *testing.T) {
	me, other := uuid.New(), uuid.New()
	card := Card{Key: "ABC-1", Summary: "Arreglar el login", Type: "Bug", TypeKey: "bug", Priority: "Alta", PriorityKey: "high",
		AssigneeID: &me, AssigneeName: "Ada", Labels: []string{"backend", "urgente"}, StatusName: "En curso", Flagged: true}
	unassigned := Card{Summary: "otra", Type: "Tarea", TypeKey: "task", Priority: "Baja", PriorityKey: "low"}
	ctx := QueryContext{CurrentUser: me}
	for _, tc := range []struct {
		query         string
		card, unassig bool
	}{
		{"", true, true},
		{"assignee = currentUser()", true, false},
		{"ASSIGNEE = CURRENTUSER()", true, false},
		{`assignee = "Ada"`, true, false},
		{"assignee != currentUser()", false, true},
		{"assignee is EMPTY", false, true},
		{"labels = backend", true, false},
		{"labels in (x, urgente)", true, false},
		{"priority in (high, medium)", true, false},
		{`priority in ("Alta")`, true, false},
		{"type = bug", true, false},
		{"type != bug", false, true},
		{`text ~ "login"`, true, false},
		{`text ~ "ABC-1"`, true, false},
		{"flagged = true", true, false},
		{"flagged = false", false, true},
		{"status = \"En curso\"", true, false},
		{"type = bug AND labels = backend AND assignee = currentUser()", true, false},
		{"type = bug AND labels = nope", false, false},
	} {
		q, err := ParseQuery(tc.query)
		if err != nil {
			t.Fatalf("%q: %v", tc.query, err)
		}
		if got := q.Match(card, ctx); got != tc.card {
			t.Errorf("%q on card = %v, want %v", tc.query, got, tc.card)
		}
		if got := q.Match(unassigned, ctx); got != tc.unassig {
			t.Errorf("%q on unassigned = %v, want %v", tc.query, got, tc.unassig)
		}
	}
	other2 := Card{AssigneeID: &other}
	if q, _ := ParseQuery("assignee = currentUser()"); q.Match(other2, ctx) {
		t.Error("currentUser() matched somebody else")
	}
	for _, bad := range []string{"foo = 1", "assignee", "assignee =", "assignee = a b", "labels in (a", "labels in a", "text = x", "type ~ x",
		"assignee is full", "labels = x OR labels = y", `text ~ "open`, "priority !", "flagged != true", "type = bug AND"} {
		if _, err := ParseQuery(bad); err == nil {
			t.Errorf("%q parsed without error", bad)
		}
	}
}

func FuzzParseQuery(f *testing.F) {
	for _, seed := range []string{"", "assignee = currentUser()", `text ~ "x"`, "priority in (a,b) AND type = bug", "((", `"`} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, input string) {
		q, err := ParseQuery(input)
		if err == nil {
			q.Match(Card{Summary: input}, QueryContext{})
		}
	})
}

func TestAssembleViewColumnsWIPAndUnmapped(t *testing.T) {
	todo, doing, done, orphan := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	board := &Board{Estimation: EstimationStoryPoints, Columns: []Column{
		{Name: "Por hacer", StatusIDs: []uuid.UUID{todo}, MinIssues: ip(2)},
		{Name: "En curso", StatusIDs: []uuid.UUID{doing, done}, WIPLimit: ip(2)},
	}}
	pts := func(v float64) *float64 { return &v }
	cards := []Card{
		{ID: uuid.New(), StatusID: todo, StoryPoints: pts(3), Type: "Bug", TypeKey: "bug"},
		{ID: uuid.New(), StatusID: doing, StoryPoints: pts(2)},
		{ID: uuid.New(), StatusID: done},
		{ID: uuid.New(), StatusID: doing},
		{ID: uuid.New(), StatusID: orphan},
	}
	quick, _ := ParseQuery("type = bug")
	view := AssembleView(board, cards, []Matcher{quick}, nil, QueryContext{})
	if view.Columns[0].Count != 1 || view.Columns[0].Total != 1 || view.Columns[0].Estimate != 3 || view.Columns[0].WIP != WIPBelowMin {
		t.Errorf("todo column = %+v", view.Columns[0])
	}
	// WIP is evaluated on the total, not on the filtered count.
	if c := view.Columns[1]; c.Count != 0 || c.Total != 3 || c.WIP != WIPOverMax {
		t.Errorf("doing column count=%d total=%d wip=%s, want 0/3/over_max", c.Count, c.Total, c.WIP)
	}
	if view.UnmappedCount != 1 || view.Unmapped.Name != "Sin asignar" {
		t.Errorf("unmapped = %d %q", view.UnmappedCount, view.Unmapped.Name)
	}
	all := AssembleView(board, cards, nil, nil, QueryContext{})
	if all.Columns[1].Count != 3 || all.Columns[1].Estimate != 2 || all.Swimlanes != nil && len(all.Swimlanes) != 0 {
		t.Errorf("unfiltered doing column = %+v lanes=%v", all.Columns[1], all.Swimlanes)
	}
}

func TestAssembleViewSwimlanes(t *testing.T) {
	st := uuid.New()
	ada, bob, epicA := uuid.New(), uuid.New(), uuid.New()
	mk := func(assignee *uuid.UUID, name string, epic *CardRef, parent *CardRef, typ string) Card {
		return Card{ID: uuid.New(), StatusID: st, AssigneeID: assignee, AssigneeName: name, Epic: epic, Parent: parent, TypeKey: typ}
	}
	cards := []Card{
		mk(&bob, "Bob", nil, nil, "task"), mk(nil, "", nil, nil, "bug"), mk(&ada, "Ada", &CardRef{ID: epicA, Key: "A-1", Summary: "Epica"}, &CardRef{ID: uuid.New(), Key: "A-2", Summary: "Hist", Type: "story"}, "bug"),
		mk(&ada, "Ada", nil, nil, "task"),
	}
	base := Board{Columns: []Column{{Name: "C", StatusIDs: []uuid.UUID{st}}}}

	byAssignee := base
	byAssignee.SwimlaneMode = SwimlanesAssignee
	v := AssembleView(&byAssignee, cards, nil, nil, QueryContext{})
	names := []string{}
	for _, l := range v.Swimlanes {
		names = append(names, l.Name)
	}
	if len(names) != 3 || names[0] != "Ada" || names[1] != "Bob" || names[2] != "Sin asignar" || v.Swimlanes[0].Count != 2 {
		t.Errorf("assignee lanes = %+v", v.Swimlanes)
	}
	if v.Columns[0].Cards[0].SwimlaneID != bob.String() || v.Columns[0].Cards[1].SwimlaneID != LaneUnassigned {
		t.Errorf("card lanes = %q %q", v.Columns[0].Cards[0].SwimlaneID, v.Columns[0].Cards[1].SwimlaneID)
	}

	byEpic := base
	byEpic.SwimlaneMode = SwimlanesEpic
	v = AssembleView(&byEpic, cards, nil, nil, QueryContext{})
	if len(v.Swimlanes) != 2 || v.Swimlanes[0].ID != epicA.String() || v.Swimlanes[1].ID != LaneNoEpic || v.Swimlanes[1].Count != 3 {
		t.Errorf("epic lanes = %+v", v.Swimlanes)
	}

	byStory := base
	byStory.SwimlaneMode = SwimlanesStory
	v = AssembleView(&byStory, cards, nil, nil, QueryContext{})
	if len(v.Swimlanes) != 2 || v.Swimlanes[1].ID != LaneNoParent {
		t.Errorf("story lanes = %+v", v.Swimlanes)
	}

	byQuery := base
	byQuery.SwimlaneMode = SwimlanesQuery
	laneID := uuid.New()
	byQuery.Swimlanes = []Swimlane{{ID: laneID, Name: "Bugs", Query: "type = bug"}, {ID: uuid.New(), Name: "Vacia", Query: "labels = zzz"}}
	q1, _ := ParseQuery("type = bug")
	q2, _ := ParseQuery("labels = zzz")
	v = AssembleView(&byQuery, cards, nil, []Matcher{q1, q2}, QueryContext{})
	if len(v.Swimlanes) != 3 || v.Swimlanes[0].Count != 2 || v.Swimlanes[1].Count != 0 || v.Swimlanes[2].ID != LaneOther || v.Swimlanes[2].Count != 2 {
		t.Errorf("query lanes = %+v", v.Swimlanes)
	}
}
