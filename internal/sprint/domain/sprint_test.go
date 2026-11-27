package domain

import (
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
)

var t0 = time.Date(2026, 11, 2, 9, 0, 0, 0, time.UTC)

func day(n int) time.Time { return time.Date(2026, 11, 2+n, 0, 0, 0, 0, time.UTC) }

func ptr[T any](v T) *T { return &v }

func TestSprintLifecycleRules(t *testing.T) {
	project := uuid.New()
	newPlanned := func() *Sprint {
		s, err := NewSprint(t0, project, "Sprint 1", "meta", ptr(day(0)), ptr(day(13)))
		if err != nil {
			t.Fatal(err)
		}
		return s
	}
	cases := []struct {
		name string
		run  func() error
		want error
	}{
		{"empty name", func() error { _, err := NewSprint(t0, project, "  ", "", nil, nil); return err }, ErrInvalidSprint},
		{"no project", func() error { _, err := NewSprint(t0, uuid.Nil, "x", "", nil, nil); return err }, ErrInvalidSprint},
		{"end before start", func() error { _, err := NewSprint(t0, project, "x", "", ptr(day(5)), ptr(day(1))); return err }, ErrInvalidSprintDates},
		{"start planned", func() error { return newPlanned().Start(t0, StartParams{}) }, nil},
		{"start twice", func() error {
			s := newPlanned()
			_ = s.Start(t0, StartParams{})
			return s.Start(t0, StartParams{})
		}, ErrSprintNotPlanned},
		{"start with bad dates", func() error {
			return newPlanned().Start(t0, StartParams{EndDate: ptr(day(-3))})
		}, ErrInvalidSprintDates},
		{"complete planned", func() error { return newPlanned().Complete(t0, 0) }, ErrSprintNotActive},
		{"complete twice", func() error {
			s := newPlanned()
			_ = s.Start(t0, StartParams{})
			_ = s.Complete(t0, 3)
			return s.Complete(t0, 3)
		}, ErrSprintNotActive},
		{"delete planned", func() error { return newPlanned().CheckDeletable() }, nil},
		{"delete active", func() error {
			s := newPlanned()
			_ = s.Start(t0, StartParams{})
			return s.CheckDeletable()
		}, ErrSprintNotDeletable},
		{"edit closed", func() error {
			s := newPlanned()
			_ = s.Start(t0, StartParams{})
			_ = s.Complete(t0, 0)
			return s.Edit(t0, Patch{Name: ptr("otro")})
		}, ErrSprintClosed},
		{"active sprint cannot lose dates", func() error {
			s := newPlanned()
			_ = s.Start(t0, StartParams{})
			return s.Edit(t0, Patch{EndDate: &time.Time{}})
		}, ErrSprintWithoutDates},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if err := tc.run(); !errors.Is(err, tc.want) && err != tc.want {
				t.Fatalf("got %v, want %v", err, tc.want)
			}
		})
	}
}

func TestStartDefaultsDatesAndFreezesData(t *testing.T) {
	s, _ := NewSprint(t0, uuid.New(), "Sprint 1", "", nil, nil)
	if err := s.Start(t0, StartParams{Name: ptr("Sprint uno"), Goal: ptr("Salir")}); err != nil {
		t.Fatal(err)
	}
	if s.Status != StatusActive || s.Name != "Sprint uno" || s.Goal != "Salir" || s.StartedAt == nil {
		t.Fatalf("unexpected sprint %+v", s)
	}
	if !s.StartDate.Equal(day(0)) || !s.EndDate.Equal(day(14)) {
		t.Fatalf("default dates = %v..%v", s.StartDate, s.EndDate)
	}
}

func TestParseDestinationAndNames(t *testing.T) {
	id := uuid.New()
	good := map[string]Destination{
		"": {Kind: DestinationBacklog}, "backlog": {Kind: DestinationBacklog}, "new": {Kind: DestinationNew},
		"sprint:" + id.String(): {Kind: DestinationSprint, SprintID: id},
	}
	for raw, want := range good {
		got, err := ParseDestination(raw)
		if err != nil || got != want {
			t.Fatalf("%q = %+v, %v", raw, got, err)
		}
	}
	for _, raw := range []string{"sprint:", "sprint:nope", "other", "NEW"} {
		if _, err := ParseDestination(raw); err != ErrInvalidCloseOption {
			t.Fatalf("%q: want invalid option, got %v", raw, err)
		}
	}
	for in, want := range map[string]string{"Sprint 12": "Sprint 13", "Sprint": "Sprint 2", "S9": "S10", "Q4 2026": "Q4 2027"} {
		if got := NextSprintName(in); got != want {
			t.Fatalf("NextSprintName(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestPlanSectionPlacesBetweenNeighbours(t *testing.T) {
	a, b, c := uuid.New(), uuid.New(), uuid.New()
	existing := []RankedID{{a, "d"}, {b, "m"}, {c, "w"}}
	m1, m2 := uuid.New(), uuid.New()
	for pos := 0; pos <= 3; pos++ {
		ranks, rebalanced := PlanSection(existing, []uuid.UUID{m1, m2}, pos)
		if rebalanced || !(ranks[m1] < ranks[m2]) {
			t.Fatalf("pos %d: ranks %v rebalanced %v", pos, ranks, rebalanced)
		}
		if pos > 0 && !(existing[pos-1].Rank < ranks[m1]) {
			t.Fatalf("pos %d: not after lower neighbour", pos)
		}
		if pos < 3 && !(ranks[m2] < existing[pos].Rank) {
			t.Fatalf("pos %d: not before upper neighbour", pos)
		}
	}
}

func TestPlanSectionRebalancesWhenRanksCollide(t *testing.T) {
	a, b := uuid.New(), uuid.New()
	existing := []RankedID{{a, "m"}, {b, "m"}}
	moved := uuid.New()
	ranks, rebalanced := PlanSection(existing, []uuid.UUID{moved}, 1)
	if !rebalanced || len(ranks) != 3 {
		t.Fatalf("want a full rebalance of 3 issues, got %v (%v)", ranks, rebalanced)
	}
	if !(ranks[a] < ranks[moved] && ranks[moved] < ranks[b]) {
		t.Fatalf("order not kept: %v", ranks)
	}
}

func TestPositionFor(t *testing.T) {
	a, b := uuid.New(), uuid.New()
	existing := []RankedID{{a, "d"}, {b, "m"}}
	if pos, err := PositionFor(existing, &b, nil); err != nil || pos != 1 {
		t.Fatalf("before b = %d, %v", pos, err)
	}
	if pos, err := PositionFor(existing, nil, &a); err != nil || pos != 1 {
		t.Fatalf("after a = %d, %v", pos, err)
	}
	if pos, err := PositionFor(existing, nil, nil); err != nil || pos != 2 {
		t.Fatalf("no anchor = %d, %v", pos, err)
	}
	if _, err := PositionFor(existing, &a, &b); err == nil {
		t.Fatal("both anchors must fail")
	}
	unknown := uuid.New()
	if _, err := PositionFor(existing, &unknown, nil); err != ErrInvalidAnchor {
		t.Fatalf("unknown anchor: %v", err)
	}
}

func ev(sprint, issue uuid.UUID, kind EventKind, points float64, at time.Time, after bool) Event {
	return Event{SprintID: sprint, IssueID: issue, Kind: kind, Points: ptr(points), HappenedAt: at, AfterStart: after}
}

// TestBuildReportScenarios covers add, remove, move, re-add and a points change mid sprint.
func TestBuildReportScenarios(t *testing.T) {
	sp := uuid.New()
	a, b, c, d, e := uuid.New(), uuid.New(), uuid.New(), uuid.New(), uuid.New()
	sprint := &Sprint{ID: sp, Status: StatusClosed, StartDate: ptr(day(0)), EndDate: ptr(day(4)), StartedAt: ptr(day(0).Add(time.Hour)),
		CompletedAt: ptr(day(4).Add(time.Hour)), CommittedPoints: ptr(10.0), CompletedPoints: ptr(5.0)}
	events := []Event{
		ev(sp, a, EventAdded, 5, day(0).Add(time.Hour), false),
		ev(sp, b, EventAdded, 3, day(0).Add(time.Hour), false),
		ev(sp, c, EventAdded, 2, day(0).Add(time.Hour), false),
		ev(sp, d, EventAdded, 8, day(1).Add(time.Hour), true),     // added after start
		ev(sp, b, EventRemoved, 3, day(1).Add(2*time.Hour), true), // removed...
		ev(sp, b, EventAdded, 3, day(2).Add(time.Hour), true),     // ...and re-added: committed, not "added after start"
		ev(sp, c, EventPointsChanged, 4, day(2).Add(time.Hour), true),
		ev(sp, e, EventAdded, 1, day(2).Add(2*time.Hour), true),
		ev(sp, e, EventMovedOut, 1, day(3).Add(time.Hour), true), // moved to another sprint
		ev(sp, a, EventCompleted, 5, day(4).Add(time.Hour), true),
		ev(sp, b, EventIncomplete, 3, day(4).Add(time.Hour), true),
		ev(sp, c, EventIncomplete, 4, day(4).Add(time.Hour), true),
		ev(sp, d, EventIncomplete, 8, day(4).Add(time.Hour), true),
	}
	alive := map[uuid.UUID]string{a: CategoryDone, b: CategoryTodo, c: CategoryInProgress, d: CategoryTodo, e: CategoryTodo}
	timelines := map[uuid.UUID]DoneTimeline{a: {Changes: []StatusChange{{At: day(2), Done: true}}}}
	r := BuildReport(sprint, events, alive, timelines, day(10))

	if r.CommittedPoints != 10 || r.CompletedPoints != 5 || r.IncompletePoints != 15 {
		t.Fatalf("points committed=%v completed=%v incomplete=%v", r.CommittedPoints, r.CompletedPoints, r.IncompletePoints)
	}
	if len(r.Completed) != 1 || r.Completed[0].IssueID != a {
		t.Fatalf("completed %+v", r.Completed)
	}
	if len(r.Incomplete) != 3 {
		t.Fatalf("incomplete %+v", r.Incomplete)
	}
	if len(r.AddedAfterStart) != 1 || r.AddedAfterStart[0].IssueID != d || r.AddedPoints != 8 {
		t.Fatalf("added after start %+v (%v)", r.AddedAfterStart, r.AddedPoints)
	}
	if len(r.Removed) != 1 || r.Removed[0].IssueID != e || r.Removed[0].RemovedKind != EventMovedOut || r.RemovedPoints != 1 {
		t.Fatalf("removed %+v", r.Removed)
	}
	for _, i := range r.Incomplete {
		if i.IssueID == c && *i.Points != 4 {
			t.Fatalf("points of c = %v, want the value after the change", *i.Points)
		}
	}
	if len(r.Burndown) != 5 {
		t.Fatalf("burndown days = %d", len(r.Burndown))
	}
	// Day 1: b out, d in = 15. Day 2: b back, c at 4, e in, a done = 21/16. Day 3: e moved out.
	want := []struct{ scope, remaining float64 }{{10, 10}, {15, 15}, {21, 16}, {20, 15}, {20, 15}}
	for i, w := range want {
		got := r.Burndown[i]
		if got.Scope != w.scope || got.Remaining != w.remaining {
			t.Fatalf("day %d: scope %v remaining %v, want %v/%v", i, got.Scope, got.Remaining, w.scope, w.remaining)
		}
	}
	if r.Burndown[0].Ideal == nil || *r.Burndown[0].Ideal != 10 || *r.Burndown[4].Ideal != 0 {
		t.Fatalf("ideal line %v .. %v", r.Burndown[0].Ideal, r.Burndown[4].Ideal)
	}
}

func TestBuildReportActiveSprintUsesLiveCategoriesAndIgnoresDeleted(t *testing.T) {
	sp := uuid.New()
	a, b, gone := uuid.New(), uuid.New(), uuid.New()
	sprint := &Sprint{ID: sp, Status: StatusActive, StartDate: ptr(day(0)), EndDate: ptr(day(6)), StartedAt: ptr(day(0))}
	events := []Event{
		ev(sp, a, EventAdded, 2, day(0), false), ev(sp, b, EventAdded, 3, day(0), false), ev(sp, gone, EventAdded, 9, day(0), false),
	}
	r := BuildReport(sprint, events, map[uuid.UUID]string{a: CategoryDone, b: CategoryInProgress}, nil, day(2))
	if r.CommittedPoints != 14 { // derived from the snapshot, deleted issue included in the frozen commitment
		t.Fatalf("committed = %v", r.CommittedPoints)
	}
	if len(r.Completed) != 1 || len(r.Incomplete) != 1 || r.CompletedPoints != 2 || r.IncompletePoints != 3 {
		t.Fatalf("report %+v", r)
	}
	if len(r.Burndown) != 3 { // day 0..2: stops today
		t.Fatalf("burndown days = %d", len(r.Burndown))
	}
}

func TestDoneTimeline(t *testing.T) {
	tl := DoneTimeline{Changes: []StatusChange{{At: day(1), Done: true}, {At: day(3), Done: false}}}
	for at, want := range map[time.Time]bool{day(0): false, day(1): true, day(2): true, day(3): false} {
		if tl.DoneBy(at) != want {
			t.Fatalf("DoneBy(%v) != %v", at, want)
		}
	}
	legacy := DoneTimeline{DoneAt: ptr(day(2))}
	if legacy.DoneBy(day(1)) || !legacy.DoneBy(day(2)) {
		t.Fatal("legacy fallback wrong")
	}
	if (DoneTimeline{}).DoneBy(day(5)) {
		t.Fatal("an open issue is never done")
	}
}

func TestPlanCompletionAndSnapshot(t *testing.T) {
	sp, a, b := uuid.New(), uuid.New(), uuid.New()
	members := []Member{{a, ptr(5.0), CategoryDone}, {b, ptr(3.0), CategoryInProgress}, {uuid.New(), nil, CategoryTodo}}
	events, committed := Snapshot(sp, nil, t0, members)
	if committed != 8 || len(events) != 3 || events[0].AfterStart {
		t.Fatalf("snapshot %v %+v", committed, events)
	}
	c := PlanCompletion(sp, nil, t0, members)
	if c.CompletedPoints != 5 || c.IncompletePoints != 3 || c.Completed != 1 || len(c.Incomplete) != 2 {
		t.Fatalf("completion %+v", c)
	}
}
