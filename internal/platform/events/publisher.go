package events

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
)

// ErrInvalidEvent is returned when an event cannot be published because it is
// missing required fields or its payload cannot be encoded.
var ErrInvalidEvent = errors.New("events: invalid event")

// Publisher records domain events.
type Publisher interface {
	Publish(ctx context.Context, e Event) error
}

// ExecutorFunc returns the database executor the publisher should use for the
// current call. Returning the transaction from the caller's context lets the
// event commit or roll back together with the change it describes.
type ExecutorFunc func(ctx context.Context) sqlx.ExtContext

// PostgresPublisher stores events in the domain_events table (transactional outbox).
type PostgresPublisher struct {
	exec ExecutorFunc
}

// NewPostgresPublisher creates a publisher that writes through exec.
func NewPostgresPublisher(exec ExecutorFunc) *PostgresPublisher {
	return &PostgresPublisher{exec: exec}
}

const insertEventSQL = `
INSERT INTO domain_events (org_id, project_id, type, entity_type, entity_id, actor_id, payload, created_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`

// Publish inserts the event with a parameterised statement.
func (p *PostgresPublisher) Publish(ctx context.Context, e Event) error {
	if e.Type == "" || e.EntityType == "" {
		return fmt.Errorf("%w: type and entity type are required", ErrInvalidEvent)
	}
	if p.exec == nil {
		return errors.New("events: publisher has no executor")
	}
	exec := p.exec(ctx)
	if exec == nil {
		return errors.New("events: publisher executor returned nil")
	}
	payload := e.Payload
	if payload == nil {
		payload = map[string]any{}
	}
	raw, err := json.Marshal(withMeta(ctx, payload))
	if err != nil {
		return fmt.Errorf("%w: encode payload: %v", ErrInvalidEvent, err)
	}
	createdAt := e.CreatedAt
	if createdAt.IsZero() {
		createdAt = time.Now().UTC()
	}
	if _, err := exec.ExecContext(ctx, insertEventSQL,
		nullableUUID(e.OrgID), nullableUUID(e.ProjectID), e.Type, e.EntityType,
		nullableUUID(e.EntityID), nullableUUID(e.ActorID), string(raw), createdAt,
	); err != nil {
		return fmt.Errorf("events: insert %s: %w", e.Type, err)
	}
	return nil
}

func nullableUUID(id *uuid.UUID) any {
	if id == nil {
		return nil
	}
	return *id
}
