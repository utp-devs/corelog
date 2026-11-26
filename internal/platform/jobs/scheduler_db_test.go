package jobs

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jhonsferg/corelog/internal/platform/database/testdb"
)

func TestSameJobNameRunsOnlyOnceAtATime(t *testing.T) {
	db := testdb.New(t)

	var active, maxActive, runs atomic.Int32
	job := func(context.Context) error {
		now := active.Add(1)
		for {
			prev := maxActive.Load()
			if now <= prev || maxActive.CompareAndSwap(prev, now) {
				break
			}
		}
		time.Sleep(150 * time.Millisecond)
		runs.Add(1)
		active.Add(-1)
		return nil
	}

	s1 := NewScheduler(db, discardLogger())
	s2 := NewScheduler(db, discardLogger())
	s1.Every("reminders", 10*time.Millisecond, job)
	s2.Every("reminders", 10*time.Millisecond, job)
	ctx := context.Background()
	s1.Start(ctx)
	s2.Start(ctx)

	waitFor(t, 10*time.Second, func() bool { return runs.Load() >= 2 })
	s1.Stop()
	s2.Stop()

	if got := maxActive.Load(); got != 1 {
		t.Fatalf("max concurrent runs = %d, want 1", got)
	}
}

func TestPanicInLockedJobIsRecovered(t *testing.T) {
	db := testdb.New(t)

	var calls atomic.Int32
	s := NewScheduler(db, discardLogger())
	s.Every("digest", 5*time.Millisecond, func(context.Context) error {
		if calls.Add(1) == 1 {
			panic("boom")
		}
		return nil
	})
	s.Start(context.Background())
	defer s.Stop()

	waitFor(t, 10*time.Second, func() bool { return calls.Load() >= 3 })
}
