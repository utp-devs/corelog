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
	entry, ok := c.mapper.Map(domain.SourceEvent{
		ID:         e.ID,
		OrgID:      e.OrgID,
		ProjectID:  e.ProjectID,
		EntityID:   e.EntityID,
		ActorID:    e.ActorID,
		Type:       e.Type,
		EntityType: e.EntityType,
		Payload:    e.Payload,
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
