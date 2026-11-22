package domain

import (
	"errors"
	"testing"
	"time"
)

func TestValidateRangeBounds(t *testing.T) {
	from := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	if err := ValidateRange(from, from); err != nil {
		t.Fatalf("one day range rejected: %v", err)
	}
	if err := ValidateRange(from, from.AddDate(0, 0, MaxRangeDays)); err != nil {
		t.Fatalf("range of exactly the maximum rejected: %v", err)
	}
	if err := ValidateRange(from, from.AddDate(0, 0, MaxRangeDays+1)); !errors.Is(err, ErrInvalidRange) {
		t.Fatalf("range over the maximum: got %v", err)
	}
	if err := ValidateRange(from.AddDate(0, 0, 1), from); !errors.Is(err, ErrInvalidRange) {
		t.Fatalf("reversed range: got %v", err)
	}
}
