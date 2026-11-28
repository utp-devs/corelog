package domain

import (
	"testing"
	"time"

	"github.com/google/uuid"
)

func ts(s string) time.Time {
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		panic(err)
	}
	return t
}

func tp(s string) *time.Time { t := ts(s); return &t }

func fp(v float64) *float64 { return &v }

func ids(n int) []uuid.UUID {
	out := make([]uuid.UUID, n)
	for i := range out {
		out[i] = uuid.New()
	}
	return out
}

func ys(s Series) []float64 {
	out := make([]float64, len(s.Points))
	for i, p := range s.Points {
		out[i] = p.Y
	}
	return out
}

func eq(t *testing.T, what string, got, want []float64) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("%s = %v, want %v", what, got, want)
	}
	for i := range got {
		if got[i] != want[i] {
			t.Fatalf("%s = %v, want %v", what, got, want)
		}
	}
}

// Fixture (UTC), sprint Mon 2024-01-01 to Fri 2024-01-05:
//
//	A 5 pts, B 3 pts, C 2 pts committed on 01-01 09:00; D 4 pts added 01-03 10:00; B removed 01-04 10:00.
//	A done 01-02 12:00, C done 01-04 09:00, D done 01-05 10:00.
func burnFixture() (SprintInfo, []ScopeEvent, []IssueRow, []StatusChange) {
	id := ids(4)
	a, b, c, d := id[0], id[1], id[2], id[3]
	sprint := SprintInfo{ID: uuid.New(), Name: "Sprint 1", Status: "closed", StartDate: tp("2024-01-01T00:00:00Z"),
		EndDate: tp("2024-01-05T00:00:00Z"), CompletedAt: tp("2024-01-05T17:00:00Z")}
	events := []ScopeEvent{
		{a, EventAdded, fp(5), ts("2024-01-01T09:00:00Z"), false},
		{b, EventAdded, fp(3), ts("2024-01-01T09:00:00Z"), false},
		{c, EventAdded, fp(2), ts("2024-01-01T09:00:00Z"), false},
		{d, EventAdded, fp(4), ts("2024-01-03T10:00:00Z"), true},
		{b, EventRemoved, fp(3), ts("2024-01-04T10:00:00Z"), true},
	}
	issues := []IssueRow{{ID: a}, {ID: b}, {ID: c}, {ID: d}}
	history := []StatusChange{
		{a, ts("2024-01-02T12:00:00Z"), "done"}, {c, ts("2024-01-04T09:00:00Z"), "done"}, {d, ts("2024-01-05T10:00:00Z"), "done"},
	}
	return sprint, events, issues, history
}

func TestBurndownPointsGolden(t *testing.T) {
	s, e, i, h := burnFixture()
	r := BuildBurn(s, e, i, h, MetricPoints, false, time.UTC, ts("2024-02-01T00:00:00Z"))
	if len(r.Series) != 2 || r.Series[0].Name != "Restante" || r.Series[1].Name != "Ideal" {
		t.Fatalf("series = %+v", r.Series)
	}
	eq(t, "remaining", ys(r.Series[0]), []float64{10, 5, 9, 4, 0})
	eq(t, "ideal", ys(r.Series[1]), []float64{10, 7.5, 5, 2.5, 0})
	if r.Summary["committed"] != 10.0 || r.Summary["scope"] != 11.0 || r.Summary["completed"] != 11.0 || r.Summary["scope_change"] != 1.0 {
		t.Fatalf("summary = %v", r.Summary)
	}
	if len(r.Table.Rows) != 5 || r.Table.Rows[2][1] != 14.0 {
		t.Fatalf("table = %v", r.Table.Rows)
	}
}

func TestBurndownIssuesMetric(t *testing.T) {
	s, e, i, h := burnFixture()
	r := BuildBurn(s, e, i, h, MetricIssues, false, time.UTC, ts("2024-02-01T00:00:00Z"))
	eq(t, "remaining", ys(r.Series[0]), []float64{3, 2, 3, 1, 0})
	eq(t, "ideal", ys(r.Series[1]), []float64{3, 2.25, 1.5, 0.75, 0})
}

func TestBurnupGolden(t *testing.T) {
	s, e, i, h := burnFixture()
	r := BuildBurn(s, e, i, h, MetricPoints, true, time.UTC, ts("2024-02-01T00:00:00Z"))
	if r.Kind != KindBurnup || len(r.Series) != 2 {
		t.Fatalf("series = %+v", r.Series)
	}
	eq(t, "scope", ys(r.Series[0]), []float64{10, 10, 14, 11, 11})
	eq(t, "completed", ys(r.Series[1]), []float64{0, 5, 5, 7, 11})
}

func TestBurndownFilterDropsIssues(t *testing.T) {
	s, e, i, h := burnFixture()
	// Only A is in the filtered set: scope 5, done on day two.
	r := BuildBurn(s, e, i[:1], h, MetricPoints, false, time.UTC, ts("2024-02-01T00:00:00Z"))
	eq(t, "remaining", ys(r.Series[0]), []float64{5, 0, 0, 0, 0})
}

func TestBurndownReopenedIssue(t *testing.T) {
	s, e, i, h := burnFixture()
	// A is reopened on 01-03 and finished again on 01-05.
	h = append(h, StatusChange{i[0].ID, ts("2024-01-03T08:00:00Z"), "in_progress"}, StatusChange{i[0].ID, ts("2024-01-05T08:00:00Z"), "done"})
	r := BuildBurn(s, e, i, h, MetricPoints, false, time.UTC, ts("2024-02-01T00:00:00Z"))
	eq(t, "remaining", ys(r.Series[0]), []float64{10, 5, 14, 9, 0})
}

func TestBurndownActiveSprintStopsToday(t *testing.T) {
	s, e, i, h := burnFixture()
	s.Status, s.CompletedAt = "active", nil
	r := BuildBurn(s, e, i, h, MetricPoints, false, time.UTC, ts("2024-01-03T15:00:00Z"))
	if got := len(r.Series[0].Points); got != 3 {
		t.Fatalf("days = %d, want 3 (start to today)", got)
	}
}

func TestBurndownPlannedAndNoDatesAreEmpty(t *testing.T) {
	for name, s := range map[string]SprintInfo{
		"planned":  {Status: "planned", StartDate: tp("2024-01-01T00:00:00Z")},
		"no dates": {Status: "active"},
	} {
		r := BuildBurn(s, nil, nil, nil, MetricPoints, false, time.UTC, ts("2024-01-03T00:00:00Z"))
		if len(r.Series) != 0 || len(r.Table.Rows) != 0 || r.Summary["scope"] != 0.0 {
			t.Fatalf("%s: %+v", name, r)
		}
	}
}

func TestBurndownWithoutEndHasNoIdeal(t *testing.T) {
	s, e, i, h := burnFixture()
	s.EndDate = nil
	r := BuildBurn(s, e, i, h, MetricPoints, false, time.UTC, ts("2024-02-01T00:00:00Z"))
	if len(r.Series) != 1 || r.Table.Rows[0][4] != nil {
		t.Fatalf("series = %+v row = %v", r.Series, r.Table.Rows[0])
	}
}

func TestBurndownTimezoneBoundary(t *testing.T) {
	bogota, _ := time.LoadLocation("America/Bogota") // UTC-5, no daylight saving
	id := uuid.New()
	s := SprintInfo{ID: uuid.New(), Status: "closed", StartDate: tp("2024-01-01T00:00:00Z"), EndDate: tp("2024-01-02T00:00:00Z"),
		CompletedAt: tp("2024-01-03T04:00:00Z")} // 2024-01-02 23:00 local
	events := []ScopeEvent{{id, EventAdded, fp(8), ts("2024-01-01T10:00:00Z"), false}}
	// Done at 2024-01-02T04:30Z = 2024-01-01 23:30 local: it counts on local day one, not day two.
	history := []StatusChange{{id, ts("2024-01-02T04:30:00Z"), "done"}}
	r := BuildBurn(s, events, []IssueRow{{ID: id}}, history, MetricPoints, false, bogota, ts("2024-02-01T00:00:00Z"))
	eq(t, "remaining", ys(r.Series[0]), []float64{0, 0})
	// The same instants read in UTC put the completion on day two.
	r = BuildBurn(s, events, []IssueRow{{ID: id}}, history, MetricPoints, false, time.UTC, ts("2024-02-01T00:00:00Z"))
	eq(t, "remaining utc", ys(r.Series[0]), []float64{8, 0, 0})
}

func TestBurnWithoutHistoryUsesResolution(t *testing.T) {
	s, e, i, _ := burnFixture()
	i[0].ResolvedAt = tp("2024-01-02T10:00:00Z")
	r := BuildBurn(s, e, i, nil, MetricPoints, false, time.UTC, ts("2024-02-01T00:00:00Z"))
	if got := ys(r.Series[0]); got[0] != 10 || got[1] != 5 {
		t.Fatalf("remaining = %v", got)
	}
}

func TestVelocity(t *testing.T) {
	sprints := []SprintInfo{
		{Name: "S2", CompletedAt: tp("2024-02-01T00:00:00Z"), Committed: fp(20), Completed: fp(15)},
		{Name: "S1", CompletedAt: tp("2024-01-01T00:00:00Z"), Committed: fp(10), Completed: fp(10)},
		{Name: "S0", Committed: nil, Completed: nil},
	}
	r := BuildVelocity(sprints)
	eq(t, "committed", ys(r.Series[0]), []float64{10, 20, 0})
	eq(t, "completed", ys(r.Series[1]), []float64{10, 15, 0})
	if r.Table.Rows[1][3] != 75.0 || r.Table.Rows[2][3] != nil {
		t.Fatalf("rows = %v", r.Table.Rows)
	}
	// (10+15+0)/(10+20+0) = 83.33
	if r.Summary["commitment_ratio"] != 83.33 || r.Summary["average_completed"] != 8.33 || r.Summary["sprints"] != 3 {
		t.Fatalf("summary = %v", r.Summary)
	}
	empty := BuildVelocity(nil)
	if len(empty.Series) != 0 || empty.Summary["commitment_ratio"] != nil {
		t.Fatalf("empty = %+v", empty)
	}
}

func TestSprintSummary(t *testing.T) {
	r := BuildSprintSummary(SprintSummary{Name: "S", Status: "closed", Committed: 10, Completed: 6, Incomplete: 4, Added: 2, Removed: 1,
		Items: []SprintItem{{Key: "A-1", Summary: "x", Points: fp(6), Outcome: "completed", Added: true}, {Key: "A-2", Summary: "y", Outcome: "removed"}}})
	if r.Summary["completion_percent"] != 60.0 || len(r.Series[0].Points) != 5 || len(r.Table.Rows) != 2 {
		t.Fatalf("%+v", r)
	}
	if r.Table.Rows[0][2] != "Completada" || r.Table.Rows[0][4] != "Sí" || r.Table.Rows[1][3] != nil {
		t.Fatalf("rows = %v", r.Table.Rows)
	}
	if BuildSprintSummary(SprintSummary{}).Summary["completion_percent"] != nil {
		t.Fatal("empty summary must not divide by zero")
	}
}
