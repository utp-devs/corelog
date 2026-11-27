package domain

import (
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/jhonsferg/corelog/internal/platform/apperror"
)

func TestParseActivityFilter(t *testing.T) {
	cases := map[string]string{"": ActivityFilterAll, "all": ActivityFilterAll, "comments": ActivityFilterComments, "history": ActivityFilterHistory}
	for in, want := range cases {
		got, err := ParseActivityFilter(in)
		if err != nil || got != want {
			t.Errorf("ParseActivityFilter(%q) = %q, %v; want %q", in, got, err, want)
		}
	}
	if _, err := ParseActivityFilter("worklog"); apperror.CodeOf(err) != "issue.invalid_activity_filter" {
		t.Errorf("unknown filter: code = %q", apperror.CodeOf(err))
	}
}

func TestNormalizeActivityLimit(t *testing.T) {
	if got, err := NormalizeActivityLimit(0, false); err != nil || got != DefaultActivityLimit {
		t.Errorf("default = %d, %v", got, err)
	}
	if got, err := NormalizeActivityLimit(100, true); err != nil || got != MaxActivityLimit {
		t.Errorf("max = %d, %v", got, err)
	}
	for _, bad := range []int{0, -1, 101} {
		if _, err := NormalizeActivityLimit(bad, true); apperror.CodeOf(err) != "issue.invalid_activity_limit" {
			t.Errorf("limit %d: code = %q", bad, apperror.CodeOf(err))
		}
	}
}

func TestDecodeActivityCursorRejectsGarbage(t *testing.T) {
	if c, err := DecodeActivityCursor(""); err != nil || c != nil {
		t.Errorf("empty cursor must mean first page: %v, %v", c, err)
	}
	for _, bad := range []string{"%%%", "e30", "eyJrIjoieCJ9"} {
		if _, err := DecodeActivityCursor(bad); apperror.CodeOf(err) != "issue.invalid_activity_cursor" {
			t.Errorf("cursor %q: code = %q", bad, apperror.CodeOf(err))
		}
	}
	valid := EncodeActivityCursor(ActivityCursor{At: time.Now().UTC(), Kind: FeedKindComment, ID: uuid.New()})
	if _, err := DecodeActivityCursor(valid); err != nil {
		t.Errorf("valid cursor: %v", err)
	}
}

func TestFieldLabelTranslatesKnownFields(t *testing.T) {
	cases := map[string]string{
		"summary": "Resumen", "description": "Descripción", "status": "Estado", "priority": "Prioridad",
		"assignee": "Responsable", "reporter": "Informador", "issue_type": "Tipo", "labels": "Etiquetas",
		"due_date": "Fecha límite", "unknown_field": "unknown_field",
	}
	for field, want := range cases {
		if got := FieldLabel(field); got != want {
			t.Errorf("FieldLabel(%q) = %q, want %q", field, got, want)
		}
	}
}
