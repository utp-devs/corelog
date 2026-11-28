// Package ratelimit implements an in-memory token bucket limiter keyed by string (IP, account hash, user id)
// with an injectable clock, plus the HTTP helpers that answer 429 with Retry-After.
package ratelimit

import (
	"math"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/jhonsferg/corelog/internal/platform/apperror"
	"github.com/jhonsferg/corelog/internal/platform/clock"
	"github.com/jhonsferg/corelog/internal/platform/httpresponse"
)

// Limit is a token bucket policy: Burst tokens at most, refilled evenly so that Burst tokens arrive every
// Per. A Limit with Burst <= 0 allows everything.
type Limit struct {
	Burst int
	Per   time.Duration
}

// PerMinute returns a limit of n requests per minute.
func PerMinute(n int) Limit { return Limit{Burst: n, Per: time.Minute} }

// PerHour returns a limit of n requests per hour.
func PerHour(n int) Limit { return Limit{Burst: n, Per: time.Hour} }

type bucket struct {
	tokens float64
	last   time.Time
}

// Limiter is a set of token buckets sharing one Limit. It is safe for concurrent use.
type Limiter struct {
	limit Limit
	clk   clock.Clock

	mu      sync.Mutex
	buckets map[string]*bucket
	calls   int
}

// New returns a limiter. A nil clock selects the real clock.
func New(limit Limit, clk clock.Clock) *Limiter {
	if clk == nil {
		clk = clock.RealClock{}
	}
	return &Limiter{limit: limit, clk: clk, buckets: make(map[string]*bucket)}
}

// Allow takes one token for key. When the bucket is empty it returns false and how long until a token is
// available.
func (l *Limiter) Allow(key string) (bool, time.Duration) {
	if l == nil || l.limit.Burst <= 0 || l.limit.Per <= 0 {
		return true, 0
	}
	now := l.clk.Now()
	rate := float64(l.limit.Burst) / float64(l.limit.Per) // tokens per nanosecond

	l.mu.Lock()
	defer l.mu.Unlock()
	l.calls++
	if l.calls%512 == 0 {
		l.sweep(now)
	}
	b, ok := l.buckets[key]
	if !ok {
		b = &bucket{tokens: float64(l.limit.Burst), last: now}
		l.buckets[key] = b
	}
	if elapsed := now.Sub(b.last); elapsed > 0 {
		b.tokens = math.Min(float64(l.limit.Burst), b.tokens+float64(elapsed)*rate)
		b.last = now
	}
	if b.tokens >= 1 {
		b.tokens--
		return true, 0
	}
	wait := time.Duration((1 - b.tokens) / rate)
	return false, wait
}

// sweep drops buckets that have been idle long enough to be full again. Callers hold l.mu.
func (l *Limiter) sweep(now time.Time) {
	for k, b := range l.buckets {
		if now.Sub(b.last) >= l.limit.Per {
			delete(l.buckets, k)
		}
	}
}

// Codes of the 429 responses.
const (
	CodeRateLimited   = "auth.rate_limited"
	CodeAccountLocked = "auth.account_locked"
)

// RetryAfterSeconds rounds d up to whole seconds, at least 1.
func RetryAfterSeconds(d time.Duration) int {
	s := int(math.Ceil(d.Seconds()))
	if s < 1 {
		s = 1
	}
	return s
}

// Write answers 429 with the Retry-After header, the stable code and a Spanish message.
func Write(w http.ResponseWriter, code, message string, retryAfter time.Duration) {
	w.Header().Set("Retry-After", strconv.Itoa(RetryAfterSeconds(retryAfter)))
	httpresponse.Error(w, apperror.WithCode(apperror.KindRateLimited, code, message))
}

// WriteLimited answers the standard rate limited response.
func WriteLimited(w http.ResponseWriter, retryAfter time.Duration) {
	Write(w, CodeRateLimited, "Demasiadas solicitudes: espera un momento antes de intentarlo de nuevo", retryAfter)
}

// Check takes a token and writes the 429 response when the key is over its limit. It returns true when the
// request may proceed. A nil limiter allows everything.
func Check(l *Limiter, w http.ResponseWriter, key string) bool {
	ok, wait := l.Allow(key)
	if !ok {
		WriteLimited(w, wait)
	}
	return ok
}
