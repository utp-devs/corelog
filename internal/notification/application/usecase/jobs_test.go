package usecase

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/jhonsferg/corelog/internal/notification/application/port"
	"github.com/jhonsferg/corelog/internal/notification/domain"
	"github.com/jhonsferg/corelog/internal/platform/apperror"
	"github.com/jhonsferg/corelog/internal/platform/clock"
	"github.com/jhonsferg/corelog/internal/platform/database"
	"github.com/jhonsferg/corelog/internal/platform/events"
	"github.com/jhonsferg/corelog/internal/platform/mailer"
)

var t0 = time.Date(2026, 11, 27, 10, 0, 0, 0, time.UTC)

func TestRetryDelaySchedule(t *testing.T) {
	want := map[int]time.Duration{1: time.Minute, 2: 5 * time.Minute, 3: 30 * time.Minute, 4: 2 * time.Hour, 5: 12 * time.Hour}
	for attempts, delay := range want {
		if got, ok := RetryDelay(attempts); !ok || got != delay {
			t.Errorf("attempts %d: %v %v, want %v", attempts, got, ok, delay)
		}
	}
	for _, attempts := range []int{0, 6, 7} {
		if _, ok := RetryDelay(attempts); ok {
			t.Errorf("attempts %d must be dead", attempts)
		}
	}
}

func TestOutboxLifecycleWithRetriesAndDeadLetter(t *testing.T) {
	ctx := context.Background()
	outbox := newMemOutbox()
	clk := &clock.FakeClock{T: t0}
	failing := &mailer.Recorder{Err: errors.New("smtp down")}
	worker := NewOutboxWorker(outbox, failing, clk, nil, 10)
	if _, err := outbox.Enqueue(ctx, port.EmailMessage{To: "a@example.test", Kind: "x", Subject: "s", Text: "t"}, clk.Now()); err != nil {
		t.Fatal(err)
	}
	entry := outbox.entries[0]

	delays := []time.Duration{time.Minute, 5 * time.Minute, 30 * time.Minute, 2 * time.Hour, 12 * time.Hour}
	for i, delay := range delays {
		sent, failed, err := worker.Run(ctx)
		if err != nil || sent != 0 || failed != 1 {
			t.Fatalf("attempt %d: sent=%d failed=%d err=%v", i+1, sent, failed, err)
		}
		if entry.Attempts != i+1 || entry.Status != port.OutboxPending || !entry.NextAttemptAt.Equal(clk.Now().Add(delay)) {
			t.Fatalf("attempt %d: %+v", i+1, entry)
		}
		// Not due before the backoff has passed.
		if sent, failed, _ := worker.Run(ctx); sent+failed != 0 {
			t.Fatalf("attempt %d: message retried before its backoff", i+1)
		}
		clk.Advance(delay)
	}
	if _, failed, _ := worker.Run(ctx); failed != 1 {
		t.Fatal("sixth attempt did not run")
	}
	if entry.Status != port.OutboxDead || entry.Attempts != 6 || entry.LastError != "smtp down" {
		t.Fatalf("dead letter: %+v", entry)
	}
	clk.Advance(48 * time.Hour)
	if sent, failed, _ := worker.Run(ctx); sent+failed != 0 {
		t.Fatal("a dead message was sent again")
	}

	admin := NewOutboxAdminService(outbox, clk)
	if err := admin.Retry(ctx, entry.ID); err != nil {
		t.Fatal(err)
	}
	ok := &mailer.Recorder{}
	worker = NewOutboxWorker(outbox, ok, clk, nil, 10)
	if sent, _, _ := worker.Run(ctx); sent != 1 || entry.Status != port.OutboxSent || len(ok.Messages) != 1 || ok.Messages[0].To != "a@example.test" {
		t.Fatalf("retry did not deliver: %+v", entry)
	}
	if err := admin.Retry(ctx, entry.ID); apperror.CodeOf(err) != CodeOutboxNotRetryable || apperror.KindOf(err) != apperror.KindConflict {
		t.Fatalf("retrying a sent message: %v", err)
	}
	if _, _, err := admin.List(ctx, "bogus", 10, 0); apperror.CodeOf(err) != CodeOutboxStatusInvalid {
		t.Fatalf("bad status: %v", err)
	}
	if items, total, err := admin.List(ctx, "sent", 10, 0); err != nil || total != 1 || len(items) != 1 {
		t.Fatalf("list sent: %d %d %v", len(items), total, err)
	}
}

func TestOutboxIdempotencyKey(t *testing.T) {
	ctx := context.Background()
	outbox := newMemOutbox()
	msg := port.EmailMessage{To: "a@example.test", Kind: "x", Subject: "s", Text: "t", IdempotencyKey: "k1"}
	first, _ := outbox.Enqueue(ctx, msg, t0)
	second, _ := outbox.Enqueue(ctx, msg, t0)
	if !first || second || len(outbox.entries) != 1 {
		t.Fatalf("first=%v second=%v entries=%d", first, second, len(outbox.entries))
	}
}

func TestDigestBundlesPendingItemsPerUser(t *testing.T) {
	ctx := context.Background()
	f := newEngineFixture(t)
	f.setProjectRule(domain.Rule{Event: domain.EventIssueCreated, Channels: []domain.Channel{domain.ChannelEmail},
		Recipients: []domain.Recipient{{Type: domain.RecipientAssignee}, {Type: domain.RecipientReporter}}})
	f.notes.prefs = map[uuid.UUID]domain.Preferences{
		f.assignee: {InApp: true, Email: domain.EmailDaily},
		f.reporter: {InApp: true, Email: domain.EmailDaily},
	}
	for i := 0; i < 3; i++ {
		n := port.Notice{Event: domain.EventIssueCreated, IssueID: f.issue, ActorID: f.actor, DedupKey: "evt:" + string(rune('a'+i))}
		if err := f.engine.Deliver(ctx, n); err != nil {
			t.Fatal(err)
		}
	}
	if len(f.delivery.items) != 6 || len(f.outbox.entries) != 0 {
		t.Fatalf("items=%d emails=%d", len(f.delivery.items), len(f.outbox.entries))
	}
	// A user who was deactivated meanwhile gets no digest but their items are closed.
	c := f.facts.contacts[f.reporter]
	c.Active = false
	f.facts.contacts[f.reporter] = c

	job := NewDigestJob(f.delivery, f.facts, f.outbox, fakeRenderer{}, f.notes, &database.FakeTxManager{}, clock.NewFixed(t0), "https://corelog.example.test")
	queued, err := job.Run(ctx)
	if err != nil || queued != 1 {
		t.Fatalf("queued=%d err=%v", queued, err)
	}
	if len(f.outbox.entries) != 1 || f.outbox.entries[0].To != "assignee@example.test" || f.outbox.entries[0].Kind != "digest" {
		t.Fatalf("outbox %+v", f.outbox.entries)
	}
	if len(f.delivery.items) != 0 {
		t.Fatalf("%d items left after the digest", len(f.delivery.items))
	}
	if queued, _ := job.Run(ctx); queued != 0 {
		t.Fatal("a second run produced another digest")
	}
}

type fakeSubs struct {
	due       []port.DueFilter
	rows      []port.FilterRow
	delivered []uuid.UUID
	runErr    map[uuid.UUID]error
}

func (s *fakeSubs) Due(context.Context) ([]port.DueFilter, error) { return s.due, nil }
func (s *fakeSubs) Run(_ context.Context, d port.DueFilter, limit int) ([]port.FilterRow, int, error) {
	if err := s.runErr[d.SubscriptionID]; err != nil {
		return nil, 0, err
	}
	return s.rows, len(s.rows), nil
}
func (s *fakeSubs) MarkDelivered(_ context.Context, id uuid.UUID) error {
	s.delivered = append(s.delivered, id)
	return nil
}

func TestFilterSubscriptionJobSendsResultsAndMarksDelivery(t *testing.T) {
	ctx := context.Background()
	f := newEngineFixture(t)
	f.notes.prefs = map[uuid.UUID]domain.Preferences{f.watcher2: {InApp: true, Email: domain.EmailOff}}
	subs := &fakeSubs{
		rows: []port.FilterRow{{Key: "WEB-1", Summary: "Uno", Status: "Abierta"}},
		due: []port.DueFilter{
			{SubscriptionID: uuid.New(), UserID: f.reporter, FilterName: "Mis abiertas", FilterID: uuid.New()},
			{SubscriptionID: uuid.New(), UserID: f.watcher2, FilterName: "Silenciada", FilterID: uuid.New()},
			{SubscriptionID: uuid.New(), UserID: f.assignee, FilterName: "Rota", FilterID: uuid.New()},
			{SubscriptionID: uuid.New(), UserID: f.watcher1, FilterName: "Otra", FilterID: uuid.New()},
		},
		runErr: map[uuid.UUID]error{},
	}
	subs.runErr[subs.due[2].SubscriptionID] = errors.New("query failed")
	job := NewFilterSubscriptionJob(subs, f.facts, f.notes, f.outbox, fakeRenderer{}, clock.NewFixed(t0), "https://corelog.example.test")
	queued, err := job.Run(ctx)
	if err == nil {
		t.Fatal("the failing subscription was not reported")
	}
	if queued != 2 || len(f.outbox.entries) != 2 {
		t.Fatalf("queued=%d emails=%d", queued, len(f.outbox.entries))
	}
	// Delivered: the two sent and the one whose owner turned email off; the failing one stays due.
	if len(subs.delivered) != 3 {
		t.Fatalf("delivered %v", subs.delivered)
	}
	for _, id := range subs.delivered {
		if id == subs.due[2].SubscriptionID {
			t.Fatal("the failing subscription was marked delivered")
		}
	}
	// Running the same due list again is idempotent for the outbox.
	subs.due = subs.due[:1]
	if _, err := job.Run(ctx); err != nil || len(f.outbox.entries) != 2 {
		t.Fatalf("second run: %v, %d emails", err, len(f.outbox.entries))
	}
}

func TestSchemeConsumerMapsEvents(t *testing.T) {
	ctx := context.Background()
	f := newEngineFixture(t)
	consumer := NewSchemeConsumer(f.engine)
	if consumer.Name() == "" {
		t.Fatal("consumer needs a cursor name")
	}
	all := []domain.Recipient{{Type: domain.RecipientReporter}}
	for _, event := range []domain.Event{domain.EventIssueCreated, domain.EventIssueUpdated, domain.EventIssueTransitioned, domain.EventIssueResolved, domain.EventVersionReleased, domain.EventSprintStarted, domain.EventSprintCompleted} {
		pid := f.project
		_ = pid
		stored := f.schemes.stored[scopeKey(f.org, &pid)]
		stored.Scheme.Rules = append(stored.Scheme.Rules, domain.Rule{Event: event, Channels: []domain.Channel{domain.ChannelInApp}, Recipients: all})
		f.schemes.stored[scopeKey(f.org, &pid)] = stored
	}
	actor, project := f.actor, f.project
	issuePayload := map[string]any{"issue_id": f.issue.String()}
	handle := func(id int64, typ string, payload map[string]any) {
		t.Helper()
		if err := consumer.Handle(ctx, events.Event{ID: id, Type: typ, ActorID: &actor, ProjectID: &project, Payload: payload}); err != nil {
			t.Fatal(err)
		}
	}
	handle(1, "issue.created", issuePayload)
	handle(2, "issue.updated", map[string]any{"issue_id": f.issue.String(), "changes": []any{map[string]any{"field": "summary"}, map[string]any{"field": "priority"}}})
	handle(3, "issue.transitioned", map[string]any{"issue_id": f.issue.String(), "from_status": "Abierta", "to_status": "Hecha", "to_category": "done"})
	handle(4, "version.released", map[string]any{"name": "2.0"})
	handle(5, "sprint.started", map[string]any{"name": "Sprint 1"})
	handle(6, "sprint.completed", map[string]any{"name": "Sprint 1"})
	handle(7, "comment.mentioned", issuePayload) // handled by the mention consumer
	handle(8, "unknown.thing", nil)
	handle(9, "issue.created", map[string]any{}) // no issue: ignored

	// issue.transitioned to a done category also raises issue_resolved: created, updated, transitioned, resolved,
	// released, started and completed notify the reporter or, without issue, nobody (project lead is not a recipient).
	var messages []string
	for _, n := range f.notes.items {
		messages = append(messages, n.Message)
	}
	// Four issue events reach the reporter; the three project events have no reporter.
	if len(messages) != 4 {
		t.Fatalf("%d notices: %v", len(messages), messages)
	}
	if want := "actor cambio el estado"; !contains(messages[2], "cambio el estado") || !contains(messages[2], "Abierta -> Hecha") {
		t.Fatalf("transition message %q, want %q", messages[2], want)
	}
	if !contains(messages[1], "campos: summary, priority") {
		t.Fatalf("update message %q", messages[1])
	}
	// Redelivery of the same events does not duplicate.
	handle(1, "issue.created", issuePayload)
	handle(3, "issue.transitioned", map[string]any{"issue_id": f.issue.String(), "to_category": "done"})
	if len(f.notes.items) != 4 {
		t.Fatalf("redelivery duplicated notices: %d", len(f.notes.items))
	}
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

type fakeProjects struct {
	err     error
	actions []string
}

func (p *fakeProjects) Authorize(_ context.Context, action string, _ uuid.UUID, _ string, _ uuid.UUID) error {
	p.actions = append(p.actions, action)
	return p.err
}

type fakeOrgs struct{ levels map[string]error }

func (o fakeOrgs) RequireRole(_ context.Context, _, _ uuid.UUID, min string) error {
	return o.levels[min]
}

func TestSchemeServicePermissionsValidationAndReset(t *testing.T) {
	ctx := context.Background()
	repo := newMemSchemes()
	org, project, actor := uuid.New(), uuid.New(), uuid.New()
	repo.projects[project] = org
	projects := &fakeProjects{}
	orgs := fakeOrgs{levels: map[string]error{}}
	svc := NewSchemeService(repo, projects, orgs, clock.NewFixed(t0))

	view, err := svc.GetProject(ctx, actor, "user", project)
	if err != nil || len(view.Scheme.Rules) != len(domain.AllEvents) || len(view.Overridden) != 0 || view.UpdatedAt != nil {
		t.Fatalf("default view: %+v %v", view, err)
	}
	if projects.actions[0] != "edit_project" {
		t.Fatalf("project actions %v", projects.actions)
	}

	put := domain.Scheme{Rules: []domain.Rule{{Event: domain.EventIssueCreated, Recipients: []domain.Recipient{{Type: domain.RecipientWatchers}, {Type: domain.RecipientWatchers}}, Channels: []domain.Channel{domain.ChannelEmail, domain.ChannelEmail}}}}
	view, err = svc.PutProject(ctx, actor, "user", project, put)
	if err != nil || !view.Overridden[domain.EventIssueCreated] || view.UpdatedAt == nil {
		t.Fatalf("put: %+v %v", view, err)
	}
	if rule, _ := view.Scheme.Rule(domain.EventIssueCreated); len(rule.Recipients) != 1 || len(rule.Channels) != 1 {
		t.Fatalf("not normalized: %+v", rule)
	}
	if _, err := svc.PutProject(ctx, actor, "user", project, domain.Scheme{Rules: []domain.Rule{{Event: "nope"}}}); apperror.CodeOf(err) != domain.CodeSchemeInvalid {
		t.Fatalf("invalid event: %v", err)
	}
	view, err = svc.ResetProject(ctx, actor, "user", project)
	if err != nil || len(view.Overridden) != 0 {
		t.Fatalf("reset: %+v %v", view, err)
	}

	// Authorization failures pass through; an unknown project is not found.
	projects.err = apperror.New(apperror.KindForbidden, "no")
	if _, err := svc.GetProject(ctx, actor, "user", project); apperror.KindOf(err) != apperror.KindForbidden {
		t.Fatalf("forbidden: %v", err)
	}
	projects.err = nil
	if _, err := svc.GetProject(ctx, actor, "user", uuid.New()); apperror.KindOf(err) != apperror.KindNotFound {
		t.Fatalf("unknown project: %v", err)
	}

	// Organization: members read, admins write.
	orgs.levels["admin"] = apperror.New(apperror.KindForbidden, "no")
	if _, err := svc.GetOrganization(ctx, actor, org); err != nil {
		t.Fatalf("member read: %v", err)
	}
	if _, err := svc.PutOrganization(ctx, actor, org, put); apperror.KindOf(err) != apperror.KindForbidden {
		t.Fatalf("member write: %v", err)
	}
	if _, err := svc.ResetOrganization(ctx, actor, org); apperror.KindOf(err) != apperror.KindForbidden {
		t.Fatalf("member reset: %v", err)
	}
	delete(orgs.levels, "admin")
	if view, err := svc.PutOrganization(ctx, actor, org, put); err != nil || !view.Overridden[domain.EventIssueCreated] {
		t.Fatalf("admin write: %+v %v", view, err)
	}
	// The project inherits the organization rule.
	view, _ = svc.GetProject(ctx, actor, "user", project)
	if rule, _ := view.Scheme.Rule(domain.EventIssueCreated); rule.Recipients[0].Type != domain.RecipientWatchers || view.Overridden[domain.EventIssueCreated] {
		t.Fatalf("inheritance: %+v", view)
	}
	if view, err := svc.ResetOrganization(ctx, actor, org); err != nil || len(view.Overridden) != 0 {
		t.Fatalf("org reset: %+v %v", view, err)
	}
}
