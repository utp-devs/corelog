package usecase

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/jhonsferg/corelog/internal/platform/apperror"
	"github.com/jhonsferg/corelog/internal/platform/clock"
	"github.com/jhonsferg/corelog/internal/platform/database"
	"github.com/jhonsferg/corelog/internal/platform/events"
	"github.com/jhonsferg/corelog/internal/webhook/application/port"
	"github.com/jhonsferg/corelog/internal/webhook/domain"
)

type fakeRepo struct {
	hooks      map[uuid.UUID]port.Stored
	deliveries []*domain.Delivery
	projects   map[uuid.UUID]uuid.UUID // project -> org
}

func newRepo() *fakeRepo {
	return &fakeRepo{hooks: map[uuid.UUID]port.Stored{}, projects: map[uuid.UUID]uuid.UUID{}}
}

func (r *fakeRepo) Create(_ context.Context, w *port.Stored) error {
	w.ID = uuid.New()
	r.hooks[w.ID] = *w
	return nil
}

func (r *fakeRepo) Get(_ context.Context, org, id uuid.UUID) (port.Stored, error) {
	if w, ok := r.hooks[id]; ok && w.OrgID == org {
		return w, nil
	}
	return port.Stored{}, domain.ErrNotFound
}

func (r *fakeRepo) StoredByID(_ context.Context, id uuid.UUID) (port.Stored, error) {
	if w, ok := r.hooks[id]; ok {
		return w, nil
	}
	return port.Stored{}, domain.ErrNotFound
}

func (r *fakeRepo) List(_ context.Context, org uuid.UUID, project *uuid.UUID) ([]domain.Webhook, error) {
	var out []domain.Webhook
	for _, w := range r.hooks {
		if w.OrgID == org && (project == nil || (w.ProjectID != nil && *w.ProjectID == *project)) {
			out = append(out, w.Webhook)
		}
	}
	return out, nil
}

func (r *fakeRepo) Update(_ context.Context, w domain.Webhook) error {
	st := r.hooks[w.ID]
	st.Webhook = w
	r.hooks[w.ID] = st
	return nil
}

func (r *fakeRepo) UpdateSecret(_ context.Context, _, id uuid.UUID, sealed []byte, _ time.Time) error {
	st := r.hooks[id]
	st.SecretEncrypted = sealed
	r.hooks[id] = st
	return nil
}

func (r *fakeRepo) Delete(_ context.Context, _, id uuid.UUID) error {
	delete(r.hooks, id)
	return nil
}

func (r *fakeRepo) ProjectInOrg(_ context.Context, org, project uuid.UUID) (bool, error) {
	return r.projects[project] == org, nil
}

func (r *fakeRepo) Matching(_ context.Context, org uuid.UUID, project *uuid.UUID, eventType string) ([]domain.Webhook, error) {
	var out []domain.Webhook
	for _, w := range r.hooks {
		if w.OrgID != org || !w.Enabled || !w.Subscribes(eventType) {
			continue
		}
		if w.ProjectID != nil && (project == nil || *w.ProjectID != *project) {
			continue
		}
		out = append(out, w.Webhook)
	}
	return out, nil
}

func (r *fakeRepo) InsertDelivery(_ context.Context, d *domain.Delivery) (bool, error) {
	if d.EventID != nil {
		for _, e := range r.deliveries {
			if e.WebhookID == d.WebhookID && e.EventID != nil && *e.EventID == *d.EventID {
				return false, nil
			}
		}
	}
	d.ID = uuid.New()
	cp := *d
	r.deliveries = append(r.deliveries, &cp)
	return true, nil
}

func (r *fakeRepo) ListDeliveries(_ context.Context, id uuid.UUID, limit int) ([]domain.Delivery, error) {
	var out []domain.Delivery
	for _, d := range r.deliveries {
		if d.WebhookID == id && len(out) < limit {
			out = append(out, *d)
		}
	}
	return out, nil
}

func (r *fakeRepo) GetDelivery(_ context.Context, id, did uuid.UUID) (domain.Delivery, error) {
	for _, d := range r.deliveries {
		if d.ID == did && d.WebhookID == id {
			return *d, nil
		}
	}
	return domain.Delivery{}, domain.ErrDeliveryNotFound
}

func (r *fakeRepo) ClaimDue(_ context.Context, now time.Time, limit int, lease time.Duration) ([]port.DueDelivery, error) {
	var out []port.DueDelivery
	for _, d := range r.deliveries {
		st := r.hooks[d.WebhookID]
		if (d.Status == domain.StatusPending || d.Status == domain.StatusFailed) && d.NextAttemptAt != nil && !d.NextAttemptAt.After(now) && st.Enabled && len(out) < limit {
			next := now.Add(lease)
			d.NextAttemptAt = &next
			out = append(out, port.DueDelivery{Delivery: *d, URL: st.URL, SecretEncrypted: st.SecretEncrypted})
		}
	}
	return out, nil
}

func (r *fakeRepo) RecordResult(_ context.Context, id uuid.UUID, res port.Result) error {
	for _, d := range r.deliveries {
		if d.ID == id {
			d.Status, d.Attempts, d.NextAttemptAt, d.ResponseStatus, d.ResponseExcerpt, d.DurationMS =
				res.Status, res.Attempts, res.NextAttemptAt, res.ResponseStatus, res.Excerpt, res.DurationMS
		}
	}
	return nil
}

type fakeAccess struct{ admins map[uuid.UUID]bool }

func (a fakeAccess) RequireAdmin(_ context.Context, user, _ uuid.UUID) error {
	if a.admins[user] {
		return nil
	}
	return apperror.New(apperror.KindForbidden, "forbidden")
}

type fakeBox struct{}

func (fakeBox) Seal(p string) ([]byte, error) { return []byte("sealed:" + p), nil }
func (fakeBox) Open(b []byte) (string, error) {
	if !strings.HasPrefix(string(b), "sealed:") {
		return "", errors.New("corrupt")
	}
	return strings.TrimPrefix(string(b), "sealed:"), nil
}

type fakeSender struct {
	status   int
	err      error
	requests []port.SendRequest
}

func (s *fakeSender) Send(_ context.Context, req port.SendRequest) port.SendResult {
	s.requests = append(s.requests, req)
	return port.SendResult{Status: s.status, Body: []byte("body"), Err: s.err, Duration: 5 * time.Millisecond}
}

type fakeDescriber struct {
	info port.EventInfo
	ok   bool
}

func (d fakeDescriber) Describe(context.Context, events.Event) (port.EventInfo, bool, error) {
	return d.info, d.ok, nil
}

type fakeMatcher struct {
	match bool
	err   error
}

func (m fakeMatcher) Matches(context.Context, uuid.UUID, uuid.UUID, string) (bool, error) {
	return m.match, m.err
}

func (m fakeMatcher) Validate(_ context.Context, q string) error {
	if strings.Contains(q, "???") {
		return errors.New("bad")
	}
	return nil
}

type recorder struct{ events []events.Event }

func (r *recorder) Publish(_ context.Context, e events.Event) error {
	r.events = append(r.events, e)
	return nil
}

type env struct {
	svc    *Service
	repo   *fakeRepo
	sender *fakeSender
	clk    *clock.FakeClock
	pub    *recorder
	org    uuid.UUID
	admin  uuid.UUID
}

func newEnv(t *testing.T) *env {
	t.Helper()
	e := &env{repo: newRepo(), sender: &fakeSender{status: 200}, pub: &recorder{}, org: uuid.New(), admin: uuid.New(),
		clk: &clock.FakeClock{T: time.Date(2026, 5, 1, 9, 0, 0, 0, time.UTC)}}
	project := uuid.New()
	e.repo.projects[project] = e.org
	e.svc = NewService(Deps{
		Repo: e.repo, Access: fakeAccess{admins: map[uuid.UUID]bool{e.admin: true}}, Box: fakeBox{}, Sender: e.sender,
		Describer: fakeDescriber{ok: true, info: port.EventInfo{OrgID: e.org, ActorName: "Ana",
			Project: &port.ProjectInfo{ID: project, Key: "CORE", Name: "Core"},
			Issue:   &port.IssueInfo{ID: uuid.New(), Key: "CORE-1", Summary: "Resumen", Status: "Open", Priority: "High", IssueType: "Task", ReporterID: uuid.New()}}},
		Matcher: fakeMatcher{match: true}, Tx: &database.FakeTxManager{}, Publisher: e.pub, Clock: e.clk,
	})
	return e
}

func (e *env) create(t *testing.T, in port.CreateInput) (domain.Webhook, string) {
	t.Helper()
	if in.Name == "" {
		in.Name = "Hook"
	}
	if in.URL == "" {
		in.URL = "https://example.com/hook"
	}
	if in.Events == nil {
		in.Events = []string{"issue.created"}
	}
	w, secret, err := e.svc.Create(context.Background(), e.admin, e.org, in)
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	return w, secret
}

func TestCreateAndPermissions(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	w, secret := e.create(t, port.CreateInput{})
	if !strings.HasPrefix(secret, "whsec_") || w.ID == uuid.Nil || !w.Enabled || w.OrgID != e.org {
		t.Fatalf("webhook %+v secret %q", w, secret)
	}
	if got := e.repo.hooks[w.ID].SecretEncrypted; string(got) == secret || !strings.Contains(string(got), secret) {
		t.Fatalf("secret must go through the box: %q", got)
	}
	if len(e.pub.events) != 1 || e.pub.events[0].Type != "webhook.created" {
		t.Fatalf("audit events %v", e.pub.events)
	}
	for _, v := range e.pub.events[0].Payload {
		if s, _ := v.(string); strings.Contains(s, secret) {
			t.Fatal("audit event leaks the secret")
		}
	}

	stranger := uuid.New()
	if _, _, err := e.svc.Create(ctx, stranger, e.org, port.CreateInput{Name: "x", URL: "https://example.com", Events: []string{"*"}}); apperror.KindOf(err) != apperror.KindForbidden {
		t.Fatalf("non admin create: %v", err)
	}
	if _, err := e.svc.List(ctx, stranger, e.org, nil); apperror.KindOf(err) != apperror.KindForbidden {
		t.Fatalf("non admin list: %v", err)
	}
	if _, err := e.svc.Get(ctx, stranger, e.org, w.ID); err == nil {
		t.Fatal("non admin get")
	}
	if _, err := e.svc.Update(ctx, stranger, e.org, w.ID, port.UpdateInput{}); err == nil {
		t.Fatal("non admin update")
	}
	if err := e.svc.Delete(ctx, stranger, e.org, w.ID); err == nil {
		t.Fatal("non admin delete")
	}
	if _, err := e.svc.RotateSecret(ctx, stranger, e.org, w.ID); err == nil {
		t.Fatal("non admin rotate")
	}
	if _, err := e.svc.Test(ctx, stranger, e.org, w.ID); err == nil {
		t.Fatal("non admin test")
	}
	if _, err := e.svc.Deliveries(ctx, stranger, e.org, w.ID, 10); err == nil {
		t.Fatal("non admin deliveries")
	}
	if _, err := e.svc.Redeliver(ctx, stranger, e.org, w.ID, uuid.New()); err == nil {
		t.Fatal("non admin redeliver")
	}
	// Cross-organization access is not found.
	if _, err := e.svc.Get(ctx, e.admin, uuid.New(), w.ID); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("other org get: %v", err)
	}
}

func TestCreateValidation(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	foreign := uuid.New()
	bad := map[string]port.CreateInput{
		"no name":         {URL: "https://example.com", Events: []string{"*"}},
		"private url":     {Name: "x", URL: "http://10.0.0.1", Events: []string{"*"}},
		"no events":       {Name: "x", URL: "https://example.com"},
		"bad event":       {Name: "x", URL: "https://example.com", Events: []string{"x.y"}},
		"foreign project": {Name: "x", URL: "https://example.com", Events: []string{"*"}, ProjectID: &foreign},
	}
	badQuery := "status = ???"
	bad["bad filter"] = port.CreateInput{Name: "x", URL: "https://example.com", Events: []string{"*"}, FilterQuery: &badQuery}
	for name, in := range bad {
		if _, _, err := e.svc.Create(ctx, e.admin, e.org, in); err == nil {
			t.Errorf("%s accepted", name)
		}
	}
	disabled := false
	w, _, err := e.svc.Create(ctx, e.admin, e.org, port.CreateInput{Name: "x", URL: "https://example.com", Events: []string{"*"}, Enabled: &disabled})
	if err != nil || w.Enabled {
		t.Fatalf("disabled create: %+v %v", w, err)
	}
}

func TestUpdateRotateDelete(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	w, secret := e.create(t, port.CreateInput{})
	name, off, filter := "Renamed", false, "status = Open"
	got, err := e.svc.Update(ctx, e.admin, e.org, w.ID, port.UpdateInput{Name: &name, Enabled: &off, FilterQuery: &filter, Events: []string{"*"}})
	if err != nil || got.Name != "Renamed" || got.Enabled || got.FilterQuery == nil || len(got.Events) != 1 {
		t.Fatalf("update: %+v %v", got, err)
	}
	empty := ""
	if got, _ = e.svc.Update(ctx, e.admin, e.org, w.ID, port.UpdateInput{FilterQuery: &empty}); got.FilterQuery != nil {
		t.Fatal("an empty filter must clear it")
	}
	badURL := "ftp://x"
	if _, err := e.svc.Update(ctx, e.admin, e.org, w.ID, port.UpdateInput{URL: &badURL}); err == nil {
		t.Fatal("bad url accepted")
	}
	rotated, err := e.svc.RotateSecret(ctx, e.admin, e.org, w.ID)
	if err != nil || rotated == secret || !strings.HasPrefix(rotated, "whsec_") {
		t.Fatalf("rotate: %q %v", rotated, err)
	}
	if err := e.svc.Delete(ctx, e.admin, e.org, w.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := e.svc.Get(ctx, e.admin, e.org, w.ID); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("get after delete: %v", err)
	}
	if err := e.svc.Delete(ctx, e.admin, e.org, w.ID); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("second delete: %v", err)
	}
	list, _ := e.svc.List(ctx, e.admin, e.org, nil)
	if len(list) != 0 {
		t.Fatal("list not empty")
	}
}

func event(id int64) events.Event {
	actor := uuid.New()
	return events.Event{ID: id, Type: "issue.created", EntityType: "issue", ActorID: &actor, CreatedAt: time.Date(2026, 5, 1, 8, 0, 0, 0, time.UTC),
		Payload: map[string]any{}}
}

func TestCaptureBuildsPayloadAndIsIdempotent(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	w, _ := e.create(t, port.CreateInput{})
	other, _ := e.create(t, port.CreateInput{Events: []string{"issue.updated"}})
	if err := e.svc.Capture(ctx, event(42)); err != nil {
		t.Fatal(err)
	}
	if err := e.svc.Capture(ctx, event(42)); err != nil { // at-least-once dispatch
		t.Fatal(err)
	}
	if len(e.repo.deliveries) != 1 || e.repo.deliveries[0].WebhookID != w.ID || e.repo.deliveries[0].WebhookID == other.ID {
		t.Fatalf("deliveries: %d", len(e.repo.deliveries))
	}
	var payload map[string]any
	if err := json.Unmarshal(e.repo.deliveries[0].Payload, &payload); err != nil {
		t.Fatal(err)
	}
	if payload["version"] != "v1" || payload["id"] != "evt_42" || payload["event"] != "issue.created" || payload["organization_id"] != e.org.String() {
		t.Fatalf("payload %v", payload)
	}
	if issue := payload["issue"].(map[string]any); issue["key"] != "CORE-1" || issue["assignee_id"] != nil {
		t.Fatalf("issue %v", issue)
	}
	if project := payload["project"].(map[string]any); project["key"] != "CORE" {
		t.Fatalf("project %v", project)
	}
	if payload["actor"].(map[string]any)["name"] != "Ana" || payload["occurred_at"] != "2026-05-01T08:00:00Z" {
		t.Fatalf("actor/time %v", payload)
	}
	if _, has := payload["changes"]; has {
		t.Fatal("creation has no changes")
	}
	// Unsupported event types and unresolvable events are ignored.
	if err := e.svc.Capture(ctx, events.Event{ID: 1, Type: "user.login", EntityType: "user"}); err != nil || len(e.repo.deliveries) != 1 {
		t.Fatal("unsupported event captured")
	}
	e.svc.Describer = fakeDescriber{ok: false}
	if err := e.svc.Capture(ctx, event(43)); err != nil || len(e.repo.deliveries) != 1 {
		t.Fatal("unresolvable event captured")
	}
}

func TestCaptureChangesAndFilters(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	filter := "priority = High"
	e.create(t, port.CreateInput{Events: []string{"*"}, FilterQuery: &filter})
	e.svc.Matcher = fakeMatcher{match: false}
	if err := e.svc.Capture(ctx, event(1)); err != nil || len(e.repo.deliveries) != 0 {
		t.Fatalf("filtered event delivered: %d", len(e.repo.deliveries))
	}
	e.svc.Matcher = fakeMatcher{err: errors.New("boom")}
	if err := e.svc.Capture(ctx, event(2)); err != nil || len(e.repo.deliveries) != 0 {
		t.Fatal("a failing filter must skip the webhook without failing the consumer")
	}
	e.svc.Matcher = fakeMatcher{match: true}
	ev := event(3)
	ev.Type = "issue.transitioned"
	ev.Payload = map[string]any{"from_status": "Open", "to_status": "Done"}
	if err := e.svc.Capture(ctx, ev); err != nil || len(e.repo.deliveries) != 1 {
		t.Fatalf("transition capture: %v", err)
	}
	var p map[string]any
	_ = json.Unmarshal(e.repo.deliveries[0].Payload, &p)
	changes := p["changes"].([]any)[0].(map[string]any)
	if changes["field"] != "status" || changes["from"] != "Open" || changes["to"] != "Done" {
		t.Fatalf("changes %v", changes)
	}
	upd := event(4)
	upd.Type = "issue.updated"
	upd.Payload = map[string]any{"changes": []any{map[string]any{"field": "summary", "from": "a", "to": "b"}}}
	_ = e.svc.Capture(ctx, upd)
	_ = json.Unmarshal(e.repo.deliveries[1].Payload, &p)
	if p["changes"].([]any)[0].(map[string]any)["field"] != "summary" {
		t.Fatalf("update changes %v", p["changes"])
	}
	if got := e.svc.Consumer().Name(); got != "webhooks" {
		t.Fatalf("consumer name %q", got)
	}
	if err := e.svc.Consumer().Handle(ctx, event(5)); err != nil {
		t.Fatal(err)
	}
}

func TestProjectScopedWebhookOnlyGetsItsProject(t *testing.T) {
	e := newEnv(t)
	otherProject := uuid.New()
	e.repo.projects[otherProject] = e.org
	e.create(t, port.CreateInput{Events: []string{"*"}, ProjectID: &otherProject})
	if err := e.svc.Capture(context.Background(), event(1)); err != nil || len(e.repo.deliveries) != 0 {
		t.Fatalf("event of another project delivered: %d", len(e.repo.deliveries))
	}
}

func TestDeliveryRetriesWithBackoffThenDies(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	w, secret := e.create(t, port.CreateInput{})
	_ = e.svc.Capture(ctx, event(7))
	e.sender.status = 500

	waits := []time.Duration{time.Minute, 5 * time.Minute, 30 * time.Minute, 2 * time.Hour, 12 * time.Hour}
	for attempt := 1; attempt <= 6; attempt++ {
		n, err := e.svc.RunDue(ctx)
		if err != nil || n != 1 {
			t.Fatalf("attempt %d: ran %d, %v", attempt, n, err)
		}
		d := e.repo.deliveries[0]
		if d.Attempts != attempt || d.ResponseStatus == nil || *d.ResponseStatus != 500 || d.ResponseExcerpt != "body" {
			t.Fatalf("attempt %d: %+v", attempt, d)
		}
		if attempt < 6 {
			if d.Status != domain.StatusFailed || d.NextAttemptAt == nil || !d.NextAttemptAt.Equal(e.clk.T.Add(waits[attempt-1])) {
				t.Fatalf("attempt %d: status %s next %v", attempt, d.Status, d.NextAttemptAt)
			}
			// Not due yet: nothing runs until the clock reaches the retry time.
			if n, _ := e.svc.RunDue(ctx); n != 0 {
				t.Fatalf("attempt %d: ran before it was due", attempt)
			}
			e.clk.Advance(waits[attempt-1])
		}
	}
	d := e.repo.deliveries[0]
	if d.Status != domain.StatusDead || d.NextAttemptAt != nil || d.Attempts != 6 {
		t.Fatalf("final: %+v", d)
	}
	if n, _ := e.svc.RunDue(ctx); n != 0 {
		t.Fatal("a dead delivery must not run again")
	}

	// Every request was signed with the secret over "timestamp.body".
	req := e.sender.requests[0]
	ts, _ := time_(req.Headers[domain.HeaderTimestamp])
	if !domain.Verify(secret, ts, req.Body, req.Headers[domain.HeaderSignature]) {
		t.Fatalf("bad signature header: %v", req.Headers)
	}
	if req.Headers[domain.HeaderEvent] != "issue.created" || req.Headers[domain.HeaderDelivery] != e.repo.deliveries[0].ID.String() || req.URL != "https://example.com/hook" {
		t.Fatalf("headers %v", req.Headers)
	}
	_ = w
}

func time_(s string) (int64, error) {
	var n int64
	for _, c := range s {
		n = n*10 + int64(c-'0')
	}
	return n, nil
}

func TestDeliverySuccessAndNetworkError(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	e.create(t, port.CreateInput{})
	_ = e.svc.Capture(ctx, event(1))
	if n, err := e.svc.RunDue(ctx); err != nil || n != 1 {
		t.Fatal(n, err)
	}
	if d := e.repo.deliveries[0]; d.Status != domain.StatusSuccess || d.NextAttemptAt != nil || d.DurationMS != 5 {
		t.Fatalf("success: %+v", d)
	}
	_ = e.svc.Capture(ctx, event(2))
	e.sender.err = errors.New("destino bloqueado: direccion no permitida")
	if _, err := e.svc.RunDue(ctx); err != nil {
		t.Fatal(err)
	}
	d := e.repo.deliveries[1]
	if d.Status != domain.StatusFailed || d.ResponseStatus != nil || !strings.Contains(d.ResponseExcerpt, "bloqueado") {
		t.Fatalf("network error: %+v", d)
	}
	// A redirect (3xx) is a failed delivery.
	e.sender.err, e.sender.status = nil, http.StatusFound
	e.clk.Advance(time.Hour)
	_, _ = e.svc.RunDue(ctx)
	if d := e.repo.deliveries[1]; d.Status != domain.StatusFailed || d.Attempts != 2 {
		t.Fatalf("redirect: %+v", d)
	}
}

func TestDisabledWebhookDoesNotDeliver(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	w, _ := e.create(t, port.CreateInput{})
	_ = e.svc.Capture(ctx, event(1))
	off := false
	_, _ = e.svc.Update(ctx, e.admin, e.org, w.ID, port.UpdateInput{Enabled: &off})
	if n, _ := e.svc.RunDue(ctx); n != 0 || len(e.sender.requests) != 0 {
		t.Fatal("disabled webhook delivered")
	}
}

func TestPingRedeliverAndDeliveries(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	w, _ := e.create(t, port.CreateInput{})
	d, err := e.svc.Test(ctx, e.admin, e.org, w.ID)
	if err != nil || d.Status != domain.StatusSuccess || d.EventType != "ping" {
		t.Fatalf("ping: %+v %v", d, err)
	}
	var p map[string]any
	_ = json.Unmarshal(d.Payload, &p)
	if p["event"] != "ping" || p["version"] != "v1" {
		t.Fatalf("ping payload %v", p)
	}
	// A failing ping is final: no retry is scheduled.
	e.sender.status = 503
	d, err = e.svc.Test(ctx, e.admin, e.org, w.ID)
	if err != nil || d.Status != domain.StatusFailed || d.NextAttemptAt != nil {
		t.Fatalf("failing ping: %+v %v", d, err)
	}
	if _, err := e.svc.Test(ctx, e.admin, e.org, uuid.New()); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("ping unknown: %v", err)
	}

	list, err := e.svc.Deliveries(ctx, e.admin, e.org, w.ID, 0)
	if err != nil || len(list) != 2 {
		t.Fatalf("deliveries %d %v", len(list), err)
	}
	again, err := e.svc.Redeliver(ctx, e.admin, e.org, w.ID, d.ID)
	if err != nil || again.Status != domain.StatusPending || again.ID == d.ID || string(again.Payload) != string(d.Payload) {
		t.Fatalf("redeliver: %+v %v", again, err)
	}
	if _, err := e.svc.Redeliver(ctx, e.admin, e.org, w.ID, uuid.New()); !errors.Is(err, domain.ErrDeliveryNotFound) {
		t.Fatalf("redeliver unknown: %v", err)
	}
	e.sender.status = 200
	if n, _ := e.svc.RunDue(ctx); n != 1 {
		t.Fatalf("queued redelivery not sent: %d", n)
	}
}

func TestCorruptSecretKillsTheDelivery(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	w, _ := e.create(t, port.CreateInput{})
	st := e.repo.hooks[w.ID]
	st.SecretEncrypted = []byte("garbage")
	e.repo.hooks[w.ID] = st
	_ = e.svc.Capture(ctx, event(1))
	_, _ = e.svc.RunDue(ctx)
	if d := e.repo.deliveries[0]; d.Status != domain.StatusDead || len(e.sender.requests) != 0 {
		t.Fatalf("corrupt secret: %+v", d)
	}
}
