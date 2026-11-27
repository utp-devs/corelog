package main

import (
	"testing"

	"github.com/google/uuid"

	"github.com/jhonsferg/corelog/internal/search/domain/coreql"
)

// TestSeededFiltersAreValidCoreQL parses and validates every query the seed stores, so a typo in a demo
// filter fails here and not when someone opens it.
func TestSeededFiltersAreValidCoreQL(t *testing.T) {
	for name, build := range map[string]func(*dataset){
		"demo": func(ds *dataset) { addDemoSearchFilters(ds, uuid.New()) },
		"e2e":  func(ds *dataset) { addE2ESearchFilters(ds, uuid.New(), uuid.New()) },
	} {
		ds := newDataset()
		build(ds)
		filters := ds.byName["filters"]
		if filters == nil || len(filters.rows) == 0 {
			t.Fatalf("%s: no filters seeded", name)
		}
		queryIdx, sortIdx := columnIndex(filters, "query_text"), columnIndex(filters, "sort")
		for _, row := range filters.rows {
			q, err := coreql.Parse(row[queryIdx].(string))
			if err != nil {
				t.Errorf("%s: %q: %v", name, row[queryIdx], err)
				continue
			}
			if errs := coreql.Validate(q, nil, nil); len(errs) > 0 {
				t.Errorf("%s: %q: %s", name, row[queryIdx], errs[0].Message)
			}
			if sort := row[sortIdx].(string); sort != "" {
				oq, err := coreql.Parse("ORDER BY " + sort)
				if err != nil || len(coreql.Validate(oq, nil, nil)) > 0 {
					t.Errorf("%s: sort %q invalid: %v", name, sort, err)
				}
			}
		}
		// Deterministic ids: building twice yields the same rows.
		again := newDataset()
		build(again)
		if len(again.byName["filters"].rows) != len(filters.rows) {
			t.Errorf("%s: row count changed between runs", name)
		}
	}
}
