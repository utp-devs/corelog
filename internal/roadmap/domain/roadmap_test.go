package domain

import (
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/jhonsferg/corelog/internal/platform/apperror"
)

func d(y int, m time.Month, day int) *time.Time {
	t := time.Date(y, m, day, 0, 0, 0, 0, time.UTC)
	return &t
}

func TestEpicBarDatesAndProgress(t *testing.T) {
	cases := []struct {
		name             string
		row              EpicRow
		start, end       *time.Time
		startSrc, endSrc Source
		dated, complete  bool
		percent          int
	}{
		{"own dates win over the children", EpicRow{Start: d(2026, 1, 5), Due: d(2026, 2, 5), ChildStart: d(2026, 1, 10), ChildDue: d(2026, 1, 20), Total: 4, Done: 1},
			d(2026, 1, 5), d(2026, 2, 5), SourceOwn, SourceOwn, true, false, 25},
		{"dates derived from the children", EpicRow{ChildStart: d(2026, 3, 1), ChildDue: d(2026, 3, 31), Total: 3, Done: 3},
			d(2026, 3, 1), d(2026, 3, 31), SourceDerived, SourceDerived, true, true, 100},
		{"own start, derived end", EpicRow{Start: d(2026, 3, 1), ChildDue: d(2026, 4, 1), Total: 2, Done: 1},
			d(2026, 3, 1), d(2026, 4, 1), SourceOwn, SourceDerived, true, false, 50},
		{"only an own due date collapses onto it", EpicRow{Due: d(2026, 5, 1)},
			d(2026, 5, 1), d(2026, 5, 1), SourceDerived, SourceOwn, true, false, 0},
		{"only an own start collapses onto it", EpicRow{Start: d(2026, 5, 1)},
			d(2026, 5, 1), d(2026, 5, 1), SourceOwn, SourceDerived, true, false, 0},
		{"no dates at all", EpicRow{Total: 2, Todo: 2}, nil, nil, SourceNone, SourceNone, false, false, 0},
		{"derived start after the own due date", EpicRow{Due: d(2026, 5, 1), ChildStart: d(2026, 6, 1), Total: 1, InProgress: 1},
			d(2026, 5, 1), d(2026, 5, 1), SourceDerived, SourceOwn, true, false, 0},
		{"done epic without children is complete", EpicRow{StatusCategory: "done"}, nil, nil, SourceNone, SourceNone, false, true, 100},
		{"rounding", EpicRow{Total: 3, Done: 2, Start: d(2026, 1, 1), Due: d(2026, 1, 2)}, d(2026, 1, 1), d(2026, 1, 2), SourceOwn, SourceOwn, true, false, 67},
		{"all children done completes the epic", EpicRow{Total: 5, Done: 5, StatusCategory: "in_progress", Start: d(2026, 1, 1), Due: d(2026, 1, 2)}, d(2026, 1, 1), d(2026, 1, 2), SourceOwn, SourceOwn, true, true, 100},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			bar := EpicBar(c.row)
			eq := func(a, b *time.Time) bool { return (a == nil && b == nil) || (a != nil && b != nil && a.Equal(*b)) }
			if !eq(bar.Start, c.start) || !eq(bar.End, c.end) || bar.StartSource != c.startSrc || bar.EndSource != c.endSrc ||
				bar.Dated != c.dated || bar.Complete != c.complete || bar.Progress.Percent != c.percent || bar.Kind != "epic" {
				t.Fatalf("bar = %+v", bar)
			}
		})
	}
}

func TestComputeProgressKeepsCounters(t *testing.T) {
	p := ComputeProgress(10, 4, 3, 3, 20, 5.5, false)
	if p.Total != 10 || p.Todo != 4 || p.InProgress != 3 || p.Done != 3 || p.Percent != 30 || p.PointsTotal != 20 || p.PointsDone != 5.5 {
		t.Fatalf("%+v", p)
	}
	if ComputeProgress(0, 0, 0, 0, 0, 0, false).Percent != 0 || ComputeProgress(0, 0, 0, 0, 0, 0, true).Percent != 100 {
		t.Fatal("empty progress")
	}
}

func TestPeriodBar(t *testing.T) {
	v := PeriodBar(PeriodRow{ID: uuid.New(), Kind: "version", Name: "v1", Status: "released", End: d(2026, 6, 1), Total: 2, Done: 2})
	if !v.Dated || !v.Complete || v.Start.Format("2006-01-02") != "2026-06-01" || v.Progress.Percent != 100 || v.Title != "v1" {
		t.Fatalf("version: %+v", v)
	}
	s := PeriodBar(PeriodRow{Kind: "sprint", Name: "S", Status: "active", Start: d(2026, 6, 1), End: d(2026, 6, 14), Total: 4, Done: 1})
	if s.Complete || s.Progress.Percent != 25 || s.StartSource != SourceOwn {
		t.Fatalf("sprint: %+v", s)
	}
	u := PeriodBar(PeriodRow{Kind: "version", Name: "sin fecha", Status: "unreleased"})
	if u.Dated || u.Start != nil {
		t.Fatalf("undated: %+v", u)
	}
}

func TestFilterBars(t *testing.T) {
	mk := func(key, title string, start, end *time.Time, done bool) Bar {
		b := EpicBar(EpicRow{Key: key, Title: title, Start: start, Due: end})
		b.Complete = done
		return b
	}
	bars := []Bar{
		mk("A-1", "Plataforma", d(2026, 1, 1), d(2026, 1, 31), false),
		mk("A-2", "Pagos", d(2026, 2, 1), d(2026, 2, 28), true),
		mk("A-3", "Sin fechas", nil, nil, false),
		mk("A-4", "Largo", d(2025, 12, 1), d(2026, 3, 1), false),
	}
	titles := func(q Query) []string {
		var out []string
		for _, b := range FilterBars(bars, q) {
			out = append(out, b.Key)
		}
		return out
	}
	cases := []struct {
		name string
		q    Query
		want string
	}{
		{"no filter", Query{Filter: FilterAll}, "A-1,A-2,A-3,A-4"},
		{"window keeps overlaps and undated", Query{Filter: FilterAll, From: d(2026, 2, 15), To: d(2026, 4, 1)}, "A-2,A-3,A-4"},
		{"window edges are inclusive", Query{Filter: FilterAll, From: d(2026, 1, 31), To: d(2026, 1, 31)}, "A-1,A-3,A-4"},
		{"only from", Query{Filter: FilterAll, From: d(2026, 3, 2)}, "A-3"},
		{"only to", Query{Filter: FilterAll, To: d(2025, 12, 15)}, "A-3,A-4"},
		{"open", Query{Filter: FilterOpen}, "A-1,A-3,A-4"},
		{"done", Query{Filter: FilterDone}, "A-2"},
		{"text by title", Query{Filter: FilterAll, Text: "PAG"}, "A-2"},
		{"text by key", Query{Filter: FilterAll, Text: "a-3"}, "A-3"},
		{"text and open", Query{Filter: FilterOpen, Text: "pagos"}, ""},
	}
	for _, c := range cases {
		got := ""
		for i, k := range titles(c.q) {
			if i > 0 {
				got += ","
			}
			got += k
		}
		if got != c.want {
			t.Errorf("%s: got %q want %q", c.name, got, c.want)
		}
	}
}

func TestParsersAndValidation(t *testing.T) {
	for in, want := range map[string]Group{"": GroupEpic, "epic": GroupEpic, "version": GroupVersion, "sprint": GroupSprint} {
		if got, err := ParseGroup(in); err != nil || got != want {
			t.Errorf("ParseGroup(%q) = %q %v", in, got, err)
		}
	}
	if _, err := ParseGroup("team"); apperror.CodeOf(err) != CodeInvalidQuery {
		t.Errorf("bad group: %v", err)
	}
	for in, want := range map[string]Filter{"": FilterAll, "all": FilterAll, "open": FilterOpen, "done": FilterDone} {
		if got, err := ParseFilter(in); err != nil || got != want {
			t.Errorf("ParseFilter(%q) = %q %v", in, got, err)
		}
	}
	if _, err := ParseFilter("late"); apperror.CodeOf(err) != CodeInvalidQuery {
		t.Errorf("bad filter: %v", err)
	}
	if err := ValidateRange(nil, d(2026, 1, 1)); err != nil {
		t.Errorf("open range: %v", err)
	}
	if err := ValidateRange(d(2026, 2, 1), d(2026, 1, 1)); apperror.CodeOf(err) != CodeInvalidRange {
		t.Errorf("reversed range: %v", err)
	}
	if err := ValidateRange(d(2010, 1, 1), d(2030, 1, 1)); apperror.CodeOf(err) != CodeInvalidRange {
		t.Errorf("huge range: %v", err)
	}
	a := uuid.New()
	if err := ValidateRankIDs([]uuid.UUID{a, a}, 10); apperror.CodeOf(err) != CodeInvalidRank {
		t.Errorf("duplicates: %v", err)
	}
	if err := ValidateRankIDs([]uuid.UUID{a, uuid.New(), uuid.New()}, 2); apperror.CodeOf(err) != CodeInvalidRank {
		t.Errorf("too many: %v", err)
	}
	if err := ValidateRankIDs([]uuid.UUID{a, uuid.New()}, 2); err != nil {
		t.Errorf("valid ids: %v", err)
	}
}
