package events

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
)

const (
	defaultBatchSize  = 100
	defaultMaxBackoff = 30 * time.Second
	defaultBaseDelay  = time.Second
	lockTimeout       = 10 * time.Second
)

// Options configures a Dispatcher. Zero values select the defaults.
type Options struct {
	BatchSize  int
	MaxBackoff time.Duration
	Logger     *slog.Logger
}

// Dispatcher delivers events from domain_events to consumers. Each consumer has
// its own cursor in event_cursors and an advisory lock so only one process
// dispatches it at a time. Delivery is at-least-once.
type Dispatcher struct {
	db        *sqlx.DB
	consumers []Consumer
	opts      Options
	baseDelay time.Duration

	mu       sync.Mutex
	failures map[string]*failureState
}

type failureState struct {
	count       int
	nextAttempt time.Time
}

// NewDispatcher validates the consumers and returns a dispatcher. Consumer
// names must be non-empty and unique.
func NewDispatcher(db *sqlx.DB, consumers []Consumer, opts Options) (*Dispatcher, error) {
	seen := make(map[string]struct{}, len(consumers))
	for _, c := range consumers {
		if c == nil {
			return nil, errors.New("events: nil consumer")
		}
		name := c.Name()
		if name == "" {
			return nil, errors.New("events: consumer name is empty")
		}
		if _, dup := seen[name]; dup {
			return nil, fmt.Errorf("events: duplicate consumer name %q", name)
		}
		seen[name] = struct{}{}
	}
	if opts.BatchSize <= 0 {
		opts.BatchSize = defaultBatchSize
	}
	if opts.MaxBackoff <= 0 {
		opts.MaxBackoff = defaultMaxBackoff
	}
	if opts.Logger == nil {
		opts.Logger = slog.Default()
	}
	return &Dispatcher{
		db:        db,
		consumers: consumers,
		opts:      opts,
		baseDelay: defaultBaseDelay,
		failures:  make(map[string]*failureState),
	}, nil
}

// RunOnce dispatches one batch per consumer. Consumer handler errors are logged
// and retried with backoff; only infrastructure errors are returned.
func (d *Dispatcher) RunOnce(ctx context.Context) error {
	var errs []error
	for _, c := range d.consumers {
		if err := ctx.Err(); err != nil {
			return errors.Join(append(errs, err)...)
		}
		if err := d.dispatchConsumer(ctx, c); err != nil {
			errs = append(errs, fmt.Errorf("events: dispatch %s: %w", c.Name(), err))
		}
	}
	return errors.Join(errs...)
}

// Run calls RunOnce every interval until ctx is cancelled.
func (d *Dispatcher) Run(ctx context.Context, interval time.Duration) error {
	if interval <= 0 {
		return errors.New("events: interval must be positive")
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		if err := d.RunOnce(ctx); err != nil && ctx.Err() == nil {
			d.opts.Logger.Error("event dispatch failed", slog.String("error", err.Error()))
		}
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
		}
	}
}

func lockKey(name string) string {
	return "events:" + name
}

func (d *Dispatcher) dispatchConsumer(ctx context.Context, c Consumer) error {
	name := c.Name()
	if d.inBackoff(name) {
		return nil
	}
	conn, err := d.db.Connx(ctx)
	if err != nil {
		return fmt.Errorf("acquire connection: %w", err)
	}
	defer func() { _ = conn.Close() }()

	var locked bool
	if err := conn.QueryRowContext(ctx,
		`SELECT pg_try_advisory_lock(hashtext($1))`, lockKey(name)).Scan(&locked); err != nil {
		return fmt.Errorf("try lock: %w", err)
	}
	if !locked {
		d.opts.Logger.Debug("event consumer locked by another process", slog.String("consumer", name))
		return nil
	}
	defer d.unlock(conn, lockKey(name))

	return d.deliverBatch(ctx, conn, c)
}

func (d *Dispatcher) deliverBatch(ctx context.Context, conn *sqlx.Conn, c Consumer) error {
	name := c.Name()
	if _, err := conn.ExecContext(ctx,
		`INSERT INTO event_cursors (consumer, last_event_id) VALUES ($1, 0) ON CONFLICT (consumer) DO NOTHING`,
		name); err != nil {
		return fmt.Errorf("ensure cursor: %w", err)
	}
	var cursor int64
	if err := conn.QueryRowContext(ctx,
		`SELECT last_event_id FROM event_cursors WHERE consumer = $1`, name).Scan(&cursor); err != nil {
		return fmt.Errorf("read cursor: %w", err)
	}

	batch, err := readEvents(ctx, conn, cursor, d.opts.BatchSize)
	if err != nil {
		return err
	}
	for _, e := range batch {
		if err := c.Handle(ctx, e); err != nil {
			d.recordFailure(name)
			d.opts.Logger.Warn("event handler failed; batch stopped",
				slog.String("consumer", name),
				slog.Int64("event_id", e.ID),
				slog.String("type", e.Type),
				slog.String("error", err.Error()))
			return nil
		}
		if _, err := conn.ExecContext(ctx,
			`UPDATE event_cursors SET last_event_id = $2, updated_at = now() WHERE consumer = $1`,
			name, e.ID); err != nil {
			return fmt.Errorf("advance cursor to %d: %w", e.ID, err)
		}
		cursor = e.ID
	}
	d.clearFailure(name)
	return nil
}

func readEvents(ctx context.Context, conn *sqlx.Conn, after int64, limit int) ([]Event, error) {
	rows, err := conn.QueryContext(ctx, `
SELECT id, org_id, project_id, entity_id, actor_id, type, entity_type, payload, created_at
FROM domain_events
WHERE id > $1
ORDER BY id
LIMIT $2`, after, limit)
	if err != nil {
		return nil, fmt.Errorf("read events: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var out []Event
	for rows.Next() {
		var (
			e                                   Event
			orgID, projectID, entityID, actorID uuid.NullUUID
			payload                             []byte
		)
		if err := rows.Scan(&e.ID, &orgID, &projectID, &entityID, &actorID,
			&e.Type, &e.EntityType, &payload, &e.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan event: %w", err)
		}
		e.OrgID = nullUUIDPtr(orgID)
		e.ProjectID = nullUUIDPtr(projectID)
		e.EntityID = nullUUIDPtr(entityID)
		e.ActorID = nullUUIDPtr(actorID)
		if err := json.Unmarshal(payload, &e.Payload); err != nil {
			return nil, fmt.Errorf("decode payload of event %d: %w", e.ID, err)
		}
		out = append(out, e)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate events: %w", err)
	}
	return out, nil
}

func nullUUIDPtr(n uuid.NullUUID) *uuid.UUID {
	if !n.Valid {
		return nil
	}
	id := n.UUID
	return &id
}

func (d *Dispatcher) unlock(conn *sqlx.Conn, key string) {
	ctx, cancel := context.WithTimeout(context.WithoutCancel(context.Background()), lockTimeout)
	defer cancel()
	if _, err := conn.ExecContext(ctx, `SELECT pg_advisory_unlock(hashtext($1))`, key); err != nil && !errors.Is(err, sql.ErrConnDone) {
		d.opts.Logger.Error("release event lock failed", slog.String("lock", key), slog.String("error", err.Error()))
	}
}

func (d *Dispatcher) inBackoff(name string) bool {
	d.mu.Lock()
	defer d.mu.Unlock()
	st, ok := d.failures[name]
	return ok && time.Now().Before(st.nextAttempt)
}

func (d *Dispatcher) recordFailure(name string) {
	d.mu.Lock()
	defer d.mu.Unlock()
	st, ok := d.failures[name]
	if !ok {
		st = &failureState{}
		d.failures[name] = st
	}
	st.count++
	delay := d.baseDelay
	for i := 1; i < st.count && delay < d.opts.MaxBackoff; i++ {
		delay *= 2
	}
	if delay > d.opts.MaxBackoff {
		delay = d.opts.MaxBackoff
	}
	st.nextAttempt = time.Now().Add(delay)
}

func (d *Dispatcher) clearFailure(name string) {
	d.mu.Lock()
	defer d.mu.Unlock()
	delete(d.failures, name)
}
