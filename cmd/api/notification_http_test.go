package main

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/jhonsferg/corelog/internal/notification/adapter/out/email"
	notificationpostgres "github.com/jhonsferg/corelog/internal/notification/adapter/out/postgres"
	notificationusecase "github.com/jhonsferg/corelog/internal/notification/application/usecase"
	"github.com/jhonsferg/corelog/internal/platform/authtoken"
	"github.com/jhonsferg/corelog/internal/platform/clock"
	"github.com/jhonsferg/corelog/internal/platform/config"
	"github.com/jhonsferg/corelog/internal/platform/events"
	"github.com/jhonsferg/corelog/internal/platform/jobs"
	"github.com/jhonsferg/corelog/internal/platform/mailer"
)

func (e *transitionEnv) token(role string) (string, uuid.UUID) {
	e.t.Helper()
	id := uuid.New()
	e.exec(`INSERT INTO users (id, name, display_name, email, password_hash, role) VALUES ($1, $2, $2, $3, 'x', $4)`, id, "u"+id.String()[:6], id.String()[:6]+"@example.test", role)
	signed, err := authtoken.Generate(transitionTestSecret, time.Hour, id, role)
	if err != nil {
		e.t.Fatal(err)
	}
	return signed, id
}

func TestNotificationSchemeEndpointsThroughTheRealRouter(t *testing.T) {
	env := newTransitionEnv(t, "scrum")
	project := "/projects/" + env.project.String() + "/notification-scheme"
	org := "/organizations/00000000-0000-4000-8000-000000000002/notification-scheme"
	body := map[string]any{"rules": []any{map[string]any{
		"event": "issue_created", "channels": []string{"in_app", "email"},
		"recipients": []any{map[string]any{"type": "project_lead"}, map[string]any{"type": "watchers"}},
	}}}

	// edit_project is held by the project lead only.
	for name, c := range map[string]struct {
		token string
		want  int
	}{"lead": {env.lead, 200}, "contributor": {env.contrib, 403}, "viewer": {env.viewer, 403}, "outsider": {env.outside, 403}, "anonymous": {"", 401}} {
		if status, _ := env.client.do(http.MethodGet, project, c.token, nil); status != c.want {
			t.Errorf("GET %s as %s: %d, want %d", project, name, status, c.want)
		}
		if status, _ := env.client.do(http.MethodPut, project, c.token, body); status != c.want {
			t.Errorf("PUT as %s: %d, want %d", name, status, c.want)
		}
	}
	status, saved := env.client.do(http.MethodGet, project, env.lead, nil)
	data := tfData(t, saved)
	if status != 200 || data["is_default"] != false || data["scope"] != "project" {
		t.Fatalf("saved scheme: %d %v", status, data)
	}
	if status, _ := env.client.do(http.MethodDelete, project, env.lead, nil); status != 200 {
		t.Fatalf("reset: %d", status)
	}
	if status, again := env.client.do(http.MethodGet, project, env.lead, nil); status != 200 || tfData(t, again)["is_default"] != true {
		t.Fatalf("after reset: %d %v", status, again)
	}
	if status, _ := env.client.do(http.MethodPut, project, env.lead, map[string]any{"rules": []any{map[string]any{"event": "nope"}}}); status != 422 {
		t.Fatalf("invalid event: %d", status)
	}
	if status, _ := env.client.do(http.MethodGet, "/projects/"+uuid.NewString()+"/notification-scheme", env.lead, nil); status != 404 && status != 403 {
		t.Fatalf("unknown project: %d", status)
	}

	// The organization default: members read, only admins write.
	if status, _ := env.client.do(http.MethodGet, org, env.lead, nil); status != 200 {
		t.Fatalf("member read: %d", status)
	}
	if status, _ := env.client.do(http.MethodPut, org, env.lead, body); status != 403 {
		t.Fatalf("member write: %d", status)
	}
	if status, _ := env.client.do(http.MethodGet, org, env.outside, nil); status != 404 {
		t.Fatalf("outsider read: %d", status)
	}
	adminToken, adminID := env.token("user")
	env.exec(`INSERT INTO organization_members (organization_id, user_id, role) VALUES ('00000000-0000-4000-8000-000000000002', $1, 'admin')`, adminID)
	if status, _ := env.client.do(http.MethodPut, org, adminToken, body); status != 200 {
		t.Fatalf("org admin write: %d", status)
	}
	// The project inherits the organization rule until it has its own.
	status, inherited := env.client.do(http.MethodGet, project, env.lead, nil)
	rules := tfData(t, inherited)["rules"].([]any)
	first := rules[0].(map[string]any)
	if status != 200 || first["event"] != "issue_created" || first["overridden"] != false || len(first["recipients"].([]any)) != 2 {
		t.Fatalf("inherited rule: %d %v", status, first)
	}

	// Platform administration of the email queue and the jobs.
	platform, _ := env.token("admin")
	for _, path := range []string{"/admin/email-outbox", "/admin/jobs"} {
		if status, _ := env.client.do(http.MethodGet, path, env.lead, nil); status != 403 {
			t.Errorf("GET %s as member: %d", path, status)
		}
		if status, _ := env.client.do(http.MethodGet, path, platform, nil); status != 200 {
			t.Errorf("GET %s as admin: %d", path, status)
		}
	}
	if status, _ := env.client.do(http.MethodPost, "/admin/email-outbox/"+uuid.NewString()+"/retry", platform, nil); status != 409 {
		t.Errorf("retry of an unknown message: %d", status)
	}
}

// TestIssueCreationNotifiesBySchemeAndSendsEmail walks the whole path: an issue is created through the API, the
// scheme consumer reads the event, the notification and the email are stored together, and the outbox worker sends it.
func TestIssueCreationNotifiesBySchemeAndSendsEmail(t *testing.T) {
	env := newTransitionEnv(t, "scrum")
	ctx := context.Background()
	cfg := &config.Config{Auth: config.AuthConfig{JWTSecret: transitionTestSecret, JWTExpiration: time.Hour}}
	cfg.Security.AppPublicURL = "https://corelog.example.test"
	status, _ := env.client.do(http.MethodPut, "/projects/"+env.project.String()+"/notification-scheme", env.lead, map[string]any{"rules": []any{map[string]any{
		"event": "issue_created", "channels": []string{"in_app", "email"}, "recipients": []any{map[string]any{"type": "project_lead"}},
	}}})
	if status != 200 {
		t.Fatalf("save scheme: %d", status)
	}
	env.createIssue(env.contrib)

	dispatcher, err := events.NewDispatcher(env.db, []events.Consumer{newSchemeConsumer(cfg, env.db)}, events.Options{Logger: slog.New(slog.NewTextHandler(io.Discard, nil))})
	if err != nil {
		t.Fatal(err)
	}
	if err := dispatcher.RunOnce(ctx); err != nil {
		t.Fatalf("dispatch: %v", err)
	}
	if err := dispatcher.RunOnce(ctx); err != nil {
		t.Fatalf("second dispatch: %v", err)
	}
	var notified, contributorNotices int
	_ = env.db.Get(&notified, `SELECT count(*) FROM notifications n JOIN users u ON u.id = n.user_id WHERE u.name = 'lead'`)
	_ = env.db.Get(&contributorNotices, `SELECT count(*) FROM notifications n JOIN users u ON u.id = n.user_id WHERE u.name = 'contributor'`)
	if notified != 1 || contributorNotices != 0 {
		t.Fatalf("lead notices %d, contributor (the actor) notices %d", notified, contributorNotices)
	}

	recorder := &mailer.Recorder{}
	worker := notificationusecase.NewOutboxWorker(notificationpostgres.NewOutboxRepository(env.db), recorder, clock.RealClock{}, nil, 10)
	sent, failed, err := worker.Run(ctx)
	if err != nil || sent != 1 || failed != 0 || len(recorder.Messages) != 1 {
		t.Fatalf("sent=%d failed=%d err=%v", sent, failed, err)
	}
	msg := recorder.Messages[0]
	if msg.To != "lead@example.test" || !strings.HasPrefix(msg.Subject, "[TRN-1] ") || !strings.Contains(msg.Text, "https://corelog.example.test/browse/TRN-1") || !strings.Contains(msg.HTML, "profile/notifications") {
		t.Fatalf("message %+v", msg)
	}
}

func TestFilterSubscriptionJobEmailsTheResults(t *testing.T) {
	env := newTransitionEnv(t, "scrum")
	ctx := context.Background()
	env.createIssue(env.contrib)
	var contributor uuid.UUID
	if err := env.db.Get(&contributor, `SELECT id FROM users WHERE name = 'contributor'`); err != nil {
		t.Fatal(err)
	}
	filter := uuid.New()
	env.exec(`INSERT INTO filters (id, owner_id, organization_id, name, query_text) VALUES ($1, $2, '00000000-0000-4000-8000-000000000002', 'Mis incidencias', 'project = TRN')`, filter, contributor)
	env.exec(`INSERT INTO filter_subscriptions (filter_id, user_id, schedule, created_at) VALUES ($1, $2, '{"frequency":"daily","hour":0,"timezone":"UTC"}', now() - interval '3 days')`, filter, contributor)

	clk := clock.RealClock{}
	outbox := notificationpostgres.NewOutboxRepository(env.db)
	job := notificationusecase.NewFilterSubscriptionJob(newFilterSubscriptions(env.db), notificationpostgres.NewFactsRepository(env.db),
		notificationpostgres.NewNotificationRepository(env.db), outbox, email.NewRenderer(), clk, "https://corelog.example.test")
	queued, err := job.Run(ctx)
	if err != nil || queued != 1 {
		t.Fatalf("queued=%d err=%v", queued, err)
	}
	var subject, text string
	if err := env.db.QueryRow(`SELECT subject, body_text FROM email_outbox WHERE to_email LIKE '%@example.test'`).Scan(&subject, &text); err != nil {
		t.Fatal(err)
	}
	if subject != `Filtro "Mis incidencias": 1 incidencia` || !strings.Contains(text, "TRN-1") || !strings.Contains(text, "https://corelog.example.test/browse/TRN-1") {
		t.Fatalf("subject %q text %q", subject, text)
	}
	var sentAt *time.Time
	if err := env.db.Get(&sentAt, `SELECT last_sent_at FROM filter_subscriptions WHERE filter_id = $1`, filter); err != nil || sentAt == nil {
		t.Fatalf("the delivery was not recorded: %v %v", sentAt, err)
	}
	if again, err := job.Run(ctx); err != nil || again != 0 {
		t.Fatalf("second run queued %d (%v): the subscription must not be due again", again, err)
	}

	// A subscriber who turned email off is skipped but the subscription stops being due.
	env.exec(`UPDATE filter_subscriptions SET last_sent_at = NULL`)
	if _, err := env.db.Exec(`INSERT INTO notification_preferences (user_id, in_app, email_mode) VALUES ($1, true, 'off')`, contributor); err != nil {
		t.Fatal(err)
	}
	if queued, err := job.Run(ctx); err != nil || queued != 0 {
		t.Fatalf("email off: queued=%d err=%v", queued, err)
	}
	var due int
	_ = env.db.Get(&due, `SELECT count(*) FROM filter_subscriptions WHERE last_sent_at IS NULL`)
	if due != 0 {
		t.Fatal("the skipped subscription is still due")
	}
}

func TestRegisterNotificationJobs(t *testing.T) {
	env := newTransitionEnv(t, "kanban")
	cfg := &config.Config{}
	t.Setenv("PURGE_ENABLED", "true")
	scheduler := jobs.NewScheduler(nil, nil)
	registerNotificationJobs(scheduler, jobDeps{Cfg: cfg, DB: env.db, Log: slog.New(slog.NewTextHandler(io.Discard, nil))})
	registerSecurityJobs(scheduler, jobDeps{Cfg: cfg, DB: env.db, Log: slog.New(slog.NewTextHandler(io.Discard, nil))})
	names := map[string]string{}
	for _, job := range scheduler.Jobs() {
		names[job.Name] = job.Schedule
	}
	for name, schedule := range map[string]string{
		"email-outbox": "every 10s", "email-digest": "daily at 08:00 UTC", "filter-subscriptions": "every 1m",
		"notification-cleanup": "daily at 03:30 UTC", "soft-delete-purge": "daily at 04:00 UTC",
		"audit-retention": "every 24h", "security-cleanup": "every 1h",
	} {
		if names[name] != schedule {
			t.Errorf("job %s has schedule %q, want %q", name, names[name], schedule)
		}
	}
}
