package events

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/jmoiron/sqlx"

	"github.com/jhonsferg/corelog/internal/platform/database/testdb"
)

func publishN(t *testing.T, db *sqlx.DB, n int) {
	t.Helper()
	pub := NewPostgresPublisher(func(context.Context) sqlx.ExtContext { return db })
	for i := 1; i <= n; i++ {
		if err := pub.Publish(context.Background(), Event{
			Type:       "issue.created",
			EntityType: "issue",
			Payload:    map[string]any{"seq": i},
		}); err != nil {
			t.Fatalf("publish %d: %v", i, err)
		}
	}
}

type recordingConsumer struct {
	name     string
	mu       sync.Mutex
	attempts []int64
	seen     []int64
	failID   int64
	failLeft int
	delay    time.Duration
}

func (c *recordingConsumer) Name() string { return c.name }

func (c *recordingConsumer) Handle(_ context.Context, e Event) error {
	if c.delay > 0 {
		time.Sleep(c.delay)
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.attempts = append(c.attempts, e.ID)
	if e.ID == c.failID && c.failLeft > 0 {
		c.failLeft--
		return errors.New("transient failure")
	}
	c.seen = append(c.seen, e.ID)
	return nil
}

func (c *recordingConsumer) snapshot() (attempts, seen []int64) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([]int64(nil), c.attempts...), append([]int64(nil), c.seen...)
}

func cursorOf(t *testing.T, db *sqlx.DB, consumer string) int64 {
	t.Helper()
	var last int64
	if err := db.QueryRowContext(context.Background(),
		`SELECT last_event_id FROM event_cursors WHERE consumer = $1`, consumer).Scan(&last); err != nil {
		t.Fatalf("read cursor: %v", err)
	}
	return last
}

func TestDispatcherDeliversInOrderAndAdvancesCursor(t *testing.T) {
	db := testdb.New(t)
	publishN(t, db, 5)

	c := &recordingConsumer{name: "audit"}
	d, err := NewDispatcher(db, []Consumer{c}, Options{BatchSize: 2})
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	for i := 0; i < 3; i++ {
		if err := d.RunOnce(ctx); err != nil {
			t.Fatalf("RunOnce: %v", err)
		}
	}
	_, seen := c.snapshot()
	want := []int64{1, 2, 3, 4, 5}
	if len(seen) != len(want) {
		t.Fatalf("delivered %v, want %v", seen, want)
	}
	for i := range want {
		if seen[i] != want[i] {
			t.Fatalf("delivered %v, want %v", seen, want)
		}
	}
	if got := cursorOf(t, db, "audit"); got != 5 {
		t.Fatalf("cursor = %d, want 5", got)
	}
}

func TestDispatcherRedeliversFailedEvent(t *testing.T) {
	db := testdb.New(t)
	publishN(t, db, 3)

	c := &recordingConsumer{name: "notify", failID: 2, failLeft: 1}
	d, err := NewDispatcher(db, []Consumer{c}, Options{})
	if err != nil {
		t.Fatal(err)
	}
	d.baseDelay = time.Millisecond

	ctx := context.Background()
	deadline := time.Now().Add(10 * time.Second)
	for {
		if err := d.RunOnce(ctx); err != nil {
			t.Fatalf("RunOnce: %v", err)
		}
		_, seen := c.snapshot()
		if len(seen) == 3 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("event never delivered, seen=%v", seen)
		}
		time.Sleep(2 * time.Millisecond)
	}

	attempts, seen := c.snapshot()
	if len(seen) != 3 || seen[0] != 1 || seen[1] != 2 || seen[2] != 3 {
		t.Fatalf("seen = %v, want [1 2 3]", seen)
	}
	// Event 2 was attempted, failed, then attempted again; it was not skipped.
	if len(attempts) < 4 || attempts[0] != 1 || attempts[1] != 2 || attempts[2] != 2 {
		t.Fatalf("attempts = %v, want event 2 redelivered after failure", attempts)
	}
	if got := cursorOf(t, db, "notify"); got != 3 {
		t.Fatalf("cursor = %d, want 3", got)
	}
}

func TestDispatcherLockGivesSingleDelivery(t *testing.T) {
	db := testdb.New(t)
	publishN(t, db, 10)

	// Both dispatchers share the consumer name, so they contend for one advisory lock.
	c1 := &recordingConsumer{name: "search", delay: 20 * time.Millisecond}
	c2 := &recordingConsumer{name: "search", delay: 20 * time.Millisecond}
	d1, err := NewDispatcher(db, []Consumer{c1}, Options{})
	if err != nil {
		t.Fatal(err)
	}
	d2, err := NewDispatcher(db, []Consumer{c2}, Options{})
	if err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()
	var wg sync.WaitGroup
	for _, d := range []*Dispatcher{d1, d2} {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := d.RunOnce(ctx); err != nil {
				t.Errorf("RunOnce: %v", err)
			}
		}()
	}
	wg.Wait()

	_, s1 := c1.snapshot()
	_, s2 := c2.snapshot()
	counts := make(map[int64]int)
	for _, id := range append(s1, s2...) {
		counts[id]++
	}
	if len(counts) != 10 {
		t.Fatalf("delivered %d distinct events, want 10 (s1=%v s2=%v)", len(counts), s1, s2)
	}
	for id, n := range counts {
		if n != 1 {
			t.Fatalf("event %d delivered %d times", id, n)
		}
	}
}
