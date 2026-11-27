package postgres

import (
	"context"

	"github.com/jmoiron/sqlx"

	"github.com/jhonsferg/corelog/internal/platform/database"
	"github.com/jhonsferg/corelog/internal/platform/events"
	"github.com/jhonsferg/corelog/internal/project/application/port"
)

// EventPublisher writes project events to the domain events outbox. It joins the transaction in
// the context when there is one, so events commit together with the change they describe.
type EventPublisher struct {
	outbox *events.PostgresPublisher
}

// NewEventPublisher returns a publisher bound to db.
func NewEventPublisher(db *sqlx.DB) *EventPublisher {
	return &EventPublisher{
		outbox: events.NewPostgresPublisher(func(ctx context.Context) sqlx.ExtContext {
			return database.Executor(ctx, db)
		}),
	}
}

var _ port.EventPublisher = (*EventPublisher)(nil)

// PublishProjectEvent records a project event with the organization and project as scope.
func (p *EventPublisher) PublishProjectEvent(ctx context.Context, event port.ProjectEvent) error {
	orgID := event.OrganizationID
	projectID := event.ProjectID
	actorID := event.ActorID
	entityID := event.ProjectID
	return p.outbox.Publish(ctx, events.Event{
		OrgID:      &orgID,
		ProjectID:  &projectID,
		ActorID:    &actorID,
		EntityID:   &entityID,
		Type:       event.Type,
		EntityType: "project",
		Payload:    event.Payload,
	})
}
