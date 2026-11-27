package domain

import (
	"testing"
)

func TestStatus_IsValid(t *testing.T) {
	tests := []struct {
		name   string
		status Status
		want   bool
	}{
		{"open is valid", StatusOpen, true},
		{"in_progress is valid", StatusInProgress, true},
		{"resolved is valid", StatusResolved, true},
		{"closed is valid", StatusClosed, true},
		{"empty string is invalid", "", false},
		{"uppercase OPEN is invalid", "OPEN", false},
		{"arbitrary string is invalid", "pending", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.status.IsValid(); got != tt.want {
				t.Errorf("Status.IsValid() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestStatus_CanTransitionTo(t *testing.T) {
	tests := []struct {
		name string
		from Status
		to   Status
		want bool
	}{
		// Transitions from Open
		{"open -> in_progress allowed", StatusOpen, StatusInProgress, true},
		{"open -> closed allowed", StatusOpen, StatusClosed, true},
		{"open -> resolved not allowed", StatusOpen, StatusResolved, false},
		{"open -> open not allowed", StatusOpen, StatusOpen, false},

		// Transitions from InProgress
		{"in_progress -> resolved allowed", StatusInProgress, StatusResolved, true},
		{"in_progress -> closed allowed", StatusInProgress, StatusClosed, true},
		{"in_progress -> open not allowed", StatusInProgress, StatusOpen, false},
		{"in_progress -> in_progress not allowed", StatusInProgress, StatusInProgress, false},

		// Transitions from Resolved
		{"resolved -> closed allowed", StatusResolved, StatusClosed, true},
		{"resolved -> in_progress allowed", StatusResolved, StatusInProgress, true},
		{"resolved -> open not allowed", StatusResolved, StatusOpen, false},
		{"resolved -> resolved not allowed", StatusResolved, StatusResolved, false},

		// Transitions from Closed
		{"closed -> open not allowed", StatusClosed, StatusOpen, false},
		{"closed -> in_progress not allowed", StatusClosed, StatusInProgress, false},
		{"closed -> resolved not allowed", StatusClosed, StatusResolved, false},
		{"closed -> closed not allowed", StatusClosed, StatusClosed, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.from.CanTransitionTo(tt.to); got != tt.want {
				t.Errorf("CanTransitionTo(%v -> %v) = %v, want %v", tt.from, tt.to, got, tt.want)
			}
		})
	}
}
