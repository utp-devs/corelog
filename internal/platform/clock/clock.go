// Package clock provides an injectable time source so that use cases and
// domain code never call time.Now directly.
package clock

import "time"

// Clock returns the current time.
type Clock interface {
	Now() time.Time
}

// RealClock reads the system clock and returns UTC times.
type RealClock struct{}

// Now returns the current time in UTC.
func (RealClock) Now() time.Time { return time.Now().UTC() }

// FixedClock always returns the same instant.
type FixedClock struct {
	T time.Time
}

// NewFixed returns a FixedClock pinned at t.
func NewFixed(t time.Time) FixedClock { return FixedClock{T: t} }

// Now returns the pinned instant.
func (c FixedClock) Now() time.Time { return c.T }

// FakeClock is a mutable clock for tests; Advance moves it forward.
type FakeClock struct {
	T time.Time
}

// Now returns the current fake instant.
func (c *FakeClock) Now() time.Time { return c.T }

// Advance moves the fake clock forward by d.
func (c *FakeClock) Advance(d time.Duration) { c.T = c.T.Add(d) }
