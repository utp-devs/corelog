package domain

import (
	"testing"

	"github.com/google/uuid"
)

func TestNewSavedFilterNormalizesAndValidatesCriteria(t *testing.T) {
	ownerID := uuid.New()
	filter, err := NewSavedFilter(ownerID, "  Current bugs  ", SavedFilterCriteria{
		Status: StatusInProgress, Priority: PriorityHigh, IssueType: IssueTypeBug, Query: "  timeout  ",
	})
	if err != nil {
		t.Fatal(err)
	}
	if filter.Name != "Current bugs" || filter.Criteria.Query != "timeout" || filter.OwnerID != ownerID {
		t.Fatalf("unexpected saved filter: %+v", filter)
	}
	for _, tc := range []struct {
		name     string
		owner    uuid.UUID
		criteria SavedFilterCriteria
	}{
		{name: "missing owner", criteria: SavedFilterCriteria{}},
		{name: "invalid status", owner: ownerID, criteria: SavedFilterCriteria{Status: "unknown"}},
		{name: "invalid priority", owner: ownerID, criteria: SavedFilterCriteria{Priority: "unknown"}},
		{name: "invalid issue type", owner: ownerID, criteria: SavedFilterCriteria{IssueType: "unknown"}},
		{name: "invalid date", owner: ownerID, criteria: SavedFilterCriteria{DueFrom: "yesterday"}},
		{name: "reversed date range", owner: ownerID, criteria: SavedFilterCriteria{DueFrom: "2026-02-01T00:00:00Z", DueTo: "2026-01-01T00:00:00Z"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := NewSavedFilter(tc.owner, "Valid", tc.criteria); err == nil {
				t.Fatal("expected invalid saved filter error")
			}
		})
	}
}
