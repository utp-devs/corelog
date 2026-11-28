package usecase

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"

	"github.com/jhonsferg/corelog/internal/notification/application/port"
	"github.com/jhonsferg/corelog/internal/notification/domain"
	"github.com/jhonsferg/corelog/internal/platform/clock"
	"github.com/jhonsferg/corelog/internal/platform/database"
	"github.com/jhonsferg/corelog/internal/platform/mailer"
)

// MaxEmailAttempts is the number of failed attempts after which a message becomes dead.
const MaxEmailAttempts = 6

// emailBackoff is the wait after the first, second, ... failed attempt.
var emailBackoff = []time.Duration{time.Minute, 5 * time.Minute, 30 * time.Minute, 2 * time.Hour, 12 * time.Hour}

// RetryDelay returns the wait before the next attempt after `attempts` failures, and false when the message is
// dead.
func RetryDelay(attempts int) (time.Duration, bool) {
	if attempts >= MaxEmailAttempts || attempts < 1 {
		return 0, false
	}
	return emailBackoff[attempts-1], true
}

// OutboxWorker sends the queued emails. Several workers (API replicas) may run it at the same time: each message
// is leased to one of them.
type OutboxWorker struct {
	outbox port.Outbox
	mailer mailer.Mailer
	clock  clock.Clock
	logger *slog.Logger
	batch  int
	lease  time.Duration
}

// NewOutboxWorker wires the worker. A batch of 0 means 25; the lease covers the time a batch may take.
func NewOutboxWorker(outbox port.Outbox, m mailer.Mailer, clk clock.Clock, logger *slog.Logger, batch int) *OutboxWorker {
	if batch <= 0 {
		batch = 25
	}
	if logger == nil {
		logger = slog.Default()
	}
	return &OutboxWorker{outbox: outbox, mailer: m, clock: clk, logger: logger, batch: batch, lease: 5 * time.Minute}
}

// Run sends one batch and returns how many messages were sent and how many failed.
func (w *OutboxWorker) Run(ctx context.Context) (sent, failed int, err error) {
	now := w.clock.Now()
	entries, err := w.outbox.ClaimDue(ctx, now, w.batch, w.lease)
	if err != nil {
		return 0, 0, err
	}
	for _, entry := range entries {
		if ctx.Err() != nil {
			// Unsent leases expire by themselves.
			return sent, failed, ctx.Err()
		}
		sendErr := w.mailer.Send(ctx, mailer.Message{To: entry.To, Subject: entry.Subject, Text: entry.Text, HTML: entry.HTML})
		at := w.clock.Now()
		if sendErr == nil {
			if err := w.outbox.MarkSent(ctx, entry.ID, at); err != nil {
				return sent, failed, err
			}
			sent++
			continue
		}
		failed++
		attempts := entry.Attempts + 1
		var next *time.Time
		if delay, ok := RetryDelay(attempts); ok {
			t := at.Add(delay)
			next = &t
		}
		// The error text may name the SMTP server but never the message body; the address is not logged.
		w.logger.Warn("email delivery failed", slog.String("id", entry.ID.String()), slog.Int("attempts", attempts), slog.Bool("dead", next == nil))
		if err := w.outbox.MarkFailed(ctx, entry.ID, attempts, next, truncateError(sendErr), at); err != nil {
			return sent, failed, err
		}
	}
	return sent, failed, nil
}

func truncateError(err error) string {
	text := err.Error()
	if len(text) > 500 {
		text = text[:500]
	}
	return text
}

// DigestJob bundles the notices of users in daily mode into one email per user.
type DigestJob struct {
	delivery port.DeliveryStore
	facts    port.FactsSource
	outbox   port.Outbox
	render   port.EmailRenderer
	notes    port.Repository
	tx       database.TxManager
	clock    clock.Clock
	baseURL  string
}

// NewDigestJob wires the job.
func NewDigestJob(delivery port.DeliveryStore, facts port.FactsSource, outbox port.Outbox, render port.EmailRenderer, notes port.Repository,
	tx database.TxManager, clk clock.Clock, baseURL string) *DigestJob {
	return &DigestJob{delivery: delivery, facts: facts, outbox: outbox, render: render, notes: notes, tx: tx, clock: clk, baseURL: trimSlash(baseURL)}
}

func trimSlash(s string) string {
	for len(s) > 0 && s[len(s)-1] == '/' {
		s = s[:len(s)-1]
	}
	return s
}

const (
	digestUsersPerRun = 500
	digestItemsMax    = 200
	digestLinesShown  = 50
)

// Run builds the digests of every user with pending items and returns how many emails it queued.
func (j *DigestJob) Run(ctx context.Context) (int, error) {
	users, err := j.delivery.UsersWithDigest(ctx, digestUsersPerRun)
	if err != nil {
		return 0, err
	}
	queued := 0
	for _, userID := range users {
		ok, err := j.digestUser(ctx, userID)
		if err != nil {
			return queued, err
		}
		if ok {
			queued++
		}
	}
	return queued, nil
}

func (j *DigestJob) digestUser(ctx context.Context, userID uuid.UUID) (bool, error) {
	items, err := j.delivery.DigestItems(ctx, userID, digestItemsMax)
	if err != nil || len(items) == 0 {
		return false, err
	}
	contacts, err := j.facts.Contacts(ctx, []uuid.UUID{userID})
	if err != nil {
		return false, err
	}
	contact := contacts[userID]
	ids := make([]int64, 0, len(items))
	for _, item := range items {
		ids = append(ids, item.ID)
	}
	queued := false
	err = j.tx.InTx(ctx, func(ctx context.Context) error {
		now := j.clock.Now()
		if contact.ID != userID || !contact.Active || contact.Email == "" {
			return j.delivery.MarkDigested(ctx, ids, now)
		}
		lines := make([]port.EmailLine, 0, len(items))
		for _, item := range items {
			if len(lines) == digestLinesShown {
				break
			}
			link := j.baseURL
			if item.IssueKey != "" {
				link += "/browse/" + item.IssueKey
			}
			lines = append(lines, port.EmailLine{Label: item.IssueKey, Text: item.Summary, Detail: describeDigest(item), Link: link})
		}
		rendered := j.render.Digest(port.Email{
			Greeting: contact.Name, Lines: lines, Total: len(items), Preference: j.baseURL + "/profile/notifications",
		})
		_, err := j.outbox.Enqueue(ctx, port.EmailMessage{
			UserID: userID, To: contact.Email, Kind: "digest", Subject: rendered.Subject, Text: rendered.Text, HTML: rendered.HTML,
			IdempotencyKey: fmt.Sprintf("digest:%s:%d", userID, ids[len(ids)-1]),
		}, now)
		if err != nil {
			return err
		}
		queued = true
		return j.delivery.MarkDigested(ctx, ids, now)
	})
	return queued, err
}

func describeDigest(item port.DigestItem) string {
	return domain.Describe(item.Event, "", "", item.ActorName, "")
}
