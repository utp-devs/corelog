package coreql

import (
	"strings"
	"testing"
)

func TestTokenizeKindsAndPositions(t *testing.T) {
	cases := []struct {
		in    string
		kinds []TokenKind
		texts []string
	}{
		{"status = Done", []TokenKind{TokWord, TokOp, TokWord, TokEOF}, []string{"status", "=", "Done", ""}},
		{`a != "x y"`, []TokenKind{TokWord, TokOp, TokString, TokEOF}, []string{"a", "!=", "x y", ""}},
		{"x >= 3", []TokenKind{TokWord, TokOp, TokNumber, TokEOF}, []string{"x", ">=", "3", ""}},
		{"x <= -3", []TokenKind{TokWord, TokOp, TokNumber, TokEOF}, []string{"x", "<=", "-3", ""}},
		{"x !~ y", []TokenKind{TokWord, TokOp, TokWord, TokEOF}, []string{"x", "!~", "y", ""}},
		{"x > -7d", []TokenKind{TokWord, TokOp, TokRelDate, TokEOF}, []string{"x", ">", "-7d", ""}},
		{"x > +3d", []TokenKind{TokWord, TokOp, TokRelDate, TokEOF}, []string{"x", ">", "+3d", ""}},
		{"x < 1M", []TokenKind{TokWord, TokOp, TokRelDate, TokEOF}, []string{"x", "<", "1M", ""}},
		{"x < 1m", []TokenKind{TokWord, TokOp, TokRelDate, TokEOF}, []string{"x", "<", "1m", ""}},
		{"x = 1.25", []TokenKind{TokWord, TokOp, TokNumber, TokEOF}, []string{"x", "=", "1.25", ""}},
		{"x = 5days", []TokenKind{TokWord, TokOp, TokWord, TokEOF}, []string{"x", "=", "5days", ""}},
		{"x = CORE-12", []TokenKind{TokWord, TokOp, TokWord, TokEOF}, []string{"x", "=", "CORE-12", ""}},
		{"cf[Story Points] = 3", []TokenKind{TokCustomField, TokOp, TokNumber, TokEOF}, []string{"cf[Story Points]", "=", "3", ""}},
		{"a IN (b, c)", []TokenKind{TokWord, TokIn, TokLParen, TokWord, TokComma, TokWord, TokRParen, TokEOF}, nil},
		{"a is not empty", []TokenKind{TokWord, TokIs, TokNot, TokEmpty, TokEOF}, nil},
		{"ORDER BY a ASC, b desc", []TokenKind{TokOrder, TokBy, TokWord, TokAsc, TokComma, TokWord, TokDesc, TokEOF}, nil},
		{"a = and", []TokenKind{TokWord, TokOp, TokAnd, TokEOF}, nil},
		{"", []TokenKind{TokEOF}, []string{""}},
	}
	for _, c := range cases {
		toks := tokenize(c.in)
		if len(toks) != len(c.kinds) {
			t.Fatalf("tokenize(%q) gave %d tokens, want %d: %+v", c.in, len(toks), len(c.kinds), toks)
		}
		for i, k := range c.kinds {
			if toks[i].Kind != k {
				t.Errorf("tokenize(%q)[%d] kind = %v, want %v", c.in, i, toks[i].Kind, k)
			}
		}
		for i, txt := range c.texts {
			if toks[i].Text != txt {
				t.Errorf("tokenize(%q)[%d] text = %q, want %q", c.in, i, toks[i].Text, txt)
			}
		}
	}
}

func TestTokenizeByteOffsetsWithUTF8(t *testing.T) {
	in := `summary ~ "canción" AND niño = Diseño`
	toks := tokenize(in)
	for _, tk := range toks {
		if tk.Kind == TokEOF {
			if tk.Pos != len(in) {
				t.Fatalf("EOF pos = %d, want %d", tk.Pos, len(in))
			}
			continue
		}
		raw := in[tk.Pos : tk.Pos+tk.Len]
		if tk.Kind == TokString {
			if raw != `"canción"` {
				t.Errorf("string raw = %q, want %q", raw, `"canción"`)
			}
			if tk.Text != "canción" {
				t.Errorf("string text = %q, want canción", tk.Text)
			}
		}
	}
	// "niño" starts after `summary ~ "canción" AND ` which is 25 characters but more bytes.
	want := strings.Index(in, "niño")
	var got int = -1
	for _, tk := range toks {
		if tk.Kind == TokWord && tk.Text == "niño" {
			got = tk.Pos
		}
	}
	if got != want {
		t.Fatalf("niño byte pos = %d, want %d", got, want)
	}
	if got := RuneOffset(in, want); got != len([]rune(in[:want])) || got >= want {
		t.Fatalf("RuneOffset(niño) = %d, want the rune count of the prefix (bytes %d)", got, want)
	}
}

func TestRuneOffsetAndByteOffsetRoundTrip(t *testing.T) {
	in := `summary ~ "canción" AND (niño = ñ)`
	for runeIdx, r := range []rune(in) {
		_ = r
		b := ByteOffset(in, runeIdx)
		if got := RuneOffset(in, b); got != runeIdx {
			t.Fatalf("RuneOffset(ByteOffset(%d)) = %d", runeIdx, got)
		}
	}
	if got := RuneOffset(in, len(in)); got != len([]rune(in)) {
		t.Fatalf("RuneOffset at end = %d, want %d", got, len([]rune(in)))
	}
	if ByteOffset(in, -4) != 0 || RuneOffset(in, -4) != 0 {
		t.Fatalf("negative offsets must clamp to 0")
	}
	if ByteOffset(in, 10000) != len(in) || RuneOffset(in, 10000) != len([]rune(in)) {
		t.Fatalf("large offsets must clamp to the end")
	}
}

func TestSyntaxErrorPositionIsReportedInRunes(t *testing.T) {
	// The unterminated string starts after two accented characters.
	in := `resumen ~ "ñandú`
	_, err := Parse(in)
	se, okErr := err.(*SyntaxError)
	if !okErr {
		t.Fatalf("want *SyntaxError, got %v", err)
	}
	if se.Pos != len("resumen ~ ") {
		t.Fatalf("byte pos = %d, want %d", se.Pos, len("resumen ~ "))
	}
	if got := RuneOffset(in, se.Pos); got != len("resumen ~ ") {
		t.Fatalf("rune pos = %d, want %d", got, len("resumen ~ "))
	}
	// Error after multi-byte text: the EOF position in runes differs from bytes.
	in2 := `summary ~ "é" AND`
	_, err = Parse(in2)
	se = err.(*SyntaxError)
	if se.Pos != len(in2) {
		t.Fatalf("byte pos = %d, want %d", se.Pos, len(in2))
	}
	if RuneOffset(in2, se.Pos) != len([]rune(in2)) || RuneOffset(in2, se.Pos) == se.Pos {
		t.Fatalf("rune pos = %d, byte pos = %d: expected them to differ", RuneOffset(in2, se.Pos), se.Pos)
	}
}

func TestTokenizeIllegalCharactersCarryMessages(t *testing.T) {
	toks := tokenize("a ? b")
	if toks[1].Kind != TokIllegal || toks[1].Pos != 2 || toks[1].Len != 1 {
		t.Fatalf("illegal token = %+v", toks[1])
	}
	if !strings.Contains(toks[1].Message, "carácter no válido") {
		t.Fatalf("message = %q", toks[1].Message)
	}
}

func TestOperatorTokensCarryOp(t *testing.T) {
	cases := map[string]Op{"=": OpEq, "!=": OpNe, ">": OpGt, ">=": OpGe, "<": OpLt, "<=": OpLe, "~": OpContains, "!~": OpNotContains}
	for in, want := range cases {
		toks := tokenize(in)
		if toks[0].Kind != TokOp || toks[0].Op != want || toks[0].Len != len(in) {
			t.Errorf("tokenize(%q) = %+v, want op %v", in, toks[0], want)
		}
		if want.String() != in {
			t.Errorf("Op(%v).String() = %q, want %q", want, want.String(), in)
		}
	}
}
