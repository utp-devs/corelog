package domain

import (
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/jhonsferg/corelog/internal/platform/apperror"
)

func TestParseCloneIncludes(t *testing.T) {
	got, err := ParseCloneIncludes([]string{"links", "links", "watchers"})
	if err != nil || len(got) != 2 || !got["links"] || !got["watchers"] {
		t.Fatalf("includes = %v, %v", got, err)
	}
	if _, err := ParseCloneIncludes([]string{"comments"}); apperror.CodeOf(err) != "issue.clone_option_invalid" {
		t.Fatalf("unknown option = %v", err)
	}
}

func TestDefaultCloneSummaryTruncatesToLimit(t *testing.T) {
	if got := DefaultCloneSummary("Corregir login"); got != "Copia de Corregir login" {
		t.Fatalf("summary = %q", got)
	}
	long := strings.Repeat("á", MaxSummaryLength)
	if got := DefaultCloneSummary(long); len([]rune(got)) != MaxSummaryLength {
		t.Fatalf("long summary has %d runes", len([]rune(got)))
	}
}

func TestCloneDraftCopiesFieldsWithoutIdentity(t *testing.T) {
	source := &Issue{
		ID: uuid.New(), ProjectID: uuid.New(), Summary: "Origen", Description: "Texto", Status: StatusResolved,
		Priority: PriorityHigh, IssueType: IssueTypeBug, ReporterID: uuid.New(), Custom: map[string]any{"f": "v"},
		Flagged: true, KeyNumber: 4, ProjectKey: "CORE",
	}
	reporter, target := uuid.New(), uuid.New()
	now := time.Date(2026, 11, 27, 10, 0, 0, 0, time.UTC)
	draft := CloneDraft(source, target, reporter, "Copia", now)
	if draft.ID != uuid.Nil || draft.KeyNumber != 0 {
		t.Fatalf("draft has an identity: id %s key %d", draft.ID, draft.KeyNumber)
	}
	if draft.ProjectID != target || draft.ReporterID != reporter || draft.Status != StatusOpen || draft.ResolvedAt != nil {
		t.Fatalf("draft = %+v", draft)
	}
	if draft.Description != "Texto" || draft.Priority != PriorityHigh || !draft.Flagged || draft.Custom["f"] != "v" {
		t.Fatalf("fields not copied: %+v", draft)
	}
	draft.Custom["f"] = "otro"
	if source.Custom["f"] != "v" {
		t.Fatal("custom map shared with the original")
	}
}
