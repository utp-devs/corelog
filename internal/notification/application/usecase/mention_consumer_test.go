package usecase

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"

	"github.com/jhonsferg/corelog/internal/notification/domain"
	"github.com/jhonsferg/corelog/internal/platform/events"
)

type sentNotice struct {
	userID  uuid.UUID
	kind    string
	message string
	link    string
}

type mentionNotifier struct {
	sent []sentNotice
	fail error
}

func (n *mentionNotifier) Notify(_ context.Context, userID uuid.UUID, kind, message, link string) error {
	if n.fail != nil {
		return n.fail
	}
	n.sent = append(n.sent, sentNotice{userID: userID, kind: kind, message: message, link: link})
	return nil
}

// The methods below are needed to satisfy the notification service port in tests that only use Notify.
func (n *mentionNotifier) List(context.Context, uuid.UUID) ([]*domain.Notification, error) {
	return nil, nil
}
func (n *mentionNotifier) UnreadCount(context.Context, uuid.UUID) (int, error) { return 0, nil }
func (n *mentionNotifier) MarkRead(context.Context, uuid.UUID, uuid.UUID) error {
	return nil
}
func (n *mentionNotifier) MarkAllRead(context.Context, uuid.UUID) error { return nil }
func (n *mentionNotifier) GetPreferences(context.Context, uuid.UUID) (domain.Preferences, error) {
	return domain.DefaultPreferences(), nil
}
func (n *mentionNotifier) UpdatePreferences(_ context.Context, _ uuid.UUID, p domain.Preferences) (domain.Preferences, error) {
	return p, nil
}

type mentionBrowser map[uuid.UUID]bool

func (b mentionBrowser) CanBrowse(_ context.Context, userID, _ uuid.UUID) (bool, error) {
	return b[userID], nil
}

type memoryDeliveries struct {
	notices  map[string]bool
	mentions map[uuid.UUID]map[uuid.UUID]bool
}

func noticeKey(commentID, userID uuid.UUID, kind string) string {
	return commentID.String() + "/" + userID.String() + "/" + kind
}

func (d *memoryDeliveries) NoticeSent(_ context.Context, commentID, userID uuid.UUID, kind string) (bool, error) {
	return d.notices[noticeKey(commentID, userID, kind)], nil
}

func (d *memoryDeliveries) RecordNotice(_ context.Context, commentID, userID uuid.UUID, kind string) error {
	d.notices[noticeKey(commentID, userID, kind)] = true
	return nil
}

func (d *memoryDeliveries) IsMentioned(_ context.Context, commentID, userID uuid.UUID) (bool, error) {
	return d.mentions[commentID][userID], nil
}

type mentionFixture struct {
	notifier   *mentionNotifier
	browser    mentionBrowser
	deliveries *memoryDeliveries
	consumer   *MentionConsumer
	project    uuid.UUID
	actor      uuid.UUID
	bob        uuid.UUID
	lead       uuid.UUID
	comment    uuid.UUID
	issue      uuid.UUID
}

func newMentionFixture() *mentionFixture {
	f := &mentionFixture{
		notifier:   &mentionNotifier{},
		browser:    mentionBrowser{},
		deliveries: &memoryDeliveries{notices: map[string]bool{}, mentions: map[uuid.UUID]map[uuid.UUID]bool{}},
		project:    uuid.New(), actor: uuid.New(), bob: uuid.New(), lead: uuid.New(),
		comment: uuid.New(), issue: uuid.New(),
	}
	f.browser[f.bob] = true
	f.browser[f.lead] = true
	f.browser[f.actor] = true
	f.consumer = NewMentionConsumer(f.notifier, f.browser, f.deliveries)
	return f
}

func (f *mentionFixture) event(eventType string, payload map[string]any) events.Event {
	actor := f.actor
	project := f.project
	payload["comment_id"] = f.comment.String()
	payload["issue_id"] = f.issue.String()
	payload["issue_key"] = "CORE-7"
	return events.Event{Type: eventType, ProjectID: &project, ActorID: &actor, Payload: payload}
}

func TestMentionConsumerNotifiesMentionedUsersOnceAndSkipsTheActor(t *testing.T) {
	f := newMentionFixture()
	ctx := context.Background()
	mention := f.event("comment.mentioned", map[string]any{
		"user_ids": []any{f.bob.String(), f.actor.String(), f.lead.String(), "no-es-uuid"},
	})
	if err := f.consumer.Handle(ctx, mention); err != nil {
		t.Fatal(err)
	}
	if len(f.notifier.sent) != 2 {
		t.Fatalf("sent %d notices, want bob and lead", len(f.notifier.sent))
	}
	first := f.notifier.sent[0]
	if first.userID != f.bob || first.kind != "comment_mention" || first.link != "/issues/"+f.issue.String()+"#comment-"+f.comment.String() {
		t.Errorf("first notice = %+v", first)
	}
	if first.message != "Te mencionaron en un comentario de CORE-7" {
		t.Errorf("message = %q", first.message)
	}

	// At-least-once delivery: the same event again must not notify twice.
	if err := f.consumer.Handle(ctx, mention); err != nil {
		t.Fatal(err)
	}
	if len(f.notifier.sent) != 2 {
		t.Fatalf("redelivery notified again: %d notices", len(f.notifier.sent))
	}
}

func TestMentionConsumerRespectsProjectVisibility(t *testing.T) {
	f := newMentionFixture()
	delete(f.browser, f.bob)
	ctx := context.Background()
	if err := f.consumer.Handle(ctx, f.event("comment.mentioned", map[string]any{"user_ids": []any{f.bob.String()}})); err != nil {
		t.Fatal(err)
	}
	if len(f.notifier.sent) != 0 {
		t.Fatal("a user who cannot browse the project must not be notified")
	}
}

func TestMentionConsumerRepliesNotifyTheParentAuthorUnlessMentioned(t *testing.T) {
	f := newMentionFixture()
	ctx := context.Background()
	if err := f.consumer.Handle(ctx, f.event("comment.created", map[string]any{"reply_to_user_id": f.bob.String()})); err != nil {
		t.Fatal(err)
	}
	if len(f.notifier.sent) != 1 || f.notifier.sent[0].kind != "comment_reply" || f.notifier.sent[0].userID != f.bob {
		t.Fatalf("reply notice = %+v", f.notifier.sent)
	}
	if f.notifier.sent[0].message != "Respondieron a tu comentario en CORE-7" {
		t.Errorf("reply message = %q", f.notifier.sent[0].message)
	}

	// A parent author who is also mentioned in the same comment gets the mention notice only.
	f.deliveries.mentions[f.comment] = map[uuid.UUID]bool{f.lead: true}
	if err := f.consumer.Handle(ctx, f.event("comment.created", map[string]any{"reply_to_user_id": f.lead.String()})); err != nil {
		t.Fatal(err)
	}
	if len(f.notifier.sent) != 1 {
		t.Fatalf("mentioned parent author got a second notice: %+v", f.notifier.sent)
	}

	// Replying to your own comment, or an event without recipient, notifies nobody.
	if err := f.consumer.Handle(ctx, f.event("comment.created", map[string]any{"reply_to_user_id": f.actor.String()})); err != nil {
		t.Fatal(err)
	}
	if err := f.consumer.Handle(ctx, f.event("comment.created", map[string]any{})); err != nil {
		t.Fatal(err)
	}
	if len(f.notifier.sent) != 1 {
		t.Fatalf("unexpected notices: %+v", f.notifier.sent)
	}
}

func TestMentionConsumerIgnoresOtherEventsAndSurfacesFailures(t *testing.T) {
	f := newMentionFixture()
	ctx := context.Background()
	if err := f.consumer.Handle(ctx, events.Event{Type: "issue.updated", Payload: map[string]any{}}); err != nil {
		t.Fatalf("unrelated event: %v", err)
	}
	if f.consumer.Name() == "" {
		t.Fatal("consumer needs a name for its cursor")
	}

	f.notifier.fail = errors.New("database down")
	if err := f.consumer.Handle(ctx, f.event("comment.mentioned", map[string]any{"user_ids": []any{f.bob.String()}})); err == nil {
		t.Fatal("a failed notification must stop the batch so the event is retried")
	}
	if f.deliveries.notices[noticeKey(f.comment, f.bob, "mention")] {
		t.Fatal("a failed notification must not be recorded as sent")
	}
}
