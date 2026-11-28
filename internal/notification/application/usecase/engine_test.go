package usecase

import (
	"context"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/jhonsferg/corelog/internal/notification/application/port"
	"github.com/jhonsferg/corelog/internal/notification/domain"
	"github.com/jhonsferg/corelog/internal/platform/clock"
	"github.com/jhonsferg/corelog/internal/platform/database"
)

type engineFixture struct {
	t        *testing.T
	engine   *Engine
	facts    *memFacts
	notes    *memoryRepo
	outbox   *memOutbox
	delivery *memDelivery
	schemes  *memSchemes
	browser  allowAll

	org, project, issue                      uuid.UUID
	reporter, assignee, watcher1, watcher2   uuid.UUID
	lead, componentLead, actor, mentioned    uuid.UUID
	replyTarget, member, teamMember, outside uuid.UUID
	team                                     uuid.UUID
}

func newEngineFixture(t *testing.T) *engineFixture {
	t.Helper()
	f := &engineFixture{
		t: t, facts: newMemFacts(), notes: &memoryRepo{}, outbox: newMemOutbox(), delivery: newMemDelivery(),
		schemes: newMemSchemes(), browser: allowAll{denied: map[uuid.UUID]bool{}},
		org: uuid.New(), project: uuid.New(), issue: uuid.New(), team: uuid.New(),
	}
	f.reporter, f.assignee = f.facts.person("reporter"), f.facts.person("assignee")
	f.watcher1, f.watcher2 = f.facts.person("watcher1"), f.facts.person("watcher2")
	f.lead, f.componentLead = f.facts.person("lead"), f.facts.person("componentlead")
	f.actor, f.mentioned = f.facts.person("actor"), f.facts.person("mentioned")
	f.replyTarget, f.member = f.facts.person("replytarget"), f.facts.person("member")
	f.teamMember, f.outside = f.facts.person("teammember"), f.facts.person("outside")
	f.facts.issues[f.issue] = &port.IssueFacts{
		IssueID: f.issue, OrgID: f.org, ProjectID: f.project, Key: "WEB-7", Summary: "Pagina <b>rota</b>",
		ReporterID: f.reporter, AssigneeID: f.assignee, WatcherIDs: []uuid.UUID{f.watcher1, f.watcher2},
		ComponentLeadIDs: []uuid.UUID{f.componentLead}, ProjectLeadID: f.lead,
	}
	f.facts.projects[f.project] = &port.ProjectFacts{ProjectID: f.project, OrgID: f.org, Name: "Web", LeadID: f.lead}
	f.facts.teams[f.team] = []uuid.UUID{f.teamMember}
	f.facts.roles["developer"] = []uuid.UUID{f.member}
	f.engine = NewEngine(EngineDeps{
		Schemes: f.schemes, Facts: f.facts, Notes: f.notes, Outbox: f.outbox, Delivery: f.delivery, Browser: f.browser,
		Render: fakeRenderer{}, Tx: &database.FakeTxManager{}, Clock: clock.NewFixed(time.Date(2026, 11, 27, 10, 0, 0, 0, time.UTC)),
		BaseURL: "https://corelog.example.test/",
	})
	return f
}

func (f *engineFixture) notified() []uuid.UUID {
	seen := map[uuid.UUID]bool{}
	for _, n := range f.notes.items {
		seen[n.UserID] = true
	}
	var out []uuid.UUID
	for id := range seen {
		out = append(out, id)
	}
	sort.Slice(out, func(a, b int) bool { return out[a].String() < out[b].String() })
	return out
}

func sortIDs(ids ...uuid.UUID) []uuid.UUID {
	sort.Slice(ids, func(a, b int) bool { return ids[a].String() < ids[b].String() })
	return ids
}

func equalIDs(a, b []uuid.UUID) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func (f *engineFixture) setProjectRule(rule domain.Rule) {
	pid := f.project
	f.schemes.stored[scopeKey(f.org, &pid)] = port.StoredScheme{Scheme: domain.Scheme{Rules: []domain.Rule{rule}}}
}

// TestRecipientMatrix checks every event against every recipient type: the rule of the event decides who is told.
func TestRecipientMatrix(t *testing.T) {
	for _, event := range domain.AllEvents {
		cases := map[domain.Recipient]func(f *engineFixture) []uuid.UUID{
			{Type: domain.RecipientReporter}:                func(f *engineFixture) []uuid.UUID { return sortIDs(f.reporter) },
			{Type: domain.RecipientAssignee}:                func(f *engineFixture) []uuid.UUID { return sortIDs(f.assignee) },
			{Type: domain.RecipientWatchers}:                func(f *engineFixture) []uuid.UUID { return sortIDs(f.watcher1, f.watcher2) },
			{Type: domain.RecipientProjectLead}:             func(f *engineFixture) []uuid.UUID { return sortIDs(f.lead) },
			{Type: domain.RecipientComponentLead}:           func(f *engineFixture) []uuid.UUID { return sortIDs(f.componentLead) },
			{Type: domain.RecipientMentioned}:               func(f *engineFixture) []uuid.UUID { return sortIDs(f.mentioned) },
			{Type: domain.RecipientReplyTarget}:             func(f *engineFixture) []uuid.UUID { return sortIDs(f.replyTarget) },
			{Type: domain.RecipientRole, Role: "developer"}: func(f *engineFixture) []uuid.UUID { return sortIDs(f.member) },
		}
		for recipient, want := range cases {
			name := string(event) + "/" + string(recipient.Type)
			t.Run(name, func(t *testing.T) {
				f := newEngineFixture(t)
				f.setProjectRule(domain.Rule{Event: event, Recipients: []domain.Recipient{recipient}, Channels: []domain.Channel{domain.ChannelInApp}})
				err := f.engine.Deliver(context.Background(), port.Notice{
					Event: event, IssueID: f.issue, ActorID: f.actor, DedupKey: "k",
					Explicit: map[domain.RecipientType][]uuid.UUID{
						domain.RecipientMentioned: {f.mentioned}, domain.RecipientReplyTarget: {f.replyTarget},
					},
				})
				if err != nil {
					t.Fatal(err)
				}
				if got := f.notified(); !equalIDs(got, want(f)) {
					t.Fatalf("notified %v, want %v", got, want(f))
				}
			})
		}
	}
}

func TestSpecificUserAndTeamRecipients(t *testing.T) {
	f := newEngineFixture(t)
	f.setProjectRule(domain.Rule{Event: domain.EventIssueCreated, Channels: []domain.Channel{domain.ChannelInApp}, Recipients: []domain.Recipient{
		{Type: domain.RecipientUser, ID: f.outside}, {Type: domain.RecipientTeam, ID: f.team}, {Type: domain.RecipientReporter}, {Type: domain.RecipientUser, ID: f.reporter},
	}})
	if err := f.engine.Deliver(context.Background(), port.Notice{Event: domain.EventIssueCreated, IssueID: f.issue, ActorID: f.actor}); err != nil {
		t.Fatal(err)
	}
	want := sortIDs(f.outside, f.teamMember, f.reporter)
	if got := f.notified(); !equalIDs(got, want) {
		t.Fatalf("notified %v, want %v (reporter once)", got, want)
	}
}

// TestDefaultSchemeKeepsTodaysBehavior checks that without any stored scheme the three legacy notices are
// delivered in the application only, with the text the caller gave, and nothing else notifies anybody.
func TestDefaultSchemeKeepsTodaysBehavior(t *testing.T) {
	f := newEngineFixture(t)
	ctx := context.Background()
	link := "/issues/" + f.issue.String()
	for kind, user := range map[string]uuid.UUID{"issue_assigned": f.assignee, "comment_mention": f.mentioned, "comment_reply": f.replyTarget} {
		handled, err := f.engine.NotifyLegacy(ctx, user, kind, "Mensaje "+kind, link+"#comment-"+uuid.NewString())
		if err != nil || !handled {
			t.Fatalf("%s: handled=%v err=%v", kind, handled, err)
		}
		got, _ := f.notes.ListByUser(ctx, user, 10)
		if len(got) != 1 || got[0].Kind != kind || got[0].Message != "Mensaje "+kind {
			t.Fatalf("%s: stored %+v", kind, got)
		}
	}
	if len(f.outbox.entries) != 0 || len(f.delivery.items) != 0 {
		t.Fatal("the default scheme must not send email")
	}
	if handled, err := f.engine.NotifyLegacy(ctx, f.assignee, "something_else", "x", ""); handled || err != nil {
		t.Fatalf("unknown kind: handled=%v err=%v", handled, err)
	}

	// Every other event notifies nobody by default.
	before := len(f.notes.items)
	for _, event := range domain.AllEvents {
		if event == domain.EventIssueAssigned || event == domain.EventIssueMentioned || event == domain.EventIssueCommented {
			continue
		}
		if err := f.engine.Deliver(ctx, port.Notice{Event: event, IssueID: f.issue, ActorID: f.actor}); err != nil {
			t.Fatal(err)
		}
	}
	if len(f.notes.items) != before {
		t.Fatal("default scheme notified for an event it does not cover")
	}
}

func TestLegacyMentionIsLimitedToTheMentionedUser(t *testing.T) {
	f := newEngineFixture(t)
	f.setProjectRule(domain.Rule{Event: domain.EventIssueMentioned, Channels: []domain.Channel{domain.ChannelInApp},
		Recipients: []domain.Recipient{{Type: domain.RecipientMentioned}, {Type: domain.RecipientWatchers}}})
	if _, err := f.engine.NotifyLegacy(context.Background(), f.mentioned, "comment_mention", "Te mencionaron", "/issues/"+f.issue.String()); err != nil {
		t.Fatal(err)
	}
	if got := f.notified(); !equalIDs(got, sortIDs(f.mentioned)) {
		t.Fatalf("notified %v, only the mentioned user expected", got)
	}
}

func TestPreferencesWinOverTheScheme(t *testing.T) {
	ctx := context.Background()
	rule := domain.Rule{Event: domain.EventIssueCreated, Channels: []domain.Channel{domain.ChannelInApp, domain.ChannelEmail},
		Recipients: []domain.Recipient{{Type: domain.RecipientReporter}, {Type: domain.RecipientAssignee}, {Type: domain.RecipientWatchers}}}

	f := newEngineFixture(t)
	f.setProjectRule(rule)
	f.notes.prefs = map[uuid.UUID]domain.Preferences{
		f.reporter: {InApp: false, Email: domain.EmailImmediate},
		f.assignee: {InApp: true, Email: domain.EmailOff},
		f.watcher1: {InApp: true, Email: domain.EmailDaily},
		f.actor:    {InApp: true, Email: domain.EmailImmediate},
	}
	if err := f.engine.Deliver(ctx, port.Notice{Event: domain.EventIssueCreated, IssueID: f.issue, ActorID: f.actor}); err != nil {
		t.Fatal(err)
	}
	// Reporter: no in-app, immediate email. Assignee: in-app, no email. Watcher1: in-app, digest item. Watcher2: defaults.
	if got := f.notified(); !equalIDs(got, sortIDs(f.assignee, f.watcher1, f.watcher2)) {
		t.Fatalf("in-app notified %v", got)
	}
	var emailed []string
	for _, e := range f.outbox.entries {
		emailed = append(emailed, e.To)
	}
	sort.Strings(emailed)
	if strings.Join(emailed, ",") != "reporter@example.test,watcher2@example.test" {
		t.Fatalf("emails to %v", emailed)
	}
	if len(f.delivery.items) != 1 || f.delivery.items[0].UserID != f.watcher1 || f.delivery.items[0].IssueKey != "WEB-7" {
		t.Fatalf("digest items %+v", f.delivery.items)
	}
}

func TestOwnActionsAreSkippedUnlessThePersonAsksForThem(t *testing.T) {
	ctx := context.Background()
	f := newEngineFixture(t)
	f.setProjectRule(domain.Rule{Event: domain.EventIssueUpdated, Channels: []domain.Channel{domain.ChannelInApp},
		Recipients: []domain.Recipient{{Type: domain.RecipientReporter}}})
	if err := f.engine.Deliver(ctx, port.Notice{Event: domain.EventIssueUpdated, IssueID: f.issue, ActorID: f.reporter}); err != nil {
		t.Fatal(err)
	}
	if len(f.notes.items) != 0 {
		t.Fatal("the actor was told about their own action")
	}
	f.notes.prefs = map[uuid.UUID]domain.Preferences{f.reporter: {InApp: true, Email: domain.EmailImmediate, NotifyOwnActions: true}}
	if err := f.engine.Deliver(ctx, port.Notice{Event: domain.EventIssueUpdated, IssueID: f.issue, ActorID: f.reporter}); err != nil {
		t.Fatal(err)
	}
	if len(f.notes.items) != 1 {
		t.Fatalf("notify_own_actions ignored: %d notices", len(f.notes.items))
	}
}

func TestDeliveryIsIdempotentPerEventAndUser(t *testing.T) {
	ctx := context.Background()
	f := newEngineFixture(t)
	f.setProjectRule(domain.Rule{Event: domain.EventIssueCreated, Channels: []domain.Channel{domain.ChannelInApp, domain.ChannelEmail},
		Recipients: []domain.Recipient{{Type: domain.RecipientAssignee}}})
	notice := port.Notice{Event: domain.EventIssueCreated, IssueID: f.issue, ActorID: f.actor, DedupKey: "evt:5"}
	for i := 0; i < 3; i++ {
		if err := f.engine.Deliver(ctx, notice); err != nil {
			t.Fatal(err)
		}
	}
	if len(f.notes.items) != 1 || len(f.outbox.entries) != 1 {
		t.Fatalf("redelivery duplicated: %d notices, %d emails", len(f.notes.items), len(f.outbox.entries))
	}
	mail := f.outbox.entries[0]
	if mail.Text != "https://corelog.example.test/browse/WEB-7" || mail.Kind != "issue_created" {
		t.Fatalf("email = %+v", mail)
	}
}

func TestRecipientsWhoCannotBrowseOrAreInactiveAreSkipped(t *testing.T) {
	ctx := context.Background()
	f := newEngineFixture(t)
	f.setProjectRule(domain.Rule{Event: domain.EventIssueCreated, Channels: []domain.Channel{domain.ChannelInApp},
		Recipients: []domain.Recipient{{Type: domain.RecipientReporter}, {Type: domain.RecipientAssignee}, {Type: domain.RecipientWatchers}}})
	f.browser.denied[f.reporter] = true
	c := f.facts.contacts[f.assignee]
	c.Active = false
	f.facts.contacts[f.assignee] = c
	if err := f.engine.Deliver(ctx, port.Notice{Event: domain.EventIssueCreated, IssueID: f.issue, ActorID: f.actor}); err != nil {
		t.Fatal(err)
	}
	if got := f.notified(); !equalIDs(got, sortIDs(f.watcher1, f.watcher2)) {
		t.Fatalf("notified %v", got)
	}
}

func TestProjectSchemeOverridesOrganizationDefault(t *testing.T) {
	ctx := context.Background()
	f := newEngineFixture(t)
	f.schemes.stored[scopeKey(f.org, nil)] = port.StoredScheme{Scheme: domain.Scheme{Rules: []domain.Rule{
		{Event: domain.EventIssueCreated, Channels: []domain.Channel{domain.ChannelInApp}, Recipients: []domain.Recipient{{Type: domain.RecipientReporter}}},
		{Event: domain.EventIssueUpdated, Channels: []domain.Channel{domain.ChannelInApp}, Recipients: []domain.Recipient{{Type: domain.RecipientAssignee}}},
	}}}
	f.setProjectRule(domain.Rule{Event: domain.EventIssueCreated, Channels: []domain.Channel{domain.ChannelInApp}, Recipients: []domain.Recipient{{Type: domain.RecipientProjectLead}}})

	scheme, err := f.engine.Effective(ctx, f.org, f.project)
	if err != nil {
		t.Fatal(err)
	}
	created, _ := scheme.Rule(domain.EventIssueCreated)
	updated, _ := scheme.Rule(domain.EventIssueUpdated)
	assigned, _ := scheme.Rule(domain.EventIssueAssigned)
	if created.Recipients[0].Type != domain.RecipientProjectLead || updated.Recipients[0].Type != domain.RecipientAssignee || assigned.Recipients[0].Type != domain.RecipientAssignee {
		t.Fatalf("effective scheme wrong: %+v / %+v / %+v", created, updated, assigned)
	}
	if len(scheme.Rules) != len(domain.AllEvents) {
		t.Fatalf("effective scheme has %d rules", len(scheme.Rules))
	}
}

func TestProjectEventsWithoutIssue(t *testing.T) {
	ctx := context.Background()
	f := newEngineFixture(t)
	f.setProjectRule(domain.Rule{Event: domain.EventVersionReleased, Channels: []domain.Channel{domain.ChannelInApp},
		Recipients: []domain.Recipient{{Type: domain.RecipientProjectLead}}})
	if err := f.engine.Deliver(ctx, port.Notice{Event: domain.EventVersionReleased, ProjectID: f.project, ActorID: f.actor, Detail: "1.0", DedupKey: "evt:9"}); err != nil {
		t.Fatal(err)
	}
	if len(f.notes.items) != 1 || f.notes.items[0].UserID != f.lead || !strings.Contains(f.notes.items[0].Message, "(1.0)") {
		t.Fatalf("notices %+v", f.notes.items)
	}
}

func TestIssueIDFromLink(t *testing.T) {
	id := uuid.New()
	cases := map[string]uuid.UUID{
		"/issues/" + id.String():                  id,
		"/issues/" + id.String() + "#comment-abc": id,
		"/issues/" + id.String() + "?tab=history": id,
		"/issues/not-a-uuid":                      uuid.Nil,
		"/projects/" + id.String():                uuid.Nil,
		"":                                        uuid.Nil,
	}
	for link, want := range cases {
		if got := issueIDFromLink(link); got != want {
			t.Errorf("%q: got %v, want %v", link, got, want)
		}
	}
}

func TestNotificationServiceRoutesKnownKindsThroughTheEngine(t *testing.T) {
	ctx := context.Background()
	f := newEngineFixture(t)
	service := NewNotificationService(f.notes).WithEngine(f.engine)
	if err := service.Notify(ctx, f.assignee, "issue_assigned", "Te asignaron", "/issues/"+f.issue.String()); err != nil {
		t.Fatal(err)
	}
	if err := service.Notify(ctx, f.assignee, "custom_kind", "Otro aviso", ""); err != nil {
		t.Fatal(err)
	}
	if got, _ := f.notes.ListByUser(ctx, f.assignee, 10); len(got) != 2 {
		t.Fatalf("stored %d notices, want 2", len(got))
	}
	// A project that removes the assignee from issue_assigned silences the notice.
	f.setProjectRule(domain.Rule{Event: domain.EventIssueAssigned, Channels: []domain.Channel{domain.ChannelInApp}, Recipients: []domain.Recipient{{Type: domain.RecipientReporter}}})
	if err := service.Notify(ctx, f.assignee, "issue_assigned", "Te asignaron", "/issues/"+f.issue.String()); err != nil {
		t.Fatal(err)
	}
	if got, _ := f.notes.ListByUser(ctx, f.assignee, 10); len(got) != 2 {
		t.Fatalf("scheme did not silence the notice: %d", len(got))
	}
	if _, err := service.UpdatePreferences(ctx, f.assignee, domain.Preferences{InApp: true, Email: "weekly"}); err == nil {
		t.Fatal("invalid email mode accepted")
	}
	if p, err := service.UpdatePreferences(ctx, f.assignee, domain.Preferences{InApp: true}); err != nil || p.Email != domain.EmailImmediate {
		t.Fatalf("default email mode: %+v %v", p, err)
	}
}
