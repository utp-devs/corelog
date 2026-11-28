package ratelimit

import (
	"net/http/httptest"
	"testing"
	"time"

	"github.com/jhonsferg/corelog/internal/platform/clock"
)

func TestTokenBucketRefill(t *testing.T) {
	t.Parallel()
	clk := &clock.FakeClock{T: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)}
	l := New(PerMinute(3), clk)
	for i := 0; i < 3; i++ {
		if ok, _ := l.Allow("a"); !ok {
			t.Fatalf("request %d denied within burst", i)
		}
	}
	ok, wait := l.Allow("a")
	if ok || wait <= 0 || wait > 20*time.Second+time.Millisecond {
		t.Fatalf("ok=%v wait=%v, want denied with wait <= 20s", ok, wait)
	}
	if ok, _ := l.Allow("b"); !ok {
		t.Fatal("other key must have its own bucket")
	}
	clk.Advance(20 * time.Second)
	if ok, _ := l.Allow("a"); !ok {
		t.Fatal("a token must be back after one refill interval")
	}
	if ok, _ := l.Allow("a"); ok {
		t.Fatal("only one token should have refilled")
	}
	clk.Advance(10 * time.Minute)
	for i := 0; i < 3; i++ {
		if ok, _ := l.Allow("a"); !ok {
			t.Fatalf("bucket must be full again (request %d)", i)
		}
	}
	if ok, _ := l.Allow("a"); ok {
		t.Fatal("burst is capped at the limit, not accumulated")
	}
}

func TestZeroLimitAndNilAllowEverything(t *testing.T) {
	t.Parallel()
	var nilLimiter *Limiter
	if ok, _ := nilLimiter.Allow("x"); !ok {
		t.Fatal("nil limiter must allow")
	}
	if ok, _ := New(Limit{}, nil).Allow("x"); !ok {
		t.Fatal("zero limit must allow")
	}
}

func TestSweepDropsIdleBuckets(t *testing.T) {
	t.Parallel()
	clk := &clock.FakeClock{T: time.Now()}
	l := New(PerMinute(2), clk)
	l.Allow("old")
	clk.Advance(2 * time.Minute)
	l.mu.Lock()
	l.sweep(clk.Now())
	n := len(l.buckets)
	l.mu.Unlock()
	if n != 0 {
		t.Fatalf("buckets = %d, want 0", n)
	}
}

func TestCheckWritesRetryAfter(t *testing.T) {
	t.Parallel()
	l := New(PerHour(1), &clock.FakeClock{T: time.Now()})
	rec := httptest.NewRecorder()
	if !Check(l, rec, "k") {
		t.Fatal("first call must pass")
	}
	rec = httptest.NewRecorder()
	if Check(l, rec, "k") {
		t.Fatal("second call must be limited")
	}
	if rec.Code != 429 || rec.Header().Get("Retry-After") == "" {
		t.Fatalf("code=%d retry=%q", rec.Code, rec.Header().Get("Retry-After"))
	}
	if body := rec.Body.String(); !contains(body, CodeRateLimited) {
		t.Fatalf("body %s lacks the code", body)
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
