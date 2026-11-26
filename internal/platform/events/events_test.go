package events

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/google/uuid"
)

func TestNewDispatcherValidatesConsumers(t *testing.T) {
	t.Parallel()
	if _, err := NewDispatcher(nil, []Consumer{&fakeConsumer{name: ""}}, Options{}); err == nil {
		t.Fatal("expected error for empty consumer name")
	}
	if _, err := NewDispatcher(nil, []Consumer{&fakeConsumer{name: "a"}, &fakeConsumer{name: "a"}}, Options{}); err == nil {
		t.Fatal("expected error for duplicate consumer name")
	}
	if _, err := NewDispatcher(nil, []Consumer{nil}, Options{}); err == nil {
		t.Fatal("expected error for nil consumer")
	}
	d, err := NewDispatcher(nil, []Consumer{&fakeConsumer{name: "a"}}, Options{})
	if err != nil {
		t.Fatalf("NewDispatcher: %v", err)
	}
	if d.opts.BatchSize != defaultBatchSize || d.opts.MaxBackoff != defaultMaxBackoff || d.opts.Logger == nil {
		t.Fatalf("defaults not applied: %+v", d.opts)
	}
}

func TestPostgresPublisherRejectsInvalidEvent(t *testing.T) {
	t.Parallel()
	p := NewPostgresPublisher(nil)
	err := p.Publish(context.Background(), Event{})
	if !errors.Is(err, ErrInvalidEvent) {
		t.Fatalf("expected ErrInvalidEvent, got %v", err)
	}
	err = p.Publish(context.Background(), Event{Type: "issue.created", EntityType: "issue"})
	if err == nil {
		t.Fatal("expected error when executor is nil")
	}
}

func TestMemoryPublisherCollectsConcurrently(t *testing.T) {
	t.Parallel()
	m := NewMemoryPublisher()
	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			id := uuid.New()
			if err := m.Publish(context.Background(), Event{Type: "issue.created", EntityType: "issue", EntityID: &id}); err != nil {
				t.Errorf("Publish: %v", err)
			}
		}()
	}
	wg.Wait()
	if got := len(m.Events()); got != 50 {
		t.Fatalf("expected 50 events, got %d", got)
	}
}

type fakeConsumer struct {
	name   string
	handle func(ctx context.Context, e Event) error
}

func (f *fakeConsumer) Name() string { return f.name }

func (f *fakeConsumer) Handle(ctx context.Context, e Event) error {
	if f.handle == nil {
		return nil
	}
	return f.handle(ctx, e)
}
