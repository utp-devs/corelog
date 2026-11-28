package postgres

import (
	"context"
	"errors"
	"sort"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"

	notificationemail "github.com/jhonsferg/corelog/internal/notification/adapter/out/email"
	"github.com/jhonsferg/corelog/internal/notification/application/port"
	"github.com/jhonsferg/corelog/internal/notification/application/usecase"
	"github.com/jhonsferg/corelog/internal/notification/domain"
	"github.com/jhonsferg/corelog/internal/platform/clock"
	"github.com/jhonsferg/corelog/internal/platform/database"
	"github.com/jhonsferg/corelog/internal/platform/database/testdb"
	"github.com/jhonsferg/corelog/internal/platform/events"
	"github.com/jhonsferg/corelog/internal/platform/mailer"
)

const defaultOrg = "00000000-0000-4000-8000-000000000002"

var base = time.Date(2026, 11, 27, 10, 0, 0, 0, time.UTC)

type world struct {
	t        *testing.T
	db       *sqlx.DB
	project  uuid.UUID
	issue    uuid.UUID
	lead     uuid.UUID
	reporter uuid.UUID
	assignee uuid.UUID
	watcher  uuid.UUID
	clead    uuid.UUID
	outsider uuid.UUID
}

func (w *world) exec(query string, args ...any) {
	w.t.Helper()
	if _, err := w.db.Exec(query, args...); err != nil {
		w.t.Fatalf("%v\n%s", err, query)
	}
}

func (w *world) user(name string) uuid.UUID {
	id := uuid.New()
	w.exec(`INSERT INTO users (id, name, display_name, email, password_hash, role) VALUES ($1, $2, $2, $3, 'x', 'user')`, id, name, name+"@example.test")
	return id
}

func newWorld(t *testing.T) *world {
	t.Helper()
	w := &world{t: t, db: testdb.New(t), project: uuid.New(), issue: uuid.New()}
	w.lead, w.reporter, w.assignee = w.user("lead"), w.user("reporter"), w.user("assignee")
	w.watcher, w.clead, w.outsider = w.user("watcher"), w.user("clead"), w.user("outsider")
	w.exec(`INSERT INTO projects (id, organization_id, key, name, type, template, lead_id, visibility) VALUES ($1, $2, 'NTF', 'Notif', 'kanban', 'scrum', $3, 'open')`, w.project, defaultOrg, w.lead)
	w.exec(`SELECT corelog_ensure_workflow($1)`, w.project)
	w.exec(`INSERT INTO issues (id, project_id, summary, description, status, priority, reporter_id, assignee_id, key_number, issue_type_id, priority_id, status_id)
		SELECT $1, $2, 'Resumen', '', 'open', 'medium', $3, $4, 1,
		       (SELECT it.id FROM issue_types it WHERE it.organization_id = $5 AND it.is_default),
		       (SELECT pr.id FROM priorities pr WHERE pr.organization_id = $5 AND pr.is_default),
		       (SELECT (wf.definition -> 'statuses' -> 0 ->> 'id')::uuid FROM workflows wf WHERE wf.project_id = $2)`,
		w.issue, w.project, w.reporter, w.assignee, defaultOrg)
	w.exec(`INSERT INTO watchers (issue_id, user_id) VALUES ($1, $2)`, w.issue, w.watcher)
	component := uuid.New()
	w.exec(`INSERT INTO components (id, project_id, name, lead_id) VALUES ($1, $2, 'Backend', $3)`, component, w.project, w.clead)
	w.exec(`INSERT INTO issue_components (issue_id, component_id) VALUES ($1, $2)`, w.issue, component)
	return w
}

func TestFactsRepository(t *testing.T) {
	w := newWorld(t)
	ctx := context.Background()
	repo := NewFactsRepository(w.db)

	issue, err := repo.Issue(ctx, w.issue)
	if err != nil || issue == nil {
		t.Fatalf("issue: %v %v", issue, err)
	}
	if issue.Key != "NTF-1" || issue.Summary != "Resumen" || issue.OrgID.String() != defaultOrg || issue.ProjectID != w.project ||
		issue.ReporterID != w.reporter || issue.AssigneeID != w.assignee || issue.ProjectLeadID != w.lead ||
		len(issue.WatcherIDs) != 1 || issue.WatcherIDs[0] != w.watcher || len(issue.ComponentLeadIDs) != 1 || issue.ComponentLeadIDs[0] != w.clead {
		t.Fatalf("facts = %+v", issue)
	}
	w.exec(`UPDATE issues SET assignee_id = NULL WHERE id = $1`, w.issue)
	if again, _ := repo.Issue(ctx, w.issue); again.AssigneeID != uuid.Nil {
		t.Fatal("unassigned issue has an assignee")
	}
	w.exec(`UPDATE issues SET deleted_at = now() WHERE id = $1`, w.issue)
	if gone, err := repo.Issue(ctx, w.issue); err != nil || gone != nil {
		t.Fatalf("deleted issue: %v %v", gone, err)
	}
	if missing, err := repo.Issue(ctx, uuid.New()); err != nil || missing != nil {
		t.Fatalf("missing issue: %v %v", missing, err)
	}

	project, err := repo.Project(ctx, w.project)
	if err != nil || project == nil || project.LeadID != w.lead || project.Name != "Notif" || project.OrgID.String() != defaultOrg {
		t.Fatalf("project: %+v %v", project, err)
	}
	if none, _ := repo.Project(ctx, uuid.New()); none != nil {
		t.Fatal("unknown project found")
	}

	team := uuid.New()
	w.exec(`INSERT INTO teams (id, name, organization_id) VALUES ($1, 'Equipo', $2)`, team, defaultOrg)
	w.exec(`INSERT INTO team_members (team_id, user_id, role) VALUES ($1, $2, 'member'), ($1, $3, 'maintainer')`, team, w.reporter, w.assignee)
	teams, err := repo.TeamMembers(ctx, []uuid.UUID{team, uuid.New()})
	if err != nil || len(teams[team]) != 2 {
		t.Fatalf("teams: %v %v", teams, err)
	}
	w.exec(`INSERT INTO project_members (project_id, user_id, role) VALUES ($1, $2, 'contributor'), ($1, $3, 'viewer')`, w.project, w.reporter, w.assignee)
	roles, err := repo.RoleMembers(ctx, w.project, []string{"contributor", "ghost"})
	if err != nil || len(roles["contributor"]) != 1 || roles["contributor"][0] != w.reporter || len(roles["ghost"]) != 0 {
		t.Fatalf("roles: %v %v", roles, err)
	}

	w.exec(`UPDATE users SET deactivated_at = now() WHERE id = $1`, w.outsider)
	contacts, err := repo.Contacts(ctx, []uuid.UUID{w.reporter, w.outsider, uuid.New()})
	if err != nil || len(contacts) != 2 || !contacts[w.reporter].Active || contacts[w.outsider].Active || contacts[w.reporter].Email != "reporter@example.test" || contacts[w.reporter].Name != "reporter" {
		t.Fatalf("contacts: %+v %v", contacts, err)
	}
}

func TestSchemeRepository(t *testing.T) {
	w := newWorld(t)
	ctx := context.Background()
	repo := NewSchemeRepository(w.db)
	org := uuid.MustParse(defaultOrg)
	team := uuid.New()
	scheme := domain.Scheme{Rules: []domain.Rule{{
		Event: domain.EventIssueCreated, Channels: []domain.Channel{domain.ChannelInApp, domain.ChannelEmail},
		Recipients: []domain.Recipient{{Type: domain.RecipientWatchers}, {Type: domain.RecipientTeam, ID: team}, {Type: domain.RecipientRole, Role: "developer"}},
	}}}

	if got, err := repo.Get(ctx, org, &w.project); err != nil || got != nil {
		t.Fatalf("empty get: %v %v", got, err)
	}
	if _, err := repo.Save(ctx, org, &w.project, scheme, w.lead, base); err != nil {
		t.Fatal(err)
	}
	got, err := repo.Get(ctx, org, &w.project)
	if err != nil || got == nil || len(got.Scheme.Rules) != 1 || got.Scheme.Rules[0].Recipients[1].ID != team || got.Scheme.Rules[0].Recipients[2].Role != "developer" || !got.UpdatedAt.Equal(base) {
		t.Fatalf("project scheme: %+v %v", got, err)
	}
	// Saving again replaces the row; the organization default is a different row.
	if _, err := repo.Save(ctx, org, &w.project, domain.Scheme{}, w.lead, base.Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.Save(ctx, org, nil, scheme, w.lead, base); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.Save(ctx, org, nil, scheme, w.lead, base.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	var rows int
	if err := w.db.Get(&rows, `SELECT count(*) FROM notification_schemes`); err != nil || rows != 2 {
		t.Fatalf("rows = %d (%v)", rows, err)
	}
	if got, _ := repo.Get(ctx, org, &w.project); len(got.Scheme.Rules) != 0 {
		t.Fatal("project scheme was not replaced")
	}
	if got, _ := repo.Get(ctx, org, nil); len(got.Scheme.Rules) != 1 {
		t.Fatal("organization scheme lost")
	}
	if org2, err := repo.ProjectOrg(ctx, w.project); err != nil || org2 != org {
		t.Fatalf("project org %v %v", org2, err)
	}
	if none, _ := repo.ProjectOrg(ctx, uuid.New()); none != uuid.Nil {
		t.Fatal("unknown project has an org")
	}
	if removed, err := repo.Delete(ctx, org, &w.project); err != nil || !removed {
		t.Fatalf("delete: %v %v", removed, err)
	}
	if removed, _ := repo.Delete(ctx, org, &w.project); removed {
		t.Fatal("deleted twice")
	}
	// Deleting the project cascades to nothing left; the org default survives the project.
	w.exec(`DELETE FROM issues WHERE project_id = $1`, w.project)
	w.exec(`DELETE FROM projects WHERE id = $1`, w.project)
	if got, _ := repo.Get(ctx, org, nil); got == nil {
		t.Fatal("organization scheme vanished with the project")
	}
}

func TestOutboxRepositoryLifecycle(t *testing.T) {
	w := newWorld(t)
	ctx := context.Background()
	repo := NewOutboxRepository(w.db)
	msg := port.EmailMessage{UserID: w.reporter, To: "reporter@example.test", Kind: "issue_created", Subject: "Asunto", Text: "texto", HTML: "<p>html</p>", IdempotencyKey: "k-1"}

	if ok, err := repo.Enqueue(ctx, msg, base); err != nil || !ok {
		t.Fatalf("enqueue: %v %v", ok, err)
	}
	if ok, err := repo.Enqueue(ctx, msg, base); err != nil || ok {
		t.Fatalf("duplicate enqueue: %v %v", ok, err)
	}
	later := msg
	later.IdempotencyKey, later.NotBefore = "", base.Add(time.Hour)
	if ok, err := repo.Enqueue(ctx, later, base); err != nil || !ok {
		t.Fatalf("scheduled enqueue: %v %v", ok, err)
	}

	claimed, err := repo.ClaimDue(ctx, base, 10, 5*time.Minute)
	if err != nil || len(claimed) != 1 || claimed[0].Subject != "Asunto" || claimed[0].Attempts != 0 || claimed[0].UserID == nil {
		t.Fatalf("claim: %+v %v", claimed, err)
	}
	if again, _ := repo.ClaimDue(ctx, base.Add(time.Minute), 10, 5*time.Minute); len(again) != 0 {
		t.Fatal("a leased message was claimed again before its lease ended")
	}
	if expired, _ := repo.ClaimDue(ctx, base.Add(6*time.Minute), 10, 5*time.Minute); len(expired) != 1 {
		t.Fatal("an expired lease was not reclaimed")
	}

	id := claimed[0].ID
	next := base.Add(time.Minute)
	if err := repo.MarkFailed(ctx, id, 1, &next, "smtp down", base); err != nil {
		t.Fatal(err)
	}
	if err := repo.MarkFailed(ctx, id, 6, nil, "smtp down", base); err != nil {
		t.Fatal(err)
	}
	dead, total, err := repo.List(ctx, "dead", 10, 0)
	if err != nil || total != 1 || dead[0].Attempts != 6 || dead[0].LastError != "smtp down" {
		t.Fatalf("dead list: %+v %d %v", dead, total, err)
	}
	if ok, err := repo.Retry(ctx, id, base); err != nil || !ok {
		t.Fatalf("retry: %v %v", ok, err)
	}
	if ok, _ := repo.Retry(ctx, id, base); ok {
		t.Fatal("a pending message was retried")
	}
	if err := repo.MarkSent(ctx, id, base); err != nil {
		t.Fatal(err)
	}
	sent, total, _ := repo.List(ctx, "sent", 10, 0)
	if total != 1 || sent[0].SentAt == nil || sent[0].LastError != "" {
		t.Fatalf("sent list: %+v", sent)
	}
	all, total, _ := repo.List(ctx, "", 1, 1)
	if total != 2 || len(all) != 1 {
		t.Fatalf("paging: %d of %d", len(all), total)
	}
	if err := w.db.Get(&total, `SELECT count(*) FROM email_outbox WHERE user_id IS NOT NULL`); err != nil || total != 2 {
		t.Fatal("rows lost their user")
	}
	if _, err := repo.Enqueue(ctx, port.EmailMessage{UserID: w.outsider, To: "o@example.test", Kind: "x", Subject: "s", Text: "t"}, base); err != nil {
		t.Fatal(err)
	}
	w.exec(`DELETE FROM users WHERE id = $1`, w.outsider)
	var orphans int
	if err := w.db.Get(&orphans, `SELECT count(*) FROM email_outbox WHERE to_email = 'o@example.test' AND user_id IS NULL`); err != nil || orphans != 1 {
		t.Fatalf("deleting a user must keep the message with a null user: %d (%v)", orphans, err)
	}
}

func TestOutboxClaimSkipsLockedRowsAndTwoWorkersNeverShareAMessage(t *testing.T) {
	w := newWorld(t)
	ctx := context.Background()
	repo := NewOutboxRepository(w.db)
	const total = 60
	for i := 0; i < total; i++ {
		msg := port.EmailMessage{To: "a@example.test", Kind: "x", Subject: "s", Text: "t"}
		if _, err := repo.Enqueue(ctx, msg, base); err != nil {
			t.Fatal(err)
		}
	}

	// A row locked by another transaction is skipped, not waited for.
	tx, err := w.db.BeginTxx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	var lockedID uuid.UUID
	if err := tx.GetContext(ctx, &lockedID, `SELECT id FROM email_outbox ORDER BY next_attempt_at, id LIMIT 1 FOR UPDATE`); err != nil {
		t.Fatal(err)
	}
	probe, err := repo.ClaimDue(ctx, base, total, time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range probe {
		if e.ID == lockedID {
			t.Fatal("a locked row was claimed")
		}
	}
	if len(probe) != total-1 {
		t.Fatalf("claimed %d, want %d", len(probe), total-1)
	}
	if err := tx.Rollback(); err != nil {
		t.Fatal(err)
	}
	// Make the claimed rows due again, then race two workers.
	w.exec(`UPDATE email_outbox SET next_attempt_at = $1`, base)

	var mu sync.Mutex
	claimedBy := map[uuid.UUID]int{}
	var wg sync.WaitGroup
	for worker := 0; worker < 2; worker++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				batch, err := repo.ClaimDue(ctx, base, 7, time.Hour)
				if err != nil {
					t.Error(err)
					return
				}
				if len(batch) == 0 {
					return
				}
				mu.Lock()
				for _, e := range batch {
					claimedBy[e.ID]++
				}
				mu.Unlock()
			}
		}()
	}
	wg.Wait()
	if len(claimedBy) != total {
		t.Fatalf("workers claimed %d distinct messages, want %d", len(claimedBy), total)
	}
	for id, n := range claimedBy {
		if n != 1 {
			t.Fatalf("message %s claimed %d times", id, n)
		}
	}
}

func TestOutboxWorkerWithFakeMailerOnPostgres(t *testing.T) {
	w := newWorld(t)
	ctx := context.Background()
	repo := NewOutboxRepository(w.db)
	clk := &clock.FakeClock{T: base}
	failing := &mailer.Recorder{Err: errors.New("smtp down")}
	worker := usecase.NewOutboxWorker(repo, failing, clk, nil, 10)
	if _, err := repo.Enqueue(ctx, port.EmailMessage{To: "a@example.test", Kind: "x", Subject: "s", Text: "t"}, clk.Now()); err != nil {
		t.Fatal(err)
	}
	for _, delay := range []time.Duration{time.Minute, 5 * time.Minute, 30 * time.Minute, 2 * time.Hour, 12 * time.Hour, 0} {
		if _, failed, err := worker.Run(ctx); err != nil || failed != 1 {
			t.Fatalf("run: failed=%d err=%v", failed, err)
		}
		clk.Advance(delay)
	}
	dead, total, _ := repo.List(ctx, "dead", 10, 0)
	if total != 1 || dead[0].Attempts != usecase.MaxEmailAttempts {
		t.Fatalf("dead = %+v", dead)
	}
	if sent, failed, _ := worker.Run(ctx); sent+failed != 0 {
		t.Fatal("dead message retried")
	}
}

func TestDeliveryRepository(t *testing.T) {
	w := newWorld(t)
	ctx := context.Background()
	repo := NewDeliveryRepository(w.db)

	if first, err := repo.Claim(ctx, "evt:1", w.reporter, base); err != nil || !first {
		t.Fatalf("first claim: %v %v", first, err)
	}
	if again, _ := repo.Claim(ctx, "evt:1", w.reporter, base); again {
		t.Fatal("claim repeated")
	}
	if other, _ := repo.Claim(ctx, "evt:1", w.assignee, base); !other {
		t.Fatal("claim is per user")
	}

	for i := 0; i < 3; i++ {
		item := port.DigestItem{UserID: w.reporter, Event: domain.EventIssueUpdated, IssueKey: "NTF-1", Summary: "s", ActorName: "Ana", Link: "/issues/x", CreatedAt: base}
		if err := repo.AddDigestItem(ctx, item); err != nil {
			t.Fatal(err)
		}
	}
	if err := repo.AddDigestItem(ctx, port.DigestItem{UserID: w.assignee, Event: domain.EventIssueCreated, CreatedAt: base}); err != nil {
		t.Fatal(err)
	}
	users, err := repo.UsersWithDigest(ctx, 10)
	if err != nil || len(users) != 2 || users[0] != w.reporter {
		t.Fatalf("users %v %v", users, err)
	}
	items, err := repo.DigestItems(ctx, w.reporter, 2)
	if err != nil || len(items) != 2 || items[0].IssueKey != "NTF-1" || items[0].Event != domain.EventIssueUpdated {
		t.Fatalf("items %+v %v", items, err)
	}
	ids := []int64{items[0].ID, items[1].ID}
	if err := repo.MarkDigested(ctx, ids, base); err != nil {
		t.Fatal(err)
	}
	rest, _ := repo.DigestItems(ctx, w.reporter, 10)
	if len(rest) != 1 {
		t.Fatalf("%d items left", len(rest))
	}
	sorted := append([]int64(nil), ids...)
	sort.Slice(sorted, func(a, b int) bool { return sorted[a] < sorted[b] })
	if rest[0].ID == sorted[0] || rest[0].ID == sorted[1] {
		t.Fatal("a digested item came back")
	}
	if err := repo.MarkDigested(ctx, nil, base); err != nil {
		t.Fatal(err)
	}
	removed, err := repo.PurgeDeliveries(ctx, base.Add(time.Hour))
	if err != nil || removed != 2 {
		t.Fatalf("purged %d (%v)", removed, err)
	}
}

func TestPreferencesPersistTheNewFields(t *testing.T) {
	w := newWorld(t)
	ctx := context.Background()
	repo := NewNotificationRepository(w.db)
	if p, err := repo.GetPreferences(ctx, w.reporter); err != nil || p != domain.DefaultPreferences() {
		t.Fatalf("defaults: %+v %v", p, err)
	}
	want := domain.Preferences{InApp: false, Email: domain.EmailDaily, NotifyOwnActions: true}
	if err := repo.SavePreferences(ctx, w.reporter, want); err != nil {
		t.Fatal(err)
	}
	if got, _ := repo.GetPreferences(ctx, w.reporter); got != want {
		t.Fatalf("got %+v, want %+v", got, want)
	}
	if err := repo.SavePreferences(ctx, w.reporter, domain.Preferences{InApp: true}); err != nil {
		t.Fatal(err)
	}
	if got, _ := repo.GetPreferences(ctx, w.reporter); got.Email != domain.EmailImmediate {
		t.Fatalf("empty mode stored as %q", got.Email)
	}
	if _, err := w.db.Exec(`UPDATE notification_preferences SET email_mode = 'weekly'`); err == nil {
		t.Fatal("the database accepted an unknown email mode")
	}
}

// failingOutbox fails after the notification was written, to prove both are one transaction.
type failingOutbox struct{ port.Outbox }

func (failingOutbox) Enqueue(context.Context, port.EmailMessage, time.Time) (bool, error) {
	return false, errors.New("outbox unavailable")
}

func newEngine(w *world, outbox port.Outbox) *usecase.Engine {
	return usecase.NewEngine(usecase.EngineDeps{
		Schemes: NewSchemeRepository(w.db), Facts: NewFactsRepository(w.db), Notes: NewNotificationRepository(w.db),
		Outbox: outbox, Delivery: NewDeliveryRepository(w.db), Render: notificationemail.NewRenderer(),
		Tx: database.NewTxManager(w.db), Clock: clock.NewFixed(base), BaseURL: "https://app.example.test",
	})
}

func TestEngineEndToEndOnPostgres(t *testing.T) {
	w := newWorld(t)
	ctx := context.Background()
	org := uuid.MustParse(defaultOrg)
	schemes := NewSchemeRepository(w.db)
	if _, err := schemes.Save(ctx, org, nil, domain.Scheme{Rules: []domain.Rule{{
		Event: domain.EventIssueTransitioned, Channels: []domain.Channel{domain.ChannelInApp, domain.ChannelEmail},
		Recipients: []domain.Recipient{{Type: domain.RecipientReporter}, {Type: domain.RecipientWatchers}, {Type: domain.RecipientComponentLead}, {Type: domain.RecipientProjectLead}},
	}}}, w.lead, base); err != nil {
		t.Fatal(err)
	}
	notes := NewNotificationRepository(w.db)
	if err := notes.SavePreferences(ctx, w.watcher, domain.Preferences{InApp: true, Email: domain.EmailDaily}); err != nil {
		t.Fatal(err)
	}
	engine := newEngine(w, NewOutboxRepository(w.db))
	consumer := usecase.NewSchemeConsumer(engine)
	actor := w.assignee
	event := events.Event{ID: 41, ProjectID: &w.project, ActorID: &actor, Type: "issue.transitioned", EntityType: "issue", Payload: map[string]any{
		"issue_id": w.issue.String(), "from_status": "Por hacer", "to_status": "En curso", "to_category": "in_progress",
	}}
	for i := 0; i < 2; i++ { // the second delivery is a redelivery
		if err := consumer.Handle(ctx, event); err != nil {
			t.Fatal(err)
		}
	}
	var inApp, mails, digest int
	_ = w.db.Get(&inApp, `SELECT count(*) FROM notifications`)
	_ = w.db.Get(&mails, `SELECT count(*) FROM email_outbox`)
	_ = w.db.Get(&digest, `SELECT count(*) FROM notification_digest_items`)
	// Reporter, watcher, component lead and project lead are told in-app; the actor (assignee) is not a recipient.
	if inApp != 4 || mails != 3 || digest != 1 {
		t.Fatalf("in-app=%d emails=%d digest=%d", inApp, mails, digest)
	}
	var subject, text string
	if err := w.db.QueryRow(`SELECT subject, body_text FROM email_outbox WHERE to_email = 'reporter@example.test'`).Scan(&subject, &text); err != nil {
		t.Fatal(err)
	}
	if subject != "[NTF-1] assignee cambio el estado de una incidencia: Resumen" {
		t.Fatalf("subject %q", subject)
	}
	if want := "https://app.example.test/browse/NTF-1"; !contains(text, want) || !contains(text, "Por hacer -> En curso") {
		t.Fatalf("body %q", text)
	}

	// The notification and its email are one transaction: when the outbox fails nothing is kept.
	failing := newEngine(w, failingOutbox{})
	err := usecase.NewSchemeConsumer(failing).Handle(ctx, events.Event{ID: 42, ProjectID: &w.project, ActorID: &actor, Type: "issue.transitioned", Payload: event.Payload})
	if err == nil {
		t.Fatal("the failure was swallowed")
	}
	var after int
	_ = w.db.Get(&after, `SELECT count(*) FROM notifications`)
	if after != inApp {
		t.Fatalf("a notification survived a failed transaction: %d -> %d", inApp, after)
	}
	var claims int
	_ = w.db.Get(&claims, `SELECT count(*) FROM notification_deliveries WHERE dedup_key = 'evt:42'`)
	if claims != 0 {
		t.Fatal("the dedupe claim survived the rollback, so the retry would be dropped")
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
