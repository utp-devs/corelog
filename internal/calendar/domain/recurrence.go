package domain

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/jhonsferg/corelog/internal/platform/apperror"
)

// MaxOccurrences caps the occurrences returned for one query window.
const MaxOccurrences = 1000

// MaxRuleCount and MaxRuleInterval bound the values accepted in a recurrence rule.
const (
	MaxRuleCount    = 5000
	MaxRuleInterval = 999
)

// CodeInvalidRecurrence is the stable code of every recurrence rule problem.
const CodeInvalidRecurrence = "calendar.invalid_recurrence"

func invalidRecurrence(format string, args ...any) *apperror.Error {
	return apperror.WithCode(apperror.KindValidation, CodeInvalidRecurrence, "regla de recurrencia no válida: "+fmt.Sprintf(format, args...))
}

// Freq is the frequency of a recurrence rule.
type Freq string

const (
	FreqDaily   Freq = "DAILY"
	FreqWeekly  Freq = "WEEKLY"
	FreqMonthly Freq = "MONTHLY"
	FreqYearly  Freq = "YEARLY"
)

// WeekdayRef is one BYDAY entry. Ordinal is 0 for "every such weekday" and 1..5 or -1 for the nth weekday of a
// month (monthly rules only).
type WeekdayRef struct {
	Ordinal int
	Weekday time.Weekday
}

var weekdayCodes = [...]string{"SU", "MO", "TU", "WE", "TH", "FR", "SA"}

func (w WeekdayRef) String() string {
	code := weekdayCodes[w.Weekday]
	if w.Ordinal == 0 {
		return code
	}
	return strconv.Itoa(w.Ordinal) + code
}

// Rule is the supported subset of RFC 5545 RRULE: FREQ, INTERVAL, BYDAY, COUNT and UNTIL.
type Rule struct {
	Freq     Freq
	Interval int
	ByDay    []WeekdayRef
	Count    int
	Until    *time.Time
}

// ParseRule parses an RRULE value (the "RRULE:" prefix is optional). Parts outside the subset are rejected so a
// stored rule always means what the expansion does.
func ParseRule(text string) (Rule, error) {
	text = strings.TrimSpace(text)
	text = strings.TrimPrefix(strings.TrimPrefix(text, "RRULE:"), "rrule:")
	if text == "" {
		return Rule{}, invalidRecurrence("está vacía")
	}
	rule := Rule{Interval: 1}
	seen := map[string]bool{}
	for _, part := range strings.Split(text, ";") {
		key, value, ok := strings.Cut(part, "=")
		key = strings.ToUpper(strings.TrimSpace(key))
		value = strings.TrimSpace(value)
		if !ok || key == "" || value == "" {
			return Rule{}, invalidRecurrence("la parte %q no es válida", part)
		}
		if seen[key] {
			return Rule{}, invalidRecurrence("%s está repetido", key)
		}
		seen[key] = true
		switch key {
		case "FREQ":
			switch Freq(strings.ToUpper(value)) {
			case FreqDaily, FreqWeekly, FreqMonthly, FreqYearly:
				rule.Freq = Freq(strings.ToUpper(value))
			default:
				return Rule{}, invalidRecurrence("FREQ %q no está soportado (DAILY, WEEKLY, MONTHLY, YEARLY)", value)
			}
		case "INTERVAL":
			n, err := strconv.Atoi(value)
			if err != nil || n < 1 || n > MaxRuleInterval {
				return Rule{}, invalidRecurrence("INTERVAL debe estar entre 1 y %d", MaxRuleInterval)
			}
			rule.Interval = n
		case "COUNT":
			n, err := strconv.Atoi(value)
			if err != nil || n < 1 || n > MaxRuleCount {
				return Rule{}, invalidRecurrence("COUNT debe estar entre 1 y %d", MaxRuleCount)
			}
			rule.Count = n
		case "UNTIL":
			until, err := parseUntil(value)
			if err != nil {
				return Rule{}, err
			}
			rule.Until = &until
		case "BYDAY":
			days, err := parseByDay(value)
			if err != nil {
				return Rule{}, err
			}
			rule.ByDay = days
		default:
			return Rule{}, invalidRecurrence("%s no está soportado", key)
		}
	}
	if rule.Freq == "" {
		return Rule{}, invalidRecurrence("falta FREQ")
	}
	if rule.Count > 0 && rule.Until != nil {
		return Rule{}, invalidRecurrence("COUNT y UNTIL no pueden usarse juntos")
	}
	for _, d := range rule.ByDay {
		if d.Ordinal != 0 && rule.Freq != FreqMonthly {
			return Rule{}, invalidRecurrence("BYDAY con ordinal solo se admite en reglas mensuales")
		}
	}
	if rule.Freq == FreqYearly && len(rule.ByDay) > 0 {
		return Rule{}, invalidRecurrence("BYDAY no se admite en reglas anuales")
	}
	return rule, nil
}

func parseUntil(value string) (time.Time, error) {
	if t, err := time.Parse("20060102T150405Z", value); err == nil {
		return t.UTC(), nil
	}
	if t, err := time.Parse("20060102", value); err == nil {
		// A date UNTIL includes the whole day.
		return t.UTC().Add(24*time.Hour - time.Second), nil
	}
	return time.Time{}, invalidRecurrence("UNTIL %q no es una fecha válida (AAAAMMDD o AAAAMMDDTHHMMSSZ)", value)
}

func parseByDay(value string) ([]WeekdayRef, error) {
	var out []WeekdayRef
	seen := map[WeekdayRef]bool{}
	for _, token := range strings.Split(value, ",") {
		token = strings.ToUpper(strings.TrimSpace(token))
		if len(token) < 2 {
			return nil, invalidRecurrence("BYDAY %q no es válido", token)
		}
		code := token[len(token)-2:]
		ordinal := 0
		if prefix := token[:len(token)-2]; prefix != "" {
			n, err := strconv.Atoi(prefix)
			if err != nil || n == 0 || n < -1 || n > 5 {
				return nil, invalidRecurrence("el ordinal de BYDAY %q no es válido (1 a 5 o -1)", token)
			}
			ordinal = n
		}
		day := -1
		for i, c := range weekdayCodes {
			if c == code {
				day = i
			}
		}
		if day < 0 {
			return nil, invalidRecurrence("el día %q de BYDAY no es válido", code)
		}
		ref := WeekdayRef{Ordinal: ordinal, Weekday: time.Weekday(day)}
		if seen[ref] {
			continue
		}
		seen[ref] = true
		out = append(out, ref)
	}
	return out, nil
}

// String renders the rule in canonical order.
func (r Rule) String() string {
	parts := []string{"FREQ=" + string(r.Freq)}
	if r.Interval > 1 {
		parts = append(parts, "INTERVAL="+strconv.Itoa(r.Interval))
	}
	if len(r.ByDay) > 0 {
		days := make([]string, len(r.ByDay))
		for i, d := range r.ByDay {
			days[i] = d.String()
		}
		parts = append(parts, "BYDAY="+strings.Join(days, ","))
	}
	if r.Count > 0 {
		parts = append(parts, "COUNT="+strconv.Itoa(r.Count))
	}
	if r.Until != nil {
		parts = append(parts, "UNTIL="+r.Until.UTC().Format("20060102T150405Z"))
	}
	return strings.Join(parts, ";")
}

// Override changes the fields of one occurrence. A nil field keeps the value of the series.
type Override struct {
	Title       *string
	Description *string
	Location    *string
	Color       *string
	StartsAt    *time.Time
	EndsAt      *time.Time
}

// Exception is a cancelled or overridden occurrence, identified by the instant the series generates it at.
type Exception struct {
	OriginalStart time.Time
	Cancelled     bool
	Override      *Override
}

// Occurrence is one expanded instance of an event.
type Occurrence struct {
	Start         time.Time
	End           time.Time
	OriginalStart time.Time
	Override      *Override
}

// Series is what the expansion needs of an event: its first start and end, the zone the wall clock is kept in
// (UTC for all-day events), the rule and the exceptions.
type Series struct {
	Start      time.Time
	End        time.Time
	Location   *time.Location
	Rule       *Rule
	Exceptions []Exception
}

// Expand returns the occurrences that overlap [from, to), ordered by start, at most MaxOccurrences; truncated is
// true when more existed. The wall-clock time of the first start is kept in the series zone, so a daily 09:00
// event stays at 09:00 across a DST change (the instant shifts). A wall time that does not exist on a given day
// (spring-forward gap) moves forward by the gap. Without a rule the event is its only occurrence.
func (s Series) Expand(from, to time.Time) (occurrences []Occurrence, truncated bool) {
	loc := s.Location
	if loc == nil {
		loc = time.UTC
	}
	dur := s.End.Sub(s.Start)
	overlaps := func(start, end time.Time) bool {
		if !end.After(start) {
			return !start.Before(from) && start.Before(to)
		}
		return start.Before(to) && end.After(from)
	}
	exceptions := map[int64]Exception{}
	for _, e := range s.Exceptions {
		exceptions[e.OriginalStart.Unix()] = e
	}
	if s.Rule == nil {
		start, end := s.Start, s.End
		var override *Override
		if e, ok := exceptions[s.Start.Unix()]; ok {
			if e.Cancelled {
				return nil, false
			}
			override = e.Override
			start, end = applyTimes(e.Override, start, end)
		}
		if overlaps(start, end) {
			return []Occurrence{{Start: start, End: end, OriginalStart: s.Start, Override: override}}, false
		}
		return nil, false
	}

	generate(s.Start, loc, *s.Rule, from.Add(-dur), to, func(t time.Time) bool {
		if _, hit := exceptions[t.Unix()]; hit {
			return true
		}
		if end := t.Add(dur); overlaps(t, end) {
			occurrences = append(occurrences, Occurrence{Start: t, End: end, OriginalStart: t})
		}
		return len(occurrences) <= MaxOccurrences
	})
	for _, e := range s.Exceptions {
		if e.Cancelled {
			continue
		}
		start, end := applyTimes(e.Override, e.OriginalStart, e.OriginalStart.Add(dur))
		if overlaps(start, end) {
			occurrences = append(occurrences, Occurrence{Start: start, End: end, OriginalStart: e.OriginalStart, Override: e.Override})
		}
	}
	sort.SliceStable(occurrences, func(i, j int) bool {
		if !occurrences[i].Start.Equal(occurrences[j].Start) {
			return occurrences[i].Start.Before(occurrences[j].Start)
		}
		return occurrences[i].OriginalStart.Before(occurrences[j].OriginalStart)
	})
	if len(occurrences) > MaxOccurrences {
		return occurrences[:MaxOccurrences], true
	}
	return occurrences, false
}

func applyTimes(o *Override, start, end time.Time) (time.Time, time.Time) {
	if o == nil {
		return start, end
	}
	dur := end.Sub(start)
	if o.StartsAt != nil {
		start = *o.StartsAt
		end = start.Add(dur)
	}
	if o.EndsAt != nil {
		end = *o.EndsAt
	}
	return start, end
}

// maxEmptyPeriods stops the walk over periods that produce no candidate (for example a yearly rule on Feb 29
// with a long interval) so a query always ends.
const maxEmptyPeriods = 2000

// generate walks the originals of the rule in order, from the first start, and calls emit with each one whose
// start is below to. Originals earlier than lowerBound are skipped without being emitted but still counted for
// COUNT. It stops when emit returns false.
func generate(start time.Time, loc *time.Location, rule Rule, lowerBound, to time.Time, emit func(time.Time) bool) {
	base := start.In(loc)
	interval := rule.Interval
	if interval < 1 {
		interval = 1
	}
	k := 0
	if rule.Count == 0 && lowerBound.After(start) {
		k = fastForward(base, loc, rule.Freq, interval, lowerBound)
	}
	count := 0
	empty := 0
	for ; ; k++ {
		anchor, candidates := period(base, loc, rule.Freq, interval, rule.ByDay, k)
		if anchor.After(to) {
			return
		}
		produced := false
		for _, c := range candidates {
			if c.Before(start) {
				continue
			}
			produced = true
			count++
			if rule.Count > 0 && count > rule.Count {
				return
			}
			if rule.Until != nil && c.After(*rule.Until) {
				return
			}
			if !c.Before(to) {
				return
			}
			if c.Before(lowerBound) {
				continue
			}
			if !emit(c) {
				return
			}
		}
		if produced {
			empty = 0
		} else if empty++; empty > maxEmptyPeriods {
			return
		}
	}
}

// fastForward returns the period index to start from so that no occurrence at or after lowerBound is skipped.
func fastForward(base time.Time, loc *time.Location, freq Freq, interval int, lowerBound time.Time) int {
	lb := lowerBound.In(loc)
	var diff int
	switch freq {
	case FreqDaily:
		diff = daysBetween(base, lb)
	case FreqWeekly:
		diff = daysBetween(mondayOf(base), lb) / 7
	case FreqMonthly:
		diff = (lb.Year()-base.Year())*12 + int(lb.Month()) - int(base.Month())
	case FreqYearly:
		diff = lb.Year() - base.Year()
	}
	k := diff/interval - 1
	if k < 0 {
		return 0
	}
	return k
}

func daysBetween(a, b time.Time) int {
	da := time.Date(a.Year(), a.Month(), a.Day(), 0, 0, 0, 0, time.UTC)
	db := time.Date(b.Year(), b.Month(), b.Day(), 0, 0, 0, 0, time.UTC)
	return int(db.Sub(da).Hours() / 24)
}

func mondayOf(t time.Time) time.Time {
	offset := (int(t.Weekday()) + 6) % 7
	return time.Date(t.Year(), t.Month(), t.Day()-offset, t.Hour(), t.Minute(), t.Second(), t.Nanosecond(), t.Location())
}

// period returns the first local day of period k and its candidate starts in ascending order.
func period(base time.Time, loc *time.Location, freq Freq, interval int, byDay []WeekdayRef, k int) (time.Time, []time.Time) {
	h, mi, s, ns := base.Hour(), base.Minute(), base.Second(), base.Nanosecond()
	at := func(y int, m time.Month, d int) time.Time {
		t := time.Date(y, m, d, h, mi, s, ns, loc)
		if t.Hour() != h || t.Minute() != mi {
			// The wall time does not exist (spring-forward gap): Go resolves it with the offset before the
			// change, which lands before the gap; move it forward by one hour so it lands after it.
			t = t.Add(time.Hour)
		}
		return t
	}
	switch freq {
	case FreqDaily:
		anchor := at(base.Year(), base.Month(), base.Day()+k*interval)
		if len(byDay) > 0 && !hasWeekday(byDay, anchor.Weekday()) {
			return anchor, nil
		}
		return anchor, []time.Time{anchor}
	case FreqWeekly:
		monday := mondayOf(base)
		weekStart := at(monday.Year(), monday.Month(), monday.Day()+k*interval*7)
		days := byDay
		if len(days) == 0 {
			days = []WeekdayRef{{Weekday: base.Weekday()}}
		}
		out := make([]time.Time, 0, len(days))
		for _, d := range days {
			offset := (int(d.Weekday) + 6) % 7
			out = append(out, at(weekStart.Year(), weekStart.Month(), weekStart.Day()+offset))
		}
		sort.Slice(out, func(i, j int) bool { return out[i].Before(out[j]) })
		return weekStart, out
	case FreqMonthly:
		index := base.Year()*12 + int(base.Month()) - 1 + k*interval
		y, m := index/12, time.Month(index%12+1)
		anchor := at(y, m, 1)
		if len(byDay) == 0 {
			if base.Day() > daysIn(y, m) {
				return anchor, nil
			}
			return anchor, []time.Time{at(y, m, base.Day())}
		}
		return anchor, monthWeekdays(y, m, byDay, at)
	default: // FreqYearly
		y := base.Year() + k*interval
		anchor := at(y, time.January, 1)
		if base.Month() == time.February && base.Day() == 29 && daysIn(y, time.February) < 29 {
			return anchor, nil
		}
		return anchor, []time.Time{at(y, base.Month(), base.Day())}
	}
}

func hasWeekday(days []WeekdayRef, w time.Weekday) bool {
	for _, d := range days {
		if d.Weekday == w {
			return true
		}
	}
	return false
}

func daysIn(y int, m time.Month) int {
	return time.Date(y, m+1, 0, 0, 0, 0, 0, time.UTC).Day()
}

// monthWeekdays lists the days of a month matched by BYDAY entries (every such weekday, or the nth one).
func monthWeekdays(y int, m time.Month, byDay []WeekdayRef, at func(int, time.Month, int) time.Time) []time.Time {
	total := daysIn(y, m)
	days := map[int]bool{}
	for _, ref := range byDay {
		var matches []int
		for d := 1; d <= total; d++ {
			if time.Date(y, m, d, 0, 0, 0, 0, time.UTC).Weekday() == ref.Weekday {
				matches = append(matches, d)
			}
		}
		switch {
		case ref.Ordinal == 0:
			for _, d := range matches {
				days[d] = true
			}
		case ref.Ordinal > 0 && ref.Ordinal <= len(matches):
			days[matches[ref.Ordinal-1]] = true
		case ref.Ordinal == -1 && len(matches) > 0:
			days[matches[len(matches)-1]] = true
		}
	}
	list := make([]int, 0, len(days))
	for d := range days {
		list = append(list, d)
	}
	sort.Ints(list)
	out := make([]time.Time, len(list))
	for i, d := range list {
		out[i] = at(y, m, d)
	}
	return out
}
