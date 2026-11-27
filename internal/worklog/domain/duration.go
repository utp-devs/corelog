// Package domain holds the time tracking rules: duration text, worklogs, remaining estimate
// adjustments and the timesheet and summary aggregates.
package domain

import (
	"math"
	"strings"

	"github.com/jhonsferg/corelog/internal/platform/apperror"
)

// Duration limits in seconds.
const (
	// MaxWorklogSeconds is the longest single worklog (three working days of 24 hours).
	MaxWorklogSeconds int64 = 3 * 86400
	// MaxEstimateSeconds caps original and remaining estimates (ten years of 24 hours).
	MaxEstimateSeconds int64 = 10 * 365 * 86400
	// MaxDescriptionRunes is the length limit of a worklog description.
	MaxDescriptionRunes = 5000
	// maxDigits bounds the number of digits of one term so the arithmetic cannot overflow.
	maxDigits = 9
)

// Stable error codes of the duration and worklog rules.
const (
	CodeDurationInvalid    = "worklog.duration_invalid"
	CodeDurationRange      = "worklog.duration_out_of_range"
	CodeStartedInFuture    = "worklog.started_in_future"
	CodeDescriptionTooLong = "worklog.description_too_long"
)

// Errors returned by the duration and worklog rules. Messages are shown to people, so they are Spanish.
var (
	ErrDurationInvalid    = apperror.WithCode(apperror.KindValidation, CodeDurationInvalid, "La duracion no es valida. Usa unidades w, d, h y m, por ejemplo 1d 2h 30m")
	ErrDurationRange      = apperror.WithCode(apperror.KindValidation, CodeDurationRange, "La duracion esta fuera del rango permitido")
	ErrStartedInFuture    = apperror.WithCode(apperror.KindValidation, CodeStartedInFuture, "La fecha de inicio no puede estar mas de un dia en el futuro")
	ErrDescriptionTooLong = apperror.WithCode(apperror.KindValidation, CodeDescriptionTooLong, "La descripcion no puede superar 5000 caracteres")
)

// Config converts working time into seconds. A working day is HoursPerDay hours and a working week is
// DaysPerWeek days. Organizations provide the values through their settings.
type Config struct {
	HoursPerDay float64
	DaysPerWeek float64
}

// DefaultConfig is used when the organization has no valid values: 8 hours a day and 5 days a week.
func DefaultConfig() Config {
	return Config{HoursPerDay: 8, DaysPerWeek: 5}
}

// Valid reports whether both factors are finite and positive.
func (c Config) Valid() bool {
	return c.HoursPerDay > 0 && c.DaysPerWeek > 0 &&
		!math.IsInf(c.HoursPerDay, 0) && !math.IsInf(c.DaysPerWeek, 0) &&
		!math.IsNaN(c.HoursPerDay) && !math.IsNaN(c.DaysPerWeek)
}

// unitSeconds returns the length of the units in seconds. A day is rounded to whole seconds once so
// that parsing and formatting agree.
func (c Config) unitSeconds() (week, day, hour, minute int64) {
	day = int64(math.Round(c.HoursPerDay * 3600))
	week = int64(math.Round(c.DaysPerWeek * float64(day)))
	return week, day, 3600, 60
}

// ParseDuration reads a worklog duration such as "1w 2d 3h 30m" and returns seconds. The result is
// always positive and at most MaxWorklogSeconds.
func ParseDuration(text string, cfg Config) (int64, error) {
	seconds, err := parseTerms(text, cfg)
	if err != nil {
		return 0, err
	}
	if seconds <= 0 || seconds > MaxWorklogSeconds {
		return 0, ErrDurationRange
	}
	return seconds, nil
}

// ParseEstimate reads an estimate. It is like ParseDuration but zero is allowed and the limit is
// MaxEstimateSeconds.
func ParseEstimate(text string, cfg Config) (int64, error) {
	seconds, err := parseTerms(text, cfg)
	if err != nil {
		return 0, err
	}
	if seconds > MaxEstimateSeconds {
		return 0, ErrDurationRange
	}
	return seconds, nil
}

// parseTerms reads the "<number><unit>" terms. Units are w, d, h and m in any case; the spaces between
// terms are optional, any order is accepted, and a unit may appear once only. Bare numbers, unknown
// units, signs, decimals and empty text are invalid.
func parseTerms(text string, cfg Config) (int64, error) {
	if !cfg.Valid() {
		cfg = DefaultConfig()
	}
	week, day, hour, minute := cfg.unitSeconds()
	units := map[byte]int64{'w': week, 'd': day, 'h': hour, 'm': minute}

	s := strings.ToLower(strings.TrimSpace(text))
	if s == "" {
		return 0, ErrDurationInvalid
	}
	var total int64
	seen := map[byte]bool{}
	i := 0
	for i < len(s) {
		if s[i] == ' ' || s[i] == '\t' {
			i++
			continue
		}
		start := i
		for i < len(s) && s[i] >= '0' && s[i] <= '9' {
			i++
		}
		digits := s[start:i]
		if digits == "" || len(digits) > maxDigits || i >= len(s) {
			return 0, ErrDurationInvalid
		}
		unit := s[i]
		size, ok := units[unit]
		if !ok || seen[unit] {
			return 0, ErrDurationInvalid
		}
		seen[unit] = true
		i++
		var count int64
		for _, d := range digits {
			count = count*10 + int64(d-'0')
		}
		total += count * size
		if total > MaxEstimateSeconds {
			return 0, ErrDurationRange
		}
	}
	return total, nil
}

// FormatDuration renders seconds as "1w 2d 3h 30m" using the same units as ParseDuration. Sub-minute
// remainders are dropped and zero is "0m".
func FormatDuration(seconds int64, cfg Config) string {
	if !cfg.Valid() {
		cfg = DefaultConfig()
	}
	if seconds <= 0 {
		return "0m"
	}
	week, day, hour, minute := cfg.unitSeconds()
	parts := []string{}
	for _, u := range []struct {
		size  int64
		label string
	}{{week, "w"}, {day, "d"}, {hour, "h"}, {minute, "m"}} {
		if u.size <= 0 {
			continue
		}
		n := seconds / u.size
		if n > 0 {
			parts = append(parts, formatInt(n)+u.label)
			seconds -= n * u.size
		}
	}
	if len(parts) == 0 {
		return "0m"
	}
	return strings.Join(parts, " ")
}

func formatInt(n int64) string {
	if n == 0 {
		return "0"
	}
	var buf [20]byte
	pos := len(buf)
	for n > 0 {
		pos--
		buf[pos] = byte('0' + n%10)
		n /= 10
	}
	return string(buf[pos:])
}
