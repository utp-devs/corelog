package main

import (
	"encoding/json"
	"testing"

	"github.com/google/uuid"

	calendardomain "github.com/jhonsferg/corelog/internal/calendar/domain"
	dashboarddomain "github.com/jhonsferg/corelog/internal/dashboard/domain"
)

// TestPlanningSeedMatchesTheDomainRules checks that the seeded gadgets are in the canonical form the API would
// store and that the seeded recurrence rules parse, so the demo data never needs a fix before it can be edited.
func TestPlanningSeedMatchesTheDomainRules(t *testing.T) {
	ds := newDataset()
	owner, guest := uuid.New(), uuid.New()
	addPlanningSeed(ds, uuid.New(), owner, guest, uuid.New())

	gadgets := ds.byName["dashboard_gadgets"]
	if gadgets == nil || len(gadgets.rows) < 5 {
		t.Fatalf("gadgets not seeded: %+v", gadgets)
	}
	typeIdx, configIdx := columnIndex(gadgets, "type"), columnIndex(gadgets, "config")
	for _, row := range gadgets.rows {
		kind, config := row[typeIdx].(string), row[configIdx].(string)
		got, err := dashboarddomain.NormalizeConfig(kind, json.RawMessage(config))
		if err != nil {
			t.Errorf("%s: %v", kind, err)
			continue
		}
		var want, have any
		_ = json.Unmarshal(got, &want)
		_ = json.Unmarshal([]byte(config), &have)
		if string(got) != config && !jsonEqual(want, have) {
			t.Errorf("%s: config %s is not canonical (%s)", kind, config, got)
		}
	}
	if layout := ds.byName["dashboards"].rows[0][columnIndex(ds.byName["dashboards"], "layout")].(string); layout != `{"template":"2-1"}` {
		t.Errorf("layout = %s", layout)
	}

	events := ds.byName["calendar_events"]
	ruleIdx, exIdx := columnIndex(events, "recurrence_rule"), columnIndex(events, "recurrence_exceptions")
	recurring := 0
	for _, row := range events.rows {
		if row[ruleIdx] != nil {
			if _, err := calendardomain.ParseRule(row[ruleIdx].(string)); err != nil {
				t.Errorf("rule %v: %v", row[ruleIdx], err)
			}
			recurring++
		}
		var ex []map[string]any
		if err := json.Unmarshal([]byte(row[exIdx].(string)), &ex); err != nil {
			t.Errorf("exceptions %v: %v", row[exIdx], err)
		}
	}
	if recurring != 2 || len(events.rows) != 3 {
		t.Errorf("events: %d rows, %d recurring", len(events.rows), recurring)
	}

	again := newDataset()
	addPlanningSeed(again, uuid.New(), owner, guest, uuid.New())
	if again.byName["dashboards"].rows[0][0] != ds.byName["dashboards"].rows[0][0] {
		t.Error("dashboard ids are not deterministic")
	}
}

func jsonEqual(a, b any) bool {
	x, _ := json.Marshal(a)
	y, _ := json.Marshal(b)
	return string(x) == string(y)
}
