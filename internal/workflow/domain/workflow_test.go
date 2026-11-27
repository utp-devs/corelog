package domain

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"github.com/jhonsferg/corelog/internal/platform/apperror"
)

func linear() Workflow {
	return Workflow{
		Initial: "a",
		Statuses: []Status{
			{ID: "a", Name: "A", Category: CategoryTodo},
			{ID: "b", Name: "B", Category: CategoryInProgress},
			{ID: "c", Name: "C", Category: CategoryDone},
		},
		Transitions: []Transition{
			{ID: "t1", Name: "Go B", From: []string{"a"}, To: "b"},
			{ID: "t2", Name: "Go C", From: []string{"b"}, To: "c"},
		},
	}
}

func issueByPath(issues []Issue, path string) (Issue, bool) {
	for _, i := range issues {
		if i.Path == path {
			return i, true
		}
	}
	return Issue{}, false
}

func TestValidate(t *testing.T) {
	tests := []struct {
		name     string
		mutate   func(w *Workflow)
		wantPath string
		wantCode string
	}{
		{"valid", func(w *Workflow) {}, "", ""},
		{"no initial", func(w *Workflow) { w.Initial = "" }, "initial", IssueNoInitialStatus},
		{"unknown initial", func(w *Workflow) { w.Initial = "zzz" }, "initial", IssueUnknownInitialStatus},
		{"no done", func(w *Workflow) { w.Statuses[2].Category = CategoryInProgress }, "statuses", IssueNoDoneStatus},
		{"bad category", func(w *Workflow) { w.Statuses[1].Category = "x" }, "statuses[1].category", IssueInvalidCategory},
		{"duplicate status name case-insensitive", func(w *Workflow) { w.Statuses[1].Name = " a " }, "statuses[1].name", IssueDuplicateStatusName},
		{"empty status name", func(w *Workflow) { w.Statuses[1].Name = " " }, "statuses[1].name", IssueRequired},
		{"duplicate status id", func(w *Workflow) { w.Statuses[1].ID = "a" }, "statuses[1].id", IssueDuplicateID},
		{"empty status id", func(w *Workflow) { w.Statuses[1].ID = "" }, "statuses[1].id", IssueRequired},
		{"unknown to", func(w *Workflow) { w.Transitions[1].To = "nope" }, "transitions[1].to", IssueUnknownStatus},
		{"unknown from", func(w *Workflow) { w.Transitions[1].From = []string{"b", "nope"} }, "transitions[1].from[1]", IssueUnknownStatus},
		{"self transition", func(w *Workflow) { w.Transitions[1].From = []string{"c"} }, "transitions[1].from[0]", IssueSelfTransition},
		{"unreachable", func(w *Workflow) { w.Transitions = w.Transitions[:1] }, "statuses[2]", IssueUnreachableStatus},
		{"duplicate transition id", func(w *Workflow) { w.Transitions[1].ID = "t1" }, "transitions[1].id", IssueDuplicateID},
		{"empty transition id", func(w *Workflow) { w.Transitions[1].ID = "" }, "transitions[1].id", IssueRequired},
		{"empty transition name", func(w *Workflow) { w.Transitions[1].Name = "" }, "transitions[1].name", IssueRequired},
		{"duplicate name per source", func(w *Workflow) {
			w.Transitions = append(w.Transitions, Transition{ID: "t3", Name: "go c", From: []string{"b"}, To: "a"})
		}, "transitions[2].name", IssueDuplicateTransition},
		{"duplicate name via any-from", func(w *Workflow) {
			w.Transitions = append(w.Transitions, Transition{ID: "t3", Name: "Go B", To: "c"})
		}, "transitions[2].name", IssueDuplicateTransition},
		{"same name different sources ok", func(w *Workflow) {
			w.Transitions = append(w.Transitions, Transition{ID: "t3", Name: "Go B", From: []string{"c"}, To: "b"})
		}, "", ""},
		{"bad required field", func(w *Workflow) { w.Transitions[0].RequiredFields = []string{"ok_field", "Bad"} }, "transitions[0].required_fields[1]", IssueInvalidField},
		{"reserved required field", func(w *Workflow) { w.Transitions[0].RequiredFields = []string{"assignee"} }, "transitions[0].required_fields[0]", IssueInvalidField},
		{"bad condition", func(w *Workflow) { w.Transitions[0].Conditions = []Condition{UserInRole{}} }, "transitions[0].conditions[0]", IssueInvalidRule},
		{"nil condition", func(w *Workflow) { w.Transitions[0].Conditions = []Condition{nil} }, "transitions[0].conditions[0]", IssueInvalidRule},
		{"bad validator", func(w *Workflow) { w.Transitions[0].Validators = []Validator{RequiredField{Field: "X"}} }, "transitions[0].validators[0]", IssueInvalidRule},
		{"nil validator", func(w *Workflow) { w.Transitions[0].Validators = []Validator{nil} }, "transitions[0].validators[0]", IssueInvalidRule},
		{"bad post function", func(w *Workflow) { w.Transitions[0].PostFunctions = []PostFunction{Emit{Event: "Bad Event"}} }, "transitions[0].post_functions[0]", IssueInvalidRule},
		{"nil post function", func(w *Workflow) { w.Transitions[0].PostFunctions = []PostFunction{nil} }, "transitions[0].post_functions[0]", IssueInvalidRule},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := linear()
			tt.mutate(&w)
			issues := w.Validate()
			if tt.wantPath == "" {
				if len(issues) != 0 {
					t.Fatalf("expected valid, got %+v", issues)
				}
				if w.Check() != nil {
					t.Fatal("Check must be nil")
				}
				return
			}
			got, ok := issueByPath(issues, tt.wantPath)
			if !ok || got.Code != tt.wantCode {
				t.Fatalf("want %s at %s, got %+v", tt.wantCode, tt.wantPath, issues)
			}
			if got.Message == "" {
				t.Error("message must be set")
			}
		})
	}
}

func TestValidateReturnsAllIssuesAndTypedError(t *testing.T) {
	w := linear()
	w.Initial = ""
	w.Transitions[0].To = "nope"
	w.Statuses[2].Category = CategoryTodo
	issues := w.Validate()
	if len(issues) < 3 {
		t.Fatalf("expected at least 3 issues, got %+v", issues)
	}
	err := w.Check()
	if apperror.CodeOf(err) != CodeInvalidWorkflow || apperror.KindOf(err) != apperror.KindValidation {
		t.Fatalf("unexpected error %v", err)
	}
	if len(apperror.DetailsOf(err)) != len(issues) {
		t.Errorf("details %d != issues %d", len(apperror.DetailsOf(err)), len(issues))
	}
	if IssuesError(nil) != nil {
		t.Error("IssuesError(nil) must be nil")
	}
}

func TestFromNilExpansionAndReachability(t *testing.T) {
	w := Workflow{
		Initial: "a",
		Statuses: []Status{
			{ID: "a", Name: "A", Category: CategoryTodo},
			{ID: "b", Name: "B", Category: CategoryInProgress},
			{ID: "c", Name: "C", Category: CategoryDone},
		},
		// Any -> c makes b and c reachable from a without explicit edges to b.
		Transitions: []Transition{{ID: "x", Name: "Cerrar", To: "c"}, {ID: "y", Name: "Volver", From: []string{"c"}, To: "b"}},
	}
	if issues := w.Validate(); len(issues) != 0 {
		t.Fatalf("unexpected issues %+v", issues)
	}
	tr := w.Transitions[0]
	for id, want := range map[string]bool{"a": true, "b": true, "c": false, "zzz": true} {
		if got := tr.AppliesFrom(id); got != want {
			t.Errorf("AppliesFrom(%s)=%v want %v", id, got, want)
		}
	}
	if got := w.expandFrom(tr); !reflect.DeepEqual(got, []string{"a", "b"}) {
		t.Errorf("expandFrom = %v", got)
	}
	// Status b is only reachable through c (Any -> c, then c -> b).
	w.Transitions = w.Transitions[1:]
	if _, ok := issueByPath(w.Validate(), "statuses[1]"); !ok {
		t.Error("b must be unreachable without the any-transition")
	}
}

func TestCategoryIsValid(t *testing.T) {
	for c, want := range map[Category]bool{CategoryTodo: true, CategoryInProgress: true, CategoryDone: true, "": false, "x": false} {
		if c.IsValid() != want {
			t.Errorf("%q valid=%v", c, !want)
		}
	}
}

// ---- Rules ----

func TestConditions(t *testing.T) {
	issue := IssueSnapshot{AssigneeID: "u1", ReporterID: "u2", Fields: map[string]string{"severity": "high"}}
	tests := []struct {
		name  string
		cond  Condition
		actor ActorContext
		want  bool
	}{
		{"role match", UserInRole{Roles: []string{"lead", "contributor"}}, ActorContext{Role: "contributor"}, true},
		{"role mismatch", UserInRole{Roles: []string{"lead"}}, ActorContext{Role: "viewer"}, false},
		{"is assignee", IsAssignee{}, ActorContext{UserID: "u1"}, true},
		{"not assignee", IsAssignee{}, ActorContext{UserID: "u9"}, false},
		{"empty actor never assignee", IsAssignee{}, ActorContext{}, false},
		{"is reporter", IsReporter{}, ActorContext{UserID: "u2"}, true},
		{"not reporter", IsReporter{}, ActorContext{UserID: "u1"}, false},
		{"permission held", Permission{Permission: "close_issue"}, ActorContext{Permissions: []string{"x", "close_issue"}}, true},
		{"permission missing", Permission{Permission: "close_issue"}, ActorContext{Permissions: []string{"x"}}, false},
		{"field equals", FieldEquals{Field: "severity", Value: "high"}, ActorContext{}, true},
		{"field differs", FieldEquals{Field: "severity", Value: "low"}, ActorContext{}, false},
		{"field absent", FieldEquals{Field: "other", Value: ""}, ActorContext{}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.cond.Allows(tt.actor, issue); got != tt.want {
				t.Errorf("got %v want %v", got, tt.want)
			}
		})
	}
}

func TestRuleParameterValidation(t *testing.T) {
	long := strings.Repeat("x", MaxCommentLength+1)
	tests := []struct {
		name string
		rule Rule
		ok   bool
	}{
		{"role ok", UserInRole{Roles: []string{"viewer"}}, true},
		{"role empty", UserInRole{}, false},
		{"role unknown", UserInRole{Roles: []string{"admin"}}, false},
		{"assignee", IsAssignee{}, true},
		{"reporter", IsReporter{}, true},
		{"permission ok", Permission{Permission: "manage_workflow"}, true},
		{"permission bad", Permission{Permission: "Bad-Perm"}, false},
		{"field_equals ok", FieldEquals{Field: "severity", Value: "x"}, true},
		{"field_equals bad", FieldEquals{Field: "status"}, false},
		{"required_field ok", RequiredField{Field: "env"}, true},
		{"required_field bad", RequiredField{Field: ""}, false},
		{"comment_required", CommentRequired{}, true},
		{"resolution_required", ResolutionRequired{}, true},
		{"subtasks_done", SubtasksDone{}, true},
		{"assignee_set", AssigneeSet{}, true},
		{"set_resolution id", SetResolution{Resolution: "r1"}, true},
		{"set_resolution done", SetResolution{Resolution: "done"}, true},
		{"set_resolution empty", SetResolution{}, false},
		{"clear_resolution", ClearResolution{}, true},
		{"assign reporter", Assign{To: "reporter"}, true},
		{"assign current_user", Assign{To: "current_user"}, true},
		{"assign lead", Assign{To: "lead"}, true},
		{"assign unassigned", Assign{To: "unassigned"}, true},
		{"assign bad", Assign{To: "nobody"}, false},
		{"set_field ok", SetField{Field: "env", Value: "prod"}, true},
		{"set_field reserved", SetField{Field: "resolution"}, false},
		{"add_comment ok", AddComment{Text: "hola"}, true},
		{"add_comment blank", AddComment{Text: "  "}, false},
		{"add_comment long", AddComment{Text: long}, false},
		{"emit ok", Emit{Event: "issue.escalated"}, true},
		{"emit bad", Emit{Event: "Issue Escalated"}, false},
		{"emit trailing dot", Emit{Event: "issue."}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.rule.Validate()
			if (err == nil) != tt.ok {
				t.Errorf("Validate()=%v want ok=%v", err, tt.ok)
			}
			if tt.rule.Type() == "" {
				t.Error("empty type")
			}
		})
	}
}

func TestValidators(t *testing.T) {
	tests := []struct {
		name  string
		v     Validator
		issue IssueSnapshot
		input TransitionInput
		want  []string // failing fields
	}{
		{"required from input", RequiredField{Field: "env"}, IssueSnapshot{}, TransitionInput{Fields: map[string]string{"env": "prod"}}, nil},
		{"required from issue", RequiredField{Field: "env"}, IssueSnapshot{Fields: map[string]string{"env": "prod"}}, TransitionInput{}, nil},
		{"required missing", RequiredField{Field: "env"}, IssueSnapshot{}, TransitionInput{}, []string{"fields.env"}},
		{"required blank input overrides issue", RequiredField{Field: "env"}, IssueSnapshot{Fields: map[string]string{"env": "prod"}}, TransitionInput{Fields: map[string]string{"env": " "}}, []string{"fields.env"}},
		{"comment ok", CommentRequired{}, IssueSnapshot{}, TransitionInput{Comment: "x"}, nil},
		{"comment blank", CommentRequired{}, IssueSnapshot{}, TransitionInput{Comment: "  "}, []string{"comment"}},
		{"resolution input", ResolutionRequired{}, IssueSnapshot{}, TransitionInput{ResolutionID: "r"}, nil},
		{"resolution issue", ResolutionRequired{}, IssueSnapshot{ResolutionID: "r"}, TransitionInput{}, nil},
		{"resolution missing", ResolutionRequired{}, IssueSnapshot{}, TransitionInput{}, []string{"resolution_id"}},
		{"subtasks done", SubtasksDone{}, IssueSnapshot{}, TransitionInput{}, nil},
		{"subtasks open", SubtasksDone{}, IssueSnapshot{OpenSubtasks: 2}, TransitionInput{}, []string{"subtasks"}},
		{"assignee issue", AssigneeSet{}, IssueSnapshot{AssigneeID: "u"}, TransitionInput{}, nil},
		{"assignee input", AssigneeSet{}, IssueSnapshot{}, TransitionInput{AssigneeID: "u"}, nil},
		{"assignee missing", AssigneeSet{}, IssueSnapshot{}, TransitionInput{}, []string{"assignee_id"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got []string
			for _, f := range tt.v.Check(tt.issue, tt.input) {
				if f.Message == "" {
					t.Error("empty message")
				}
				got = append(got, f.Field)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("got %v want %v", got, tt.want)
			}
		})
	}
}

func TestPostFunctions(t *testing.T) {
	actor := ActorContext{UserID: "me", LeadID: "boss"}
	issue := IssueSnapshot{ReporterID: "rep"}
	tests := []struct {
		name string
		p    PostFunction
		pre  Result
		want Result
	}{
		{"set_resolution id", SetResolution{Resolution: "r1"}, Result{}, Result{Resolution: ResolutionChange{Action: ResolutionSet, ID: "r1"}}},
		{"set_resolution done", SetResolution{Resolution: "done"}, Result{}, Result{Resolution: ResolutionChange{Action: ResolutionSet, UseDefaultDone: true}}},
		{"clear_resolution", ClearResolution{}, Result{Resolution: ResolutionChange{Action: ResolutionSet, ID: "x"}}, Result{Resolution: ResolutionChange{Action: ResolutionClear}}},
		{"assign reporter", Assign{To: "reporter"}, Result{}, Result{Assignee: AssigneeChange{Set: true, UserID: "rep"}}},
		{"assign current_user", Assign{To: "current_user"}, Result{}, Result{Assignee: AssigneeChange{Set: true, UserID: "me"}}},
		{"assign lead", Assign{To: "lead"}, Result{}, Result{Assignee: AssigneeChange{Set: true, UserID: "boss"}}},
		{"assign unassigned", Assign{To: "unassigned"}, Result{Assignee: AssigneeChange{Set: true, UserID: "z"}}, Result{Assignee: AssigneeChange{Set: true}}},
		{"set_field new", SetField{Field: "env", Value: "prod"}, Result{}, Result{FieldSets: []FieldSet{{"env", "prod"}}}},
		{"set_field overwrites", SetField{Field: "env", Value: "prod"}, Result{FieldSets: []FieldSet{{"env", "dev"}}}, Result{FieldSets: []FieldSet{{"env", "prod"}}}},
		{"add_comment", AddComment{Text: "hola"}, Result{Comments: []string{"a"}}, Result{Comments: []string{"a", "hola"}}},
		{"emit", Emit{Event: "issue.escalated"}, Result{}, Result{Events: []string{"issue.escalated"}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			plan := tt.pre
			tt.p.Apply(&plan, actor, issue)
			if !reflect.DeepEqual(plan, tt.want) {
				t.Errorf("got %+v want %+v", plan, tt.want)
			}
		})
	}
	t.Run("assign leaves untouched when target unknown", func(t *testing.T) {
		plan := Result{}
		Assign{To: "reporter"}.Apply(&plan, ActorContext{}, IssueSnapshot{})
		Assign{To: "lead"}.Apply(&plan, ActorContext{}, IssueSnapshot{})
		if plan.Assignee.Set {
			t.Error("assignee must stay untouched")
		}
	})
}

// ---- JSON ----

func richWorkflow() Workflow {
	return Workflow{
		Initial: "a",
		Statuses: []Status{
			{ID: "a", Name: "A", Category: CategoryTodo},
			{ID: "b", Name: "B", Category: CategoryDone},
		},
		Transitions: []Transition{{
			ID: "t1", Name: "Hecho", From: []string{"a"}, To: "b", RequiredFields: []string{"env"},
			Conditions:    []Condition{UserInRole{Roles: []string{"lead", "contributor"}}, IsAssignee{}, IsReporter{}, Permission{Permission: "p"}, FieldEquals{Field: "f", Value: "v"}},
			Validators:    []Validator{RequiredField{Field: "env"}, CommentRequired{}, ResolutionRequired{}, SubtasksDone{}, AssigneeSet{}},
			PostFunctions: []PostFunction{SetResolution{Resolution: "done"}, ClearResolution{}, Assign{To: "lead"}, SetField{Field: "x", Value: "y"}, AddComment{Text: "t"}, Emit{Event: "e.v"}},
		}, {ID: "t2", Name: "Any", To: "a"}},
	}
}

func TestJSONRoundTrip(t *testing.T) {
	w := richWorkflow()
	data, err := Encode(w)
	if err != nil {
		t.Fatal(err)
	}
	got, err := Decode(data)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, w) {
		t.Fatalf("round trip differs:\n%+v\n%+v", got, w)
	}
	again, _ := Encode(got)
	if string(again) != string(data) {
		t.Error("encoding is not stable")
	}
	var env map[string]any
	if err := json.Unmarshal(data, &env); err != nil || env["version"].(float64) != FormatVersion {
		t.Errorf("bad envelope %v %v", env, err)
	}
	for _, tpl := range TemplateNames() {
		tw, _ := TemplateWorkflow(tpl)
		d, err := Encode(tw)
		if err != nil {
			t.Fatal(err)
		}
		back, err := Decode(d)
		if err != nil || !reflect.DeepEqual(back, tw) {
			t.Errorf("template %s round trip failed: %v", tpl, err)
		}
	}
}

func TestDecodeErrors(t *testing.T) {
	good := `{"version":1,"initial":"a","statuses":[],"transitions":[%s]}`
	tr := func(extra string) string {
		return strings.Replace(good, "%s", `{"id":"t","name":"n","from":[],"to":"a","conditions":[`+extra+`],"validators":[],"post_functions":[],"required_fields":[]}`, 1)
	}
	tests := []struct {
		name string
		in   string
		code string
	}{
		{"not json", "nope", CodeMalformed},
		{"trailing data", `{"version":1,"initial":"","statuses":[],"transitions":[]} {}`, CodeMalformed},
		{"unknown field", `{"version":1,"extra":1}`, CodeMalformed},
		{"bad version", `{"version":2,"initial":"","statuses":[],"transitions":[]}`, CodeUnsupportedVersion},
		{"missing version", `{"initial":""}`, CodeUnsupportedVersion},
		{"unknown rule", tr(`{"type":"nope"}`), CodeUnknownRuleType},
		{"unknown param", tr(`{"type":"permission","params":{"permission":"x","extra":1}}`), CodeMalformed},
		{"bad param type", tr(`{"type":"permission","params":{"permission":1}}`), CodeMalformed},
		{"params trailing", tr(`{"type":"is_assignee","params":{} {}}`), CodeMalformed},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Decode([]byte(tt.in))
			if apperror.CodeOf(err) != tt.code {
				t.Fatalf("code=%q err=%v want %q", apperror.CodeOf(err), err, tt.code)
			}
		})
	}
	t.Run("missing params defaults to empty", func(t *testing.T) {
		w, err := Decode([]byte(tr(`{"type":"is_assignee"}`)))
		if err != nil || len(w.Transitions[0].Conditions) != 1 {
			t.Fatalf("%v %+v", err, w)
		}
	})
	t.Run("encode rejects nil rules", func(t *testing.T) {
		w := linear()
		w.Transitions[0].Conditions = []Condition{nil}
		if _, err := Encode(w); err == nil {
			t.Error("expected error")
		}
	})
}

func TestRegistry(t *testing.T) {
	c, v, p := RuleTypes()
	if len(c) != 5 || len(v) != 5 || len(p) != 6 {
		t.Fatalf("registry sizes %d %d %d", len(c), len(v), len(p))
	}
	for _, typ := range c {
		cond, err := DecodeCondition(typ, nil)
		if err != nil || cond.Type() != typ {
			t.Errorf("condition %s: %v", typ, err)
		}
	}
	for _, typ := range v {
		val, err := DecodeValidator(typ, nil)
		if err != nil || val.Type() != typ {
			t.Errorf("validator %s: %v", typ, err)
		}
	}
	for _, typ := range p {
		pf, err := DecodePostFunction(typ, nil)
		if err != nil || pf.Type() != typ {
			t.Errorf("post function %s: %v", typ, err)
		}
	}
	if _, err := DecodeCondition("required_field", nil); apperror.CodeOf(err) != CodeUnknownRuleType {
		t.Errorf("kinds must not mix: %v", err)
	}
}

// ---- Diff ----

func TestDiff(t *testing.T) {
	old := linear()
	t.Run("identical", func(t *testing.T) {
		d := Diff(old, linear())
		if !reflect.DeepEqual(d, WorkflowDiff{}) || d.RequiresMapping() {
			t.Errorf("unexpected diff %+v", d)
		}
	})
	t.Run("everything", func(t *testing.T) {
		upd := linear()
		upd.Initial = "b"
		upd.Statuses = []Status{
			{ID: "b", Name: "B renamed", Category: CategoryInProgress},
			{ID: "c", Name: "C", Category: CategoryDone},
			{ID: "d", Name: "D", Category: CategoryTodo},
		}
		upd.Transitions = []Transition{
			{ID: "t2", Name: "Go C", From: []string{"b"}, To: "c", Conditions: []Condition{IsAssignee{}}},
			{ID: "t9", Name: "New", From: []string{"b"}, To: "d"},
		}
		d := Diff(old, upd)
		if !d.InitialChanged || !d.RequiresMapping() {
			t.Error("initial/mapping flags")
		}
		if len(d.RemovedStatuses) != 1 || d.RemovedStatuses[0].ID != "a" {
			t.Errorf("removed statuses %+v", d.RemovedStatuses)
		}
		if len(d.AddedStatuses) != 1 || d.AddedStatuses[0].ID != "d" {
			t.Errorf("added statuses %+v", d.AddedStatuses)
		}
		if len(d.ChangedStatuses) != 1 || d.ChangedStatuses[0].New.Name != "B renamed" {
			t.Errorf("changed statuses %+v", d.ChangedStatuses)
		}
		if len(d.RemovedTransitions) != 1 || d.RemovedTransitions[0].ID != "t1" {
			t.Errorf("removed transitions %+v", d.RemovedTransitions)
		}
		if len(d.AddedTransitions) != 1 || d.AddedTransitions[0].ID != "t9" {
			t.Errorf("added transitions %+v", d.AddedTransitions)
		}
		if len(d.ChangedTransitions) != 1 || d.ChangedTransitions[0].New.ID != "t2" {
			t.Errorf("changed transitions %+v", d.ChangedTransitions)
		}
	})
	t.Run("added only needs no mapping", func(t *testing.T) {
		upd := linear()
		upd.Statuses = append(upd.Statuses, Status{ID: "d", Name: "D", Category: CategoryTodo})
		if Diff(old, upd).RequiresMapping() {
			t.Error("adding must not require mapping")
		}
	})
}

// ---- Engine ----

func gated() Workflow {
	return Workflow{
		Initial: "todo",
		Statuses: []Status{
			{ID: "todo", Name: "Por hacer", Category: CategoryTodo},
			{ID: "doing", Name: "En curso", Category: CategoryInProgress},
			{ID: "done", Name: "Hecho", Category: CategoryDone},
		},
		Transitions: []Transition{
			{ID: "start", Name: "Iniciar", From: []string{"todo"}, To: "doing", Conditions: []Condition{IsAssignee{}}},
			{ID: "finish", Name: "Terminar", From: []string{"doing"}, To: "done",
				Conditions:     []Condition{UserInRole{Roles: []string{"lead", "contributor"}}},
				RequiredFields: []string{"env"},
				Validators:     []Validator{CommentRequired{}, SubtasksDone{}, RequiredField{Field: "env"}},
				PostFunctions:  []PostFunction{SetField{Field: "closed_by", Value: "x"}, Assign{To: "current_user"}, AddComment{Text: "auto"}, Emit{Event: "issue.finished"}}},
			{ID: "reopen", Name: "Reabrir", From: []string{"done"}, To: "todo", Conditions: []Condition{UserInRole{Roles: []string{"lead"}}}},
			{ID: "cancel", Name: "Cancelar", To: "done", Conditions: []Condition{Permission{Permission: "cancel"}}},
		},
	}
}

func TestEvaluateDenied(t *testing.T) {
	w := gated()
	tests := []struct {
		name  string
		issue IssueSnapshot
		actor ActorContext
		id    string
	}{
		{"unknown transition", IssueSnapshot{StatusID: "todo"}, ActorContext{}, "nope"},
		{"wrong source", IssueSnapshot{StatusID: "todo"}, ActorContext{Role: "lead"}, "finish"},
		{"unknown status", IssueSnapshot{StatusID: "ghost"}, ActorContext{}, "start"},
		{"condition fails", IssueSnapshot{StatusID: "todo", AssigneeID: "u1"}, ActorContext{UserID: "u2"}, "start"},
		{"viewer cannot finish", IssueSnapshot{StatusID: "doing"}, ActorContext{Role: "viewer"}, "finish"},
		{"any-from excludes target", IssueSnapshot{StatusID: "done"}, ActorContext{Permissions: []string{"cancel"}}, "cancel"},
		{"permission missing", IssueSnapshot{StatusID: "todo"}, ActorContext{}, "cancel"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Evaluate(w, tt.issue, tt.actor, tt.id, TransitionInput{Comment: "c", Fields: map[string]string{"env": "x"}})
			if apperror.CodeOf(err) != CodeTransitionNotAllowed || apperror.KindOf(err) != apperror.KindForbidden {
				t.Fatalf("got %v", err)
			}
		})
	}
	t.Run("missing target status", func(t *testing.T) {
		bad := gated()
		bad.Transitions[0].To = "ghost"
		_, err := Evaluate(bad, IssueSnapshot{StatusID: "todo", AssigneeID: "u"}, ActorContext{UserID: "u"}, "start", TransitionInput{})
		if apperror.CodeOf(err) != CodeTransitionNotAllowed {
			t.Fatalf("got %v", err)
		}
	})
}

func TestEvaluateValidationFailed(t *testing.T) {
	w := gated()
	issue := IssueSnapshot{StatusID: "doing", OpenSubtasks: 1}
	_, err := Evaluate(w, issue, ActorContext{Role: "contributor"}, "finish", TransitionInput{})
	if apperror.CodeOf(err) != CodeValidationFailed || apperror.KindOf(err) != apperror.KindValidation {
		t.Fatalf("got %v", err)
	}
	var fields []string
	for _, d := range apperror.DetailsOf(err) {
		fields = append(fields, d.Field)
	}
	// RequiredFields and the required_field validator report the same field once.
	want := []string{"fields.env", "comment", "subtasks"}
	if !reflect.DeepEqual(fields, want) {
		t.Errorf("details %v want %v", fields, want)
	}
	// Package-level error values must not be mutated by evaluation.
	if len(ErrValidationFailed.Details) != 0 {
		t.Error("shared error mutated")
	}
}

func TestEvaluateSuccess(t *testing.T) {
	w := gated()
	issue := IssueSnapshot{StatusID: "doing", ReporterID: "rep", Fields: map[string]string{}}
	actor := ActorContext{UserID: "me", Role: "contributor"}
	input := TransitionInput{Comment: "listo", Fields: map[string]string{"env": "prod", "ignored": "z"}}
	got, err := Evaluate(w, issue, actor, "finish", input)
	if err != nil {
		t.Fatal(err)
	}
	want := Result{
		TransitionID: "finish", FromStatusID: "doing", ToStatusID: "done",
		Resolution: ResolutionChange{Action: ResolutionSet, UseDefaultDone: true},
		Assignee:   AssigneeChange{Set: true, UserID: "me"},
		FieldSets:  []FieldSet{{"env", "prod"}, {"closed_by", "x"}},
		Comment:    "listo", Comments: []string{"auto"}, Events: []string{"issue.finished"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got  %+v\nwant %+v", got, want)
	}
	if !reflect.DeepEqual(issue, IssueSnapshot{StatusID: "doing", ReporterID: "rep", Fields: map[string]string{}}) {
		t.Error("snapshot mutated")
	}
}

func TestEvaluateResolutionRules(t *testing.T) {
	w := gated()
	lead := ActorContext{Role: "lead", Permissions: []string{"cancel"}}
	tests := []struct {
		name  string
		id    string
		issue IssueSnapshot
		input TransitionInput
		want  ResolutionChange
	}{
		{"auto done on done category", "cancel", IssueSnapshot{StatusID: "todo"}, TransitionInput{}, ResolutionChange{Action: ResolutionSet, UseDefaultDone: true}},
		{"input resolution wins", "cancel", IssueSnapshot{StatusID: "todo"}, TransitionInput{ResolutionID: "wontdo"}, ResolutionChange{Action: ResolutionSet, ID: "wontdo"}},
		{"keeps existing resolution", "cancel", IssueSnapshot{StatusID: "todo", ResolutionID: "r"}, TransitionInput{}, ResolutionChange{}},
		{"clear on reopen", "reopen", IssueSnapshot{StatusID: "done", ResolutionID: "r"}, TransitionInput{}, ResolutionChange{Action: ResolutionClear}},
		{"nothing on plain move", "reopen", IssueSnapshot{StatusID: "done"}, TransitionInput{}, ResolutionChange{}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Evaluate(w, tt.issue, lead, tt.id, tt.input)
			if err != nil {
				t.Fatal(err)
			}
			if got.Resolution != tt.want {
				t.Errorf("got %+v want %+v", got.Resolution, tt.want)
			}
		})
	}
	t.Run("post function beats auto", func(t *testing.T) {
		w := linear()
		w.Transitions[1].PostFunctions = []PostFunction{SetResolution{Resolution: "fixed"}}
		got, err := Evaluate(w, IssueSnapshot{StatusID: "b"}, ActorContext{}, "t2", TransitionInput{})
		if err != nil || got.Resolution != (ResolutionChange{Action: ResolutionSet, ID: "fixed"}) {
			t.Fatalf("%v %+v", err, got)
		}
	})
	t.Run("input assignee", func(t *testing.T) {
		got, err := Evaluate(gated(), IssueSnapshot{StatusID: "todo", AssigneeID: "u"}, ActorContext{UserID: "u"}, "start", TransitionInput{AssigneeID: "other"})
		if err != nil || got.Assignee != (AssigneeChange{Set: true, UserID: "other"}) {
			t.Fatalf("%v %+v", err, got)
		}
	})
}

func TestAvailableTransitionsPerRole(t *testing.T) {
	w := gated()
	names := func(items []AvailableTransition) []string {
		out := []string{}
		for _, i := range items {
			out = append(out, i.TransitionID)
		}
		return out
	}
	tests := []struct {
		name  string
		issue IssueSnapshot
		actor ActorContext
		want  []string
	}{
		{"assignee in todo", IssueSnapshot{StatusID: "todo", AssigneeID: "u"}, ActorContext{UserID: "u", Role: "contributor"}, []string{"start"}},
		{"non assignee in todo", IssueSnapshot{StatusID: "todo", AssigneeID: "u"}, ActorContext{UserID: "v", Role: "lead"}, []string{}},
		{"lead with permission in todo", IssueSnapshot{StatusID: "todo"}, ActorContext{Permissions: []string{"cancel"}}, []string{"cancel"}},
		{"contributor doing", IssueSnapshot{StatusID: "doing"}, ActorContext{Role: "contributor", Permissions: []string{"cancel"}}, []string{"finish", "cancel"}},
		{"viewer doing", IssueSnapshot{StatusID: "doing"}, ActorContext{Role: "viewer"}, []string{}},
		{"lead done", IssueSnapshot{StatusID: "done"}, ActorContext{Role: "lead", Permissions: []string{"cancel"}}, []string{"reopen"}},
		{"contributor done", IssueSnapshot{StatusID: "done"}, ActorContext{Role: "contributor"}, []string{}},
		{"unknown status", IssueSnapshot{StatusID: "ghost"}, ActorContext{Role: "lead"}, []string{}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := names(AvailableTransitions(w, tt.issue, tt.actor)); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("got %v want %v", got, tt.want)
			}
		})
	}

	t.Run("required info", func(t *testing.T) {
		w := gated()
		w.Transitions[1].Validators = append(w.Transitions[1].Validators, ResolutionRequired{}, AssigneeSet{})
		items := AvailableTransitions(w, IssueSnapshot{StatusID: "doing"}, ActorContext{Role: "lead"})
		if len(items) != 1 {
			t.Fatalf("got %+v", items)
		}
		it := items[0]
		if !reflect.DeepEqual(it.RequiredFields, []string{"env"}) || !it.RequiresComment || !it.RequiresResolution || !it.RequiresAssignee || !it.NeedsScreen() {
			t.Errorf("unexpected %+v", it)
		}
		if it.ToStatusID != "done" || it.ToStatusName != "Hecho" || it.ToCategory != CategoryDone || it.Name != "Terminar" {
			t.Errorf("unexpected target info %+v", it)
		}
	})
	t.Run("no screen needed", func(t *testing.T) {
		items := AvailableTransitions(w, IssueSnapshot{StatusID: "todo", AssigneeID: "u"}, ActorContext{UserID: "u"})
		if items[0].NeedsScreen() || items[0].RequiredFields == nil {
			t.Errorf("unexpected %+v", items[0])
		}
	})
}

// ---- Templates ----

func TestTemplateWorkflows(t *testing.T) {
	counts := map[string]int{TemplateScrum: 4, TemplateKanban: 4, TemplateTaskTracking: 3, TemplateBugTracking: 5}
	for _, name := range TemplateNames() {
		t.Run(name, func(t *testing.T) {
			w, err := TemplateWorkflow(name)
			if err != nil {
				t.Fatal(err)
			}
			if issues := w.Validate(); len(issues) != 0 {
				t.Fatalf("invalid template: %+v", issues)
			}
			if len(w.Statuses) != counts[name] {
				t.Errorf("statuses=%d want %d", len(w.Statuses), counts[name])
			}
			// Every non-initial status can be reached and, for this default, every
			// transition is usable by someone: a lead can leave every status that has outgoing edges.
			for _, s := range w.Statuses {
				if s.ID == w.Initial {
					continue
				}
				found := false
				for _, tr := range w.Transitions {
					found = found || tr.To == s.ID
				}
				if !found {
					t.Errorf("status %s has no incoming transition", s.ID)
				}
			}
			// Running the first available transition from the initial status works.
			av := AvailableTransitions(w, IssueSnapshot{StatusID: w.Initial}, ActorContext{Role: "contributor"})
			if len(av) == 0 {
				t.Fatal("no transitions from initial")
			}
			if _, err := Evaluate(w, IssueSnapshot{StatusID: w.Initial}, ActorContext{Role: "contributor"}, av[0].TransitionID, TransitionInput{}); err != nil {
				t.Fatal(err)
			}
		})
	}
	t.Run("case and spaces", func(t *testing.T) {
		if _, err := TemplateWorkflow("  Scrum "); err != nil {
			t.Error(err)
		}
	})
	t.Run("unknown", func(t *testing.T) {
		if _, err := TemplateWorkflow("nope"); apperror.CodeOf(err) != CodeUnknownTemplate {
			t.Errorf("got %v", err)
		}
	})
	t.Run("copies are independent", func(t *testing.T) {
		a, _ := TemplateWorkflow(TemplateScrum)
		a.Statuses[0].Name = "mutated"
		a.Transitions[0].From[0] = "mutated"
		b, _ := TemplateWorkflow(TemplateScrum)
		if b.Statuses[0].Name != "Por hacer" || b.Transitions[0].From[0] != "todo" {
			t.Error("template data shared between calls")
		}
	})
	t.Run("done transitions set resolution and reopen clears it", func(t *testing.T) {
		w, _ := TemplateWorkflow(TemplateTaskTracking)
		got, err := Evaluate(w, IssueSnapshot{StatusID: "todo"}, ActorContext{}, "complete", TransitionInput{})
		if err != nil || !got.Resolution.UseDefaultDone {
			t.Fatalf("%v %+v", err, got)
		}
		got, err = Evaluate(w, IssueSnapshot{StatusID: "done", ResolutionID: "r"}, ActorContext{}, "reopen", TransitionInput{})
		if err != nil || got.Resolution.Action != ResolutionClear {
			t.Fatalf("%v %+v", err, got)
		}
	})
}

func TestErrorsHaveStableCodesAndSpanishMessages(t *testing.T) {
	for _, e := range []*apperror.Error{ErrTransitionNotAllowed, ErrValidationFailed, ErrMalformed, ErrUnsupportedVersion, ErrUnknownTemplate} {
		if !strings.HasPrefix(e.Code, "workflow.") || e.Message == "" {
			t.Errorf("bad error %+v", e)
		}
	}
}
