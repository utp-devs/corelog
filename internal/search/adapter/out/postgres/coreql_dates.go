package postgres

import (
	"strings"
	"time"

	"github.com/jhonsferg/corelog/internal/search/domain"
	"github.com/jhonsferg/corelog/internal/search/domain/coreql"
)

// tsMode says how a date column is stored and therefore how values are bound.
type tsMode int

const (
	mTimestamptz tsMode = iota // timestamptz column
	mDate                      // DATE column
	mTextDate                  // text column holding YYYY-MM-DD
	mTextTS                    // text column holding an RFC 3339 UTC instant
)

const (
	layoutDate     = "2006-01-02"
	layoutDateTime = "2006-01-02 15:04"
)

// shift applies a relative amount (m, h, d, w, M, y) to t. Calendar units use the viewer's calendar.
func shift(t time.Time, amount int, unit string) time.Time {
	switch unit {
	case "m":
		return t.Add(time.Duration(amount) * time.Minute)
	case "h":
		return t.Add(time.Duration(amount) * time.Hour)
	case "d":
		return t.AddDate(0, 0, amount)
	case "w":
		return t.AddDate(0, 0, 7*amount)
	case "M":
		return t.AddDate(0, amount, 0)
	case "y":
		return t.AddDate(amount, 0, 0)
	}
	return t
}

func startOfDay(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location())
}

// startOfWeek is the Monday of the week of t.
func startOfWeek(t time.Time) time.Time {
	d := startOfDay(t)
	return d.AddDate(0, 0, -((int(d.Weekday()) + 6) % 7))
}

func startOfMonth(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, t.Location())
}

// instantOf resolves a date value to an instant. dateOnly is true for a YYYY-MM-DD literal, which stands for
// the whole day.
func (b *builder) instantOf(v coreql.Value) (t time.Time, dateOnly bool, ok bool) {
	switch x := v.(type) {
	case *coreql.RelativeDate:
		return shift(b.now, x.Amount, x.Unit), false, true
	case *coreql.Func:
		return b.dateFunc(x)
	case *coreql.String, *coreql.Word:
		text, _ := textOf(v)
		if d, err := time.ParseInLocation(layoutDateTime, text, b.loc); err == nil {
			return d, false, true
		}
		if d, err := time.ParseInLocation(layoutDate, text, b.loc); err == nil {
			return d, true, true
		}
	}
	return time.Time{}, false, false
}

func (b *builder) dateFunc(f *coreql.Func) (time.Time, bool, bool) {
	base := b.now
	if len(f.Args) == 1 {
		rel, ok := f.Args[0].(*coreql.RelativeDate)
		if !ok {
			return time.Time{}, false, false
		}
		base = shift(base, rel.Amount, rel.Unit)
	}
	end := func(next time.Time) time.Time { return next.Add(-time.Microsecond) }
	switch strings.ToLower(f.Name) {
	case "now":
		return b.now, false, true
	case "startofday":
		return startOfDay(base), false, true
	case "endofday":
		return end(startOfDay(base).AddDate(0, 0, 1)), false, true
	case "startofweek":
		return startOfWeek(base), false, true
	case "endofweek":
		return end(startOfWeek(base).AddDate(0, 0, 7)), false, true
	case "startofmonth":
		return startOfMonth(base), false, true
	case "endofmonth":
		return end(startOfMonth(base).AddDate(0, 1, 0)), false, true
	}
	return time.Time{}, false, false
}

// timestampCompare compares a date column with a date value. A YYYY-MM-DD literal on a timestamp column
// stands for the whole day in the viewer's time zone.
func (b *builder) timestampCompare(col string, mode tsMode, op coreql.Op, v coreql.Value, f resolved) string {
	t, dateOnly, ok := b.instantOf(v)
	if !ok {
		b.fail(v.Location(), domain.CodeInvalidQuery, "se esperaba una fecha YYYY-MM-DD, una fecha y hora o una fecha relativa en el campo %s", f.name)
		return "FALSE"
	}
	if mode == mDate || mode == mTextDate {
		d := b.bindText(t.In(b.loc).Format(layoutDate))
		if mode == mDate {
			d += "::date"
		}
		return col + " " + sqlCompareOp(op) + " " + d
	}
	bindInstant := func(x time.Time) string {
		if mode == mTextTS {
			return b.bindText(x.UTC().Format(time.RFC3339))
		}
		return b.bind(x.UTC(), "timestamptz")
	}
	if !dateOnly {
		return col + " " + sqlCompareOp(op) + " " + bindInstant(t)
	}
	start := startOfDay(t)
	next := start.AddDate(0, 0, 1)
	switch op {
	case coreql.OpEq:
		return "(" + col + " >= " + bindInstant(start) + " AND " + col + " < " + bindInstant(next) + ")"
	case coreql.OpGt:
		return col + " >= " + bindInstant(next)
	case coreql.OpGe:
		return col + " >= " + bindInstant(start)
	case coreql.OpLt:
		return col + " < " + bindInstant(start)
	case coreql.OpLe:
		return col + " < " + bindInstant(next)
	}
	return "FALSE"
}
