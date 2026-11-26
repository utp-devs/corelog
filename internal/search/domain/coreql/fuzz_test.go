package coreql

import (
	"errors"
	"testing"
)

// FuzzParse checks three properties for arbitrary input:
//   - Parse never panics, and every failure is a *SyntaxError whose position lies inside the input;
//   - a successful parse has a String form that parses again;
//   - the reparsed AST equals the original (round trip).
//
// Validate and Suggest must not panic on any input either.
func FuzzParse(f *testing.F) {
	for _, c := range allGrammarCases() {
		f.Add(c.in)
	}
	f.Add("status = Done AND (")
	f.Add(`summary ~ "unterminated`)
	f.Add("cf[")
	f.Add("created > -7d ORDER BY created DESC")
	f.Add("\xff\xfe = 1")

	f.Fuzz(func(t *testing.T, in string) {
		q, err := Parse(in)
		if err != nil {
			var se *SyntaxError
			if !errors.As(err, &se) {
				t.Fatalf("Parse(%q) returned %T, want *SyntaxError", in, err)
			}
			if se.Pos < 0 || se.Pos > len(in) {
				t.Fatalf("Parse(%q) error position %d outside input", in, se.Pos)
			}
			return
		}
		s := q.String()
		q2, err := Parse(s)
		if err != nil {
			t.Fatalf("Parse(%q) ok but String() %q does not parse: %v", in, s, err)
		}
		if !Equal(q, q2) {
			t.Fatalf("round trip changed the AST for %q (printed %q)", in, s)
		}
		_ = Validate(q, nil, nil)
		_ = Suggest(in, RuneOffset(in, len(in)), nil)
	})
}
