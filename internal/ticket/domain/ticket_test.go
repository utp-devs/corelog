package domain

import (
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestNewTicket(t *testing.T) {
	validRequester := uuid.New()

	t.Run("creates valid ticket with open status", func(t *testing.T) {
		ticket, err := NewTicket("Issue title", "Issue description", PriorityHigh, validRequester)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if ticket.ID == uuid.Nil {
			t.Errorf("expected non-nil ticket ID")
		}
		if ticket.Title != "Issue title" {
			t.Errorf("got title %s, want 'Issue title'", ticket.Title)
		}
		if ticket.Description != "Issue description" {
			t.Errorf("got description %s, want 'Issue description'", ticket.Description)
		}
		if ticket.Priority != PriorityHigh {
			t.Errorf("got priority %v, want %v", ticket.Priority, PriorityHigh)
		}
		if ticket.Status != StatusOpen {
			t.Errorf("got status %v, want %v", ticket.Status, StatusOpen)
		}
		if ticket.RequesterID != validRequester {
			t.Errorf("got requester %v, want %v", ticket.RequesterID, validRequester)
		}
		if ticket.AssigneeID != nil {
			t.Errorf("expected nil AssigneeID, got %v", ticket.AssigneeID)
		}
		if ticket.CreatedAt.IsZero() || ticket.UpdatedAt.IsZero() {
			t.Errorf("expected non-zero timestamps")
		}
	})

	t.Run("rejects empty title", func(t *testing.T) {
		_, err := NewTicket("", "description", PriorityLow, validRequester)
		if !errors.Is(err, ErrInvalidTitle) {
			t.Errorf("expected ErrInvalidTitle, got %v", err)
		}
	})

	t.Run("rejects empty description", func(t *testing.T) {
		_, err := NewTicket("title", "", PriorityLow, validRequester)
		if !errors.Is(err, ErrInvalidDescription) {
			t.Errorf("expected ErrInvalidDescription, got %v", err)
		}
	})

	t.Run("rejects invalid priority", func(t *testing.T) {
		_, err := NewTicket("title", "description", Priority("invalid"), validRequester)
		if !errors.Is(err, ErrInvalidPriority) {
			t.Errorf("expected ErrInvalidPriority, got %v", err)
		}
	})

	t.Run("rejects nil requester ID", func(t *testing.T) {
		_, err := NewTicket("title", "description", PriorityLow, uuid.Nil)
		if !errors.Is(err, ErrInvalidRequester) {
			t.Errorf("expected ErrInvalidRequester, got %v", err)
		}
	})
}

func TestTicket_ChangeStatus(t *testing.T) {
	requester := uuid.New()

	t.Run("valid transition succeeds and updates UpdatedAt", func(t *testing.T) {
		ticket, err := NewTicket("Test", "Desc", PriorityMedium, requester)
		if err != nil {
			t.Fatalf("failed to create ticket: %v", err)
		}
		initialUpdatedAt := ticket.UpdatedAt
		time.Sleep(10 * time.Millisecond)

		err = ticket.ChangeStatus(StatusInProgress)
		if err != nil {
			t.Fatalf("unexpected transition error: %v", err)
		}
		if ticket.Status != StatusInProgress {
			t.Errorf("got status %v, want %v", ticket.Status, StatusInProgress)
		}
		if !ticket.UpdatedAt.After(initialUpdatedAt) {
			t.Errorf("expected UpdatedAt (%v) to be after initial (%v)", ticket.UpdatedAt, initialUpdatedAt)
		}
	})

	t.Run("rejects invalid status enum", func(t *testing.T) {
		ticket, _ := NewTicket("Test", "Desc", PriorityMedium, requester)
		err := ticket.ChangeStatus(Status("nonexistent"))
		if !errors.Is(err, ErrInvalidStatus) {
			t.Errorf("expected ErrInvalidStatus, got %v", err)
		}
	})

	t.Run("rejects illegal transition", func(t *testing.T) {
		ticket, _ := NewTicket("Test", "Desc", PriorityMedium, requester)
		// Open cannot transition directly to Resolved
		err := ticket.ChangeStatus(StatusResolved)
		if !errors.Is(err, ErrIllegalTransition) {
			t.Errorf("expected ErrIllegalTransition, got %v", err)
		}
	})
}

func TestTicket_AssignTo(t *testing.T) {
	requester := uuid.New()
	assignee := uuid.New()

	t.Run("assigning open ticket moves it to in_progress", func(t *testing.T) {
		ticket, _ := NewTicket("Test", "Desc", PriorityMedium, requester)
		if ticket.Status != StatusOpen {
			t.Fatalf("expected initial StatusOpen")
		}

		err := ticket.AssignTo(assignee)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if ticket.AssigneeID == nil || *ticket.AssigneeID != assignee {
			t.Errorf("got assignee %v, want %v", ticket.AssigneeID, assignee)
		}
		if ticket.Status != StatusInProgress {
			t.Errorf("got status %v, want %v", ticket.Status, StatusInProgress)
		}
	})

	t.Run("assigning in_progress ticket keeps status in_progress", func(t *testing.T) {
		ticket, _ := NewTicket("Test", "Desc", PriorityMedium, requester)
		_ = ticket.ChangeStatus(StatusInProgress)

		newAssignee := uuid.New()
		err := ticket.AssignTo(newAssignee)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if ticket.AssigneeID == nil || *ticket.AssigneeID != newAssignee {
			t.Errorf("got assignee %v, want %v", ticket.AssigneeID, newAssignee)
		}
		if ticket.Status != StatusInProgress {
			t.Errorf("got status %v, want %v", ticket.Status, StatusInProgress)
		}
	})
}

func TestTicket_IsClosed(t *testing.T) {
	requester := uuid.New()
	ticket, _ := NewTicket("Test", "Desc", PriorityMedium, requester)

	if ticket.IsClosed() {
		t.Errorf("expected open ticket not to be closed")
	}

	_ = ticket.ChangeStatus(StatusClosed)
	if !ticket.IsClosed() {
		t.Errorf("expected closed ticket to return true for IsClosed()")
	}
}
