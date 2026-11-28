package domain

import "time"

// LockoutState is the persisted failed login counter of one account (keyed by a hash of its email).
type LockoutState struct {
	Failures       int
	LockCount      int
	FirstFailureAt *time.Time
	LockedUntil    *time.Time
}

// LockoutPolicy decides when an account is locked. After Threshold failures inside Window the account is
// locked for Duration, doubling with every consecutive lockout up to MaxDuration.
type LockoutPolicy struct {
	Threshold   int
	Window      time.Duration
	Duration    time.Duration
	MaxDuration time.Duration
}

// DefaultLockoutPolicy is 5 failures in 15 minutes, locking for 15 minutes (doubling up to 24 hours).
func DefaultLockoutPolicy() LockoutPolicy {
	return LockoutPolicy{Threshold: 5, Window: 15 * time.Minute, Duration: 15 * time.Minute, MaxDuration: 24 * time.Hour}
}

// Locked reports whether the account is locked at now and for how much longer.
func (s LockoutState) Locked(now time.Time) (bool, time.Duration) {
	if s.LockedUntil != nil && s.LockedUntil.After(now) {
		return true, s.LockedUntil.Sub(now)
	}
	return false, 0
}

// RecordFailure returns the state after one more failed login at now and whether it started a lockout.
func (p LockoutPolicy) RecordFailure(s LockoutState, now time.Time) (LockoutState, bool) {
	if p.Threshold <= 0 {
		return s, false
	}
	// A long quiet period after the last lockout forgives the escalation.
	if s.LockedUntil != nil && now.Sub(*s.LockedUntil) > p.MaxDuration {
		s.LockCount = 0
	}
	if s.FirstFailureAt == nil || now.Sub(*s.FirstFailureAt) > p.Window {
		first := now
		s.FirstFailureAt = &first
		s.Failures = 0
	}
	s.Failures++
	if s.Failures < p.Threshold {
		return s, false
	}
	d := p.Duration
	for i := 0; i < s.LockCount && d < p.MaxDuration; i++ {
		d *= 2
	}
	if p.MaxDuration > 0 && d > p.MaxDuration {
		d = p.MaxDuration
	}
	until := now.Add(d)
	s.LockedUntil = &until
	s.LockCount++
	s.Failures = 0
	s.FirstFailureAt = nil
	return s, true
}
