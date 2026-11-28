package usecase

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/jhonsferg/corelog/internal/automation/application/port"
	"github.com/jhonsferg/corelog/internal/automation/domain"
	"github.com/jhonsferg/corelog/internal/platform/apperror"
	"github.com/jhonsferg/corelog/internal/platform/clock"
	"github.com/jhonsferg/corelog/internal/platform/events"
)

type fakeRepo struct {
	org     uuid.UUID
	rules   map[uuid.UUID]domain.Rule
	runs    []domain.Run
	touched []uuid.UUID
}

func newRepo(org uuid.UUID) *fakeRepo { return &fakeRepo{org: org, rules: map[uuid.UUID]domain.Rule{}} }

func (r *fakeRepo) Create(_ context.Context, rule *domain.Rule) error {
	rule.ID = uuid.New()
	r.rules[rule.ID] = *rule
	return nil
}
func (r *fakeRepo) Get(_ context.Context, id uuid.UUID) (domain.Rule, error) {
	if rule, ok := r.rules[id]; ok {
		return rule, nil
	}
	return domain.Rule{}, domain.ErrNotFound
}
func (r *fakeRepo) ListByProject(_ context.Context, p uuid.UUID) ([]domain.Rule, error) {
	var out []domain.Rule
	for _, rule := range r.rules {
		if rule.ProjectID != nil && *rule.ProjectID == p {
			out = append(out, rule)
		}
	}
	return out, nil
}
func (r *fakeRepo) Update(_ context.Context, rule *domain.Rule) error {
	if r.rules[rule.ID].Version != rule.Version {
		return domain.ErrVersionConflict
	}
	rule.Version++
	r.rules[rule.ID] = *rule
	return nil
}
func (r *fakeRepo) SetEnabled(_ context.Context, id uuid.UUID, on bool, _ time.Time) (domain.Rule, error) {
	rule := r.rules[id]
	rule.Enabled = on
	r.rules[id] = rule
	return rule, nil
}
func (r *fakeRepo) Delete(_ context.Context, id uuid.UUID) error { delete(r.rules, id); return nil }
func (r *fakeRepo) ProjectOrg(context.Context, uuid.UUID) (uuid.UUID, bool, error) {
	return r.org, true, nil
}
func (r *fakeRepo) ActiveFor(_ context.Context, _, p uuid.UUID) ([]domain.Rule, error) {
	var out []domain.Rule
	for _, rule := range r.rules {
		if rule.Enabled && (rule.ProjectID == nil || *rule.ProjectID == p) && rule.Trigger.Type != domain.TriggerScheduled {
			out = append(out, rule)
		}
	}
	return out, nil
}
func (r *fakeRepo) ActiveScheduled(context.Context) ([]domain.Rule, error) {
	var out []domain.Rule
	for _, rule := range r.rules {
		if rule.Enabled && rule.Trigger.Type == domain.TriggerScheduled {
			out = append(out, rule)
		}
	}
	return out, nil
}
func (r *fakeRepo) RecordRun(_ context.Context, run *domain.Run) error {
	run.ID = uuid.New()
	r.runs = append(r.runs, *run)
	if run.Status != domain.RunSkipped {
		rule := r.rules[run.RuleID]
		rule.RunCount++
		rule.LastRunAt = &run.CreatedAt
		r.rules[run.RuleID] = rule
	}
	return nil
}
func (r *fakeRepo) TouchScheduled(_ context.Context, id uuid.UUID, at time.Time) error {
	rule := r.rules[id]
	rule.LastRunAt = &at
	r.rules[id] = rule
	r.touched = append(r.touched, id)
	return nil
}
func (r *fakeRepo) CountRunsSince(_ context.Context, id uuid.UUID, since time.Time) (int, error) {
	n := 0
	for _, run := range r.runs {
		if run.RuleID == id && run.Status != domain.RunSkipped && !run.CreatedAt.Before(since) {
			n++
		}
	}
	return n, nil
}
func (r *fakeRepo) ListRuns(_ context.Context, id uuid.UUID, status string, limit int) ([]domain.Run, error) {
	var out []domain.Run
	for _, run := range r.runs {
		if run.RuleID == id && (status == "" || run.Status == status) && len(out) < limit {
			out = append(out, run)
		}
	}
	return out, nil
}

type fakeAuth struct{ allowed map[uuid.UUID]bool }

func (a fakeAuth) Authorize(_ context.Context, _ string, actor uuid.UUID, _ string, _ uuid.UUID) error {
	if a.allowed[actor] {
		return nil
	}
	return apperror.New(apperror.KindForbidden, "forbidden")
}

type fakeIssues struct{ snap domain.Snapshot }

func (f fakeIssues) Snapshot(context.Context, uuid.UUID) (domain.Snapshot, error) { return f.snap, nil }

type fakeActions struct {
	calls   []string
	failOn  string
	onCall  func(ctx context.Context, name string)
	subtask string
}

func (a *fakeActions) record(ctx context.Context, name string) error {
	a.calls = append(a.calls, name)
	if a.onCall != nil {
		a.onCall(ctx, name)
	}
	if a.failOn == name {
		return errors.New("secret database failure")
	}
	return nil
}
func (a *fakeActions) Transition(ctx context.Context, _ uuid.UUID, s string) error {
	return a.record(ctx, "transition:"+s)
}
func (a *fakeActions) Assign(ctx context.Context, _ uuid.UUID, u uuid.UUID) error {
	return a.record(ctx, "assign:"+u.String())
}
func (a *fakeActions) SetField(ctx context.Context, _ uuid.UUID, f, v string) error {
	return a.record(ctx, "set:"+f+"="+v)
}
func (a *fakeActions) AddComment(ctx context.Context, _ uuid.UUID, b string) error {
	return a.record(ctx, "comment:"+b)
}
func (a *fakeActions) AddLabel(ctx context.Context, _ uuid.UUID, l string) error {
	return a.record(ctx, "label+"+l)
}
func (a *fakeActions) RemoveLabel(ctx context.Context, _ uuid.UUID, l string) error {
	return a.record(ctx, "label-"+l)
}
func (a *fakeActions) CreateSubtask(ctx context.Context, _ uuid.UUID, s string) (string, error) {
	return "K-2", a.record(ctx, "subtask:"+s)
}

type fakeMatcher struct {
	match bool
	ids   []uuid.UUID
}

func (m fakeMatcher) Matches(context.Context, uuid.UUID, uuid.UUID, string) (bool, error) {
	return m.match, nil
}
func (m fakeMatcher) Find(context.Context, uuid.UUID, string, int) ([]uuid.UUID, error) {
	return m.ids, nil
}
func (m fakeMatcher) Validate(_ context.Context, q string) error {
	if strings.Contains(q, "???") {
		return errors.New("bad")
	}
	return nil
}

type fakeDir struct {
	lead    uuid.UUID
	members []uuid.UUID
	role    string
}

func (d fakeDir) ProjectLead(context.Context, uuid.UUID) (uuid.UUID, error)   { return d.lead, nil }
func (d fakeDir) TeamMembers(context.Context, uuid.UUID) ([]uuid.UUID, error) { return d.members, nil }
func (d fakeDir) ActorRole(context.Context, uuid.UUID, uuid.UUID) (string, error) {
	return d.role, nil
}
func (d fakeDir) UserName(context.Context, uuid.UUID) (string, error) { return "Ana", nil }

type fakeNotifier struct{ sent []string }

func (n *fakeNotifier) Notify(_ context.Context, u uuid.UUID, kind, msg, link string) error {
	n.sent = append(n.sent, u.String()+"|"+kind+"|"+msg+"|"+link)
	return nil
}

type fakePoster struct{ posted []string }

func (p *fakePoster) Post(_ context.Context, url string, body []byte) error {
	p.posted = append(p.posted, url+" "+string(body))
	return nil
}

type recorder struct{ events []events.Event }

func (r *recorder) Publish(_ context.Context, e events.Event) error {
	r.events = append(r.events, e)
	return nil
}

type env struct {
	engine  *Engine
	svc     *Service
	repo    *fakeRepo
	actions *fakeActions
	dir     fakeDir
	notif   *fakeNotifier
	poster  *fakePoster
	pub     *recorder
	clk     *clock.FakeClock
	project uuid.UUID
	lead    uuid.UUID
	issue   uuid.UUID
	snap    domain.Snapshot
}

func newEnv() *env {
	e := &env{project: uuid.New(), lead: uuid.New(), issue: uuid.New(), actions: &fakeActions{}, notif: &fakeNotifier{},
		poster: &fakePoster{}, pub: &recorder{}, clk: &clock.FakeClock{T: time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)}}
	e.repo = newRepo(uuid.New())
	e.snap = domain.Snapshot{IssueID: e.issue, Key: "CORE-1", Summary: "Resumen", Status: "Open", Priority: "High", IssueType: "Bug",
		ReporterID: uuid.New(), ProjectKey: "CORE", ProjectName: "Core", ProjectID: e.project}
	e.dir = fakeDir{lead: uuid.New(), role: "contributor"}
	e.rebuild(fakeMatcher{match: true})
	return e
}

func (e *env) rebuild(m fakeMatcher) {
	deps := Deps{Repo: e.repo, Auth: fakeAuth{allowed: map[uuid.UUID]bool{e.lead: true}}, Issues: fakeIssues{snap: e.snap},
		Actions: e.actions, Matcher: m, Directory: e.dir, Notifier: e.notif, Poster: e.poster, Publisher: e.pub, Clock: e.clk}
	e.engine, e.svc = NewEngine(deps), NewService(deps)
}

func (e *env) addRule(name string, trigger domain.Trigger, conds []domain.Condition, actions ...domain.Action) domain.Rule {
	rule := domain.Rule{ID: uuid.New(), OrgID: e.repo.org, ProjectID: &e.project, Name: name, Enabled: true, Trigger: trigger,
		Conditions: conds, Actions: actions, Version: 1}
	e.repo.rules[rule.ID] = rule
	return rule
}

func (e *env) event(kind string, payload map[string]any) events.Event {
	actor := uuid.New()
	if payload == nil {
		payload = map[string]any{}
	}
	return events.Event{ID: 10, ProjectID: &e.project, EntityID: &e.issue, EntityType: "issue", ActorID: &actor, Type: kind, Payload: payload}
}

func (e *env) runsOf(rule domain.Rule) []domain.Run {
	r, _ := e.repo.ListRuns(context.Background(), rule.ID, "", 10000)
	return r
}

func TestEngineRunsActionsWithSmartValues(t *testing.T) {
	e := newEnv()
	rule := e.addRule("Cierre", domain.Trigger{Type: domain.TriggerIssueTransitioned, To: "Done"}, nil,
		domain.Action{Type: domain.ActionAddComment, Text: "{{issue.key}} paso de {{trigger.from}} a {{trigger.to}} por {{actor.name}}"},
		domain.Action{Type: domain.ActionAddLabel, Label: "hecha"},
		domain.Action{Type: domain.ActionTransition, Status: "Closed"},
		domain.Action{Type: domain.ActionSetField, Field: "summary", Value: "{{issue.summary}} <ok>"},
		domain.Action{Type: domain.ActionCreateSubtask, Text: "Revisar {{issue.key}}"},
		domain.Action{Type: domain.ActionRemoveLabel, Label: "wip"},
		domain.Action{Type: domain.ActionSendWebhook, URL: "https://example.com/h"},
		domain.Action{Type: domain.ActionSendNotification, Who: domain.WhoLead, Text: "Cerrada {{issue.key}}"},
	)
	e.addRule("Otra transicion", domain.Trigger{Type: domain.TriggerIssueTransitioned, To: "Review"}, nil, domain.Action{Type: domain.ActionAddLabel, Label: "no"})
	if err := e.engine.Handle(context.Background(), e.event("issue.transitioned", map[string]any{"from_status": "Open", "to_status": "Done"})); err != nil {
		t.Fatal(err)
	}
	want := []string{
		"comment:Automatización: Cierre\n\nCORE-1 paso de Open a Done por Ana", "label+hecha", "transition:Closed", "set:summary=Resumen <ok>",
		"subtask:Revisar CORE-1", "label-wip",
	}
	if strings.Join(e.actions.calls, "|") != strings.Join(want, "|") {
		t.Fatalf("calls:\n%q\nwant:\n%q", e.actions.calls, want)
	}
	if len(e.poster.posted) != 1 || !strings.Contains(e.poster.posted[0], `"key":"CORE-1"`) {
		t.Fatalf("webhook %v", e.poster.posted)
	}
	if len(e.notif.sent) != 1 || !strings.Contains(e.notif.sent[0], e.dir.lead.String()+"|automation|Cerrada CORE-1|/issues/CORE-1") {
		t.Fatalf("notification %v", e.notif.sent)
	}
	runs := e.runsOf(rule)
	if len(runs) != 1 || runs[0].Status != domain.RunSuccess || runs[0].Detail != "8 acciones ejecutadas" || e.repo.rules[rule.ID].RunCount != 1 {
		t.Fatalf("runs %+v", runs)
	}
	if e.engine.Consumer().Name() != "automation" {
		t.Fatal("consumer name")
	}
	if err := e.engine.Consumer().Handle(context.Background(), e.event("user.login", nil)); err != nil {
		t.Fatal(err)
	}
}

func TestEngineEventsCarryTheOriginOfTheRun(t *testing.T) {
	e := newEnv()
	rule := e.addRule("R", domain.Trigger{Type: domain.TriggerIssueCreated}, nil, domain.Action{Type: domain.ActionAddLabel, Label: "x"})
	var got events.Origin
	e.actions.onCall = func(ctx context.Context, _ string) { got, _ = events.OriginFrom(ctx) }
	if err := e.engine.Handle(context.Background(), e.event("issue.created", map[string]any{events.PayloadAutomationDepth: float64(2)})); err != nil {
		t.Fatal(err)
	}
	if got.RuleID != rule.ID || got.Depth != 3 || got.Name != "R" {
		t.Fatalf("origin %+v", got)
	}
}

func TestLoopPreventionAndDepthCap(t *testing.T) {
	e := newEnv()
	a := e.addRule("A", domain.Trigger{Type: domain.TriggerIssueUpdated}, nil, domain.Action{Type: domain.ActionAddLabel, Label: "a"})
	b := e.addRule("B", domain.Trigger{Type: domain.TriggerIssueUpdated}, nil, domain.Action{Type: domain.ActionAddLabel, Label: "b"})
	// An event produced by rule A is skipped by A but still processed by B.
	ev := e.event("issue.updated", map[string]any{events.PayloadAutomationRuleID: a.ID.String(), events.PayloadAutomationDepth: float64(1)})
	if err := e.engine.Handle(context.Background(), ev); err != nil {
		t.Fatal(err)
	}
	if ra := e.runsOf(a); len(ra) != 1 || ra[0].Status != domain.RunSkipped {
		t.Fatalf("rule A runs %+v", ra)
	}
	if rb := e.runsOf(b); len(rb) != 1 || rb[0].Status != domain.RunSuccess {
		t.Fatalf("rule B runs %+v", rb)
	}
	if strings.Join(e.actions.calls, ",") != "label+b" {
		t.Fatalf("calls %v", e.actions.calls)
	}
	// A chain A -> B -> A is cut at the depth cap, whichever rule comes next.
	e.actions.calls = nil
	deep := e.event("issue.updated", map[string]any{events.PayloadAutomationRuleID: b.ID.String(), events.PayloadAutomationDepth: float64(domain.MaxChainDepth)})
	if err := e.engine.Handle(context.Background(), deep); err != nil {
		t.Fatal(err)
	}
	if len(e.actions.calls) != 0 {
		t.Fatalf("depth cap ignored: %v", e.actions.calls)
	}
	// Simulated chain: every action publishes the next event until the cap stops it.
	e.actions.calls = nil
	hops := 0
	var next events.Event
	e.actions.onCall = func(ctx context.Context, _ string) {
		o, _ := events.OriginFrom(ctx)
		next = e.event("issue.updated", map[string]any{events.PayloadAutomationRuleID: o.RuleID.String(), events.PayloadAutomationDepth: o.Depth})
	}
	cur := e.event("issue.updated", nil)
	for i := 0; i < 20; i++ {
		before := len(e.actions.calls)
		if err := e.engine.Handle(context.Background(), cur); err != nil {
			t.Fatal(err)
		}
		if len(e.actions.calls) == before {
			break
		}
		hops++
		cur = next
	}
	if hops == 0 || hops > domain.MaxChainDepth {
		t.Fatalf("chain ran %d hops, cap is %d", hops, domain.MaxChainDepth)
	}
}

func TestRateLimitPerRule(t *testing.T) {
	e := newEnv()
	rule := e.addRule("R", domain.Trigger{Type: domain.TriggerIssueCreated}, nil, domain.Action{Type: domain.ActionAddLabel, Label: "x"})
	for i := 0; i < domain.MaxRunsPerHour+5; i++ {
		_ = e.engine.Handle(context.Background(), e.event("issue.created", nil))
	}
	executed, skipped := 0, 0
	for _, r := range e.runsOf(rule) {
		if r.Status == domain.RunSkipped {
			skipped++
		} else {
			executed++
		}
	}
	if executed != domain.MaxRunsPerHour || skipped != 5 || len(e.actions.calls) != domain.MaxRunsPerHour {
		t.Fatalf("executed %d skipped %d calls %d", executed, skipped, len(e.actions.calls))
	}
	// After an hour the quota is back.
	e.clk.Advance(61 * time.Minute)
	_ = e.engine.Handle(context.Background(), e.event("issue.created", nil))
	if len(e.actions.calls) != domain.MaxRunsPerHour+1 {
		t.Fatal("the quota did not reset")
	}
}

func TestConditions(t *testing.T) {
	cases := []struct {
		name  string
		conds []domain.Condition
		match bool
		role  string
		run   bool
	}{
		{"no conditions", nil, true, "", true},
		{"query matches", []domain.Condition{{Type: domain.ConditionQuery, Query: "priority = High"}}, true, "", true},
		{"query does not match", []domain.Condition{{Type: domain.ConditionQuery, Query: "priority = High"}}, false, "", false},
		{"field equals", []domain.Condition{{Type: domain.ConditionFieldEq, Field: "status", Value: "Open"}}, true, "", true},
		{"field differs", []domain.Condition{{Type: domain.ConditionFieldEq, Field: "status", Value: "Done"}}, true, "", false},
		{"field in", []domain.Condition{{Type: domain.ConditionFieldIn, Field: "priority", Values: []string{"high", "low"}}}, true, "", true},
		{"issue type", []domain.Condition{{Type: domain.ConditionIssueType, Values: []string{"Bug"}}}, true, "", true},
		{"issue type differs", []domain.Condition{{Type: domain.ConditionIssueType, Values: []string{"Epic"}}}, true, "", false},
		{"actor role matches", []domain.Condition{{Type: domain.ConditionActorRole, Role: "contributor"}}, true, "", true},
		{"actor role differs", []domain.Condition{{Type: domain.ConditionActorRole, Role: "lead"}}, true, "", false},
		{"all must hold", []domain.Condition{{Type: domain.ConditionFieldEq, Field: "status", Value: "Open"}, {Type: domain.ConditionIssueType, Values: []string{"Epic"}}}, true, "", false},
	}
	for _, c := range cases {
		e := newEnv()
		e.rebuild(fakeMatcher{match: c.match})
		rule := e.addRule("R", domain.Trigger{Type: domain.TriggerIssueCreated}, c.conds, domain.Action{Type: domain.ActionAddLabel, Label: "x"})
		if err := e.engine.Handle(context.Background(), e.event("issue.created", nil)); err != nil {
			t.Fatal(err)
		}
		runs := e.runsOf(rule)
		if ran := len(e.actions.calls) == 1; ran != c.run {
			t.Errorf("%s: ran=%v", c.name, ran)
		}
		want := domain.RunSkipped
		if c.run {
			want = domain.RunSuccess
		}
		if len(runs) != 1 || runs[0].Status != want {
			t.Errorf("%s: runs %+v", c.name, runs)
		}
	}
}

func TestFailuresAreIsolatedPerRule(t *testing.T) {
	e := newEnv()
	broken := e.addRule("Rota", domain.Trigger{Type: domain.TriggerIssueCreated}, nil,
		domain.Action{Type: domain.ActionAddLabel, Label: "boom"}, domain.Action{Type: domain.ActionAddLabel, Label: "never"})
	healthy := e.addRule("Sana", domain.Trigger{Type: domain.TriggerIssueCreated}, nil, domain.Action{Type: domain.ActionAddLabel, Label: "ok"})
	e.actions.failOn = "label+boom"
	if err := e.engine.Handle(context.Background(), e.event("issue.created", nil)); err != nil {
		t.Fatalf("a failing rule must not fail the event: %v", err)
	}
	rb := e.runsOf(broken)
	if len(rb) != 1 || rb[0].Status != domain.RunFailed || !strings.HasPrefix(rb[0].Detail, "accion 1:") || strings.Contains(rb[0].Detail, "secret") {
		t.Fatalf("broken run %+v (internal errors must not leak)", rb)
	}
	if rh := e.runsOf(healthy); len(rh) != 1 || rh[0].Status != domain.RunSuccess {
		t.Fatalf("healthy run %+v", rh)
	}
	for _, c := range e.actions.calls {
		if c == "label+never" {
			t.Fatal("actions after a failure must not run")
		}
	}
}

func TestPanicInARuleIsContained(t *testing.T) {
	e := newEnv()
	rule := e.addRule("P", domain.Trigger{Type: domain.TriggerIssueCreated}, nil, domain.Action{Type: domain.ActionAddLabel, Label: "x"})
	e.actions.onCall = func(context.Context, string) { panic("kaboom") }
	if err := e.engine.Handle(context.Background(), e.event("issue.created", nil)); err != nil {
		t.Fatal(err)
	}
	if r := e.runsOf(rule); len(r) != 1 || r[0].Status != domain.RunFailed {
		t.Fatalf("runs %+v", r)
	}
}

func TestAssignmentTargets(t *testing.T) {
	e := newEnv()
	team := uuid.New()
	e.dir.members = []uuid.UUID{uuid.New(), uuid.New(), uuid.New()}
	e.rebuild(fakeMatcher{match: true})
	user := uuid.New()
	e.addRule("Reporter", domain.Trigger{Type: domain.TriggerIssueCreated}, nil, domain.Action{Type: domain.ActionAssign, Who: domain.WhoReporter})
	e.addRule("Lead", domain.Trigger{Type: domain.TriggerIssueCreated}, nil, domain.Action{Type: domain.ActionAssign, Who: domain.WhoLead})
	e.addRule("User", domain.Trigger{Type: domain.TriggerIssueCreated}, nil, domain.Action{Type: domain.ActionAssign, Who: domain.WhoUser, UserID: user.String()})
	rr := e.addRule("Round", domain.Trigger{Type: domain.TriggerIssueCreated}, nil, domain.Action{Type: domain.ActionAssign, Who: domain.WhoRoundRobin, TeamID: team.String()})
	_ = e.engine.Handle(context.Background(), e.event("issue.created", nil))
	joined := strings.Join(e.actions.calls, " ")
	for _, want := range []string{e.snap.ReporterID.String(), e.dir.lead.String(), user.String(), e.dir.members[0].String()} {
		if !strings.Contains(joined, want) {
			t.Errorf("assignee %s missing in %v", want, e.actions.calls)
		}
	}
	// Round robin advances with the run count of the rule.
	for i := 1; i <= 3; i++ {
		e.actions.calls = nil
		rule := e.repo.rules[rr.ID]
		rule.RunCount = int64(i)
		e.repo.rules[rr.ID] = rule
		_ = e.engine.Handle(context.Background(), e.event("issue.created", nil))
		if !strings.Contains(strings.Join(e.actions.calls, " "), e.dir.members[i%3].String()) {
			t.Errorf("round robin %d did not pick member %d", i, i%3)
		}
	}
	e.dir.members = nil
	e.rebuild(fakeMatcher{match: true})
	e.actions.calls = nil
	_ = e.engine.Handle(context.Background(), e.event("issue.created", nil))
	if failed, _ := e.repo.ListRuns(context.Background(), rr.ID, domain.RunFailed, 10); len(failed) == 0 {
		t.Fatal("an empty team must fail the run")
	}
}

func TestEventsWithoutIssueOrProject(t *testing.T) {
	e := newEnv()
	rule := e.addRule("Sprint", domain.Trigger{Type: domain.TriggerSprintStarted}, nil, domain.Action{Type: domain.ActionAddComment, Text: "x"})
	notify := e.addRule("Aviso", domain.Trigger{Type: domain.TriggerSprintStarted}, nil, domain.Action{Type: domain.ActionSendNotification, Who: domain.WhoLead, Text: "Sprint {{project.key}}"})
	ev := events.Event{ID: 1, ProjectID: &e.project, EntityType: "sprint", Type: "sprint.started", Payload: map[string]any{}}
	if err := e.engine.Handle(context.Background(), ev); err != nil {
		t.Fatal(err)
	}
	if r := e.runsOf(rule); len(r) != 1 || r[0].Status != domain.RunFailed || !strings.Contains(r[0].Detail, "incidencia") {
		t.Fatalf("comment without issue: %+v", r)
	}
	if r := e.runsOf(notify); len(r) != 1 || r[0].Status != domain.RunSuccess {
		t.Fatalf("notification without issue: %+v", r)
	}
	if err := e.engine.Handle(context.Background(), events.Event{Type: "issue.created"}); err != nil {
		t.Fatal(err) // no project: ignored
	}
}

func TestScheduledRules(t *testing.T) {
	e := newEnv()
	id1, id2 := uuid.New(), uuid.New()
	e.rebuild(fakeMatcher{match: true, ids: []uuid.UUID{id1, id2}})
	rule := e.addRule("Recordatorio", domain.Trigger{Type: domain.TriggerScheduled, EveryMinutes: 60, Query: "status = Open"}, nil,
		domain.Action{Type: domain.ActionAddLabel, Label: "revisar"})
	ctx := context.Background()
	if err := e.engine.RunScheduled(ctx, e.clk.T); err != nil {
		t.Fatal(err)
	}
	if len(e.actions.calls) != 2 || len(e.runsOf(rule)) != 2 {
		t.Fatalf("first run: %d calls, %d runs", len(e.actions.calls), len(e.runsOf(rule)))
	}
	// Not due again until its interval has passed.
	_ = e.engine.RunScheduled(ctx, e.clk.T.Add(30*time.Minute))
	if len(e.actions.calls) != 2 {
		t.Fatal("ran before the interval")
	}
	_ = e.engine.RunScheduled(ctx, e.clk.T.Add(61*time.Minute))
	if len(e.actions.calls) != 4 {
		t.Fatalf("second run: %d calls", len(e.actions.calls))
	}
	// A disabled scheduled rule never runs.
	e.repo.SetEnabled(ctx, rule.ID, false, e.clk.T)
	_ = e.engine.RunScheduled(ctx, e.clk.T.Add(48*time.Hour))
	if len(e.actions.calls) != 4 {
		t.Fatal("disabled rule ran")
	}
}

func TestServicePermissionsAndLifecycle(t *testing.T) {
	e := newEnv()
	ctx := context.Background()
	stranger := uuid.New()
	in := port.RuleInput{Name: " Regla ", Trigger: domain.Trigger{Type: domain.TriggerIssueCreated}, Actions: []domain.Action{{Type: domain.ActionAddLabel, Label: "x"}}}
	if _, err := e.svc.Create(ctx, stranger, "user", e.project, in); apperror.KindOf(err) != apperror.KindForbidden {
		t.Fatalf("stranger create: %v", err)
	}
	rule, err := e.svc.Create(ctx, e.lead, "user", e.project, in)
	if err != nil || rule.Name != "Regla" || rule.Version != 1 || !rule.Enabled || rule.CreatedBy == nil || len(e.pub.events) != 1 {
		t.Fatalf("create: %+v %v", rule, err)
	}
	if _, err := e.svc.List(ctx, stranger, "user", e.project); err == nil {
		t.Fatal("stranger list")
	}
	if _, err := e.svc.Get(ctx, stranger, "user", rule.ID); err == nil {
		t.Fatal("stranger get")
	}
	if list, err := e.svc.List(ctx, e.lead, "user", e.project); err != nil || len(list) != 1 {
		t.Fatalf("list %v %v", list, err)
	}
	// Invalid definitions and queries.
	badQuery := port.RuleInput{Name: "q", Trigger: domain.Trigger{Type: domain.TriggerIssueCreated},
		Conditions: []domain.Condition{{Type: domain.ConditionQuery, Query: "???"}}, Actions: in.Actions}
	if _, err := e.svc.Create(ctx, e.lead, "user", e.project, badQuery); apperror.CodeOf(err) != "automation.invalid_query" {
		t.Fatalf("bad query: %v", err)
	}
	if _, err := e.svc.Create(ctx, e.lead, "user", e.project, port.RuleInput{Name: "x"}); apperror.KindOf(err) != apperror.KindValidation {
		t.Fatalf("invalid rule: %v", err)
	}
	// Patch with versions.
	name, one := "Nueva", 1
	patched, err := e.svc.Patch(ctx, e.lead, "user", rule.ID, port.PatchInput{Name: &name, Version: &one})
	if err != nil || patched.Name != "Nueva" || patched.Version != 2 {
		t.Fatalf("patch: %+v %v", patched, err)
	}
	if _, err := e.svc.Patch(ctx, e.lead, "user", rule.ID, port.PatchInput{Name: &name, Version: &one}); !errors.Is(err, domain.ErrVersionConflict) {
		t.Fatalf("stale patch: %v", err)
	}
	off := false
	trig, acts, conds := domain.Trigger{Type: domain.TriggerCommentAdded}, []domain.Action{{Type: domain.ActionAddLabel, Label: "y"}}, []domain.Condition{}
	if got, err := e.svc.Patch(ctx, e.lead, "user", rule.ID, port.PatchInput{Enabled: &off, Trigger: &trig, Actions: &acts, Conditions: &conds}); err != nil || got.Enabled || got.Trigger.Type != domain.TriggerCommentAdded {
		t.Fatalf("patch all: %+v %v", got, err)
	}
	empty := ""
	if _, err := e.svc.Patch(ctx, e.lead, "user", rule.ID, port.PatchInput{Name: &empty}); err == nil {
		t.Fatal("empty name accepted")
	}
	if _, err := e.svc.SetEnabled(ctx, e.lead, "user", rule.ID, true); err != nil {
		t.Fatal(err)
	}
	if _, err := e.svc.SetEnabled(ctx, stranger, "user", rule.ID, true); err == nil {
		t.Fatal("stranger enable")
	}
	// Runs.
	_ = e.engine.Handle(ctx, e.event("comment.created", nil))
	if runs, err := e.svc.Runs(ctx, e.lead, "user", rule.ID, "success", 0); err != nil || len(runs) != 1 {
		t.Fatalf("runs %v %v", runs, err)
	}
	if _, err := e.svc.Runs(ctx, e.lead, "user", rule.ID, "weird", 10); apperror.KindOf(err) != apperror.KindValidation {
		t.Fatalf("bad status filter: %v", err)
	}
	if _, err := e.svc.Runs(ctx, stranger, "user", rule.ID, "", 10); err == nil {
		t.Fatal("stranger runs")
	}
	// Rules without project are hidden, unknown rules are not found.
	orphan := domain.Rule{ID: uuid.New(), OrgID: e.repo.org}
	e.repo.rules[orphan.ID] = orphan
	if _, err := e.svc.Get(ctx, e.lead, "user", orphan.ID); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("orphan: %v", err)
	}
	if _, err := e.svc.Get(ctx, e.lead, "user", uuid.New()); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("unknown: %v", err)
	}
	if err := e.svc.Delete(ctx, stranger, "user", rule.ID); err == nil {
		t.Fatal("stranger delete")
	}
	if err := e.svc.Delete(ctx, e.lead, "user", rule.ID); err != nil {
		t.Fatal(err)
	}
}

func TestDryRunValidation(t *testing.T) {
	e := newEnv()
	ctx := context.Background()
	in := port.RuleInput{Name: "x", Trigger: domain.Trigger{Type: domain.TriggerIssueCreated},
		Actions: []domain.Action{{Type: domain.ActionAddComment, Text: "{{issue.key}} creada"}, {Type: domain.ActionTransition, Status: "Done"},
			{Type: domain.ActionSendWebhook, URL: "https://x.com"}, {Type: domain.ActionAddLabel, Label: "l"}, {Type: domain.ActionRemoveLabel, Label: "m"},
			{Type: domain.ActionAssign, Who: domain.WhoLead}, {Type: domain.ActionSetField, Field: "summary", Value: "v"},
			{Type: domain.ActionCreateSubtask, Text: "t"}, {Type: domain.ActionSendNotification, Who: domain.WhoLead, Text: "n"}}}
	res, err := e.svc.Validate(ctx, e.lead, "user", e.project, in, &e.issue)
	if err != nil || !res.Valid || len(res.Planned) != 9 || res.Planned[0] != "Comentar: CORE-1 creada" {
		t.Fatalf("validate: %+v %v", res, err)
	}
	if _, err := e.svc.Validate(ctx, uuid.New(), "user", e.project, in, nil); err == nil {
		t.Fatal("stranger validate")
	}
	res, _ = e.svc.Validate(ctx, e.lead, "user", e.project, port.RuleInput{Name: "x"}, nil)
	if res.Valid || len(res.Errors) != 1 {
		t.Fatalf("invalid definition: %+v", res)
	}
	in.Conditions = []domain.Condition{{Type: domain.ConditionQuery, Query: "???"}}
	if res, _ = e.svc.Validate(ctx, e.lead, "user", e.project, in, nil); res.Valid {
		t.Fatalf("bad query: %+v", res)
	}
	// The dry run never executes anything.
	if len(e.actions.calls) != 0 || len(e.poster.posted) != 0 {
		t.Fatal("dry run executed actions")
	}
}
