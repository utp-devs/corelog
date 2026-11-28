package domain

import (
	"strings"
	"testing"
	"time"
)

func mustRule(t *testing.T, text string) Rule {
	t.Helper()
	r, err := ParseRule(text)
	if err != nil {
		t.Fatalf("ParseRule(%q): %v", text, err)
	}
	return r
}

func utc(y int, m time.Month, d, h, mi int) time.Time {
	return time.Date(y, m, d, h, mi, 0, 0, time.UTC)
}

func days(occ []Occurrence, loc *time.Location) []string {
	out := make([]string, len(occ))
	for i, o := range occ {
		out[i] = o.Start.In(loc).Format("2006-01-02T15:04")
	}
	return out
}

func TestParseRuleRoundTripAndErrors(t *testing.T) {
	ok := map[string]string{
		"FREQ=DAILY": "FREQ=DAILY",
		"RRULE:freq=weekly;interval=2;byday=mo,we": "FREQ=WEEKLY;INTERVAL=2;BYDAY=MO,WE",
		"FREQ=MONTHLY;BYDAY=-1FR;COUNT=4":          "FREQ=MONTHLY;BYDAY=-1FR;COUNT=4",
		"FREQ=YEARLY;UNTIL=20301231":               "FREQ=YEARLY;UNTIL=20301231T235959Z",
		"FREQ=WEEKLY;BYDAY=MO,MO":                  "FREQ=WEEKLY;BYDAY=MO",
	}
	for in, want := range ok {
		if got := mustRule(t, in).String(); got != want {
			t.Errorf("%q -> %q, want %q", in, got, want)
		}
	}
	bad := []string{
		"", "FREQ", "INTERVAL=2", "FREQ=HOURLY", "FREQ=DAILY;INTERVAL=0", "FREQ=DAILY;INTERVAL=abc", "FREQ=DAILY;COUNT=0",
		"FREQ=DAILY;COUNT=9999", "FREQ=DAILY;COUNT=2;UNTIL=20300101", "FREQ=DAILY;UNTIL=soon", "FREQ=WEEKLY;BYDAY=XX",
		"FREQ=WEEKLY;BYDAY=2MO", "FREQ=MONTHLY;BYDAY=6MO", "FREQ=YEARLY;BYDAY=MO", "FREQ=DAILY;BYMONTHDAY=3",
		"FREQ=DAILY;FREQ=DAILY", "FREQ=DAILY;BYDAY=M",
	}
	for _, in := range bad {
		_, err := ParseRule(in)
		if err == nil {
			t.Errorf("%q accepted", in)
			continue
		}
		if !strings.Contains(err.Error(), "recurrencia") {
			t.Errorf("%q: message not in Spanish: %v", in, err)
		}
	}
}

func TestExpandTable(t *testing.T) {
	ny, _ := time.LoadLocation("America/New_York")
	cases := []struct {
		name   string
		start  time.Time
		loc    *time.Location
		rule   string
		from   time.Time
		to     time.Time
		want   []string
		inZone *time.Location
	}{
		{"daily interval 2 count 3", utc(2026, 1, 1, 9, 0), time.UTC, "FREQ=DAILY;INTERVAL=2;COUNT=3", utc(2026, 1, 1, 0, 0), utc(2026, 2, 1, 0, 0),
			[]string{"2026-01-01T09:00", "2026-01-03T09:00", "2026-01-05T09:00"}, time.UTC},
		{"weekly mon wed fri", utc(2026, 11, 2, 10, 0), time.UTC, "FREQ=WEEKLY;BYDAY=FR,MO,WE", utc(2026, 11, 2, 0, 0), utc(2026, 11, 10, 0, 0),
			[]string{"2026-11-02T10:00", "2026-11-04T10:00", "2026-11-06T10:00", "2026-11-09T10:00"}, time.UTC},
		{"weekly interval 2 sunday week boundary", utc(2026, 11, 1, 8, 0), time.UTC, "FREQ=WEEKLY;INTERVAL=2;BYDAY=SU,MO", utc(2026, 11, 1, 0, 0), utc(2026, 12, 1, 0, 0),
			[]string{"2026-11-01T08:00", "2026-11-09T08:00", "2026-11-15T08:00", "2026-11-23T08:00", "2026-11-29T08:00"}, time.UTC},
		{"monthly on 31 skips short months", utc(2026, 1, 31, 12, 0), time.UTC, "FREQ=MONTHLY", utc(2026, 1, 1, 0, 0), utc(2026, 6, 1, 0, 0),
			[]string{"2026-01-31T12:00", "2026-03-31T12:00", "2026-05-31T12:00"}, time.UTC},
		{"monthly last friday", utc(2026, 1, 30, 12, 0), time.UTC, "FREQ=MONTHLY;BYDAY=-1FR", utc(2026, 1, 1, 0, 0), utc(2026, 4, 1, 0, 0),
			[]string{"2026-01-30T12:00", "2026-02-27T12:00", "2026-03-27T12:00"}, time.UTC},
		{"monthly second monday and first friday", utc(2026, 1, 1, 12, 0), time.UTC, "FREQ=MONTHLY;BYDAY=2MO,1FR", utc(2026, 2, 1, 0, 0), utc(2026, 3, 1, 0, 0),
			[]string{"2026-02-06T12:00", "2026-02-09T12:00"}, time.UTC},
		{"yearly leap day", utc(2024, 2, 29, 0, 0), time.UTC, "FREQ=YEARLY", utc(2024, 1, 1, 0, 0), utc(2033, 1, 1, 0, 0),
			[]string{"2024-02-29T00:00", "2028-02-29T00:00", "2032-02-29T00:00"}, time.UTC},
		{"yearly interval", utc(2026, 5, 5, 5, 0), time.UTC, "FREQ=YEARLY;INTERVAL=2", utc(2026, 1, 1, 0, 0), utc(2031, 1, 1, 0, 0),
			[]string{"2026-05-05T05:00", "2028-05-05T05:00", "2030-05-05T05:00"}, time.UTC},
		{"until is inclusive", utc(2026, 3, 1, 9, 0), time.UTC, "FREQ=DAILY;UNTIL=20260303T090000Z", utc(2026, 3, 1, 0, 0), utc(2026, 3, 10, 0, 0),
			[]string{"2026-03-01T09:00", "2026-03-02T09:00", "2026-03-03T09:00"}, time.UTC},
		{"date until includes the whole day", utc(2026, 3, 1, 9, 0), time.UTC, "FREQ=DAILY;UNTIL=20260302", utc(2026, 3, 1, 0, 0), utc(2026, 3, 10, 0, 0),
			[]string{"2026-03-01T09:00", "2026-03-02T09:00"}, time.UTC},
		{"daily byday filter", utc(2026, 11, 2, 9, 0), time.UTC, "FREQ=DAILY;BYDAY=MO,TU", utc(2026, 11, 2, 0, 0), utc(2026, 11, 10, 0, 0),
			[]string{"2026-11-02T09:00", "2026-11-03T09:00", "2026-11-09T09:00"}, time.UTC},
		{"window after the series start", utc(2020, 1, 1, 9, 0), time.UTC, "FREQ=WEEKLY;BYDAY=WE", utc(2026, 11, 1, 0, 0), utc(2026, 11, 15, 0, 0),
			[]string{"2026-11-04T09:00", "2026-11-11T09:00"}, time.UTC},
		{"spring forward keeps wall time", time.Date(2026, 3, 7, 9, 0, 0, 0, ny), ny, "FREQ=DAILY", utc(2026, 3, 7, 0, 0), utc(2026, 3, 11, 0, 0),
			[]string{"2026-03-07T09:00", "2026-03-08T09:00", "2026-03-09T09:00", "2026-03-10T09:00"}, ny},
		{"fall back keeps wall time", time.Date(2026, 10, 31, 9, 0, 0, 0, ny), ny, "FREQ=DAILY", utc(2026, 10, 31, 0, 0), utc(2026, 11, 3, 20, 0),
			[]string{"2026-10-31T09:00", "2026-11-01T09:00", "2026-11-02T09:00", "2026-11-03T09:00"}, ny},
		{"time inside the spring gap moves forward", time.Date(2026, 3, 7, 2, 30, 0, 0, ny), ny, "FREQ=DAILY", utc(2026, 3, 8, 0, 0), utc(2026, 3, 9, 12, 0),
			[]string{"2026-03-08T03:30", "2026-03-09T02:30"}, ny},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			rule := mustRule(t, c.rule)
			s := Series{Start: c.start, End: c.start.Add(time.Hour), Location: c.loc, Rule: &rule}
			occ, truncated := s.Expand(c.from, c.to)
			if truncated {
				t.Fatal("unexpected truncation")
			}
			got := days(occ, c.inZone)
			if strings.Join(got, ",") != strings.Join(c.want, ",") {
				t.Fatalf("got %v\nwant %v", got, c.want)
			}
		})
	}
}

func TestExpandDSTShiftsInstant(t *testing.T) {
	ny, _ := time.LoadLocation("America/New_York")
	start := time.Date(2026, 3, 7, 9, 0, 0, 0, ny)
	rule := mustRule(t, "FREQ=DAILY")
	s := Series{Start: start, End: start.Add(time.Hour), Location: ny, Rule: &rule}
	occ, _ := s.Expand(utc(2026, 3, 7, 0, 0), utc(2026, 3, 10, 0, 0))
	if len(occ) != 3 {
		t.Fatalf("occurrences = %d", len(occ))
	}
	if occ[0].Start.UTC().Hour() != 14 || occ[1].Start.UTC().Hour() != 13 {
		t.Fatalf("UTC hours = %d, %d; want 14 then 13", occ[0].Start.UTC().Hour(), occ[1].Start.UTC().Hour())
	}
	if occ[1].End.Sub(occ[1].Start) != time.Hour {
		t.Fatalf("duration changed: %v", occ[1].End.Sub(occ[1].Start))
	}
}

func TestExpandExceptions(t *testing.T) {
	rule := mustRule(t, "FREQ=DAILY;COUNT=4")
	start := utc(2026, 5, 1, 9, 0)
	title := "Cambiado"
	moved := utc(2026, 5, 3, 15, 0)
	s := Series{Start: start, End: start.Add(time.Hour), Location: time.UTC, Rule: &rule, Exceptions: []Exception{
		{OriginalStart: utc(2026, 5, 2, 9, 0), Cancelled: true},
		{OriginalStart: utc(2026, 5, 3, 9, 0), Override: &Override{Title: &title, StartsAt: &moved}},
	}}
	occ, _ := s.Expand(utc(2026, 5, 1, 0, 0), utc(2026, 6, 1, 0, 0))
	got := days(occ, time.UTC)
	want := []string{"2026-05-01T09:00", "2026-05-03T15:00", "2026-05-04T09:00"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("got %v want %v", got, want)
	}
	if occ[1].Override == nil || *occ[1].Override.Title != title || !occ[1].OriginalStart.Equal(utc(2026, 5, 3, 9, 0)) {
		t.Fatalf("override lost: %+v", occ[1])
	}
	if !occ[1].End.Equal(moved.Add(time.Hour)) {
		t.Fatalf("moved end = %v", occ[1].End)
	}
	// An override moved into the window is found even though its original start is outside it.
	occ, _ = s.Expand(utc(2026, 5, 3, 14, 0), utc(2026, 5, 3, 16, 0))
	if len(occ) != 1 || !occ[0].Start.Equal(moved) {
		t.Fatalf("moved occurrence not found: %+v", occ)
	}
}

func TestExpandNoRule(t *testing.T) {
	start := utc(2026, 5, 1, 9, 0)
	s := Series{Start: start, End: start.Add(time.Hour)}
	if occ, _ := s.Expand(utc(2026, 5, 1, 9, 30), utc(2026, 5, 2, 0, 0)); len(occ) != 1 {
		t.Fatalf("overlapping event not returned: %v", occ)
	}
	if occ, _ := s.Expand(utc(2026, 5, 1, 10, 0), utc(2026, 5, 2, 0, 0)); len(occ) != 0 {
		t.Fatalf("event ending at from returned: %v", occ)
	}
	s.Exceptions = []Exception{{OriginalStart: start, Cancelled: true}}
	if occ, _ := s.Expand(utc(2026, 5, 1, 0, 0), utc(2026, 5, 2, 0, 0)); len(occ) != 0 {
		t.Fatalf("cancelled event returned: %v", occ)
	}
	moved := utc(2026, 5, 5, 9, 0)
	s.Exceptions = []Exception{{OriginalStart: start, Override: &Override{StartsAt: &moved}}}
	if occ, _ := s.Expand(utc(2026, 5, 5, 0, 0), utc(2026, 5, 6, 0, 0)); len(occ) != 1 || !occ[0].End.Equal(moved.Add(time.Hour)) {
		t.Fatalf("moved single event: %v", occ)
	}
	// Zero-length events count when they start inside the window.
	z := Series{Start: start, End: start}
	if occ, _ := z.Expand(utc(2026, 5, 1, 9, 0), utc(2026, 5, 1, 9, 1)); len(occ) != 1 {
		t.Fatalf("zero-length event: %v", occ)
	}
}

func TestExpandCapsOccurrences(t *testing.T) {
	rule := mustRule(t, "FREQ=DAILY")
	start := utc(2026, 1, 1, 0, 0)
	s := Series{Start: start, End: start.Add(time.Minute), Location: time.UTC, Rule: &rule}
	occ, truncated := s.Expand(start, start.AddDate(10, 0, 0))
	if len(occ) != MaxOccurrences || !truncated {
		t.Fatalf("len=%d truncated=%v", len(occ), truncated)
	}
	short := mustRule(t, "FREQ=DAILY;COUNT=10")
	s.Rule = &short
	if occ, truncated := s.Expand(start, start.AddDate(10, 0, 0)); len(occ) != 10 || truncated {
		t.Fatalf("len=%d truncated=%v", len(occ), truncated)
	}
}

func TestExpandFastForwardMatchesFullWalk(t *testing.T) {
	ny, _ := time.LoadLocation("America/New_York")
	rules := []string{"FREQ=DAILY;INTERVAL=3", "FREQ=WEEKLY;BYDAY=TU,SA;INTERVAL=2", "FREQ=MONTHLY;INTERVAL=5", "FREQ=MONTHLY;BYDAY=3TH", "FREQ=YEARLY;INTERVAL=2"}
	start := time.Date(2022, 2, 28, 18, 30, 0, 0, ny)
	for _, text := range rules {
		rule := mustRule(t, text)
		s := Series{Start: start, End: start.Add(90 * time.Minute), Location: ny, Rule: &rule}
		from, to := utc(2026, 6, 1, 0, 0), utc(2027, 2, 1, 0, 0)
		fast, _ := s.Expand(from, to)
		full, _ := s.Expand(start, to)
		var filtered []Occurrence
		for _, o := range full {
			if o.End.After(from) {
				filtered = append(filtered, o)
			}
		}
		if strings.Join(days(fast, ny), ",") != strings.Join(days(filtered, ny), ",") {
			t.Errorf("%s: fast-forward differs from the full walk\nfast %v\nfull %v", text, days(fast, ny), days(filtered, ny))
		}
	}
}

func TestExpandOverlongEventStartingBeforeWindow(t *testing.T) {
	rule := mustRule(t, "FREQ=WEEKLY")
	start := utc(2026, 11, 2, 22, 0)
	s := Series{Start: start, End: start.Add(48 * time.Hour), Location: time.UTC, Rule: &rule}
	occ, _ := s.Expand(utc(2026, 11, 10, 0, 0), utc(2026, 11, 11, 0, 0))
	if len(occ) != 1 || !occ[0].Start.Equal(utc(2026, 11, 9, 22, 0)) {
		t.Fatalf("multi-day occurrence in progress not returned: %v", occ)
	}
}

func TestExpandYearlyLeapDayLongIntervalTerminates(t *testing.T) {
	rule := mustRule(t, "FREQ=YEARLY;INTERVAL=3")
	start := utc(2024, 2, 29, 0, 0)
	s := Series{Start: start, End: start.Add(time.Hour), Location: time.UTC, Rule: &rule}
	occ, _ := s.Expand(utc(2024, 1, 1, 0, 0), utc(2060, 1, 1, 0, 0))
	// 2024, 2036, 2048 (every 12 years the leap day lines up with the 3-year step).
	if got := strings.Join(days(occ, time.UTC), ","); got != "2024-02-29T00:00,2036-02-29T00:00,2048-02-29T00:00" {
		t.Fatalf("got %s", got)
	}
}

func FuzzParseRule(f *testing.F) {
	for _, seed := range []string{"FREQ=DAILY", "FREQ=WEEKLY;BYDAY=MO,TU;COUNT=3", "FREQ=MONTHLY;BYDAY=-1FR", "", ";;;", "FREQ=YEARLY;UNTIL=20300101"} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, in string) {
		rule, err := ParseRule(in)
		if err != nil {
			return
		}
		again, err := ParseRule(rule.String())
		if err != nil || again.String() != rule.String() {
			t.Fatalf("round trip of %q failed: %v", in, err)
		}
	})
}
