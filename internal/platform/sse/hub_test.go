package sse

import (
	"sync"
	"testing"

	"github.com/google/uuid"
)

func recv(t *testing.T, s *Subscription) Signal {
	t.Helper()
	select {
	case sig := <-s.C:
		return sig
	default:
		t.Fatal("expected a signal")
		return Signal{}
	}
}

func TestHubFansOutToEverySubscriberOfTheUser(t *testing.T) {
	h := NewHub(Options{})
	u := uuid.New()
	a, b := h.Subscribe(u, uuid.New()), h.Subscribe(u, uuid.New())
	if n := h.Publish(Signal{UserID: u, Kind: KindNotification, ID: "1"}); n != 2 {
		t.Fatalf("delivered to %d", n)
	}
	if recv(t, a).ID != "1" || recv(t, b).ID != "1" {
		t.Fatal("both subscribers must get the signal")
	}
}

func TestHubIsolatesUsers(t *testing.T) {
	h := NewHub(Options{})
	u1, u2 := uuid.New(), uuid.New()
	s1, s2 := h.Subscribe(u1, uuid.Nil), h.Subscribe(u2, uuid.Nil)
	h.Publish(Signal{UserID: u1, Kind: KindNotification, ID: "x"})
	if len(s2.C) != 0 {
		t.Fatal("signal leaked to another user")
	}
	if len(s1.C) != 1 {
		t.Fatal("owner did not receive")
	}
	if h.Publish(Signal{UserID: uuid.New(), Kind: KindNotification}) != 0 {
		t.Fatal("no subscribers expected")
	}
}

func TestHubDropsSlowConsumers(t *testing.T) {
	h := NewHub(Options{Buffer: 2})
	u := uuid.New()
	slow, fast := h.Subscribe(u, uuid.Nil), h.Subscribe(u, uuid.Nil)
	for i := 0; i < 3; i++ {
		h.Publish(Signal{UserID: u, Kind: KindNotification})
		<-fast.C
	}
	select {
	case <-slow.Done():
	default:
		t.Fatal("slow subscriber must be dropped")
	}
	if slow.Reason() != ReasonDropped {
		t.Fatalf("reason = %q", slow.Reason())
	}
	if h.Count(u) != 1 {
		t.Fatalf("count = %d", h.Count(u))
	}
	if fast.Reason() != "" {
		t.Fatal("reason is empty while open")
	}
}

func TestHubLimitReplacesTheOldest(t *testing.T) {
	h := NewHub(Options{MaxPerUser: 2})
	u := uuid.New()
	first := h.Subscribe(u, uuid.Nil)
	h.Subscribe(u, uuid.Nil)
	h.Subscribe(u, uuid.Nil)
	if first.Reason() != ReasonReplaced {
		t.Fatalf("oldest reason = %q", first.Reason())
	}
	if h.Count(u) != 2 {
		t.Fatalf("count = %d", h.Count(u))
	}
}

func TestHubCloseAndSubscriptionClose(t *testing.T) {
	h := NewHub(Options{})
	u := uuid.New()
	s := h.Subscribe(u, uuid.Nil)
	s.Close()
	s.Close()
	if s.Reason() != ReasonClosed || h.Count(u) != 0 {
		t.Fatalf("close: %q %d", s.Reason(), h.Count(u))
	}
	open := h.Subscribe(u, uuid.Nil)
	h.Close()
	if open.Reason() != ReasonShutdown {
		t.Fatalf("shutdown reason = %q", open.Reason())
	}
	if late := h.Subscribe(u, uuid.Nil); late.Reason() != ReasonShutdown {
		t.Fatal("closed hub must refuse subscriptions")
	}
}

// The hub is guarded by one mutex and every subscription ends through a sync.Once, so concurrent use is
// race-free by design; this test exercises it (the race detector is not available on every host).
func TestHubConcurrentUse(t *testing.T) {
	h := NewHub(Options{MaxPerUser: 3, Buffer: 4})
	u := uuid.New()
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(2)
		go func() {
			defer wg.Done()
			for j := 0; j < 50; j++ {
				s := h.Subscribe(u, uuid.New())
				select {
				case <-s.C:
				default:
				}
				s.Close()
			}
		}()
		go func() {
			defer wg.Done()
			for j := 0; j < 50; j++ {
				h.Publish(Signal{UserID: u, Kind: KindNotification, ID: "c"})
			}
		}()
	}
	wg.Wait()
	if h.Count(u) != 0 {
		t.Fatalf("leaked %d subscriptions", h.Count(u))
	}
}
