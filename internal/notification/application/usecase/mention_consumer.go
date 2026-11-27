package usecase

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"github.com/jhonsferg/corelog/internal/notification/application/port"
	"github.com/jhonsferg/corelog/internal/platform/events"
)

// Event types of comments, as published by the issue module.
const (
	mentionEventType = "comment.mentioned"
	replyEventType   = "comment.created"
)

// Notice kinds recorded per comment (the comment_notices table) and notification kinds shown to users.
const (
	noticeMention       = "mention"
	noticeReply         = "reply"
	notificationMention = "comment_mention"
	notificationReply   = "comment_reply"
)

// MentionConsumer turns comment events into in-app notifications. A mentioned user is notified once per comment;
// a reply notifies the parent author unless that author was mentioned in the same comment. Preferences are applied
// by the notification service, and users who cannot browse the project any more are skipped.
type MentionConsumer struct {
	notifications port.Service
	browser       port.ProjectBrowser
	deliveries    port.MentionDeliveries
}

// NewMentionConsumer wires the consumer.
func NewMentionConsumer(notifications port.Service, browser port.ProjectBrowser, deliveries port.MentionDeliveries) *MentionConsumer {
	return &MentionConsumer{notifications: notifications, browser: browser, deliveries: deliveries}
}

var _ events.Consumer = (*MentionConsumer)(nil)

// Name keys the cursor of the consumer.
func (c *MentionConsumer) Name() string { return "notification.comment_mentions" }

// Handle processes one event. Events of other types are ignored.
func (c *MentionConsumer) Handle(ctx context.Context, e events.Event) error {
	switch e.Type {
	case mentionEventType:
		return c.handleMentions(ctx, e)
	case replyEventType:
		return c.handleReply(ctx, e)
	default:
		return nil
	}
}

func (c *MentionConsumer) handleMentions(ctx context.Context, e events.Event) error {
	commentID, ok := payloadUUID(e.Payload, "comment_id")
	if !ok {
		return nil
	}
	for _, userID := range payloadUUIDs(e.Payload, "user_ids") {
		if e.ActorID != nil && *e.ActorID == userID {
			continue
		}
		if err := c.deliver(ctx, e, commentID, userID, noticeMention, notificationMention); err != nil {
			return err
		}
	}
	return nil
}

func (c *MentionConsumer) handleReply(ctx context.Context, e events.Event) error {
	commentID, ok := payloadUUID(e.Payload, "comment_id")
	if !ok {
		return nil
	}
	target, ok := payloadUUID(e.Payload, "reply_to_user_id")
	if !ok || (e.ActorID != nil && *e.ActorID == target) {
		return nil
	}
	mentioned, err := c.deliveries.IsMentioned(ctx, commentID, target)
	if err != nil {
		return err
	}
	if mentioned {
		// The mention notice already tells this user about the comment.
		return nil
	}
	return c.deliver(ctx, e, commentID, target, noticeReply, notificationReply)
}

// deliver notifies one user about a comment, at most once per comment and kind.
func (c *MentionConsumer) deliver(ctx context.Context, e events.Event, commentID, userID uuid.UUID, noticeKind, kind string) error {
	if e.ProjectID == nil {
		return nil
	}
	browse, err := c.browser.CanBrowse(ctx, userID, *e.ProjectID)
	if err != nil {
		return err
	}
	if !browse {
		return nil
	}
	sent, err := c.deliveries.NoticeSent(ctx, commentID, userID, noticeKind)
	if err != nil {
		return err
	}
	if sent {
		return nil
	}
	issueID, _ := payloadString(e.Payload, "issue_id")
	issueKey, _ := payloadString(e.Payload, "issue_key")
	subject := "una incidencia"
	if issueKey != "" {
		subject = issueKey
	}
	message := fmt.Sprintf("Te mencionaron en un comentario de %s", subject)
	if noticeKind == noticeReply {
		message = fmt.Sprintf("Respondieron a tu comentario en %s", subject)
	}
	link := "/issues/" + issueID + "#comment-" + commentID.String()
	if err := c.notifications.Notify(ctx, userID, kind, message, link); err != nil {
		return err
	}
	return c.deliveries.RecordNotice(ctx, commentID, userID, noticeKind)
}

func payloadString(payload map[string]any, key string) (string, bool) {
	value, ok := payload[key].(string)
	return value, ok
}

func payloadUUID(payload map[string]any, key string) (uuid.UUID, bool) {
	raw, ok := payloadString(payload, key)
	if !ok {
		return uuid.Nil, false
	}
	id, err := uuid.Parse(raw)
	if err != nil || id == uuid.Nil {
		return uuid.Nil, false
	}
	return id, true
}

// payloadUUIDs reads a list of identifiers; malformed entries are skipped.
func payloadUUIDs(payload map[string]any, key string) []uuid.UUID {
	list, _ := payload[key].([]any)
	out := make([]uuid.UUID, 0, len(list))
	for _, item := range list {
		raw, ok := item.(string)
		if !ok {
			continue
		}
		if id, err := uuid.Parse(raw); err == nil && id != uuid.Nil {
			out = append(out, id)
		}
	}
	return out
}
