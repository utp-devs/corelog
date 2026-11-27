package domain

import (
	"testing"
)

func TestPriority_IsValid(t *testing.T) {
	tests := []struct {
		name     string
		priority Priority
		want     bool
	}{
		{"low is valid", PriorityLow, true},
		{"medium is valid", PriorityMedium, true},
		{"high is valid", PriorityHigh, true},
		{"critical is valid", PriorityCritical, true},
		{"empty is invalid", "", false},
		{"uppercase LOW is invalid", "LOW", false},
		{"unknown priority is invalid", "urgent", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.priority.IsValid(); got != tt.want {
				t.Errorf("Priority.IsValid() = %v, want %v", got, tt.want)
			}
		})
	}
}
