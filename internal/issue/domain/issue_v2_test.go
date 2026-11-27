package domain

import (
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/jhonsferg/corelog/internal/platform/apperror"
)

func TestParentRuleTable(t *testing.T) {
	cases := []struct {
		name        string
		child       int
		hasParent   bool
		parent      int
		sameProject bool
		want        error
	}{
		{"epic without parent", HierarchyEpic, false, 0, true, nil},
		{"epic with parent", HierarchyEpic, true, HierarchyEpic, true, ErrEpicHasParent},
		{"standard without parent", HierarchyStandard, false, 0, true, nil},
		{"standard under epic", HierarchyStandard, true, HierarchyEpic, true, nil},
		{"standard under epic of other project", HierarchyStandard, true, HierarchyEpic, false, ErrParentOtherProject},
		{"standard under standard", HierarchyStandard, true, HierarchyStandard, true, ErrStandardParentInvalid},
		{"standard under subtask", HierarchyStandard, true, HierarchySubtask, true, ErrStandardParentInvalid},
		{"subtask without parent", HierarchySubtask, false, 0, true, ErrSubtaskNeedsParent},
		{"subtask under standard", HierarchySubtask, true, HierarchyStandard, true, nil},
		{"subtask under epic", HierarchySubtask, true, HierarchyEpic, true, ErrSubtaskParentInvalid},
		{"subtask under subtask", HierarchySubtask, true, HierarchySubtask, true, ErrSubtaskParentInvalid},
		{"subtask under standard of other project", HierarchySubtask, true, HierarchyStandard, false, ErrParentOtherProject},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := ParentRule(tc.child, tc.hasParent, tc.parent, tc.sameProject); got != tc.want {
				t.Fatalf("got %v, want %v", got, tc.want)
			}
		})
	}
}

func TestCheckParentDetectsCycles(t *testing.T) {
	project := uuid.New()
	a, b := uuid.New(), uuid.New()
	child := HierarchyNode{ID: a, ProjectID: project, Level: HierarchyStandard}
	if err := CheckParent(child, &HierarchyNode{ID: a, ProjectID: project, Level: HierarchyEpic}, nil); err != ErrParentCycle {
		t.Fatalf("self parent: %v", err)
	}
	parent := &HierarchyNode{ID: b, ProjectID: project, Level: HierarchyEpic, ParentID: &a}
	if err := CheckParent(child, parent, []uuid.UUID{a}); err != ErrParentCycle {
		t.Fatalf("ancestor cycle: %v", err)
	}
	if err := CheckParent(child, parent, nil); err != nil {
		t.Fatalf("valid parent: %v", err)
	}
}

func TestCheckChildren(t *testing.T) {
	if err := CheckChildren(HierarchyStandard, []int{HierarchySubtask}); err != nil {
		t.Fatalf("subtasks under a standard issue: %v", err)
	}
	if err := CheckChildren(HierarchyEpic, []int{HierarchyStandard}); err != nil {
		t.Fatalf("standard under an epic: %v", err)
	}
	for _, levels := range [][]int{{HierarchySubtask}, {HierarchyStandard, HierarchySubtask}} {
		if err := CheckChildren(HierarchyEpic, levels); err != ErrChildrenInvalid {
			t.Fatalf("epic with children %v: %v", levels, err)
		}
	}
	if err := CheckChildren(HierarchyStandard, []int{HierarchyStandard}); err != ErrChildrenInvalid {
		t.Fatalf("standard under standard: %v", err)
	}
}

func TestFieldValidators(t *testing.T) {
	f := func(v float64) *float64 { return &v }
	i := func(v int64) *int64 { return &v }
	for _, ok := range []*float64{nil, f(0), f(0.5), f(13), f(999.5)} {
		if err := ValidateStoryPoints(ok); err != nil {
			t.Errorf("points %v rejected: %v", ok, err)
		}
	}
	for _, bad := range []float64{-0.5, 0.25, 1000, 999.6} {
		if ValidateStoryPoints(f(bad)) == nil {
			t.Errorf("points %v accepted", bad)
		}
	}
	if ValidateEstimate(i(-1)) == nil || ValidateEstimate(i(MaxEstimateSeconds+1)) == nil || ValidateEstimate(i(0)) != nil || ValidateEstimate(nil) != nil {
		t.Error("estimate bounds")
	}
	if _, err := NormalizeSummary(strings.Repeat("a", 256)); err != ErrSummaryTooLong {
		t.Errorf("long summary: %v", err)
	}
	if got, err := NormalizeSummary("  hola  "); err != nil || got != "hola" {
		t.Errorf("trim: %q %v", got, err)
	}
	if _, err := NormalizeDescription(strings.Repeat("a", MaxDescriptionLength+1)); err != ErrDescriptionTooLong {
		t.Errorf("long description: %v", err)
	}
	start, due := time.Date(2026, 12, 2, 5, 0, 0, 0, time.UTC), time.Date(2026, 12, 2, 20, 0, 0, 0, time.UTC)
	if ValidateDates(&start, &due) != nil {
		t.Error("same day must be valid")
	}
	later := due.AddDate(0, 0, -1)
	if apperror.CodeOf(ValidateDates(&start, &later)) != "issue.invalid_dates" {
		t.Error("start after due accepted")
	}
	if FormatDate(&start) != "2026-12-02" || FormatDate(nil) != "" {
		t.Error("date format")
	}
}

func TestLegacyHierarchyLevel(t *testing.T) {
	for kind, want := range map[IssueType]int{IssueTypeEpic: HierarchyEpic, IssueTypeSubtask: HierarchySubtask, IssueTypeTask: HierarchyStandard, IssueTypeBug: HierarchyStandard} {
		if got := (&Issue{IssueType: kind}).HierarchyLevel(); got != want {
			t.Errorf("%s: %d want %d", kind, got, want)
		}
	}
	withRef := &Issue{IssueType: IssueTypeTask, IssueTypeRef: &CatalogRef{HierarchyLevel: HierarchyEpic}}
	if withRef.HierarchyLevel() != HierarchyEpic {
		t.Error("the catalog level must win over the legacy type")
	}
}
