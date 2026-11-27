package domain

import (
	"strings"
	"testing"

	"github.com/jhonsferg/corelog/internal/platform/apperror"
)

func TestNormalizeLabelNames(t *testing.T) {
	got, err := NormalizeLabelNames([]string{" bug ", "UX", "ux", "bug"})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0] != "bug" || got[1] != "UX" {
		t.Errorf("normalized = %v, want [bug UX]", got)
	}
	if _, err := NormalizeLabelNames([]string{"  "}); apperror.CodeOf(err) != "issue.invalid_label" {
		t.Errorf("blank: code = %q", apperror.CodeOf(err))
	}
	if _, err := NormalizeLabelNames([]string{strings.Repeat("é", MaxLabelNameLength+1)}); err == nil {
		t.Error("name longer than 40 characters must fail")
	}
	many := make([]string, 0, MaxIssueLabels+1)
	for i := 0; i <= MaxIssueLabels; i++ {
		many = append(many, string(rune('a'+i)))
	}
	if _, err := NormalizeLabelNames(many); err == nil {
		t.Error("more than 20 labels must fail")
	}
}

func TestLabelIDsTextIsOrderIndependent(t *testing.T) {
	if LabelIDsText([]string{"b", "a"}) != LabelIDsText([]string{"a", "b"}) {
		t.Error("label id text must not depend on order")
	}
	if LabelIDsText(nil) != "" {
		t.Error("no labels must render as empty text")
	}
}
