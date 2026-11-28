// Package usecase implements the audit log: the event consumer that stores
// entries and the retention job that purges old ones.
package usecase

import (
	"context"
	"fmt"

	"github.com/jhonsferg/corelog/internal/audit/application/port"
	"github.com/jhonsferg/corelog/internal/audit/domain"
	"github.com/jhonsferg/corelog/internal/platform/clock"
	"github.com/jhonsferg/corelog/internal/platform/events"
)

// ConsumerName keys the audit cursor in event_cursors and its advisory lock.
const ConsumerName = "audit"

// Consumer maps domain events to audit entries and stores them. It implements
// events.Consumer. Handle is idempotent: the repository ignores entries whose
// source event was already stored.
type Consumer struct {
	repo   port.Repository
	mapper *domain.Mapper
	clk    clock.Clock
}

var _ events.Consumer = (*Consumer)(nil)

// NewConsumer returns a consumer that uses the default rule table.
func NewConsumer(repo port.Repository, clk clock.Clock) *Consumer {
	return &Consumer{repo: repo, mapper: domain.NewMapper(domain.DefaultRules), clk: clk}
}

// Name returns the consumer name "audit".
func (c *Consumer) Name() string { return ConsumerName }

// Handle stores an audit entry for events that have a mapping rule. Other
// event types are skipped without error.
func (c *Consumer) Handle(ctx context.Context, e events.Event) error {
	payload := c.withActorName(ctx, e)
	entry, ok := c.mapper.Map(domain.SourceEvent{
		ID:         e.ID,
		OrgID:      e.OrgID,
		ProjectID:  e.ProjectID,
		EntityID:   e.EntityID,
		ActorID:    e.ActorID,
		Type:       e.Type,
		EntityType: e.EntityType,
		Payload:    payload,
		CreatedAt:  e.CreatedAt,
	})
	if !ok {
		return nil
	}
	if entry.CreatedAt.IsZero() {
		entry.CreatedAt = c.clk.Now()
	}
	if err := c.repo.Save(ctx, entry); err != nil {
		return fmt.Errorf("audit: save entry for event %d: %w", e.ID, err)
	}
	return nil
}

// withActorName returns the event payload with actor_name filled from the user directory when the producer
// did not include it. Lookup failures fall back to the generic actor name instead of failing the event.
func (c *Consumer) withActorName(ctx context.Context, e events.Event) map[string]any {
	if e.ActorID == nil {
		return e.Payload
	}
	if name, _ := e.Payload["actor_name"].(string); name != "" {
		return e.Payload
	}
	name, err := c.repo.ActorName(ctx, *e.ActorID)
	if err != nil || name == "" {
		return e.Payload
	}
	out := make(map[string]any, len(e.Payload)+1)
	for k, v := range e.Payload {
		out[k] = v
	}
	out["actor_name"] = name
	return out
}
