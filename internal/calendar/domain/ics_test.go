package domain

import (
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/jhonsferg/corelog/internal/platform/apperror"
)

func TestICSEscape(t *testing.T) {
	got := ICSEscape("a\\b;c,d\r\ne\nf\rg")
	want := `a\\b\;c\,d\ne\nf\ng`
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestICSFoldNeverExceedsLimitNorSplitsRunes(t *testing.T) {
	for _, text := range []string{strings.Repeat("a", 200), strings.Repeat("ñ", 100), strings.Repeat("日本", 60), "corto"} {
		folded := ICSFold("SUMMARY:" + text)
		lines := strings.Split(folded, "\r\n")
		var unfolded strings.Builder
		for i, l := range lines {
			if len(l) > 75 {
				t.Fatalf("line of %d octets", len(l))
			}
			if !utf8.ValidString(l) {
				t.Fatalf("split rune in %q", l)
			}
			if i > 0 {
				if !strings.HasPrefix(l, " ") {
					t.Fatalf("continuation without space: %q", l)
				}
				l = l[1:]
			}
			unfolded.WriteString(l)
		}
		if unfolded.String() != "SUMMARY:"+text {
			t.Fatalf("unfold differs")
		}
	}
	if got := ICSFold(strings.Repeat("x", 75)); strings.Contains(got, "\r\n") {
		t.Fatal("a 75 octet line must not fold")
	}
	if got := ICSFold(strings.Repeat("x", 76)); !strings.Contains(got, "\r\n ") {
		t.Fatal("a 76 octet line must fold")
	}
}

func eventID(n byte) uuid.UUID { return uuid.UUID{15: n} }

func TestRenderICSGolden(t *testing.T) {
	stamp := utc(2026, 11, 1, 8, 0)
	rule := "FREQ=WEEKLY;COUNT=3"
	cancelled := utc(2026, 11, 9, 10, 0)
	moved := utc(2026, 11, 16, 12, 0)
	title := "Cambio; de, sala"
	events := []Event{
		{ID: eventID(2), Title: "Revisión; semanal, equipo", Description: "Línea 1\nLínea 2", Location: "Sala \\ 4",
			StartsAt: utc(2026, 11, 2, 10, 0), EndsAt: utc(2026, 11, 2, 11, 0), Timezone: "UTC", RecurrenceRule: &rule, UpdatedAt: stamp,
			Exceptions: []Exception{{OriginalStart: cancelled, Cancelled: true}, {OriginalStart: utc(2026, 11, 16, 10, 0), Override: &Override{Title: &title, StartsAt: &moved}}}},
		{ID: eventID(1), Title: "Festivo", AllDay: true, StartsAt: utc(2026, 11, 5, 0, 0), EndsAt: utc(2026, 11, 6, 0, 0), Timezone: "UTC", UpdatedAt: stamp},
	}
	got := RenderICS("CoreLog", events, utc(2026, 10, 1, 0, 0), utc(2027, 1, 1, 0, 0))
	want := strings.Join([]string{
		"BEGIN:VCALENDAR", "VERSION:2.0", "PRODID:-//CoreLog//Calendario//ES", "CALSCALE:GREGORIAN", "METHOD:PUBLISH", "X-WR-CALNAME:CoreLog",
		"BEGIN:VEVENT", "UID:00000000-0000-0000-0000-000000000002@corelog", "DTSTAMP:20261101T080000Z", "DTSTART:20261102T100000Z", "DTEND:20261102T110000Z",
		"RRULE:FREQ=WEEKLY;COUNT=3", "EXDATE:20261109T100000Z", "EXDATE:20261116T100000Z",
		`SUMMARY:Revisión\; semanal\, equipo`, `DESCRIPTION:Línea 1\nLínea 2`, `LOCATION:Sala \\ 4`, "END:VEVENT",
		"BEGIN:VEVENT", "UID:00000000-0000-0000-0000-000000000002@corelog", "DTSTAMP:20261101T080000Z", "DTSTART:20261116T120000Z", "DTEND:20261116T130000Z",
		"RECURRENCE-ID:20261116T100000Z", `SUMMARY:Cambio\; de\, sala`, `DESCRIPTION:Línea 1\nLínea 2`, `LOCATION:Sala \\ 4`, "END:VEVENT",
		"BEGIN:VEVENT", "UID:00000000-0000-0000-0000-000000000001@corelog", "DTSTAMP:20261101T080000Z", "DTSTART;VALUE=DATE:20261105", "DTEND;VALUE=DATE:20261106",
		"SUMMARY:Festivo", "END:VEVENT",
		"END:VCALENDAR", "",
	}, "\r\n")
	// The golden text is compared after folding the expected lines the same way the writer does.
	var folded []string
	for _, l := range strings.Split(strings.TrimSuffix(want, "\r\n"), "\r\n") {
		folded = append(folded, ICSFold(l))
	}
	want = strings.Join(folded, "\r\n") + "\r\n"
	if got != want {
		t.Fatalf("ICS differs\n--- got\n%s\n--- want\n%s", got, want)
	}
}

func TestRenderICSNonUTCRecurringIsExpanded(t *testing.T) {
	ny, _ := time.LoadLocation("America/New_York")
	rule := "FREQ=DAILY;COUNT=3"
	start := time.Date(2026, 3, 7, 9, 0, 0, 0, ny)
	e := Event{ID: eventID(3), Title: "Diaria", StartsAt: start.UTC(), EndsAt: start.Add(time.Hour).UTC(), Timezone: "America/New_York", RecurrenceRule: &rule, UpdatedAt: utc(2026, 1, 1, 0, 0)}
	got := RenderICS("CoreLog", []Event{e}, utc(2026, 3, 1, 0, 0), utc(2026, 4, 1, 0, 0))
	if strings.Contains(got, "RRULE") {
		t.Fatal("a zoned recurring event must not be exported with a UTC RRULE")
	}
	for _, want := range []string{"DTSTART:20260307T140000Z", "DTSTART:20260308T130000Z", "DTSTART:20260309T130000Z"} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %s in\n%s", want, got)
		}
	}
	if n := strings.Count(got, "BEGIN:VEVENT"); n != 3 {
		t.Fatalf("VEVENTs = %d", n)
	}
	if strings.Count(got, "UID:") != 3 {
		t.Fatal("each occurrence needs its own UID")
	}
}

func TestRenderICSAllDayRecurringUntilIsADate(t *testing.T) {
	rule := "FREQ=DAILY;UNTIL=20261110"
	e := Event{ID: eventID(4), Title: "x", AllDay: true, StartsAt: utc(2026, 11, 5, 0, 0), EndsAt: utc(2026, 11, 6, 0, 0), RecurrenceRule: &rule, UpdatedAt: utc(2026, 1, 1, 0, 0),
		Exceptions: []Exception{{OriginalStart: utc(2026, 11, 7, 0, 0), Cancelled: true}}}
	got := RenderICS("c", []Event{e}, utc(2026, 11, 1, 0, 0), utc(2026, 12, 1, 0, 0))
	if !strings.Contains(got, "RRULE:FREQ=DAILY;UNTIL=20261110\r\n") || !strings.Contains(got, "EXDATE;VALUE=DATE:20261107\r\n") {
		t.Fatalf("all-day rule not rendered with dates:\n%s", got)
	}
}

func TestRenderICSEveryLineIsFoldedAndCRLF(t *testing.T) {
	e := Event{ID: eventID(5), Title: strings.Repeat("título largo ", 20), StartsAt: utc(2026, 11, 5, 9, 0), EndsAt: utc(2026, 11, 5, 10, 0), UpdatedAt: utc(2026, 1, 1, 0, 0)}
	got := RenderICS("c", []Event{e}, utc(2026, 11, 1, 0, 0), utc(2026, 12, 1, 0, 0))
	if strings.Contains(strings.ReplaceAll(got, "\r\n", ""), "\n") {
		t.Fatal("bare LF in output")
	}
	for _, l := range strings.Split(got, "\r\n") {
		if len(l) > 75 {
			t.Fatalf("line of %d octets: %q", len(l), l)
		}
	}
}

func TestEventNormalize(t *testing.T) {
	base := func() Event {
		return Event{Title: " Reunión ", StartsAt: utc(2026, 11, 5, 9, 0), EndsAt: utc(2026, 11, 5, 10, 0)}
	}
	e := base()
	if err := e.Normalize(); err != nil || e.Title != "Reunión" || e.Timezone != "UTC" || e.Visibility != VisibilityPrivate || e.Color != "blue" {
		t.Fatalf("defaults: %+v %v", e, err)
	}
	bad := map[string]func(*Event){
		"empty title":      func(e *Event) { e.Title = "  " },
		"long title":       func(e *Event) { e.Title = strings.Repeat("a", 201) },
		"long description": func(e *Event) { e.Description = strings.Repeat("a", 8001) },
		"long location":    func(e *Event) { e.Location = strings.Repeat("a", 301) },
		"timezone":         func(e *Event) { e.Timezone = "Mars/Base" },
		"color":            func(e *Event) { e.Color = "chartreuse" },
		"visibility":       func(e *Event) { e.Visibility = "public" },
		"project no id":    func(e *Event) { e.Visibility = VisibilityProject },
		"end before start": func(e *Event) { e.EndsAt = e.StartsAt.Add(-time.Hour) },
		"all day time":     func(e *Event) { e.AllDay = true },
		"all day same":     func(e *Event) { e.AllDay = true; e.StartsAt = utc(2026, 11, 5, 0, 0); e.EndsAt = e.StartsAt },
		"bad rule":         func(e *Event) { r := "FREQ=HOURLY"; e.RecurrenceRule = &r },
		"attendees":        func(e *Event) { e.Attendees = make([]Attendee, 101) },
	}
	for name, mutate := range bad {
		e := base()
		mutate(&e)
		err := e.Normalize()
		if err == nil || apperror.KindOf(err) != apperror.KindValidation {
			t.Errorf("%s: err = %v", name, err)
		}
	}
	r := "freq=daily;interval=1"
	e = base()
	e.RecurrenceRule = &r
	if err := e.Normalize(); err != nil || *e.RecurrenceRule != "FREQ=DAILY" {
		t.Fatalf("rule not canonical: %v %v", err, e.RecurrenceRule)
	}
}

func TestEventExceptionsAndOccurrences(t *testing.T) {
	r := "FREQ=WEEKLY;BYDAY=MO"
	e := Event{Title: "x", StartsAt: utc(2026, 11, 2, 9, 0), EndsAt: utc(2026, 11, 2, 10, 0), RecurrenceRule: &r}
	if err := e.Normalize(); err != nil {
		t.Fatal(err)
	}
	if !e.IsOccurrence(utc(2026, 11, 9, 9, 0)) || e.IsOccurrence(utc(2026, 11, 10, 9, 0)) {
		t.Fatal("IsOccurrence wrong for a recurring event")
	}
	single := Event{StartsAt: utc(2026, 11, 2, 9, 0), EndsAt: utc(2026, 11, 2, 10, 0)}
	if !single.IsOccurrence(single.StartsAt) || single.IsOccurrence(utc(2026, 11, 9, 9, 0)) {
		t.Fatal("IsOccurrence wrong for a single event")
	}
	x := Exception{OriginalStart: utc(2026, 11, 9, 9, 0), Cancelled: true}
	if err := e.SetException(x); err != nil {
		t.Fatal(err)
	}
	x.Cancelled = false
	if err := e.SetException(x); err != nil || len(e.Exceptions) != 1 || e.Exceptions[0].Cancelled {
		t.Fatalf("exception not replaced: %+v %v", e.Exceptions, err)
	}
	e.Exceptions = make([]Exception, MaxExceptions)
	if err := e.SetException(Exception{OriginalStart: utc(2030, 1, 1, 0, 0)}); err == nil {
		t.Fatal("exception limit not enforced")
	}
	if !ValidResponse(ResponseTentative) || ValidResponse("maybe") {
		t.Fatal("ValidResponse")
	}
}
