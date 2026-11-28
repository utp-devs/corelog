package usecase

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"

	"github.com/jhonsferg/corelog/internal/notification/application/port"
	"github.com/jhonsferg/corelog/internal/notification/domain"
	"github.com/jhonsferg/corelog/internal/platform/events"
)

// SchemeConsumer turns domain events into notices for the notification engine. Comment mentions and replies are
// handled by the MentionConsumer, which keeps its own per-comment bookkeeping.
type SchemeConsumer struct {
	engine *Engine
}

var _ events.Consumer = (*SchemeConsumer)(nil)

// NewSchemeConsumer wires the consumer.
func NewSchemeConsumer(engine *Engine) *SchemeConsumer { return &SchemeConsumer{engine: engine} }

// Name keys the cursor of the consumer.
func (c *SchemeConsumer) Name() string { return "notification.schemes" }

// Handle processes one event. Events without a notification meaning are ignored.
func (c *SchemeConsumer) Handle(ctx context.Context, e events.Event) error {
	base := port.Notice{ActorID: derefID(e.ActorID), ProjectID: derefID(e.ProjectID), OrgID: derefID(e.OrgID)}
	dedup := fmt.Sprintf("evt:%d", e.ID)
	switch e.Type {
	case "issue.created":
		return c.issue(ctx, base, domain.EventIssueCreated, e, "", dedup)
	case "issue.updated":
		return c.issue(ctx, base, domain.EventIssueUpdated, e, changedFields(e.Payload), dedup)
	case "issue.transitioned":
		detail := transitionDetail(e.Payload)
		if err := c.issue(ctx, base, domain.EventIssueTransitioned, e, detail, dedup); err != nil {
			return err
		}
		if category, _ := e.Payload["to_category"].(string); category == "done" {
			return c.issue(ctx, base, domain.EventIssueResolved, e, "", dedup+":resolved")
		}
		return nil
	case "version.released":
		name, _ := e.Payload["name"].(string)
		return c.engine.Deliver(ctx, withNotice(base, domain.EventVersionReleased, uuid.Nil, name, dedup))
	case "sprint.started":
		name, _ := e.Payload["name"].(string)
		return c.engine.Deliver(ctx, withNotice(base, domain.EventSprintStarted, uuid.Nil, name, dedup))
	case "sprint.completed":
		name, _ := e.Payload["name"].(string)
		return c.engine.Deliver(ctx, withNotice(base, domain.EventSprintCompleted, uuid.Nil, name, dedup))
	case "worklog.logged":
		return c.issue(ctx, base, domain.EventWorklogLogged, e, "", dedup)
	case "comment.updated":
		return c.issue(ctx, base, domain.EventCommentEdited, e, "", dedup)
	default:
		return nil
	}
}

func (c *SchemeConsumer) issue(ctx context.Context, base port.Notice, event domain.Event, e events.Event, detail, dedup string) error {
	issueID, ok := payloadUUID(e.Payload, "issue_id")
	if !ok && e.EntityID != nil && e.EntityType == "issue" {
		issueID, ok = *e.EntityID, true
	}
	if !ok {
		return nil
	}
	return c.engine.Deliver(ctx, withNotice(base, event, issueID, detail, dedup))
}

func withNotice(base port.Notice, event domain.Event, issueID uuid.UUID, detail, dedup string) port.Notice {
	base.Event, base.IssueID, base.Detail, base.DedupKey = event, issueID, detail, dedup
	return base
}

func derefID(id *uuid.UUID) uuid.UUID {
	if id == nil {
		return uuid.Nil
	}
	return *id
}

func transitionDetail(payload map[string]any) string {
	from, _ := payload["from_status"].(string)
	to, _ := payload["to_status"].(string)
	if from == "" && to == "" {
		return ""
	}
	return strings.TrimSpace(from) + " -> " + strings.TrimSpace(to)
}

// changedFields lists the names of the fields an update touched, as "campos: a, b".
func changedFields(payload map[string]any) string {
	list, _ := payload["changes"].([]any)
	var names []string
	for _, item := range list {
		if m, ok := item.(map[string]any); ok {
			if field, _ := m["field"].(string); field != "" {
				names = append(names, field)
			}
		}
	}
	if len(names) == 0 {
		return ""
	}
	return "campos: " + strings.Join(names, ", ")
}
