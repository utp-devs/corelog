package usecase

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/jhonsferg/corelog/internal/platform/events"
	"github.com/jhonsferg/corelog/internal/webhook/application/port"
	"github.com/jhonsferg/corelog/internal/webhook/domain"
)

const (
	// consumerName is the cursor name of the event consumer.
	consumerName = "webhooks"
	claimBatch   = 50
	claimLease   = 2 * time.Minute
)

// Consumer returns the events consumer that turns domain events into pending deliveries.
func (s *Service) Consumer() events.Consumer { return consumer{s: s} }

type consumer struct{ s *Service }

func (consumer) Name() string { return consumerName }

func (c consumer) Handle(ctx context.Context, e events.Event) error { return c.s.Capture(ctx, e) }

// Capture stores a pending delivery for every webhook that subscribes to the event. It is idempotent per
// webhook and event id, so at-least-once dispatch does not duplicate deliveries.
func (s *Service) Capture(ctx context.Context, e events.Event) error {
	if !domain.IsSubscribable(e.Type) || e.Type == domain.EventWildcard {
		return nil
	}
	info, ok, err := s.Describer.Describe(ctx, e)
	if err != nil {
		return err
	}
	if !ok {
		return nil
	}
	var projectID *uuid.UUID
	if info.Project != nil {
		projectID = &info.Project.ID
	}
	hooks, err := s.Repo.Matching(ctx, info.OrgID, projectID, e.Type)
	if err != nil || len(hooks) == 0 {
		return err
	}
	payload, err := json.Marshal(BuildPayload(e, info))
	if err != nil {
		return err
	}
	now := s.Clock.Now()
	for _, h := range hooks {
		if h.FilterQuery != nil && s.Matcher != nil && info.Issue != nil && info.Project != nil {
			match, err := s.Matcher.Matches(ctx, info.Project.ID, info.Issue.ID, *h.FilterQuery)
			if err != nil {
				// A broken filter must not block the other webhooks; the delivery is skipped.
				slog.Warn("webhook filter failed", "webhook_id", h.ID.String())
				continue
			}
			if !match {
				continue
			}
		}
		id := e.ID
		d := domain.Delivery{WebhookID: h.ID, EventType: e.Type, Payload: payload, Status: domain.StatusPending, NextAttemptAt: &now, CreatedAt: now}
		if id != 0 {
			d.EventID = &id
		}
		if _, err := s.Repo.InsertDelivery(ctx, &d); err != nil {
			return err
		}
	}
	return nil
}

// BuildPayload builds the v1 envelope: {version,id,event,occurred_at,organization_id,project,issue?,actor,changes?}.
func BuildPayload(e events.Event, info port.EventInfo) map[string]any {
	out := map[string]any{
		"version":         domain.PayloadVersion,
		"id":              eventID(e.ID),
		"event":           e.Type,
		"occurred_at":     e.CreatedAt.UTC().Format(time.RFC3339),
		"organization_id": info.OrgID.String(),
		"project":         nil,
	}
	if info.Project != nil {
		out["project"] = map[string]any{"id": info.Project.ID.String(), "key": info.Project.Key, "name": info.Project.Name}
	}
	if info.Issue != nil {
		is := map[string]any{
			"id": info.Issue.ID.String(), "key": info.Issue.Key, "summary": info.Issue.Summary, "status": info.Issue.Status,
			"priority": info.Issue.Priority, "type": info.Issue.IssueType, "reporter_id": info.Issue.ReporterID.String(),
			"assignee_id": nil,
		}
		if info.Issue.AssigneeID != nil {
			is["assignee_id"] = info.Issue.AssigneeID.String()
		}
		out["issue"] = is
	}
	actor := map[string]any{"id": nil, "name": info.ActorName}
	if e.ActorID != nil {
		actor["id"] = e.ActorID.String()
	}
	out["actor"] = actor
	if changes := changesOf(e); changes != nil {
		out["changes"] = changes
	}
	return out
}

func changesOf(e events.Event) any {
	if list, ok := e.Payload["changes"]; ok {
		return list
	}
	if e.Type == "issue.transitioned" {
		return []map[string]any{{"field": "status", "from": e.Payload["from_status"], "to": e.Payload["to_status"]}}
	}
	return nil
}

// RunDue sends the deliveries that are due and returns how many were attempted.
func (s *Service) RunDue(ctx context.Context) (int, error) {
	due, err := s.Repo.ClaimDue(ctx, s.Clock.Now(), claimBatch, claimLease)
	if err != nil {
		return 0, err
	}
	for _, d := range due {
		if err := ctx.Err(); err != nil {
			return 0, err
		}
		if err := s.deliver(ctx, d, true); err != nil {
			slog.Error("webhook delivery bookkeeping failed", "delivery_id", d.ID.String(), "error", err)
		}
	}
	return len(due), nil
}

// Run calls RunDue every interval until ctx is done.
func (s *Service) Run(ctx context.Context, interval time.Duration) {
	t := time.NewTicker(interval)
	defer t.Stop()
	for {
		if _, err := s.RunDue(ctx); err != nil && ctx.Err() == nil {
			slog.Error("webhook delivery run failed", "error", err)
		}
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		}
	}
}

// deliver sends one delivery and records the outcome. With retry false a failure is final (used by pings).
func (s *Service) deliver(ctx context.Context, d port.DueDelivery, retry bool) error {
	secret, err := s.Box.Open(d.SecretEncrypted)
	if err != nil {
		return s.record(ctx, d, domain.StatusDead, nil, nil, "no se pudo abrir el secreto del webhook", 0)
	}
	now := s.Clock.Now()
	ts := now.Unix()
	body := d.Payload
	res := s.Sender.Send(ctx, port.SendRequest{URL: d.URL, Body: body, Headers: map[string]string{
		"Content-Type":         "application/json",
		"User-Agent":           "CoreLog-Webhooks/1",
		domain.HeaderSignature: domain.Sign(secret, ts, body),
		domain.HeaderTimestamp: strconv.FormatInt(ts, 10),
		domain.HeaderEvent:     d.EventType,
		domain.HeaderDelivery:  d.ID.String(),
	}})
	attempts := d.Attempts + 1
	success := res.Err == nil && res.Status >= http.StatusOK && res.Status < http.StatusMultipleChoices
	status, next := domain.NextAttempt(success, attempts, s.Clock.Now())
	if !retry && !success {
		status, next = domain.StatusFailed, nil
	}
	var code *int
	excerpt := domain.Excerpt(res.Body)
	if res.Err != nil {
		excerpt = domain.Excerpt([]byte(res.Err.Error()))
	} else {
		c := res.Status
		code = &c
	}
	d.Attempts = attempts
	return s.recordAt(ctx, d, status, next, code, excerpt, int(res.Duration/time.Millisecond))
}

func (s *Service) record(ctx context.Context, d port.DueDelivery, st domain.DeliveryStatus, next *time.Time, code *int, excerpt string, ms int) error {
	d.Attempts++
	return s.recordAt(ctx, d, st, next, code, excerpt, ms)
}

func (s *Service) recordAt(ctx context.Context, d port.DueDelivery, st domain.DeliveryStatus, next *time.Time, code *int, excerpt string, ms int) error {
	return s.Repo.RecordResult(ctx, d.ID, port.Result{
		Status: st, Attempts: d.Attempts, NextAttemptAt: next, ResponseStatus: code,
		Excerpt: strings.TrimSpace(excerpt), DurationMS: ms,
	})
}
