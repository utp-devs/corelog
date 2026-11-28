// Package stream serves the per-user Server-Sent Events channel of notifications and security events.
//
// One long-lived GET per client carries two event types: "notification" (a new in-app notification, same
// JSON as the notifications API) and "session" (security events: new_login, session_revoked,
// stream_replaced), plus comment lines used as heartbeat. The server only pushes; the client reconnects after
// any close, sending the id of the last notification it saw in Last-Event-ID to receive what it missed.
package stream

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"

	httpadapter "github.com/jhonsferg/corelog/internal/notification/adapter/in/http"
	"github.com/jhonsferg/corelog/internal/notification/domain"
	"github.com/jhonsferg/corelog/internal/platform/apperror"
	"github.com/jhonsferg/corelog/internal/platform/clock"
	"github.com/jhonsferg/corelog/internal/platform/httpresponse"
	appmw "github.com/jhonsferg/corelog/internal/platform/middleware"
	"github.com/jhonsferg/corelog/internal/platform/sse"
)

// ReplayLimit bounds how many missed notifications a reconnection receives.
const ReplayLimit = 50

// Reader loads the notifications the stream sends.
type Reader interface {
	FindByID(ctx context.Context, userID, id uuid.UUID) (*domain.Notification, error)
	// ListAfter returns, oldest first, up to limit notifications created after afterID, skipping the login
	// alert of excludeSession.
	ListAfter(ctx context.Context, userID, afterID, excludeSession uuid.UUID, limit int) ([]*domain.Notification, error)
}

// SessionValidator confirms a session is still open.
type SessionValidator func(ctx context.Context, sessionID, userID uuid.UUID) error

// Ticker returns a channel that fires every d and a function that stops it. Tests replace it.
type Ticker func(d time.Duration) (<-chan time.Time, func())

// Options configure the handler; zero values take the defaults.
type Options struct {
	Heartbeat    time.Duration // comment line period (default 20s)
	Revalidate   time.Duration // session re-check period (default 60s)
	WriteTimeout time.Duration // deadline of each write (default 10s); it replaces the server WriteTimeout
	RetryMillis  int           // reconnection hint sent to the client (default 3000)
	Validate     SessionValidator
	Ticker       Ticker
	Clock        clock.Clock
	// LoadAttempts and LoadDelay bound the wait for a row that is not committed yet (default 5 x 100ms).
	LoadAttempts int
	LoadDelay    time.Duration
	Sleep        func(ctx context.Context, d time.Duration)
	Logger       *slog.Logger
}

// Handler is the http.Handler of GET /notifications/stream. It must run behind the authentication middleware.
type Handler struct {
	hub    *sse.Hub
	reader Reader
	opts   Options
}

// NewHandler builds the handler.
func NewHandler(hub *sse.Hub, reader Reader, opts Options) *Handler {
	if opts.Heartbeat <= 0 {
		opts.Heartbeat = 20 * time.Second
	}
	if opts.Revalidate <= 0 {
		opts.Revalidate = time.Minute
	}
	if opts.WriteTimeout <= 0 {
		opts.WriteTimeout = 10 * time.Second
	}
	if opts.RetryMillis <= 0 {
		opts.RetryMillis = 3000
	}
	if opts.Ticker == nil {
		opts.Ticker = func(d time.Duration) (<-chan time.Time, func()) {
			t := time.NewTicker(d)
			return t.C, t.Stop
		}
	}
	if opts.Clock == nil {
		opts.Clock = clock.RealClock{}
	}
	if opts.LoadAttempts <= 0 {
		opts.LoadAttempts = 5
	}
	if opts.LoadDelay <= 0 {
		opts.LoadDelay = 100 * time.Millisecond
	}
	if opts.Sleep == nil {
		opts.Sleep = func(ctx context.Context, d time.Duration) {
			select {
			case <-ctx.Done():
			case <-time.After(d):
			}
		}
	}
	if opts.Logger == nil {
		opts.Logger = slog.Default()
	}
	return &Handler{hub: hub, reader: reader, opts: opts}
}

// sessionEvent is the data of a "session" event.
type sessionEvent struct {
	Type           string         `json:"type"`
	SessionID      string         `json:"session_id,omitempty"`
	NotificationID string         `json:"notification_id,omitempty"`
	Message        string         `json:"message,omitempty"`
	At             string         `json:"at,omitempty"`
	Data           map[string]any `json:"data,omitempty"`
}

// conn is one open stream.
type conn struct {
	h      *Handler
	w      http.ResponseWriter
	rc     *http.ResponseController
	userID uuid.UUID
	sid    uuid.UUID
	sent   map[string]bool // notification ids already sent: replay and live delivery must not repeat
}

// ServeHTTP implements http.Handler.
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	userID, ok := appmw.UserIDFromContext(r.Context())
	if !ok {
		httpresponse.Error(w, apperror.New(apperror.KindUnauthorized, "not authenticated"))
		return
	}
	if _, viaToken := appmw.APITokenIDFromContext(r.Context()); viaToken {
		httpresponse.Error(w, apperror.WithCode(apperror.KindForbidden, "notification.stream_session_only", "El canal en vivo solo admite sesiones de usuario"))
		return
	}
	sid, _ := appmw.SessionIDFromContext(r.Context())
	ctx := r.Context()

	hd := w.Header()
	hd.Set("Content-Type", "text/event-stream")
	hd.Set("Cache-Control", "no-store")
	hd.Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)

	c := &conn{h: h, w: w, rc: http.NewResponseController(w), userID: userID, sid: sid, sent: map[string]bool{}}
	// The server timeouts protect ordinary requests. A stream lives for hours, so its read deadline is lifted
	// and each write gets its own short deadline instead.
	_ = c.rc.SetReadDeadline(time.Time{})

	// Subscribe before replaying so nothing created in between is lost.
	sub := h.hub.Subscribe(userID, sid)
	defer sub.Close()

	if err := c.write(fmt.Sprintf("retry: %d\n\n", h.opts.RetryMillis)); err != nil {
		return
	}
	if last := strings.TrimSpace(r.Header.Get("Last-Event-ID")); last != "" {
		if err := c.replay(ctx, last); err != nil {
			return
		}
	}
	c.loop(ctx, sub)
}

func (c *conn) loop(ctx context.Context, sub *sse.Subscription) {
	o := c.h.opts
	beat, stopBeat := o.Ticker(o.Heartbeat)
	defer stopBeat()
	var check <-chan time.Time
	if o.Validate != nil && c.sid != uuid.Nil {
		var stopCheck func()
		check, stopCheck = o.Ticker(o.Revalidate)
		defer stopCheck()
	}
	for {
		select {
		case <-ctx.Done():
			return
		case <-sub.Done():
			if sub.Reason() == sse.ReasonReplaced {
				_ = c.sendSession(sessionEvent{Type: "stream_replaced", At: o.Clock.Now().UTC().Format(time.RFC3339)})
			}
			// Dropped and shutdown close without a message: the client reconnects and replays.
			return
		case sig := <-sub.C:
			if !c.handle(ctx, sig) {
				return
			}
		case <-beat:
			if err := c.write(": ping\n\n"); err != nil {
				return
			}
		case <-check:
			if err := o.Validate(ctx, c.sid, c.userID); err != nil && apperror.KindOf(err) == apperror.KindUnauthorized {
				_ = c.sendRevoked()
				return
			}
		}
	}
}

// handle processes one signal and reports whether the stream stays open.
func (c *conn) handle(ctx context.Context, sig sse.Signal) bool {
	switch sig.Kind {
	case sse.KindNotification:
		id, err := uuid.Parse(sig.ID)
		if err != nil {
			return true
		}
		note := c.load(ctx, id)
		if note == nil {
			return true
		}
		return c.sendNotification(note) == nil
	case sse.KindSessionRevoked:
		if c.sid != uuid.Nil && sig.ID == c.sid.String() {
			_ = c.sendRevoked()
			return false
		}
	case sse.KindSessionsRevokedExcept:
		if c.sid == uuid.Nil || sig.ID != c.sid.String() {
			_ = c.sendRevoked()
			return false
		}
	}
	return true
}

// load reads a notification, waiting briefly for a transaction that has not committed yet.
func (c *conn) load(ctx context.Context, id uuid.UUID) *domain.Notification {
	o := c.h.opts
	for attempt := 0; attempt < o.LoadAttempts; attempt++ {
		note, err := c.h.reader.FindByID(ctx, c.userID, id)
		if err == nil {
			return note
		}
		if !errors.Is(err, domain.ErrNotificationNotFound) || ctx.Err() != nil {
			return nil
		}
		o.Sleep(ctx, o.LoadDelay)
	}
	return nil
}

func (c *conn) replay(ctx context.Context, lastID string) error {
	id, err := uuid.Parse(lastID)
	if err != nil {
		return nil
	}
	notes, err := c.h.reader.ListAfter(ctx, c.userID, id, c.sid, ReplayLimit)
	if err != nil {
		c.h.opts.Logger.Warn("stream replay failed", "error", err)
		return nil
	}
	for _, n := range notes {
		if err := c.sendNotification(n); err != nil {
			return err
		}
	}
	return nil
}

func (c *conn) sendNotification(n *domain.Notification) error {
	if n.Kind == domain.KindNewLogin && c.sid != uuid.Nil && n.Data["session_id"] == c.sid.String() {
		return nil // the session that just logged in is not warned about itself
	}
	if c.sent[n.ID.String()] {
		return nil
	}
	c.sent[n.ID.String()] = true
	body, err := json.Marshal(httpadapter.ToNotificationResponse(n))
	if err != nil {
		return nil
	}
	if err := c.write(fmt.Sprintf("id: %s\nevent: notification\ndata: %s\n\n", n.ID, body)); err != nil {
		return err
	}
	if n.Kind == domain.KindNewLogin {
		sid, _ := n.Data["session_id"].(string)
		return c.sendSession(sessionEvent{
			Type: "new_login", SessionID: sid, NotificationID: n.ID.String(), Message: n.Message,
			At: n.CreatedAt.UTC().Format(time.RFC3339), Data: n.Data,
		})
	}
	return nil
}

func (c *conn) sendRevoked() error {
	ev := sessionEvent{Type: "session_revoked", At: c.h.opts.Clock.Now().UTC().Format(time.RFC3339)}
	if c.sid != uuid.Nil {
		ev.SessionID = c.sid.String()
	}
	return c.sendSession(ev)
}

func (c *conn) sendSession(ev sessionEvent) error {
	body, err := json.Marshal(ev)
	if err != nil {
		return nil
	}
	return c.write(fmt.Sprintf("event: session\ndata: %s\n\n", body))
}

// write sends one frame and flushes it, under its own deadline.
func (c *conn) write(frame string) error {
	_ = c.rc.SetWriteDeadline(time.Now().Add(c.h.opts.WriteTimeout))
	if _, err := c.w.Write([]byte(frame)); err != nil {
		return err
	}
	return c.rc.Flush()
}
