package events

import (
	"context"
	"sync"
)

// MemoryPublisher collects published events in memory. It is intended for tests.
type MemoryPublisher struct {
	mu     sync.Mutex
	events []Event
}

// NewMemoryPublisher returns an empty in-memory publisher.
func NewMemoryPublisher() *MemoryPublisher {
	return &MemoryPublisher{}
}

// Publish appends the event.
func (m *MemoryPublisher) Publish(_ context.Context, e Event) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.events = append(m.events, e)
	return nil
}

// Events returns a copy of the collected events in publish order.
func (m *MemoryPublisher) Events() []Event {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]Event, len(m.events))
	copy(out, m.events)
	return out
}
