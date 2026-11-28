package usecase

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"log/slog"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/jhonsferg/corelog/internal/platform/clock"
	"github.com/jhonsferg/corelog/internal/platform/events"
	"github.com/jhonsferg/corelog/internal/user/application/port"
	"github.com/jhonsferg/corelog/internal/user/domain"
)

// Event types published by the login guard.
const (
	EventLogin         = "auth.login"
	EventLoginFailed   = "auth.login_failed"
	EventAccountLocked = "auth.account_locked"
)

// LockoutKey is the storage key of an account: the SHA-256 of the normalized email, so it also protects
// addresses without an account and never stores the address itself.
func LockoutKey(email string) string {
	sum := sha256.Sum256([]byte(strings.ToLower(strings.TrimSpace(email))))
	return hex.EncodeToString(sum[:])
}

// LoginGuard counts failed logins per account and locks the account with an escalating delay.
type LoginGuard struct {
	store     port.LockoutStore
	policy    domain.LockoutPolicy
	clock     clock.Clock
	publisher events.Publisher
}

var _ port.LoginGuard = (*LoginGuard)(nil)

// NewLoginGuard returns a guard. The publisher may be nil (no audit events).
func NewLoginGuard(store port.LockoutStore, policy domain.LockoutPolicy, clk clock.Clock, publisher events.Publisher) *LoginGuard {
	return &LoginGuard{store: store, policy: policy, clock: clk, publisher: publisher}
}

// Check reports whether the account is locked.
func (g *LoginGuard) Check(ctx context.Context, email string) (bool, time.Duration, error) {
	state, err := g.store.Get(ctx, LockoutKey(email))
	if err != nil {
		return false, 0, err
	}
	locked, wait := state.Locked(g.clock.Now())
	return locked, wait, nil
}

// RecordFailure counts a failed login and starts a lockout when the policy says so.
func (g *LoginGuard) RecordFailure(ctx context.Context, email string) (bool, time.Duration, error) {
	key := LockoutKey(email)
	now := g.clock.Now()
	state, err := g.store.Get(ctx, key)
	if err != nil {
		return false, 0, err
	}
	next, started := g.policy.RecordFailure(state, now)
	if err := g.store.Save(ctx, key, next, now); err != nil {
		return false, 0, err
	}
	label := strings.ToLower(strings.TrimSpace(email))
	g.publish(ctx, EventLoginFailed, nil, now, label)
	if !started {
		return false, 0, nil
	}
	g.publish(ctx, EventAccountLocked, nil, now, label)
	_, wait := next.Locked(now)
	return true, wait, nil
}

// RecordSuccess clears the failure counter and records the login event.
func (g *LoginGuard) RecordSuccess(ctx context.Context, user *domain.User) error {
	if err := g.store.Delete(ctx, LockoutKey(user.Email)); err != nil {
		return err
	}
	id := user.ID
	g.publish(ctx, EventLogin, &id, g.clock.Now(), user.FullName())
	return nil
}

// publish records the event best effort: a failing audit write must not block a login.
func (g *LoginGuard) publish(ctx context.Context, typ string, actor *uuid.UUID, at time.Time, label string) {
	if g.publisher == nil {
		return
	}
	payload := map[string]any{}
	if label != "" {
		payload["actor_name"] = label
	}
	e := events.Event{Type: typ, EntityType: "user", EntityID: actor, ActorID: actor, Payload: payload, CreatedAt: at}
	if err := g.publisher.Publish(ctx, e); err != nil {
		slog.Warn("login event not recorded", slog.String("type", typ), slog.String("error", err.Error()))
	}
}
