package main

import (
	"context"
	"io"
	"log/slog"
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/jhonsferg/corelog/internal/platform/config"
)

// settledGoroutines waits for the goroutine count to stop changing and returns it.
func settledGoroutines() int {
	last := -1
	for i := 0; i < 40; i++ {
		runtime.GC()
		n := runtime.NumGoroutine()
		if n == last {
			return n
		}
		last = n
		time.Sleep(25 * time.Millisecond)
	}
	return last
}

// TestBackgroundWorkersStopWithTheirContext starts every background worker of the API (comment mentions, audit,
// integrations with webhook delivery, imports and scheduled rules, and the job scheduler), lets them run, cancels
// the context and checks that each one returns in time and that no goroutine is left behind.
func TestBackgroundWorkersStopWithTheirContext(t *testing.T) {
	f := newAuthzFixture(t) // builds the real router, which publishes the integration services
	cfg := &config.Config{}
	log := slog.New(slog.NewTextHandler(io.Discard, nil))

	// Warm up once so pools and lazily started helpers are part of the baseline.
	warm, stopWarm := context.WithCancel(context.Background())
	var warmWG sync.WaitGroup
	warmWG.Add(1)
	go func() { defer warmWG.Done(); _ = startCommentMentionDispatcher(warm, cfg, f.db, log) }()
	time.Sleep(200 * time.Millisecond)
	stopWarm()
	warmWG.Wait()
	baseline := settledGoroutines()

	ctx, cancel := context.WithCancel(context.Background())
	workers := map[string]func(){
		"comment mentions": func() { _ = startCommentMentionDispatcher(ctx, cfg, f.db, log) },
		"audit":            func() { _ = startSecurityRuntime(ctx, cfg, f.db, log) },
		"integrations":     func() { _ = startIntegrationRuntime(ctx, f.db, log) },
		"jobs":             func() { startJobs(ctx, cfg, f.db, log) },
	}
	done := map[string]chan struct{}{}
	for name, run := range workers {
		ch := make(chan struct{})
		done[name] = ch
		go func() { defer close(ch); run() }()
	}
	time.Sleep(1500 * time.Millisecond) // long enough for several polls
	cancel()

	deadline := time.After(10 * time.Second)
	for name, ch := range done {
		select {
		case <-ch:
		case <-deadline:
			t.Fatalf("worker %q did not stop after its context was cancelled", name)
		}
	}
	after := settledGoroutines()
	if after > baseline+2 {
		buf := make([]byte, 1<<16)
		n := runtime.Stack(buf, true)
		t.Fatalf("goroutines after the workers stopped: %d, before: %d\n%s", after, baseline, buf[:n])
	}
}
