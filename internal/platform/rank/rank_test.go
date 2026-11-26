package rank

import (
	"errors"
	"math/rand"
	"testing"
)

// assertBetween checks the strict ordering a < r < b, treating empty bounds as open.
func assertBetween(t *testing.T, a, r, b string) {
	t.Helper()
	if err := validate(r); err != nil {
		t.Fatalf("Between(%q, %q) returned invalid rank %q: %v", a, b, r, err)
	}
	if a != "" && !(a < r) {
		t.Fatalf("Between(%q, %q) = %q, want %q < result", a, b, r, a)
	}
	if b != "" && !(r < b) {
		t.Fatalf("Between(%q, %q) = %q, want result < %q", a, b, r, b)
	}
}

// assertStrictlyIncreasing fails if any adjacent pair is not strictly ordered.
func assertStrictlyIncreasing(t *testing.T, list []string) {
	t.Helper()
	for i := 1; i < len(list); i++ {
		if !(list[i-1] < list[i]) {
			t.Fatalf("order broken at %d: %q >= %q", i, list[i-1], list[i])
		}
	}
}

func TestInitial(t *testing.T) {
	if got := Initial(); got != "V" {
		t.Fatalf("Initial() = %q, want %q", got, "V")
	}
	if err := validate(Initial()); err != nil {
		t.Fatalf("Initial() is invalid: %v", err)
	}
}

func TestBetweenTable(t *testing.T) {
	tests := []struct {
		name    string
		a, b    string
		want    string // exact expected result; empty means only ordering is checked
		wantErr error
	}{
		{name: "both empty returns initial", a: "", b: "", want: "V"},
		{name: "start before initial", a: "", b: "V", want: "F"},
		{name: "end after initial", a: "V", b: "", want: "k"},
		{name: "adjacent chars a and b", a: "a", b: "b", want: "aV"},
		{name: "prefix a and a1", a: "a", b: "a1", want: "a0V"},
		{name: "prefix a and aV", a: "a", b: "aV", want: "aF"},
		{name: "start before 1", a: "", b: "1", want: "0V"},
		{name: "start before 01", a: "", b: "01", want: "00V"},
		{name: "a before z-ending run", a: "aZ", b: "b", want: "am"},
		{name: "neighbours V and W", a: "V", b: "W"},
		{name: "neighbours with long prefix", a: "V0V", b: "V1"},
		{name: "wide gap digits", a: "1", b: "z"},
		{name: "after z", a: "z", b: ""},
		{name: "after zV", a: "zV", b: ""},
		{name: "before 0V", a: "", b: "0V"},
		{name: "prefix V and V0V", a: "V", b: "V0V"},
		{name: "equal bounds", a: "aV", b: "aV", wantErr: ErrOrder},
		{name: "reversed bounds", a: "b", b: "a", wantErr: ErrOrder},
		{name: "upper bound with trailing zero", a: "", b: "0", wantErr: ErrInvalid},
		{name: "lower bound with trailing zero", a: "a0", b: "b", wantErr: ErrInvalid},
		{name: "character outside alphabet", a: "a!", b: "b", wantErr: ErrInvalid},
		{name: "space in bound", a: "", b: "a b", wantErr: ErrInvalid},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := Between(tc.a, tc.b)
			if tc.wantErr != nil {
				if !errors.Is(err, tc.wantErr) {
					t.Fatalf("Between(%q, %q) error = %v, want %v", tc.a, tc.b, err, tc.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("Between(%q, %q) unexpected error: %v", tc.a, tc.b, err)
			}
			if tc.want != "" && got != tc.want {
				t.Fatalf("Between(%q, %q) = %q, want %q", tc.a, tc.b, got, tc.want)
			}
			assertBetween(t, tc.a, got, tc.b)
		})
	}
}

func TestNeedsRebalance(t *testing.T) {
	long33 := make([]byte, 33)
	for i := range long33 {
		long33[i] = 'V'
	}
	tests := []struct {
		s    string
		want bool
	}{
		{"", false},
		{"V", false},
		{string(long33[:32]), false},
		{string(long33), true},
	}
	for _, tc := range tests {
		if got := NeedsRebalance(tc.s); got != tc.want {
			t.Errorf("NeedsRebalance(len %d) = %v, want %v", len(tc.s), got, tc.want)
		}
	}
}

func TestRebalanceEdgeCases(t *testing.T) {
	for _, n := range []int{0, -1} {
		if got := Rebalance(n); len(got) != 0 {
			t.Fatalf("Rebalance(%d) = %v, want empty", n, got)
		}
	}
	for _, n := range []int{1, 2, 61, 62, 63} {
		got := Rebalance(n)
		if len(got) != n {
			t.Fatalf("Rebalance(%d) returned %d ranks", n, len(got))
		}
		assertStrictlyIncreasing(t, got)
		for _, r := range got {
			if err := validate(r); err != nil {
				t.Fatalf("Rebalance(%d) produced invalid rank %q: %v", n, r, err)
			}
		}
	}
}

func TestRebalance100(t *testing.T) {
	got := Rebalance(100)
	if len(got) != 100 {
		t.Fatalf("Rebalance(100) returned %d ranks", len(got))
	}
	assertStrictlyIncreasing(t, got)
	for _, r := range got {
		if err := validate(r); err != nil {
			t.Fatalf("Rebalance(100) produced invalid rank %q: %v", r, err)
		}
		if NeedsRebalance(r) {
			t.Fatalf("Rebalance(100) produced over-long rank %q", r)
		}
	}
	first, err := Between("", got[0])
	if err != nil {
		t.Fatalf("Between(start, first) error: %v", err)
	}
	assertBetween(t, "", first, got[0])
	last, err := Between(got[len(got)-1], "")
	if err != nil {
		t.Fatalf("Between(last, end) error: %v", err)
	}
	assertBetween(t, got[len(got)-1], last, "")
	for i := 1; i < len(got); i++ {
		r, err := Between(got[i-1], got[i])
		if err != nil {
			t.Fatalf("Between(%q, %q) error: %v", got[i-1], got[i], err)
		}
		assertBetween(t, got[i-1], r, got[i])
	}
}

func TestRandomInsertions(t *testing.T) {
	rng := rand.New(rand.NewSource(20260614))
	var list []string
	maxLen := 0
	for i := 0; i < 10000; i++ {
		pos := rng.Intn(len(list) + 1)
		lo, hi := "", ""
		if pos > 0 {
			lo = list[pos-1]
		}
		if pos < len(list) {
			hi = list[pos]
		}
		r, err := Between(lo, hi)
		if err != nil {
			t.Fatalf("insertion %d: Between(%q, %q) error: %v", i, lo, hi, err)
		}
		assertBetween(t, lo, r, hi)
		if len(r) > maxLen {
			maxLen = len(r)
		}
		list = append(list, "")
		copy(list[pos+1:], list[pos:])
		list[pos] = r
		assertStrictlyIncreasing(t, list)
	}
	t.Logf("max rank length after 10000 random insertions: %d", maxLen)
}

func TestRepeatedFrontInsertions(t *testing.T) {
	var list []string
	for i := 0; i < 200; i++ {
		hi := ""
		if len(list) > 0 {
			hi = list[0]
		}
		r, err := Between("", hi)
		if err != nil {
			t.Fatalf("front insertion %d: error: %v", i, err)
		}
		assertBetween(t, "", r, hi)
		list = append([]string{r}, list...)
	}
	assertStrictlyIncreasing(t, list)
	t.Logf("max rank length after 200 front insertions: %d", maxLenOf(list))
}

func TestRepeatedEndInsertions(t *testing.T) {
	var list []string
	for i := 0; i < 200; i++ {
		lo := ""
		if len(list) > 0 {
			lo = list[len(list)-1]
		}
		r, err := Between(lo, "")
		if err != nil {
			t.Fatalf("end insertion %d: error: %v", i, err)
		}
		assertBetween(t, lo, r, "")
		list = append(list, r)
	}
	assertStrictlyIncreasing(t, list)
	t.Logf("max rank length after 200 end insertions: %d", maxLenOf(list))
}

func maxLenOf(list []string) int {
	m := 0
	for _, s := range list {
		if len(s) > m {
			m = len(s)
		}
	}
	return m
}
