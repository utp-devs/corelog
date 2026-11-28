package domain

import (
	"fmt"
	"sort"
	"strings"
	"time"
	"unicode/utf8"
)

const (
	icsDateTime = "20060102T150405Z"
	icsDate     = "20060102"
	icsLineMax  = 75
)

// ICSEscape escapes a TEXT value (RFC 5545 section 3.3.11): backslash, semicolon, comma and line breaks.
func ICSEscape(s string) string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.ReplaceAll(s, "\r", "\n")
	var b strings.Builder
	for _, r := range s {
		switch r {
		case '\\':
			b.WriteString(`\\`)
		case ';':
			b.WriteString(`\;`)
		case ',':
			b.WriteString(`\,`)
		case '\n':
			b.WriteString(`\n`)
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}

// ICSFold folds a content line so no line is longer than 75 octets, never splitting a UTF-8 sequence. The
// continuation lines start with one space, which counts toward their 75 octets.
func ICSFold(line string) string {
	if len(line) <= icsLineMax {
		return line
	}
	var b strings.Builder
	limit := icsLineMax
	used := 0
	for _, r := range line {
		size := utf8.RuneLen(r)
		if used+size > limit {
			b.WriteString("\r\n ")
			limit = icsLineMax - 1
			used = 0
		}
		b.WriteRune(r)
		used += size
	}
	return b.String()
}

// ICSWriter builds a VCALENDAR with CRLF line ends.
type ICSWriter struct {
	b strings.Builder
}

func (w *ICSWriter) line(format string, args ...any) {
	w.b.WriteString(ICSFold(fmt.Sprintf(format, args...)))
	w.b.WriteString("\r\n")
}

// RenderICS renders the events of one viewer as an RFC 5545 calendar. Times are UTC. A recurring event whose
// zone is UTC is written as one VEVENT with RRULE and EXDATE plus one VEVENT per overridden occurrence
// (RECURRENCE-ID). A recurring event in another zone is written as its expanded occurrences inside
// [from, to): an RRULE in UTC would drift by an hour across daylight saving changes and could land on another
// weekday. DTSTAMP is the last update, so the output is stable between requests.
func RenderICS(calendarName string, events []Event, from, to time.Time) string {
	w := &ICSWriter{}
	w.line("BEGIN:VCALENDAR")
	w.line("VERSION:2.0")
	w.line("PRODID:-//CoreLog//Calendario//ES")
	w.line("CALSCALE:GREGORIAN")
	w.line("METHOD:PUBLISH")
	w.line("X-WR-CALNAME:%s", ICSEscape(calendarName))
	sorted := append([]Event(nil), events...)
	sort.SliceStable(sorted, func(i, j int) bool {
		if !sorted[i].StartsAt.Equal(sorted[j].StartsAt) {
			return sorted[i].StartsAt.Before(sorted[j].StartsAt)
		}
		return sorted[i].ID.String() < sorted[j].ID.String()
	})
	for i := range sorted {
		e := &sorted[i]
		switch {
		case e.Recurring() && e.location() == time.UTC:
			w.recurringMaster(e)
		default:
			occ, _ := e.Series().Expand(from, to)
			for _, o := range occ {
				uid := e.ID.String() + "@corelog"
				if e.Recurring() {
					uid = fmt.Sprintf("%s-%d@corelog", e.ID, o.OriginalStart.Unix())
				}
				w.vevent(e, uid, o.Start, o.End, o.Override, nil)
			}
		}
	}
	w.line("END:VCALENDAR")
	return w.b.String()
}

func (w *ICSWriter) recurringMaster(e *Event) {
	uid := e.ID.String() + "@corelog"
	rule, _ := ParseRule(*e.RecurrenceRule)
	text := rule.String()
	if e.AllDay && rule.Until != nil {
		text = strings.Replace(text, "UNTIL="+rule.Until.UTC().Format(icsDateTime), "UNTIL="+rule.Until.UTC().Format(icsDate), 1)
	}
	var excluded []time.Time
	var overrides []Exception
	for _, x := range e.Exceptions {
		excluded = append(excluded, x.OriginalStart)
		if !x.Cancelled {
			overrides = append(overrides, x)
		}
	}
	sort.Slice(excluded, func(i, j int) bool { return excluded[i].Before(excluded[j]) })
	extra := []string{"RRULE:" + text}
	for _, t := range excluded {
		if e.AllDay {
			extra = append(extra, "EXDATE;VALUE=DATE:"+t.Format(icsDate))
		} else {
			extra = append(extra, "EXDATE:"+t.Format(icsDateTime))
		}
	}
	w.vevent(e, uid, e.StartsAt, e.EndsAt, nil, extra)
	sort.Slice(overrides, func(i, j int) bool { return overrides[i].OriginalStart.Before(overrides[j].OriginalStart) })
	dur := e.EndsAt.Sub(e.StartsAt)
	for _, x := range overrides {
		start, end := applyTimes(x.Override, x.OriginalStart, x.OriginalStart.Add(dur))
		recurrenceID := "RECURRENCE-ID:" + x.OriginalStart.Format(icsDateTime)
		if e.AllDay {
			recurrenceID = "RECURRENCE-ID;VALUE=DATE:" + x.OriginalStart.Format(icsDate)
		}
		w.vevent(e, uid, start, end, x.Override, []string{recurrenceID})
	}
}

func (w *ICSWriter) vevent(e *Event, uid string, start, end time.Time, o *Override, extra []string) {
	title, description, location := e.Title, e.Description, e.Location
	if o != nil {
		if o.Title != nil {
			title = *o.Title
		}
		if o.Description != nil {
			description = *o.Description
		}
		if o.Location != nil {
			location = *o.Location
		}
	}
	w.line("BEGIN:VEVENT")
	w.line("UID:%s", uid)
	w.line("DTSTAMP:%s", e.UpdatedAt.UTC().Format(icsDateTime))
	if e.AllDay {
		w.line("DTSTART;VALUE=DATE:%s", start.UTC().Format(icsDate))
		w.line("DTEND;VALUE=DATE:%s", end.UTC().Format(icsDate))
	} else {
		w.line("DTSTART:%s", start.UTC().Format(icsDateTime))
		w.line("DTEND:%s", end.UTC().Format(icsDateTime))
	}
	for _, l := range extra {
		w.line("%s", l)
	}
	w.line("SUMMARY:%s", ICSEscape(title))
	if description != "" {
		w.line("DESCRIPTION:%s", ICSEscape(description))
	}
	if location != "" {
		w.line("LOCATION:%s", ICSEscape(location))
	}
	w.line("END:VEVENT")
}
