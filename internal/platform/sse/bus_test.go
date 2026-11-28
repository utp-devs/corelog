package sse

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"

	_ "github.com/jackc/pgx/v5/stdlib"
)

func TestEncodeDecode(t *testing.T) {
	u := uuid.New()
	sig, ok := decode(encode(Signal{UserID: u, Kind: KindSessionsRevokedExcept, ID: ""}))
	if !ok || sig.UserID != u || sig.Kind != KindSessionsRevokedExcept || sig.ID != "" {
		t.Fatalf("round trip: %+v %v", sig, ok)
	}
	for _, bad := range []string{"", "n|x", "n|not-a-uuid|1", "|" + u.String() + "|1"} {
		if _, ok := decode(bad); ok {
			t.Errorf("%q must not decode", bad)
		}
	}
}

func TestBusWithoutDatabaseDeliversLocally(t *testing.T) {
	hub := NewHub(Options{})
	bus := NewBus(hub, nil, "", "", nil)
	u := uuid.New()
	s := hub.Subscribe(u, uuid.Nil)
	bus.Publish(context.Background(), Signal{UserID: u, Kind: KindNotification, ID: "1"})
	if recv(t, s).ID != "1" {
		t.Fatal("local delivery expected")
	}
	if bus.Listening() {
		t.Fatal("not listening without a dsn")
	}
	bus.Run(context.Background()) // returns at once: no dsn
	bad := NewBus(hub, nil, "x", "bad channel;", nil)
	bad.Run(context.Background())
}

// Two buses (two API instances) share a database: a signal published through one reaches the subscriber of
// the other through LISTEN/NOTIFY, and the publisher does not deliver twice to its own subscribers.
func TestBusDeliversAcrossInstances(t *testing.T) {
	dsn := os.Getenv("CORELOG_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("set CORELOG_TEST_DATABASE_URL to run PostgreSQL integration tests")
	}
	db, err := sqlx.Open("pgx", dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Close() }()
	channel := "corelog_test_" + strings.ReplaceAll(uuid.NewString()[:12], "-", "")
	hubA, hubB := NewHub(Options{}), NewHub(Options{})
	busA, busB := NewBus(hubA, db, dsn, channel, nil), NewBus(hubB, db, dsn, channel, nil)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go busA.Run(ctx)
	go busB.Run(ctx)
	deadline := time.Now().Add(10 * time.Second)
	for !(busA.Listening() && busB.Listening()) {
		if time.Now().After(deadline) {
			t.Fatal("listeners did not start")
		}
		time.Sleep(20 * time.Millisecond)
	}

	u := uuid.New()
	onA, onB := hubA.Subscribe(u, uuid.Nil), hubB.Subscribe(u, uuid.Nil)
	other := hubB.Subscribe(uuid.New(), uuid.Nil)
	busA.Publish(ctx, Signal{UserID: u, Kind: KindNotification, ID: "abc"})
	for name, s := range map[string]*Subscription{"A": onA, "B": onB} {
		select {
		case sig := <-s.C:
			if sig.ID != "abc" || sig.UserID != u {
				t.Errorf("%s got %+v", name, sig)
			}
		case <-time.After(5 * time.Second):
			t.Fatalf("instance %s did not receive the signal", name)
		}
	}
	time.Sleep(100 * time.Millisecond)
	if len(onA.C) != 0 || len(other.C) != 0 {
		t.Fatalf("duplicate or leaked delivery: %d %d", len(onA.C), len(other.C))
	}
}
