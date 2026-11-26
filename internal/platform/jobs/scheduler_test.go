package jobs

import (
	"context"
	"io"
	"log/slog"
	"sync/atomic"
	"testing"
	"time"
)

func discardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func TestEnabledParsesEnv(t *testing.T) {
	cases := map[string]bool{
		"":      true,
		"true":  true,
		"0":     true,
		"false": false,
	}
	for value, want := range cases {
		t.Run("value_"+value, func(t *testing.T) {
			t.Setenv("JOBS_ENABLED", value)
			if got := Enabled(); got != want {
				t.Fatalf("JOBS_ENABLED=%q: Enabled()=%v, want %v", value, got, want)
			}
		})
	}
}

func TestJobPanicIsRecoveredAndNextTickRuns(t *testing.T) {
	t.Parallel()
	s := NewScheduler(nil, discardLogger())
	var calls atomic.Int32
	s.Every("panicky", 5*time.Millisecond, func(context.Context) error {
		if calls.Add(1) == 1 {
			panic("boom")
		}
		return nil
	})
	s.Start(context.Background())
	defer s.Stop()

	waitFor(t, 5*time.Second, func() bool { return calls.Load() >= 3 })
}

func TestJobErrorIsLoggedAndSchedulerKeepsRunning(t *testing.T) {
	t.Parallel()
	s := NewScheduler(nil, discardLogger())
	var calls atomic.Int32
	s.Every("failing", 5*time.Millisecond, func(context.Context) error {
		calls.Add(1)
		return context.DeadlineExceeded
	})
	s.Start(context.Background())
	defer s.Stop()

	waitFor(t, 5*time.Second, func() bool { return calls.Load() >= 2 })
}

func TestStopWithoutStartIsSafe(t *testing.T) {
	t.Parallel()
	NewScheduler(nil, nil).Stop()
}

func waitFor(t *testing.T, timeout time.Duration, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for !cond() {
		if time.Now().After(deadline) {
			t.Fatal("condition not met before timeout")
		}
		time.Sleep(5 * time.Millisecond)
	}
}
