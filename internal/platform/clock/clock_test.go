package clock

import (
	"testing"
	"time"
)

func TestRealClockReturnsUTC(t *testing.T) {
	before := time.Now().UTC()
	got := RealClock{}.Now()
	after := time.Now().UTC()
	if got.Location() != time.UTC {
		t.Errorf("location = %v, want UTC", got.Location())
	}
	if got.Before(before.Add(-time.Second)) || got.After(after.Add(time.Second)) {
		t.Errorf("now %v outside [%v, %v]", got, before, after)
	}
}

func TestFixedAndFakeClocks(t *testing.T) {
	base := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)

	tests := []struct {
		name string
		clk  Clock
		want time.Time
	}{
		{"fixed clock returns pinned instant", NewFixed(base), base},
		{"fixed clock value literal", FixedClock{T: base}, base},
		{"fake clock returns its time", &FakeClock{T: base}, base},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.clk.Now(); !got.Equal(tt.want) {
				t.Errorf("Now() = %v, want %v", got, tt.want)
			}
		})
	}

	t.Run("advance moves fake clock forward", func(t *testing.T) {
		fc := &FakeClock{T: base}
		fc.Advance(90 * time.Minute)
		if want := base.Add(90 * time.Minute); !fc.Now().Equal(want) {
			t.Errorf("Now() = %v, want %v", fc.Now(), want)
		}
		fc.Advance(-time.Hour)
		if want := base.Add(30 * time.Minute); !fc.Now().Equal(want) {
			t.Errorf("after negative advance Now() = %v, want %v", fc.Now(), want)
		}
	})
}
