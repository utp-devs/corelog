package domain

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestNewNotificationValidatesAndMarkReadOnce(t *testing.T) {
	user := uuid.New()
	if _, err := New(uuid.Nil, "issue_assigned", "x", ""); !errors.Is(err, ErrInvalidNotification) {
		t.Fatalf("no user: got %v", err)
	}
	if _, err := New(user, "", "x", ""); !errors.Is(err, ErrInvalidNotification) {
		t.Fatalf("no kind: got %v", err)
	}
	if _, err := New(user, "issue_assigned", strings.Repeat("a", 301), ""); !errors.Is(err, ErrInvalidNotification) {
		t.Fatalf("message over 300 runes: got %v", err)
	}

	n, err := New(user, " issue_assigned ", "  Te asignaron  ", "/issues/1")
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if n.Kind != "issue_assigned" || n.Message != "Te asignaron" || n.ReadAt != nil {
		t.Fatalf("normalized notification = %+v", n)
	}

	first := time.Date(2026, 11, 22, 10, 0, 0, 0, time.UTC)
	n.MarkRead(first)
	n.MarkRead(first.Add(time.Hour))
	if n.ReadAt == nil || !n.ReadAt.Equal(first) {
		t.Fatalf("MarkRead must keep the first read time, got %v", n.ReadAt)
	}
}
