package domain

import (
	"testing"
	"time"
)

func TestLockoutEscalation(t *testing.T) {
	t.Parallel()
	p := DefaultLockoutPolicy()
	now := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	var s LockoutState
	locked := false
	for i := 0; i < 5; i++ {
		s, locked = p.RecordFailure(s, now.Add(time.Duration(i)*time.Second))
		if locked != (i == 4) {
			t.Fatalf("failure %d locked=%v", i+1, locked)
		}
	}
	if is, d := s.Locked(now.Add(5 * time.Second)); !is || d > 15*time.Minute {
		t.Fatalf("locked=%v remaining=%v", is, d)
	}
	// Second lockout lasts twice as long.
	later := now.Add(20 * time.Minute)
	if is, _ := s.Locked(later); is {
		t.Fatal("lock must have expired")
	}
	for i := 0; i < 5; i++ {
		s, locked = p.RecordFailure(s, later)
	}
	if !locked {
		t.Fatal("second lockout expected")
	}
	if _, d := s.Locked(later); d != 30*time.Minute {
		t.Fatalf("second lockout = %v, want 30m", d)
	}
}

func TestLockoutWindowResetsCounter(t *testing.T) {
	t.Parallel()
	p := DefaultLockoutPolicy()
	now := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	var s LockoutState
	for i := 0; i < 4; i++ {
		s, _ = p.RecordFailure(s, now)
	}
	s, locked := p.RecordFailure(s, now.Add(16*time.Minute))
	if locked || s.Failures != 1 {
		t.Fatalf("failures=%d locked=%v, want a fresh window", s.Failures, locked)
	}
}

func TestLockoutCap(t *testing.T) {
	t.Parallel()
	p := LockoutPolicy{Threshold: 1, Window: time.Minute, Duration: time.Hour, MaxDuration: 3 * time.Hour}
	now := time.Now()
	s := LockoutState{LockCount: 10}
	s, _ = p.RecordFailure(s, now)
	if _, d := s.Locked(now); d != 3*time.Hour {
		t.Fatalf("duration %v, want capped 3h", d)
	}
}
