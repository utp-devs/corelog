package domain

import (
	"strings"
	"testing"

	"github.com/google/uuid"
)

func validRule() Rule {
	return Rule{
		Name:    "Etiquetar",
		Trigger: Trigger{Type: TriggerIssueCreated},
		Actions: []Action{{Type: ActionAddLabel, Label: "auto"}},
	}
}

func TestRuleValidation(t *testing.T) {
	if err := validRule().Validate(); err != nil {
		t.Fatalf("valid rule: %v", err)
	}
	userID, teamID := uuid.NewString(), uuid.NewString()
	mutate := func(f func(*Rule)) Rule {
		r := validRule()
		f(&r)
		return r
	}
	bad := map[string]Rule{
		"no name":            mutate(func(r *Rule) { r.Name = " " }),
		"long name":          mutate(func(r *Rule) { r.Name = strings.Repeat("x", 121) }),
		"unknown trigger":    mutate(func(r *Rule) { r.Trigger.Type = "nope" }),
		"scheduled too fast": mutate(func(r *Rule) { r.Trigger = Trigger{Type: TriggerScheduled, EveryMinutes: 1, Query: "status = Open"} }),
		"scheduled no query": mutate(func(r *Rule) { r.Trigger = Trigger{Type: TriggerScheduled, EveryMinutes: 60} }),
		"scheduled too slow": mutate(func(r *Rule) {
			r.Trigger = Trigger{Type: TriggerScheduled, EveryMinutes: MaxEveryMinutes + 1, Query: "x"}
		}),
		"huge trigger value": mutate(func(r *Rule) { r.Trigger.Field = strings.Repeat("x", 2001) }),
		"no actions":         mutate(func(r *Rule) { r.Actions = nil }),
		"too many actions":   mutate(func(r *Rule) { r.Actions = make([]Action, 11) }),
		"too many conditions": mutate(func(r *Rule) {
			r.Conditions = make([]Condition, 11)
		}),
		"unknown condition":     mutate(func(r *Rule) { r.Conditions = []Condition{{Type: "weird"}} }),
		"query without text":    mutate(func(r *Rule) { r.Conditions = []Condition{{Type: ConditionQuery}} }),
		"equals bad field":      mutate(func(r *Rule) { r.Conditions = []Condition{{Type: ConditionFieldEq, Field: "password", Value: "x"}} }),
		"equals no value":       mutate(func(r *Rule) { r.Conditions = []Condition{{Type: ConditionFieldEq, Field: "status"}} }),
		"in without values":     mutate(func(r *Rule) { r.Conditions = []Condition{{Type: ConditionFieldIn, Field: "status"}} }),
		"role unknown":          mutate(func(r *Rule) { r.Conditions = []Condition{{Type: ConditionActorRole, Role: "god"}} }),
		"issue type no values":  mutate(func(r *Rule) { r.Conditions = []Condition{{Type: ConditionIssueType}} }),
		"unknown action":        mutate(func(r *Rule) { r.Actions = []Action{{Type: "rm -rf"}} }),
		"transition no status":  mutate(func(r *Rule) { r.Actions = []Action{{Type: ActionTransition}} }),
		"assign unknown who":    mutate(func(r *Rule) { r.Actions = []Action{{Type: ActionAssign, Who: "everyone"}} }),
		"assign user no id":     mutate(func(r *Rule) { r.Actions = []Action{{Type: ActionAssign, Who: WhoUser, UserID: "x"}} }),
		"round robin no team":   mutate(func(r *Rule) { r.Actions = []Action{{Type: ActionAssign, Who: WhoRoundRobin}} }),
		"set unknown field":     mutate(func(r *Rule) { r.Actions = []Action{{Type: ActionSetField, Field: "reporter", Value: "x"}} }),
		"set bad custom id":     mutate(func(r *Rule) { r.Actions = []Action{{Type: ActionSetField, Field: "custom:abc"}} }),
		"comment no text":       mutate(func(r *Rule) { r.Actions = []Action{{Type: ActionAddComment}} }),
		"comment unknown value": mutate(func(r *Rule) { r.Actions = []Action{{Type: ActionAddComment, Text: "{{user.password}}"}} }),
		"comment loose braces":  mutate(func(r *Rule) { r.Actions = []Action{{Type: ActionAddComment, Text: "hola {{ issue.key"}} }),
		"comment too long":      mutate(func(r *Rule) { r.Actions = []Action{{Type: ActionAddComment, Text: strings.Repeat("x", 2001)}} }),
		"label empty":           mutate(func(r *Rule) { r.Actions = []Action{{Type: ActionAddLabel, Label: " "}} }),
		"label long":            mutate(func(r *Rule) { r.Actions = []Action{{Type: ActionRemoveLabel, Label: strings.Repeat("x", 51)}} }),
		"webhook ftp":           mutate(func(r *Rule) { r.Actions = []Action{{Type: ActionSendWebhook, URL: "ftp://x"}} }),
		"webhook userinfo":      mutate(func(r *Rule) { r.Actions = []Action{{Type: ActionSendWebhook, URL: "https://u:p@x.com"}} }),
		"notify bad recipient":  mutate(func(r *Rule) { r.Actions = []Action{{Type: ActionSendNotification, Who: "all", Text: "x"}} }),
		"notify no text":        mutate(func(r *Rule) { r.Actions = []Action{{Type: ActionSendNotification, Who: WhoLead}} }),
		"notify user bad id": mutate(func(r *Rule) {
			r.Actions = []Action{{Type: ActionSendNotification, Who: WhoUser, UserID: "x", Text: "x"}}
		}),
		"subtask no text": mutate(func(r *Rule) { r.Actions = []Action{{Type: ActionCreateSubtask}} }),
	}
	for name, r := range bad {
		if err := r.Validate(); err == nil {
			t.Errorf("%s accepted", name)
		}
	}
	good := []Rule{
		mutate(func(r *Rule) {
			r.Trigger = Trigger{Type: TriggerScheduled, EveryMinutes: 60, Query: "status = Open"}
			r.Conditions = []Condition{
				{Type: ConditionQuery, Query: "priority = High"}, {Type: ConditionFieldEq, Field: "status", Value: "Open"},
				{Type: ConditionFieldIn, Field: "priority", Values: []string{"High", "Low"}}, {Type: ConditionActorRole, Role: "lead"},
				{Type: ConditionIssueType, Values: []string{"Bug"}},
			}
			r.Actions = []Action{
				{Type: ActionTransition, Status: "Done"}, {Type: ActionAssign, Who: WhoReporter}, {Type: ActionAssign, Who: WhoLead},
				{Type: ActionAssign, Who: WhoUser, UserID: userID}, {Type: ActionAssign, Who: WhoRoundRobin, TeamID: teamID},
			}
		}),
		mutate(func(r *Rule) {
			r.Actions = []Action{
				{Type: ActionSetField, Field: "summary", Value: "{{issue.summary}} (revisada)"},
				{Type: ActionSetField, Field: "custom:" + userID, Value: "7"},
				{Type: ActionAddComment, Text: "{{ issue.key }} movida por {{actor.name}} de {{trigger.from}} a {{trigger.to}}"},
				{Type: ActionRemoveLabel, Label: "old"}, {Type: ActionCreateSubtask, Text: "Revisar {{issue.key}}"},
				{Type: ActionSendWebhook, URL: "https://example.com/hook"},
				{Type: ActionSendNotification, Who: WhoAssignee, Text: "{{rule.name}}"}, {Type: ActionSendNotification, Who: WhoUser, UserID: userID, Text: "x"},
			}
		}),
	}
	for _, r := range good {
		if err := r.Validate(); err != nil {
			t.Errorf("valid rule rejected: %v", err)
		}
	}
	if len(SmartValueNames()) != len(smartValues) {
		t.Fatal("smart value names")
	}
}

func TestRenderIsSafe(t *testing.T) {
	facts := map[string]string{
		"issue.key": "CORE-1", "issue.summary": `<script>alert("x")</script> & {{actor.name}}`, "actor.name": "Ana <b>",
		"issue.status": strings.Repeat("é", 800),
	}
	cases := []struct{ in, want string }{
		{"{{issue.key}}: {{ issue.summary }}", `CORE-1: &lt;script&gt;alert(&#34;x&#34;)&lt;/script&gt; &amp; {{actor.name}}`},
		{"Hola {{actor.name}}", "Hola Ana &lt;b&gt;"},
		{"{{unknown.value}} {{secret}} {{issue.priority}}", "  "},
		{"sin valores", "sin valores"},
		{"{{issue.__proto__}} {{issue.key}", " {{issue.key}"},
	}
	for _, c := range cases {
		if got := Render(c.in, facts); got != c.want {
			t.Errorf("Render(%q) = %q, want %q", c.in, got, c.want)
		}
	}
	// A value containing a placeholder stays literal (no second expansion) and long values are cut.
	if got := Render("{{issue.summary}}", facts); strings.Contains(got, "Ana") {
		t.Errorf("value was expanded twice: %q", got)
	}
	if got := Render("{{issue.status}}", facts); len([]rune(got)) != 500 {
		t.Errorf("long value length %d", len([]rune(got)))
	}
}

func TestTriggerMatching(t *testing.T) {
	cases := []struct {
		name    string
		trigger Trigger
		facts   Facts
		want    bool
	}{
		{"created", Trigger{Type: TriggerIssueCreated}, Facts{Type: "issue.created"}, true},
		{"created vs updated", Trigger{Type: TriggerIssueCreated}, Facts{Type: "issue.updated"}, false},
		{"updated any field", Trigger{Type: TriggerIssueUpdated}, Facts{Type: "issue.updated", ChangedFields: []string{"summary"}}, true},
		{"updated that field", Trigger{Type: TriggerIssueUpdated, Field: "Priority"}, Facts{Type: "issue.updated", ChangedFields: []string{"summary", "priority"}}, true},
		{"updated other field", Trigger{Type: TriggerIssueUpdated, Field: "assignee"}, Facts{Type: "issue.updated", ChangedFields: []string{"summary"}}, false},
		{"transition any", Trigger{Type: TriggerIssueTransitioned}, Facts{Type: "issue.transitioned", From: "Open", To: "Done"}, true},
		{"transition to", Trigger{Type: TriggerIssueTransitioned, To: "done"}, Facts{Type: "issue.transitioned", From: "Open", To: "Done"}, true},
		{"transition from mismatch", Trigger{Type: TriggerIssueTransitioned, From: "Review", To: "Done"}, Facts{Type: "issue.transitioned", From: "Open", To: "Done"}, false},
		{"transition to mismatch", Trigger{Type: TriggerIssueTransitioned, To: "Closed"}, Facts{Type: "issue.transitioned", From: "Open", To: "Done"}, false},
		{"comment", Trigger{Type: TriggerCommentAdded}, Facts{Type: "comment.created"}, true},
		{"sprint started", Trigger{Type: TriggerSprintStarted}, Facts{Type: "sprint.started"}, true},
		{"sprint completed", Trigger{Type: TriggerSprintCompleted}, Facts{Type: "sprint.started"}, false},
		{"unknown event", Trigger{Type: TriggerIssueCreated}, Facts{Type: "user.login"}, false},
		{"scheduled never fires on events", Trigger{Type: TriggerScheduled}, Facts{Type: "issue.created"}, false},
	}
	for _, c := range cases {
		if got := c.trigger.Matches(c.facts); got != c.want {
			t.Errorf("%s: Matches = %v", c.name, got)
		}
	}
}

func TestConditionsHold(t *testing.T) {
	assignee := uuid.New()
	snap := Snapshot{Status: "In Progress", StatusCategory: "indeterminate", Priority: "High", IssueType: "Bug", Summary: "Crash", AssigneeID: &assignee, ReporterID: uuid.New()}
	cases := []struct {
		c    Condition
		want bool
	}{
		{Condition{Type: ConditionFieldEq, Field: "status", Value: "in progress"}, true},
		{Condition{Type: ConditionFieldEq, Field: "status", Value: "Done"}, false},
		{Condition{Type: ConditionFieldEq, Field: "assignee", Value: assignee.String()}, true},
		{Condition{Type: ConditionFieldEq, Field: "reporter", Value: snap.ReporterID.String()}, true},
		{Condition{Type: ConditionFieldEq, Field: "status_category", Value: "indeterminate"}, true},
		{Condition{Type: ConditionFieldEq, Field: "type", Value: "bug"}, true},
		{Condition{Type: ConditionFieldEq, Field: "summary", Value: "crash"}, true},
		{Condition{Type: ConditionFieldIn, Field: "priority", Values: []string{"Low", "high"}}, true},
		{Condition{Type: ConditionFieldIn, Field: "priority", Values: []string{"Low"}}, false},
		{Condition{Type: ConditionIssueType, Values: []string{"Task", "Bug"}}, true},
		{Condition{Type: ConditionIssueType, Values: []string{"Task"}}, false},
		{Condition{Type: ConditionQuery, Query: "x"}, false},
	}
	for i, c := range cases {
		if got := c.c.Holds(snap); got != c.want {
			t.Errorf("case %d (%s): Holds = %v", i, c.c.Type, got)
		}
	}
	if (Snapshot{}).Field("assignee") != "" || (Snapshot{}).Field("nope") != "" {
		t.Error("empty snapshot fields")
	}
}

func TestLoopPrevention(t *testing.T) {
	rule, other := uuid.New(), uuid.New()
	cases := []struct {
		name   string
		origin uuid.UUID
		depth  int
		want   bool
	}{
		{"human event", uuid.Nil, 0, false},
		{"produced by another rule", other, 1, false},
		{"produced by this rule", rule, 1, true},
		{"deep chain", other, MaxChainDepth, true},
		{"one below the cap", other, MaxChainDepth - 1, false},
	}
	for _, c := range cases {
		if skip, why := Loop(rule, c.origin, c.depth); skip != c.want || (skip && why == "") {
			t.Errorf("%s: skip=%v reason=%q", c.name, skip, why)
		}
	}
}

func TestSmartFacts(t *testing.T) {
	f := SmartFacts(Snapshot{Key: "A-1", Summary: "s", ProjectKey: "A", ProjectName: "Proj"}, "Ana", "Regla", Facts{From: "x", To: "y"})
	for k := range smartValues {
		if _, ok := f[k]; !ok {
			t.Errorf("smart value %s has no fact", k)
		}
	}
	if f["actor.name"] != "Ana" || f["trigger.to"] != "y" || f["rule.name"] != "Regla" {
		t.Errorf("facts %v", f)
	}
	if kind, ok := TriggerFor("issue.created"); !ok || kind != TriggerIssueCreated {
		t.Error("TriggerFor")
	}
	if _, ok := TriggerFor("x"); ok {
		t.Error("TriggerFor unknown")
	}
}
