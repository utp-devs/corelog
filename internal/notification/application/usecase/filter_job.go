package usecase

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"

	"github.com/jhonsferg/corelog/internal/notification/application/port"
	"github.com/jhonsferg/corelog/internal/notification/domain"
	"github.com/jhonsferg/corelog/internal/platform/clock"
)

const filterRowsShown = 50

// FilterSubscriptionJob emails the results of the saved filters people subscribed to. It runs each filter as its
// subscriber, so nobody receives issues they cannot browse.
type FilterSubscriptionJob struct {
	subs    port.FilterSubscriptions
	facts   port.FactsSource
	notes   port.Repository
	outbox  port.Outbox
	render  port.EmailRenderer
	clock   clock.Clock
	baseURL string
}

// NewFilterSubscriptionJob wires the job.
func NewFilterSubscriptionJob(subs port.FilterSubscriptions, facts port.FactsSource, notes port.Repository, outbox port.Outbox,
	render port.EmailRenderer, clk clock.Clock, baseURL string) *FilterSubscriptionJob {
	return &FilterSubscriptionJob{subs: subs, facts: facts, notes: notes, outbox: outbox, render: render, clock: clk, baseURL: trimSlash(baseURL)}
}

// Run delivers every due subscription and returns how many emails it queued. A subscriber who turned email off
// (or deactivated) is marked delivered without sending, so the subscription does not stay due.
func (j *FilterSubscriptionJob) Run(ctx context.Context) (int, error) {
	due, err := j.subs.Due(ctx)
	if err != nil {
		return 0, err
	}
	queued := 0
	var failures error
	for _, item := range due {
		sent, err := j.deliver(ctx, item)
		if err != nil {
			// One broken subscription must not starve the others.
			failures = errors.Join(failures, fmt.Errorf("subscription %s: %w", item.SubscriptionID, err))
			continue
		}
		if sent {
			queued++
		}
	}
	return queued, failures
}

func (j *FilterSubscriptionJob) deliver(ctx context.Context, item port.DueFilter) (bool, error) {
	contacts, err := j.facts.Contacts(ctx, []uuid.UUID{item.UserID})
	if err != nil {
		return false, err
	}
	contact := contacts[item.UserID]
	prefs, err := j.notes.GetPreferences(ctx, item.UserID)
	if err != nil {
		return false, err
	}
	if contact.ID == uuid.Nil || !contact.Active || contact.Email == "" || prefs.Email == domain.EmailOff {
		return false, j.subs.MarkDelivered(ctx, item.SubscriptionID)
	}
	rows, total, err := j.subs.Run(ctx, item, filterRowsShown)
	if err != nil {
		return false, err
	}
	lines := make([]port.EmailLine, 0, len(rows))
	for _, row := range rows {
		lines = append(lines, port.EmailLine{Label: row.Key, Text: row.Summary, Detail: row.Status, Link: j.baseURL + "/browse/" + row.Key})
	}
	now := j.clock.Now()
	rendered := j.render.FilterSubscription(port.Email{
		Event: domain.EventFilterSubscription, Greeting: contact.Name, FilterName: item.FilterName, Lines: lines, Total: total,
		Link: j.baseURL + "/issues?filter=" + item.FilterID.String(), Preference: j.baseURL + "/profile/notifications",
	})
	last := int64(0)
	if item.LastSentAt != nil {
		last = item.LastSentAt.Unix()
	}
	if _, err := j.outbox.Enqueue(ctx, port.EmailMessage{
		UserID: item.UserID, To: contact.Email, Kind: string(domain.EventFilterSubscription), Subject: rendered.Subject,
		Text: rendered.Text, HTML: rendered.HTML, IdempotencyKey: fmt.Sprintf("filtersub:%s:%d", item.SubscriptionID, last),
	}, now); err != nil {
		return false, err
	}
	return true, j.subs.MarkDelivered(ctx, item.SubscriptionID)
}
