// Package sse holds the in-process fan-out of server-sent events and its PostgreSQL LISTEN/NOTIFY bridge,
// so any API instance can deliver an event raised by another one.
package sse

import (
	"sync"

	"github.com/google/uuid"
)

// Signal kinds. A signal carries only identifiers: the subscriber loads whatever it needs to render.
const (
	// KindNotification announces a new in-app notification; ID is the notification id.
	KindNotification = "n"
	// KindSessionRevoked announces that one session was closed; ID is the session id.
	KindSessionRevoked = "r"
	// KindSessionsRevokedExcept announces that every session of the user but ID was closed; an empty ID
	// closes all of them.
	KindSessionsRevokedExcept = "x"
)

// Signal is one event addressed to every stream of a user.
type Signal struct {
	UserID uuid.UUID
	Kind   string
	ID     string
}

// Reasons a subscription ends.
const (
	ReasonClosed   = "closed"   // the subscriber left
	ReasonDropped  = "dropped"  // its buffer overflowed: it was too slow
	ReasonReplaced = "replaced" // the user opened more streams than allowed and this was the oldest
	ReasonShutdown = "shutdown" // the hub was closed
)

// Options tune a Hub.
type Options struct {
	// MaxPerUser is the number of simultaneous subscriptions of one user (default 5). The oldest is closed.
	MaxPerUser int
	// Buffer is the number of pending signals per subscription before it is dropped (default 32).
	Buffer int
}

// Hub registers subscriptions by user and fans signals out to them. It is safe for concurrent use.
type Hub struct {
	mu     sync.Mutex
	opts   Options
	byUser map[uuid.UUID][]*Subscription
	closed bool
}

// NewHub builds an empty hub.
func NewHub(opts Options) *Hub {
	if opts.MaxPerUser <= 0 {
		opts.MaxPerUser = 5
	}
	if opts.Buffer <= 0 {
		opts.Buffer = 32
	}
	return &Hub{opts: opts, byUser: map[uuid.UUID][]*Subscription{}}
}

// Subscription is one open stream. Read C until Done is closed, then read Reason.
type Subscription struct {
	UserID    uuid.UUID
	SessionID uuid.UUID
	C         <-chan Signal

	hub    *Hub
	ch     chan Signal
	done   chan struct{}
	once   sync.Once
	reason string
}

// Done is closed when the subscription ends for any reason.
func (s *Subscription) Done() <-chan struct{} { return s.done }

// Reason says why the subscription ended; it is only meaningful after Done is closed.
func (s *Subscription) Reason() string {
	select {
	case <-s.done:
		return s.reason
	default:
		return ""
	}
}

// finish ends the subscription once. The caller holds no hub lock requirement: it only touches the subscription.
func (s *Subscription) finish(reason string) {
	s.once.Do(func() {
		s.reason = reason
		close(s.done)
	})
}

// Close removes the subscription from the hub.
func (s *Subscription) Close() {
	s.hub.remove(s, ReasonClosed)
}

// Subscribe registers a stream of the user opened from the given session. When the user is at the limit the
// oldest subscription is closed with ReasonReplaced. On a closed hub the subscription is born finished.
func (h *Hub) Subscribe(userID, sessionID uuid.UUID) *Subscription {
	ch := make(chan Signal, h.opts.Buffer)
	sub := &Subscription{UserID: userID, SessionID: sessionID, C: ch, hub: h, ch: ch, done: make(chan struct{})}
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.closed {
		sub.finish(ReasonShutdown)
		return sub
	}
	list := h.byUser[userID]
	for len(list) >= h.opts.MaxPerUser {
		list[0].finish(ReasonReplaced)
		list = list[1:]
	}
	h.byUser[userID] = append(list, sub)
	return sub
}

func (h *Hub) remove(sub *Subscription, reason string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.removeLocked(sub, reason)
}

func (h *Hub) removeLocked(sub *Subscription, reason string) {
	list := h.byUser[sub.UserID]
	for i, s := range list {
		if s == sub {
			list = append(list[:i:i], list[i+1:]...)
			break
		}
	}
	if len(list) == 0 {
		delete(h.byUser, sub.UserID)
	} else {
		h.byUser[sub.UserID] = list
	}
	sub.finish(reason)
}

// Publish delivers the signal to every subscription of the user, never blocking: a subscriber whose buffer
// is full is dropped and must reconnect. It returns how many subscriptions received the signal.
func (h *Hub) Publish(sig Signal) int {
	h.mu.Lock()
	defer h.mu.Unlock()
	delivered := 0
	for _, sub := range append([]*Subscription(nil), h.byUser[sig.UserID]...) {
		select {
		case sub.ch <- sig:
			delivered++
		default:
			h.removeLocked(sub, ReasonDropped)
		}
	}
	return delivered
}

// Count returns the number of live subscriptions of the user.
func (h *Hub) Count(userID uuid.UUID) int {
	h.mu.Lock()
	defer h.mu.Unlock()
	return len(h.byUser[userID])
}

// Close ends every subscription and refuses new ones.
func (h *Hub) Close() {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.closed = true
	for _, list := range h.byUser {
		for _, sub := range list {
			sub.finish(ReasonShutdown)
		}
	}
	h.byUser = map[uuid.UUID][]*Subscription{}
}
