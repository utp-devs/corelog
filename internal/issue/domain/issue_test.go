package domain

import (
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestNewIssue(t *testing.T) {
	validReporter := uuid.New()

	t.Run("creates valid issue with open status", func(t *testing.T) {
		issue, err := NewIssue("Issue summary", "Issue description", PriorityHigh, validReporter)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if issue.ID != uuid.Nil {
			t.Errorf("constructor must leave ID for the database to generate, got %s", issue.ID)
		}
		if issue.Summary != "Issue summary" {
			t.Errorf("got summary %s, want 'Issue summary'", issue.Summary)
		}
		if issue.Description != "Issue description" {
			t.Errorf("got description %s, want 'Issue description'", issue.Description)
		}
		if issue.Priority != PriorityHigh {
			t.Errorf("got priority %v, want %v", issue.Priority, PriorityHigh)
		}
		if issue.Status != StatusOpen {
			t.Errorf("got status %v, want %v", issue.Status, StatusOpen)
		}
		if issue.ReporterID != validReporter {
			t.Errorf("got reporter %v, want %v", issue.ReporterID, validReporter)
		}
		if issue.AssigneeID != nil {
			t.Errorf("expected nil AssigneeID, got %v", issue.AssigneeID)
		}
		if issue.CreatedAt.IsZero() || issue.UpdatedAt.IsZero() {
			t.Errorf("expected non-zero timestamps")
		}
	})

	t.Run("rejects empty summary", func(t *testing.T) {
		_, err := NewIssue("", "description", PriorityLow, validReporter)
		if !errors.Is(err, ErrInvalidSummary) {
			t.Errorf("expected ErrInvalidSummary, got %v", err)
		}
	})

	t.Run("rejects empty description", func(t *testing.T) {
		_, err := NewIssue("summary", "", PriorityLow, validReporter)
		if !errors.Is(err, ErrInvalidDescription) {
			t.Errorf("expected ErrInvalidDescription, got %v", err)
		}
	})

	t.Run("rejects invalid priority", func(t *testing.T) {
		_, err := NewIssue("summary", "description", Priority("invalid"), validReporter)
		if !errors.Is(err, ErrInvalidPriority) {
			t.Errorf("expected ErrInvalidPriority, got %v", err)
		}
	})

	t.Run("rejects nil reporter ID", func(t *testing.T) {
		_, err := NewIssue("summary", "description", PriorityLow, uuid.Nil)
		if !errors.Is(err, ErrInvalidReporter) {
			t.Errorf("expected ErrInvalidReporter, got %v", err)
		}
	})
}

func TestIssue_ChangeStatus(t *testing.T) {
	reporter := uuid.New()

	t.Run("valid transition succeeds and updates UpdatedAt", func(t *testing.T) {
		issue, err := NewIssue("Test", "Desc", PriorityMedium, reporter)
		if err != nil {
			t.Fatalf("failed to create issue: %v", err)
		}
		initialUpdatedAt := issue.UpdatedAt
		time.Sleep(10 * time.Millisecond)

		err = issue.ChangeStatus(StatusInProgress)
		if err != nil {
			t.Fatalf("unexpected transition error: %v", err)
		}
		if issue.Status != StatusInProgress {
			t.Errorf("got status %v, want %v", issue.Status, StatusInProgress)
		}
		if !issue.UpdatedAt.After(initialUpdatedAt) {
			t.Errorf("expected UpdatedAt (%v) to be after initial (%v)", issue.UpdatedAt, initialUpdatedAt)
		}
	})

	t.Run("rejects invalid status enum", func(t *testing.T) {
		issue, _ := NewIssue("Test", "Desc", PriorityMedium, reporter)
		err := issue.ChangeStatus(Status("nonexistent"))
		if !errors.Is(err, ErrInvalidStatus) {
			t.Errorf("expected ErrInvalidStatus, got %v", err)
		}
	})

	t.Run("rejects illegal transition", func(t *testing.T) {
		issue, _ := NewIssue("Test", "Desc", PriorityMedium, reporter)
		// Open cannot transition directly to Resolved
		err := issue.ChangeStatus(StatusResolved)
		if !errors.Is(err, ErrIllegalTransition) {
			t.Errorf("expected ErrIllegalTransition, got %v", err)
		}
	})
}

func TestIssue_AssignTo(t *testing.T) {
	reporter := uuid.New()
	assignee := uuid.New()

	t.Run("assigning an open issue keeps its status", func(t *testing.T) {
		issue, _ := NewIssue("Test", "Desc", PriorityMedium, reporter)
		if issue.Status != StatusOpen {
			t.Fatalf("expected initial StatusOpen")
		}

		err := issue.AssignTo(assignee)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if issue.AssigneeID == nil || *issue.AssigneeID != assignee {
			t.Errorf("got assignee %v, want %v", issue.AssigneeID, assignee)
		}
		if issue.Status != StatusOpen {
			t.Errorf("got status %v, want %v: assigning no longer moves the issue", issue.Status, StatusOpen)
		}
	})

	t.Run("assigning in_progress issue keeps status in_progress", func(t *testing.T) {
		issue, _ := NewIssue("Test", "Desc", PriorityMedium, reporter)
		_ = issue.ChangeStatus(StatusInProgress)

		newAssignee := uuid.New()
		err := issue.AssignTo(newAssignee)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if issue.AssigneeID == nil || *issue.AssigneeID != newAssignee {
			t.Errorf("got assignee %v, want %v", issue.AssigneeID, newAssignee)
		}
		if issue.Status != StatusInProgress {
			t.Errorf("got status %v, want %v", issue.Status, StatusInProgress)
		}
	})
}

func TestIssue_IsClosed(t *testing.T) {
	reporter := uuid.New()
	issue, _ := NewIssue("Test", "Desc", PriorityMedium, reporter)

	if issue.IsClosed() {
		t.Errorf("expected open issue not to be closed")
	}

	_ = issue.ChangeStatus(StatusClosed)
	if !issue.IsClosed() {
		t.Errorf("expected closed issue to return true for IsClosed()")
	}
}
