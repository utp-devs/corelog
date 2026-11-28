package stream

import (
	"bufio"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/jhonsferg/corelog/internal/notification/domain"
	"github.com/jhonsferg/corelog/internal/platform/apperror"
	"github.com/jhonsferg/corelog/internal/platform/authtoken"
	appmw "github.com/jhonsferg/corelog/internal/platform/middleware"
	"github.com/jhonsferg/corelog/internal/platform/sse"
)

const secret = "stream-test-secret"

// fakeReader is an in-memory Reader.
type fakeReader struct {
	mu       sync.Mutex
	rows     map[uuid.UUID]*domain.Notification
	missing  int // FindByID answers not found this many times before succeeding
	after    []*domain.Notification
	gotAfter uuid.UUID
	gotExcl  uuid.UUID
}

func (f *fakeReader) FindByID(_ context.Context, userID, id uuid.UUID) (*domain.Notification, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.missing > 0 {
		f.missing--
		return nil, domain.ErrNotificationNotFound
	}
	n, ok := f.rows[id]
	if !ok || n.UserID != userID {
		return nil, domain.ErrNotificationNotFound
	}
	return n, nil
}

func (f *fakeReader) ListAfter(_ context.Context, _, afterID, excl uuid.UUID, _ int) ([]*domain.Notification, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.gotAfter, f.gotExcl = afterID, excl
	return f.after, nil
}

func (f *fakeReader) add(n *domain.Notification) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.rows == nil {
		f.rows = map[uuid.UUID]*domain.Notification{}
	}
	n.ID = uuid.New()
	f.rows[n.ID] = n
}

// manualTicks hands out tickers that fire only when the test says so.
type manualTicks struct {
	mu    sync.Mutex
	chans map[time.Duration]chan time.Time
}

func newTicks() *manualTicks { return &manualTicks{chans: map[time.Duration]chan time.Time{}} }

func (m *manualTicks) ticker(d time.Duration) (<-chan time.Time, func()) {
	m.mu.Lock()
	defer m.mu.Unlock()
	ch := make(chan time.Time, 1)
	m.chans[d] = ch
	return ch, func() {}
}

func (m *manualTicks) fire(d time.Duration) {
	m.mu.Lock()
	ch := m.chans[d]
	m.mu.Unlock()
	ch <- time.Now()
}

type env struct {
	t      *testing.T
	hub    *sse.Hub
	reader *fakeReader
	ticks  *manualTicks
	srv    *httptest.Server
	revoke map[uuid.UUID]bool
	mu     sync.Mutex
}

func newEnv(t *testing.T, hubOpts sse.Options, cfg func(*Options)) *env {
	t.Helper()
	e := &env{t: t, hub: sse.NewHub(hubOpts), reader: &fakeReader{}, ticks: newTicks(), revoke: map[uuid.UUID]bool{}}
	appmw.SetSessionValidator(func(_ context.Context, sid, _ uuid.UUID) error {
		e.mu.Lock()
		defer e.mu.Unlock()
		if e.revoke[sid] {
			return apperror.New(apperror.KindUnauthorized, "revoked")
		}
		return nil
	})
	t.Cleanup(func() { appmw.SetSessionValidator(nil) })
	opts := Options{
		Ticker: e.ticks.ticker, Sleep: func(context.Context, time.Duration) {},
		Validate: func(_ context.Context, sid, _ uuid.UUID) error {
			e.mu.Lock()
			defer e.mu.Unlock()
			if e.revoke[sid] {
				return apperror.New(apperror.KindUnauthorized, "revoked")
			}
			return nil
		},
	}
	if cfg != nil {
		cfg(&opts)
	}
	h := NewHandler(e.hub, e.reader, opts)
	r := chi.NewRouter()
	r.Route("/notifications", func(r chi.Router) {
		r.Use(appmw.Authenticate(secret))
		r.Get("/stream", h.ServeHTTP)
	})
	e.srv = httptest.NewServer(r)
	t.Cleanup(e.srv.Close)
	t.Cleanup(e.hub.Close)
	return e
}

func (e *env) token(user, session uuid.UUID) string {
	tok, err := authtoken.GenerateWithSession(secret, time.Hour, user, "user", session)
	if err != nil {
		e.t.Fatal(err)
	}
	return tok
}

// client is an open stream with its lines delivered on a channel.
type client struct {
	t      *testing.T
	resp   *http.Response
	lines  chan string
	cancel context.CancelFunc
}

func (e *env) open(user, session uuid.UUID, lastID string) *client {
	e.t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, e.srv.URL+"/notifications/stream", nil)
	req.Header.Set("Authorization", "Bearer "+e.token(user, session))
	if lastID != "" {
		req.Header.Set("Last-Event-ID", lastID)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		cancel()
		e.t.Fatal(err)
	}
	c := &client{t: e.t, resp: resp, lines: make(chan string, 100), cancel: cancel}
	go func() {
		defer close(c.lines)
		sc := bufio.NewScanner(resp.Body)
		for sc.Scan() {
			c.lines <- sc.Text()
		}
	}()
	e.t.Cleanup(cancel)
	if resp.StatusCode != http.StatusOK {
		return c
	}
	if got := c.next(); got != "retry: 3000" {
		e.t.Fatalf("first line = %q", got)
	}
	c.next() // blank
	return c
}

func (c *client) next() string {
	c.t.Helper()
	select {
	case l, ok := <-c.lines:
		if !ok {
			c.t.Fatal("stream closed")
		}
		return l
	case <-time.After(5 * time.Second):
		c.t.Fatal("timed out waiting for a line")
		return ""
	}
}

// event reads one full frame as lines up to the blank separator.
func (c *client) event() []string {
	c.t.Helper()
	var frame []string
	for {
		l := c.next()
		if l == "" {
			return frame
		}
		frame = append(frame, l)
	}
}

func (c *client) closed() bool {
	select {
	case _, ok := <-c.lines:
		return !ok
	case <-time.After(5 * time.Second):
		return false
	}
}

func (c *client) silent() bool {
	select {
	case <-c.lines:
		return false
	case <-time.After(150 * time.Millisecond):
		return true
	}
}

func note(user uuid.UUID, kind, message string) *domain.Notification {
	return &domain.Notification{UserID: user, Kind: kind, Message: message, Link: "/x", CreatedAt: time.Date(2026, 11, 27, 22, 0, 0, 0, time.UTC)}
}

func TestStreamRequiresAuthentication(t *testing.T) {
	e := newEnv(t, sse.Options{}, nil)
	resp, err := http.Get(e.srv.URL + "/notifications/stream")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status = %d", resp.StatusCode)
	}
}

func TestStreamHeadersAndNotificationEvent(t *testing.T) {
	e := newEnv(t, sse.Options{}, nil)
	user := uuid.New()
	c := e.open(user, uuid.New(), "")
	h := c.resp.Header
	if h.Get("Content-Type") != "text/event-stream" || h.Get("Cache-Control") != "no-store" || h.Get("X-Accel-Buffering") != "no" {
		t.Fatalf("headers: %v", h)
	}
	n := note(user, "issue.assigned", "Te asignaron CL-1")
	e.reader.add(n)
	e.hub.Publish(sse.Signal{UserID: user, Kind: sse.KindNotification, ID: n.ID.String()})
	frame := c.event()
	if len(frame) != 3 || frame[0] != "id: "+n.ID.String() || frame[1] != "event: notification" {
		t.Fatalf("frame: %v", frame)
	}
	var body map[string]any
	if err := json.Unmarshal([]byte(strings.TrimPrefix(frame[2], "data: ")), &body); err != nil {
		t.Fatal(err)
	}
	if body["id"] != n.ID.String() || body["kind"] != "issue.assigned" || body["message"] != "Te asignaron CL-1" || body["read"] != false || body["created_at"] != "2026-11-27T22:00:00Z" {
		t.Fatalf("body: %v", body)
	}
	if _, has := body["data"]; has {
		t.Fatal("data must be omitted when empty")
	}
}

func TestStreamDoesNotLeakAcrossUsers(t *testing.T) {
	e := newEnv(t, sse.Options{}, nil)
	user, other := uuid.New(), uuid.New()
	c := e.open(user, uuid.New(), "")
	foreign := note(other, "issue.assigned", "secret")
	e.reader.add(foreign)
	e.hub.Publish(sse.Signal{UserID: other, Kind: sse.KindNotification, ID: foreign.ID.String()})
	// Even a signal addressed to this user that names a foreign row must not deliver it.
	e.hub.Publish(sse.Signal{UserID: user, Kind: sse.KindNotification, ID: foreign.ID.String()})
	if !c.silent() {
		t.Fatal("nothing may be delivered")
	}
}

func TestStreamHeartbeat(t *testing.T) {
	e := newEnv(t, sse.Options{}, nil)
	c := e.open(uuid.New(), uuid.New(), "")
	e.ticks.fire(20 * time.Second)
	if got := c.next(); got != ": ping" {
		t.Fatalf("heartbeat = %q", got)
	}
}

func TestStreamLoginAlertGoesToOtherSessionsOnly(t *testing.T) {
	e := newEnv(t, sse.Options{}, nil)
	user, oldSession, newSession := uuid.New(), uuid.New(), uuid.New()
	older := e.open(user, oldSession, "")
	fresh := e.open(user, newSession, "")

	alert := note(user, domain.KindNewLogin, "Se inició sesión desde Chrome en Windows")
	alert.Data = map[string]any{"session_id": newSession.String(), "title": "Nuevo inicio de sesión"}
	e.reader.add(alert)
	e.hub.Publish(sse.Signal{UserID: user, Kind: sse.KindNotification, ID: alert.ID.String()})

	if frame := older.event(); frame[1] != "event: notification" {
		t.Fatalf("frame: %v", frame)
	}
	frame := older.event()
	if frame[0] != "event: session" || !strings.Contains(frame[1], `"type":"new_login"`) || !strings.Contains(frame[1], newSession.String()) {
		t.Fatalf("session frame: %v", frame)
	}
	if !fresh.silent() {
		t.Fatal("the session that logged in must not be alerted")
	}
}

func TestStreamReplaysAfterLastEventID(t *testing.T) {
	e := newEnv(t, sse.Options{}, nil)
	user, session := uuid.New(), uuid.New()
	a, b := note(user, "k", "one"), note(user, "k", "two")
	a.ID, b.ID = uuid.New(), uuid.New()
	e.reader.after = []*domain.Notification{a, b}
	last := uuid.New()
	c := e.open(user, session, last.String())
	if f := c.event(); f[0] != "id: "+a.ID.String() {
		t.Fatalf("first replayed: %v", f)
	}
	if f := c.event(); f[0] != "id: "+b.ID.String() {
		t.Fatalf("second replayed: %v", f)
	}
	if e.reader.gotAfter != last || e.reader.gotExcl != session {
		t.Fatalf("replay args: %v %v", e.reader.gotAfter, e.reader.gotExcl)
	}
	// A live signal for a row that was already replayed is not repeated.
	e.reader.mu.Lock()
	e.reader.rows = map[uuid.UUID]*domain.Notification{a.ID: a}
	e.reader.mu.Unlock()
	e.hub.Publish(sse.Signal{UserID: user, Kind: sse.KindNotification, ID: a.ID.String()})
	if !c.silent() {
		t.Fatal("duplicate delivery")
	}
}

func TestStreamIgnoresBadLastEventID(t *testing.T) {
	e := newEnv(t, sse.Options{}, nil)
	c := e.open(uuid.New(), uuid.New(), "not-a-uuid")
	if !c.silent() {
		t.Fatal("no replay expected")
	}
}

func TestStreamWaitsForUncommittedRow(t *testing.T) {
	e := newEnv(t, sse.Options{}, nil)
	user := uuid.New()
	e.reader.missing = 2
	c := e.open(user, uuid.New(), "")
	n := note(user, "k", "late")
	e.reader.add(n)
	e.hub.Publish(sse.Signal{UserID: user, Kind: sse.KindNotification, ID: n.ID.String()})
	if f := c.event(); f[0] != "id: "+n.ID.String() {
		t.Fatalf("frame: %v", f)
	}
	// A row that never appears is skipped without closing the stream.
	e.reader.mu.Lock()
	e.reader.missing = 100
	e.reader.mu.Unlock()
	e.hub.Publish(sse.Signal{UserID: user, Kind: sse.KindNotification, ID: uuid.NewString()})
	e.hub.Publish(sse.Signal{UserID: user, Kind: sse.KindNotification, ID: "garbage"})
	e.ticks.fire(20 * time.Second)
	if got := c.next(); got != ": ping" {
		t.Fatalf("stream should stay open: %q", got)
	}
}

func TestStreamClosesWhenItsSessionIsRevoked(t *testing.T) {
	e := newEnv(t, sse.Options{}, nil)
	user, mine, other := uuid.New(), uuid.New(), uuid.New()
	c := e.open(user, mine, "")
	e.hub.Publish(sse.Signal{UserID: user, Kind: sse.KindSessionRevoked, ID: other.String()})
	if !c.silent() {
		t.Fatal("another session revoked: stream stays")
	}
	e.hub.Publish(sse.Signal{UserID: user, Kind: sse.KindSessionRevoked, ID: mine.String()})
	frame := c.event()
	if frame[0] != "event: session" || !strings.Contains(frame[1], `"type":"session_revoked"`) || !strings.Contains(frame[1], mine.String()) {
		t.Fatalf("frame: %v", frame)
	}
	if !c.closed() {
		t.Fatal("stream must close")
	}
}

func TestStreamClosesWhenOtherSessionsAreRevoked(t *testing.T) {
	e := newEnv(t, sse.Options{}, nil)
	user, keep, gone := uuid.New(), uuid.New(), uuid.New()
	kept, closed := e.open(user, keep, ""), e.open(user, gone, "")
	e.hub.Publish(sse.Signal{UserID: user, Kind: sse.KindSessionsRevokedExcept, ID: keep.String()})
	if f := closed.event(); !strings.Contains(f[1], "session_revoked") || !closed.closed() {
		t.Fatalf("revoked session: %v", f)
	}
	if !kept.silent() {
		t.Fatal("the kept session stays open")
	}
}

func TestStreamRevalidatesTheSession(t *testing.T) {
	e := newEnv(t, sse.Options{}, nil)
	user, session := uuid.New(), uuid.New()
	c := e.open(user, session, "")
	e.ticks.fire(time.Minute)
	if !c.silent() {
		t.Fatal("a valid session keeps the stream open")
	}
	e.mu.Lock()
	e.revoke[session] = true
	e.mu.Unlock()
	// The middleware validator would now refuse new connections; the open one notices on its next check.
	e.ticks.fire(time.Minute)
	frame := c.event()
	if !strings.Contains(frame[1], "session_revoked") || !c.closed() {
		t.Fatalf("frame: %v", frame)
	}
}

func TestStreamLimitReplacesTheOldest(t *testing.T) {
	e := newEnv(t, sse.Options{MaxPerUser: 1}, nil)
	user := uuid.New()
	first := e.open(user, uuid.New(), "")
	second := e.open(user, uuid.New(), "")
	frame := first.event()
	if !strings.Contains(frame[1], "stream_replaced") || !first.closed() {
		t.Fatalf("frame: %v", frame)
	}
	if !second.silent() {
		t.Fatal("the newest stream stays")
	}
}

func TestStreamEndsOnShutdown(t *testing.T) {
	e := newEnv(t, sse.Options{}, nil)
	c := e.open(uuid.New(), uuid.New(), "")
	e.hub.Close()
	if !c.closed() {
		t.Fatal("stream must end when the hub closes")
	}
}

func TestStreamSurvivesServerTimeouts(t *testing.T) {
	e := newEnv(t, sse.Options{}, nil)
	e.srv.Config.WriteTimeout = 200 * time.Millisecond
	e.srv.Config.ReadTimeout = 200 * time.Millisecond
	user := uuid.New()
	c := e.open(user, uuid.New(), "")
	time.Sleep(700 * time.Millisecond) // longer than both timeouts
	n := note(user, "k", "still here")
	e.reader.add(n)
	e.hub.Publish(sse.Signal{UserID: user, Kind: sse.KindNotification, ID: n.ID.String()})
	if f := c.event(); f[0] != "id: "+n.ID.String() {
		t.Fatalf("frame: %v", f)
	}
}

func TestOptionsDefaults(t *testing.T) {
	h := NewHandler(sse.NewHub(sse.Options{}), &fakeReader{}, Options{})
	o := h.opts
	if o.Heartbeat != 20*time.Second || o.Revalidate != time.Minute || o.RetryMillis != 3000 || o.WriteTimeout <= 0 || o.LoadAttempts != 5 {
		t.Fatalf("defaults: %+v", o)
	}
	ch, stop := o.Ticker(time.Hour)
	if ch == nil {
		t.Fatal("ticker")
	}
	stop()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	o.Sleep(ctx, time.Hour) // returns at once on a cancelled context
}
