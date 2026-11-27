package domain

import (
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/jhonsferg/corelog/internal/platform/apperror"
)

func TestValidateBulkRefsLimits(t *testing.T) {
	if err := ValidateBulkRefs(nil); apperror.CodeOf(err) != "issue.bulk_empty" {
		t.Fatalf("empty = %v", err)
	}
	max := make([]string, MaxBulkIssues)
	for i := range max {
		max[i] = "CORE-1"
	}
	if err := ValidateBulkRefs(max); err != nil {
		t.Fatalf("200 refs rejected: %v", err)
	}
	if err := ValidateBulkRefs(append(max, "CORE-2")); apperror.CodeOf(err) != "issue.bulk_too_many" {
		t.Fatalf("201 refs = %v, want too many", err)
	}
	if err := ValidateBulkRefs([]string{"  "}); apperror.CodeOf(err) != "issue.bulk_params_invalid" {
		t.Fatalf("blank ref = %v", err)
	}
}

func TestParseBulkActionAcceptsOnlyKnownActions(t *testing.T) {
	for _, name := range []string{"assign", "set_priority", "set_labels", "set_field", "transition", "move", "delete"} {
		if _, err := ParseBulkAction(name); err != nil {
			t.Errorf("%s rejected: %v", name, err)
		}
	}
	if _, err := ParseBulkAction("DELETE"); apperror.CodeOf(err) != "issue.bulk_action_unknown" {
		t.Fatalf("upper case action = %v", err)
	}
}

func TestDedupeRefsKeepsFirstOccurrence(t *testing.T) {
	got := DedupeRefs([]string{"CORE-1", " CORE-1", "CORE-2", "CORE-1"})
	if strings.Join(got, ",") != "CORE-1,CORE-2" {
		t.Fatalf("dedupe = %v", got)
	}
}

func TestValidateLabelChange(t *testing.T) {
	id := uuid.New()
	if err := ValidateLabelChange(nil, nil); err == nil {
		t.Fatal("empty change accepted")
	}
	if err := ValidateLabelChange([]uuid.UUID{id}, nil); err != nil {
		t.Fatalf("add only rejected: %v", err)
	}
	if err := ValidateLabelChange(nil, []uuid.UUID{uuid.Nil}); err == nil {
		t.Fatal("nil label id accepted")
	}
}
