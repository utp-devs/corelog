package jobs

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	// Embedded zone database so named time zones resolve in minimal container images.
	_ "time/tzdata"
)

// Schedule says when a job runs next.
type Schedule interface {
	// Next returns the first run strictly after the given instant.
	Next(after time.Time) time.Time
	// String returns the canonical text of the schedule (it parses back to the same schedule).
	String() string
}

// ErrInvalidSchedule is wrapped by every ParseSchedule failure.
var ErrInvalidSchedule = errors.New("jobs: invalid schedule")

func invalid(format string, args ...any) error {
	return fmt.Errorf("%w: %s", ErrInvalidSchedule, fmt.Sprintf(format, args...))
}

// ParseSchedule parses "every 10s", "every 1h", "daily at 03:30 [time zone]" and "cron m h dom mon dow [time zone]".
// Cron fields accept *, lists (1,2), ranges (1-5) and steps (*/10, 0-30/5); day of week is 0-6 (7 also means Sunday).
func ParseSchedule(spec string) (Schedule, error) {
	fields := strings.Fields(strings.TrimSpace(spec))
	if len(fields) == 0 {
		return nil, invalid("empty schedule")
	}
	switch strings.ToLower(fields[0]) {
	case "every":
		if len(fields) != 2 {
			return nil, invalid("use \"every <duration>\"")
		}
		d, err := time.ParseDuration(fields[1])
		if err != nil || d < time.Second {
			return nil, invalid("interval must be a duration of at least 1s")
		}
		return Interval(d), nil
	case "daily":
		if len(fields) < 3 || len(fields) > 4 || strings.ToLower(fields[1]) != "at" {
			return nil, invalid("use \"daily at HH:MM [time zone]\"")
		}
		hour, minute, err := parseClock(fields[2])
		if err != nil {
			return nil, err
		}
		loc, err := locationOf(fields[3:])
		if err != nil {
			return nil, err
		}
		return dailySchedule{hour: hour, minute: minute, loc: loc}, nil
	case "cron":
		if len(fields) != 6 && len(fields) != 7 {
			return nil, invalid("use \"cron m h dom mon dow [time zone]\"")
		}
		loc, err := locationOf(fields[6:])
		if err != nil {
			return nil, err
		}
		return parseCron(fields[1:6], loc)
	default:
		return nil, invalid("unknown schedule kind %q", fields[0])
	}
}

func locationOf(rest []string) (*time.Location, error) {
	if len(rest) == 0 {
		return time.UTC, nil
	}
	loc, err := time.LoadLocation(rest[0])
	if err != nil {
		return nil, invalid("unknown time zone %q", rest[0])
	}
	return loc, nil
}

func parseClock(text string) (int, int, error) {
	hh, mm, ok := strings.Cut(text, ":")
	hour, errH := strconv.Atoi(hh)
	minute, errM := strconv.Atoi(mm)
	if !ok || errH != nil || errM != nil || hour < 0 || hour > 23 || minute < 0 || minute > 59 {
		return 0, 0, invalid("time of day must be HH:MM, got %q", text)
	}
	return hour, minute, nil
}

// Interval runs every d.
type Interval time.Duration

// Next implements Schedule.
func (i Interval) Next(after time.Time) time.Time { return after.Add(time.Duration(i)) }

// String implements Schedule, shortening 1h0m0s to 1h and 5m0s to 5m.
func (i Interval) String() string {
	text := time.Duration(i).String()
	if strings.HasSuffix(text, "m0s") {
		text = strings.TrimSuffix(text, "0s")
	}
	if strings.HasSuffix(text, "h0m") {
		text = strings.TrimSuffix(text, "0m")
	}
	return "every " + text
}

type dailySchedule struct {
	hour, minute int
	loc          *time.Location
}

func (d dailySchedule) Next(after time.Time) time.Time {
	local := after.In(d.loc)
	candidate := time.Date(local.Year(), local.Month(), local.Day(), d.hour, d.minute, 0, 0, d.loc)
	if !candidate.After(after) {
		candidate = time.Date(local.Year(), local.Month(), local.Day()+1, d.hour, d.minute, 0, 0, d.loc)
	}
	return candidate.UTC()
}

func (d dailySchedule) String() string {
	return fmt.Sprintf("daily at %02d:%02d %s", d.hour, d.minute, d.loc.String())
}

type cronSchedule struct {
	minute, hour, dom, month, dow []bool
	domAny, dowAny                bool
	loc                           *time.Location
	text                          string
}

func parseCron(fields []string, loc *time.Location) (Schedule, error) {
	bounds := [5][2]int{{0, 59}, {0, 23}, {1, 31}, {1, 12}, {0, 7}}
	sets := make([][]bool, 5)
	for i, field := range fields {
		set, err := parseCronField(field, bounds[i][0], bounds[i][1])
		if err != nil {
			return nil, err
		}
		sets[i] = set
	}
	sets[4][0] = sets[4][0] || sets[4][7]
	return cronSchedule{
		minute: sets[0], hour: sets[1], dom: sets[2], month: sets[3], dow: sets[4][:7],
		domAny: fields[2] == "*", dowAny: fields[4] == "*", loc: loc,
		text: "cron " + strings.Join(fields, " ") + " " + loc.String(),
	}, nil
}

func parseCronField(field string, lo, hi int) ([]bool, error) {
	set := make([]bool, hi+1)
	for _, part := range strings.Split(field, ",") {
		rangePart, stepText, hasStep := strings.Cut(part, "/")
		step := 1
		if hasStep {
			n, err := strconv.Atoi(stepText)
			if err != nil || n < 1 {
				return nil, invalid("bad step in %q", field)
			}
			step = n
		}
		from, to := lo, hi
		if rangePart != "*" {
			a, b, isRange := strings.Cut(rangePart, "-")
			var err error
			if from, err = strconv.Atoi(a); err != nil {
				return nil, invalid("bad value in %q", field)
			}
			to = from
			if isRange {
				if to, err = strconv.Atoi(b); err != nil {
					return nil, invalid("bad range in %q", field)
				}
			} else if hasStep {
				to = hi
			}
		}
		if from < lo || to > hi || from > to {
			return nil, invalid("value out of range in %q", field)
		}
		for v := from; v <= to; v += step {
			set[v] = true
		}
	}
	return set, nil
}

func (c cronSchedule) String() string { return c.text }

func (c cronSchedule) dayMatches(t time.Time) bool {
	domOK, dowOK := c.dom[t.Day()], c.dow[int(t.Weekday())]
	switch {
	case c.domAny && c.dowAny:
		return true
	case c.domAny:
		return dowOK
	case c.dowAny:
		return domOK
	default:
		return domOK || dowOK
	}
}

func (c cronSchedule) Next(after time.Time) time.Time {
	t := after.In(c.loc).Truncate(time.Minute).Add(time.Minute)
	for i := 0; i < 200000; i++ {
		switch {
		case !c.month[int(t.Month())]:
			t = time.Date(t.Year(), t.Month()+1, 1, 0, 0, 0, 0, c.loc)
		case !c.dayMatches(t):
			t = time.Date(t.Year(), t.Month(), t.Day()+1, 0, 0, 0, 0, c.loc)
		case !c.hour[t.Hour()]:
			t = time.Date(t.Year(), t.Month(), t.Day(), t.Hour()+1, 0, 0, 0, c.loc)
		case !c.minute[t.Minute()]:
			t = t.Add(time.Minute)
		default:
			return t.UTC()
		}
	}
	// Unreachable for valid schedules (for example 31 February): never run.
	return after.AddDate(100, 0, 0)
}
