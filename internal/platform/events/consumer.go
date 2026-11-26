package events

import "context"

// Consumer processes domain events. Delivery is at-least-once, so Handle must be
// idempotent: the same event may arrive more than once after a failure.
type Consumer interface {
	// Name identifies the consumer. It keys the cursor and the advisory lock.
	Name() string
	// Handle processes one event. Returning an error stops the batch and the
	// event is retried later.
	Handle(ctx context.Context, e Event) error
}
