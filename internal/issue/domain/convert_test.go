package domain

import (
	"testing"

	"github.com/google/uuid"

	"github.com/jhonsferg/corelog/internal/platform/apperror"
)

// TestConvertRulesTable checks the hierarchy rules a type conversion must satisfy, for each source and target
// level and whether the caller names a parent. The parent of a subtask is a standard issue unless noted.
func TestConvertRulesTable(t *testing.T) {
	epicParent := &HierarchyNode{ID: uuid.New(), Level: HierarchyEpic}
	standardParent := &HierarchyNode{ID: uuid.New(), Level: HierarchyStandard}
	cases := []struct {
		name   string
		target int
		parent *HierarchyNode
		want   string
	}{
		{"standard to epic without parent", HierarchyEpic, nil, ""},
		{"standard to subtask without parent", HierarchySubtask, nil, "issue.hierarchy_subtask_parent_required"},
		{"standard to subtask under standard", HierarchySubtask, standardParent, ""},
		{"standard to subtask under epic", HierarchySubtask, epicParent, "issue.hierarchy_subtask_parent_invalid"},
		{"subtask to standard with epic parent", HierarchyStandard, epicParent, ""},
		{"subtask to standard with standard parent", HierarchyStandard, standardParent, "issue.hierarchy_standard_parent_invalid"},
		{"subtask to standard without parent", HierarchyStandard, nil, ""},
		{"standard to epic under a parent", HierarchyEpic, epicParent, "issue.hierarchy_epic_parent"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			child := HierarchyNode{ID: uuid.New(), Level: tc.target}
			err := CheckParent(child, tc.parent, nil)
			if tc.want == "" {
				if err != nil {
					t.Fatalf("rejected: %v", err)
				}
				return
			}
			if err == nil || apperror.CodeOf(err) != tc.want {
				t.Fatalf("err = %v, want %s", err, tc.want)
			}
		})
	}
}
