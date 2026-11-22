package usecase

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"

	"github.com/jhonsferg/corelog/internal/notification/domain"
)

// memoryRepo guarda notificaciones en memoria y aplica el mismo filtro por usuario que el adaptador real.
type memoryRepo struct {
	items []*domain.Notification
}

func (r *memoryRepo) Save(_ context.Context, n *domain.Notification) error {
	n.ID = uuid.New()
	r.items = append(r.items, n)
	return nil
}

func (r *memoryRepo) ListByUser(_ context.Context, userID uuid.UUID, _ int) ([]*domain.Notification, error) {
	var out []*domain.Notification
	for _, n := range r.items {
		if n.UserID == userID {
			out = append(out, n)
		}
	}
	return out, nil
}

func (r *memoryRepo) UnreadCount(_ context.Context, userID uuid.UUID) (int, error) {
	count := 0
	for _, n := range r.items {
		if n.UserID == userID && n.ReadAt == nil {
			count++
		}
	}
	return count, nil
}

func (r *memoryRepo) MarkRead(_ context.Context, userID, id uuid.UUID) error {
	for _, n := range r.items {
		if n.ID == id && n.UserID == userID {
			n.MarkRead(n.CreatedAt)
			return nil
		}
	}
	return domain.ErrNotificationNotFound
}

func (r *memoryRepo) MarkAllRead(_ context.Context, userID uuid.UUID) error {
	for _, n := range r.items {
		if n.UserID == userID {
			n.MarkRead(n.CreatedAt)
		}
	}
	return nil
}

func TestNotificationServiceNotifyValidatesAndCounts(t *testing.T) {
	ctx := context.Background()
	repo := &memoryRepo{}
	svc := NewNotificationService(repo)
	ana := uuid.New()

	if err := svc.Notify(ctx, ana, "issue_assigned", "Te asignaron CORE-1", "/issues/1"); err != nil {
		t.Fatalf("Notify: %v", err)
	}
	if err := svc.Notify(ctx, uuid.Nil, "issue_assigned", "x", ""); !errors.Is(err, domain.ErrInvalidNotification) {
		t.Fatalf("notification without user: got %v", err)
	}
	if err := svc.Notify(ctx, ana, "issue_assigned", "   ", ""); !errors.Is(err, domain.ErrInvalidNotification) {
		t.Fatalf("empty message: got %v", err)
	}

	count, err := svc.UnreadCount(ctx, ana)
	if err != nil || count != 1 {
		t.Fatalf("UnreadCount = %d, %v; want 1", count, err)
	}
}

func TestNotificationServiceMarkReadIsScopedToTheOwner(t *testing.T) {
	ctx := context.Background()
	repo := &memoryRepo{}
	svc := NewNotificationService(repo)
	ana, luis := uuid.New(), uuid.New()
	_ = svc.Notify(ctx, ana, "issue_assigned", "Para Ana", "")
	id := repo.items[0].ID

	if err := svc.MarkRead(ctx, luis, id); !errors.Is(err, domain.ErrNotificationNotFound) {
		t.Fatalf("another user marking the notification: got %v, want not found", err)
	}
	if err := svc.MarkRead(ctx, ana, id); err != nil {
		t.Fatalf("owner marks read: %v", err)
	}
	if count, _ := svc.UnreadCount(ctx, ana); count != 0 {
		t.Fatalf("unread after marking = %d, want 0", count)
	}
}

func TestNotificationServiceMarkAllRead(t *testing.T) {
	ctx := context.Background()
	repo := &memoryRepo{}
	svc := NewNotificationService(repo)
	ana := uuid.New()
	_ = svc.Notify(ctx, ana, "issue_assigned", "Uno", "")
	_ = svc.Notify(ctx, ana, "issue_assigned", "Dos", "")

	if err := svc.MarkAllRead(ctx, ana); err != nil {
		t.Fatalf("MarkAllRead: %v", err)
	}
	if count, _ := svc.UnreadCount(ctx, ana); count != 0 {
		t.Fatalf("unread after mark all = %d, want 0", count)
	}
}
