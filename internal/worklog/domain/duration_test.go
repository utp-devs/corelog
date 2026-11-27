package domain

import (
	"testing"

	"github.com/jhonsferg/corelog/internal/platform/apperror"
)

var defaultCfg = Config{HoursPerDay: 8, DaysPerWeek: 5}

func TestParseDurationTable(t *testing.T) {
	cases := []struct {
		name string
		in   string
		cfg  Config
		want int64
		code string
	}{
		{"single hour", "3h", defaultCfg, 3 * 3600, ""},
		{"minutes only", "30m", defaultCfg, 1800, ""},
		{"day uses hours per day", "2d", defaultCfg, 2 * 8 * 3600, ""},
		{"week uses days per week", "1w", defaultCfg, 5 * 8 * 3600, ""},
		{"full mix", "1w 2d 3h 30m", defaultCfg, 5*8*3600 + 2*8*3600 + 3*3600 + 1800, ""},
		{"no spaces", "1h30m", defaultCfg, 5400, ""},
		{"extra spaces and tabs", "  1h \t 30m  ", defaultCfg, 5400, ""},
		{"uppercase units", "2H 15M", defaultCfg, 2*3600 + 900, ""},
		{"any order", "30m 1h", defaultCfg, 5400, ""},
		{"custom day length", "1d", Config{HoursPerDay: 7.5, DaysPerWeek: 4}, 27000, ""},
		{"custom week length", "1w", Config{HoursPerDay: 6, DaysPerWeek: 3}, 18 * 3600, ""},
		{"fractional day rounds to seconds", "1d", Config{HoursPerDay: 7.25, DaysPerWeek: 5}, 26100, ""},
		{"invalid config falls back to default", "1d", Config{}, 8 * 3600, ""},
		{"zero is rejected for worklogs", "0m", defaultCfg, 0, CodeDurationRange},
		{"empty", "", defaultCfg, 0, CodeDurationInvalid},
		{"spaces only", "   ", defaultCfg, 0, CodeDurationInvalid},
		{"bare number has no unit", "30", defaultCfg, 0, CodeDurationInvalid},
		{"unknown unit", "3x", defaultCfg, 0, CodeDurationInvalid},
		{"seconds unit is not supported", "30s", defaultCfg, 0, CodeDurationInvalid},
		{"unit without number", "h", defaultCfg, 0, CodeDurationInvalid},
		{"unit at the end is missing", "1h 2", defaultCfg, 0, CodeDurationInvalid},
		{"duplicate unit", "1h 2h", defaultCfg, 0, CodeDurationInvalid},
		{"negative sign", "-1h", defaultCfg, 0, CodeDurationInvalid},
		{"decimal", "1.5h", defaultCfg, 0, CodeDurationInvalid},
		{"letters before digits", "h1", defaultCfg, 0, CodeDurationInvalid},
		{"exceeds worklog limit", "10d", defaultCfg, 0, CodeDurationRange},
		{"ten digit overflow guard", "9999999999h", defaultCfg, 0, CodeDurationInvalid},
		{"huge sum overflows the estimate limit", "999999999w 999999999w", defaultCfg, 0, CodeDurationRange},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ParseDuration(tc.in, tc.cfg)
			if tc.code != "" {
				if err == nil || apperror.CodeOf(err) != tc.code {
					t.Fatalf("ParseDuration(%q) error = %v, want code %s", tc.in, err, tc.code)
				}
				return
			}
			if err != nil {
				t.Fatalf("ParseDuration(%q) unexpected error: %v", tc.in, err)
			}
			if got != tc.want {
				t.Fatalf("ParseDuration(%q) = %d, want %d", tc.in, got, tc.want)
			}
		})
	}
}

func TestParseEstimateAllowsZero(t *testing.T) {
	got, err := ParseEstimate("0m", defaultCfg)
	if err != nil || got != 0 {
		t.Fatalf("ParseEstimate(0m) = %d, %v", got, err)
	}
	if _, err := ParseEstimate("10y", defaultCfg); apperror.CodeOf(err) != CodeDurationInvalid {
		t.Fatalf("ParseEstimate(10y) code = %v", err)
	}
	if _, err := ParseEstimate("9999w", defaultCfg); apperror.CodeOf(err) != CodeDurationRange {
		t.Fatalf("ParseEstimate(9999w) code = %v", err)
	}
}

func TestFormatDurationTable(t *testing.T) {
	cases := []struct {
		name    string
		seconds int64
		cfg     Config
		want    string
	}{
		{"zero", 0, defaultCfg, "0m"},
		{"negative", -5, defaultCfg, "0m"},
		{"sub minute", 45, defaultCfg, "0m"},
		{"minutes", 1800, defaultCfg, "30m"},
		{"hours and minutes", 5400, defaultCfg, "1h 30m"},
		{"one day is eight hours", 8 * 3600, defaultCfg, "1d"},
		{"one week is five days", 5 * 8 * 3600, defaultCfg, "1w"},
		{"full mix", 5*8*3600 + 2*8*3600 + 3*3600 + 1800, defaultCfg, "1w 2d 3h 30m"},
		{"custom day", 27000, Config{HoursPerDay: 7.5, DaysPerWeek: 4}, "1d"},
		{"invalid config falls back", 8 * 3600, Config{}, "1d"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := FormatDuration(tc.seconds, tc.cfg); got != tc.want {
				t.Fatalf("FormatDuration(%d) = %q, want %q", tc.seconds, got, tc.want)
			}
		})
	}
}

// Round trip: every formatted value parses back to the same seconds (whole minutes only).
func TestFormatParseRoundTrip(t *testing.T) {
	for seconds := int64(60); seconds <= MaxWorklogSeconds; seconds += 60 * 37 {
		text := FormatDuration(seconds, defaultCfg)
		got, err := ParseDuration(text, defaultCfg)
		if err != nil || got != seconds {
			t.Fatalf("round trip %d -> %q -> %d (%v)", seconds, text, got, err)
		}
	}
}

func FuzzParseDuration(f *testing.F) {
	for _, seed := range []string{"1w 2d 3h 30m", "1h", "", "9999999999999999999m", "1x", "h", "0m"} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, in string) {
		got, err := ParseDuration(in, defaultCfg)
		if err == nil && (got <= 0 || got > MaxWorklogSeconds) {
			t.Fatalf("accepted %q with out of range value %d", in, got)
		}
	})
}
