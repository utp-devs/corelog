package sse

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"regexp"
	"strings"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jmoiron/sqlx"

	"github.com/jhonsferg/corelog/internal/platform/database"
)

// DefaultChannel is the PostgreSQL channel the instances share.
const DefaultChannel = "corelog_events"

var channelPattern = regexp.MustCompile(`^[a-z_][a-z0-9_]{0,62}$`)

// Bus publishes signals to every API instance through LISTEN/NOTIFY. The payload is "kind|user|id": just
// identifiers. NOTIFY runs on the transaction found in the context, so it is delivered only after the commit.
// While the LISTEN connection is not established the bus delivers to the local hub directly, so a single
// instance keeps working without it (the subscriber then retries loading a row that is not committed yet).
type Bus struct {
	hub       *Hub
	db        *sqlx.DB
	dsn       string
	channel   string
	log       *slog.Logger
	retry     time.Duration
	listening atomic.Bool
}

// NewBus builds the bus. dsn is the connection string of the dedicated LISTEN connection; an empty dsn or db
// makes the bus local only.
func NewBus(hub *Hub, db *sqlx.DB, dsn, channel string, log *slog.Logger) *Bus {
	if channel == "" {
		channel = DefaultChannel
	}
	if log == nil {
		log = slog.Default()
	}
	return &Bus{hub: hub, db: db, dsn: dsn, channel: channel, log: log, retry: 3 * time.Second}
}

// Listening reports whether the LISTEN connection is up.
func (b *Bus) Listening() bool { return b.listening.Load() }

func encode(sig Signal) string {
	return sig.Kind + "|" + sig.UserID.String() + "|" + sig.ID
}

func decode(payload string) (Signal, bool) {
	parts := strings.SplitN(payload, "|", 3)
	if len(parts) != 3 {
		return Signal{}, false
	}
	id, err := uuid.Parse(parts[1])
	if err != nil || parts[0] == "" {
		return Signal{}, false
	}
	return Signal{Kind: parts[0], UserID: id, ID: parts[2]}, true
}

// Publish sends the signal to every instance. Failures are logged, never returned: an event that cannot be
// pushed is still found by the client on its next reconnect or list call.
func (b *Bus) Publish(ctx context.Context, sig Signal) {
	if b.db != nil {
		if _, err := database.Executor(ctx, b.db).ExecContext(ctx, `SELECT pg_notify($1, $2)`, b.channel, encode(sig)); err != nil {
			b.log.Warn("stream notify failed", "error", err)
		}
	}
	if !b.listening.Load() {
		b.hub.Publish(sig)
	}
}

// Run keeps a LISTEN connection until ctx ends, reconnecting after failures. Every received signal goes to
// the local hub. It returns immediately when the bus has no dsn.
func (b *Bus) Run(ctx context.Context) {
	if b.dsn == "" || !channelPattern.MatchString(b.channel) {
		return
	}
	for ctx.Err() == nil {
		if err := b.listenOnce(ctx); err != nil && ctx.Err() == nil {
			b.log.Warn("stream listener lost, retrying", "error", err)
		}
		b.listening.Store(false)
		select {
		case <-ctx.Done():
			return
		case <-time.After(b.retry):
		}
	}
}

func (b *Bus) listenOnce(ctx context.Context) error {
	conn, err := pgx.Connect(ctx, b.dsn)
	if err != nil {
		return fmt.Errorf("connect: %w", err)
	}
	defer func() { _ = conn.Close(context.Background()) }()
	if _, err := conn.Exec(ctx, "LISTEN "+b.channel); err != nil {
		return fmt.Errorf("listen: %w", err)
	}
	b.listening.Store(true)
	for {
		n, err := conn.WaitForNotification(ctx)
		if err != nil {
			if errors.Is(err, context.Canceled) {
				return nil
			}
			return err
		}
		if sig, ok := decode(n.Payload); ok {
			b.hub.Publish(sig)
		}
	}
}
