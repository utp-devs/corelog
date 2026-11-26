package rank

import "testing"

// FuzzBetween checks that Between never panics, returns only valid ranks, keeps
// the strict ordering, and fails only when the inputs are invalid or misordered.
func FuzzBetween(f *testing.F) {
	seeds := [][2]string{
		{"", ""},
		{"", "V"},
		{"V", ""},
		{"a", "b"},
		{"a", "a1"},
		{"a", "aV"},
		{"", "1"},
		{"", "0V"},
		{"aZ", "b"},
		{"V", "V0V"},
		{"0", "1"},
		{"a0", "b"},
		{"zzzz", ""},
		{"b", "a"},
	}
	for _, s := range seeds {
		f.Add(s[0], s[1])
	}
	f.Fuzz(func(t *testing.T, a, b string) {
		r, err := Between(a, b)
		validInputs := validate(a) == nil && validate(b) == nil &&
			(a == "" || b == "" || a < b)
		if err != nil {
			if validInputs {
				t.Fatalf("Between(%q, %q) unexpected error for valid input: %v", a, b, err)
			}
			return
		}
		if validate(r) != nil {
			t.Fatalf("Between(%q, %q) returned invalid rank %q", a, b, r)
		}
		if a != "" && !(a < r) {
			t.Fatalf("Between(%q, %q) = %q, not above lower bound", a, b, r)
		}
		if b != "" && !(r < b) {
			t.Fatalf("Between(%q, %q) = %q, not below upper bound", a, b, r)
		}
	})
}
