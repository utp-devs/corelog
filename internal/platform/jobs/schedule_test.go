package jobs

import (
	"errors"
	"testing"
	"time"
)

func mustParse(t *testing.T, spec string) Schedule {
	t.Helper()
	s, err := ParseSchedule(spec)
	if err != nil {
		t.Fatalf("ParseSchedule(%q): %v", spec, err)
	}
	return s
}

func at(value string) time.Time {
	parsed, err := time.Parse(time.RFC3339, value)
	if err != nil {
		panic(err)
	}
	return parsed
}

func TestParseScheduleTable(t *testing.T) {
	cases := []struct {
		spec  string
		after string
		next  string
		text  string
	}{
		{"every 10s", "2026-11-27T10:00:00Z", "2026-11-27T10:00:10Z", "every 10s"},
		{"every 1h", "2026-11-27T10:00:00Z", "2026-11-27T11:00:00Z", "every 1h"},
		{"EVERY 5m", "2026-11-27T10:00:00Z", "2026-11-27T10:05:00Z", "every 5m"},
		{"every 90m", "2026-11-27T10:00:00Z", "2026-11-27T11:30:00Z", "every 1h30m"},
		{"daily at 03:00", "2026-11-27T02:59:00Z", "2026-11-27T03:00:00Z", "daily at 03:00 UTC"},
		{"daily at 03:00", "2026-11-27T03:00:00Z", "2026-11-28T03:00:00Z", "daily at 03:00 UTC"},
		{"daily at 08:30 America/Bogota", "2026-11-27T10:00:00Z", "2026-11-27T13:30:00Z", "daily at 08:30 America/Bogota"},
		{"daily at 08:30 America/Bogota", "2026-11-27T14:00:00Z", "2026-11-28T13:30:00Z", "daily at 08:30 America/Bogota"},
		{"cron */15 * * * *", "2026-11-27T10:07:30Z", "2026-11-27T10:15:00Z", "cron */15 * * * * UTC"},
		{"cron 0 4 * * *", "2026-11-27T10:00:00Z", "2026-11-28T04:00:00Z", "cron 0 4 * * * UTC"},
		{"cron 30 9 * * 1-5", "2026-11-27T10:00:00Z", "2026-11-30T09:30:00Z", "cron 30 9 * * 1-5 UTC"}, // Friday -> Monday
		{"cron 0 0 1 * *", "2026-11-27T10:00:00Z", "2026-12-01T00:00:00Z", "cron 0 0 1 * * UTC"},
		{"cron 0 12 * * 0,6", "2026-11-27T10:00:00Z", "2026-11-28T12:00:00Z", "cron 0 12 * * 0,6 UTC"},
		{"cron 0 12 * * 7", "2026-11-27T10:00:00Z", "2026-11-29T12:00:00Z", "cron 0 12 * * 7 UTC"},
		{"cron 5-10/5 1,2 * * *", "2026-11-27T01:06:00Z", "2026-11-27T01:10:00Z", "cron 5-10/5 1,2 * * * UTC"},
		{"cron 0 0 29 2 *", "2026-11-27T10:00:00Z", "2028-02-29T00:00:00Z", "cron 0 0 29 2 * UTC"},
		{"cron 0 9 * * * Europe/Madrid", "2026-11-27T10:00:00Z", "2026-11-28T08:00:00Z", "cron 0 9 * * * Europe/Madrid"},
	}
	for _, c := range cases {
		t.Run(c.spec+"@"+c.after, func(t *testing.T) {
			s := mustParse(t, c.spec)
			if got := s.Next(at(c.after)); !got.Equal(at(c.next)) {
				t.Fatalf("Next = %v, want %v", got, c.next)
			}
			if s.String() != c.text {
				t.Fatalf("String = %q, want %q", s.String(), c.text)
			}
			again := mustParse(t, s.String())
			if !again.Next(at(c.after)).Equal(s.Next(at(c.after))) {
				t.Fatalf("String() %q does not parse back to the same schedule", s.String())
			}
		})
	}
}

func TestParseScheduleRejectsBadInput(t *testing.T) {
	for _, spec := range []string{
		"", "   ", "hourly", "every", "every soon", "every 500ms", "every 10s extra", "every -1h",
		"daily", "daily at", "daily at 25:00", "daily at 10:60", "daily at 7", "daily at 03:00 Mars/Base", "daily on 03:00",
		"cron", "cron * * * *", "cron * * * * * UTC extra", "cron 60 * * * *", "cron * 24 * * *", "cron * * 0 * *", "cron * * * 13 *",
		"cron * * * * 8", "cron */0 * * * *", "cron 5-1 * * * *", "cron a * * * *", "cron 1-x * * * *", "cron * * * * * Mars/Base",
	} {
		_, err := ParseSchedule(spec)
		if err == nil || !errors.Is(err, ErrInvalidSchedule) {
			t.Errorf("ParseSchedule(%q) = %v, want ErrInvalidSchedule", spec, err)
		}
	}
}

func TestImpossibleCronNeverRuns(t *testing.T) {
	s := mustParse(t, "cron 0 0 31 2 *")
	if next := s.Next(at("2026-11-27T10:00:00Z")); next.Before(at("2100-01-01T00:00:00Z")) {
		t.Fatalf("31 February ran at %v", next)
	}
}

func TestDailyScheduleFollowsDaylightSaving(t *testing.T) {
	s := mustParse(t, "daily at 09:00 Europe/Madrid")
	winter := s.Next(at("2026-01-10T00:00:00Z"))
	summer := s.Next(at("2026-07-10T00:00:00Z"))
	if winter.Hour() != 8 || summer.Hour() != 7 {
		t.Fatalf("winter %v summer %v: the local time must stay 09:00", winter, summer)
	}
}

func FuzzParseSchedule(f *testing.F) {
	for _, seed := range []string{"every 10s", "daily at 03:00 UTC", "cron */5 * * * *", "cron 1-3/2 0 1,15 * 1-5", "", "every"} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, spec string) {
		s, err := ParseSchedule(spec)
		if err != nil {
			return
		}
		now := at("2026-11-27T10:00:00Z")
		next := s.Next(now)
		if !next.After(now) {
			t.Fatalf("%q: Next %v is not after %v", spec, next, now)
		}
		if _, err := ParseSchedule(s.String()); err != nil {
			t.Fatalf("%q: canonical text %q does not parse: %v", spec, s.String(), err)
		}
	})
}
