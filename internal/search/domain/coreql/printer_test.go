package coreql

import "testing"

func TestPrinterNormalizesAndKeepsPrecedence(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"", ""},
		{"status=Done", "status = Done"},
		{"status = Done and priority = High or key = CORE-1", "status = Done AND priority = High OR key = CORE-1"},
		{"a = 1 OR b = 2 AND c = 3", "a = 1 OR b = 2 AND c = 3"},
		{"(a = 1 OR b = 2) AND c = 3", "(a = 1 OR b = 2) AND c = 3"},
		{"a = 1 AND (b = 2 AND c = 3)", "a = 1 AND (b = 2 AND c = 3)"},
		{"(a = 1 AND b = 2) AND c = 3", "a = 1 AND b = 2 AND c = 3"},
		{"a = 1 OR (b = 2 OR c = 3)", "a = 1 OR (b = 2 OR c = 3)"},
		{"((a = 1))", "a = 1"},
		{"NOT (a = 1 OR b = 2)", "NOT (a = 1 OR b = 2)"},
		{"NOT a = 1 AND b = 2", "NOT a = 1 AND b = 2"},
		{"NOT NOT a = 1", "NOT NOT a = 1"},
		{"a NOT IN (x,y)", "a NOT IN (x, y)"},
		{"a is null", "a IS EMPTY"},
		{"a IS NOT NULL", "a IS NOT EMPTY"},
		{`summary ~ 'it\'s'`, `summary ~ "it's"`},
		{`summary = "back\\slash \"q\""`, `summary = "back\\slash \"q\""`},
		{"created > -7d", "created > -7d"},
		{"created > +3d", "created > 3d"},
		{"due < 1M", "due < 1M"},
		{"created >= startOfDay(-1d)", "created >= startOfDay(-1d)"},
		{"assignee = currentUser ()", "assignee = currentUser()"},
		{"order by created desc, key", "ORDER BY created DESC, key"},
		{"ORDER BY created ASC", "ORDER BY created"},
		{"a = 1 ORDER BY x DESC", "a = 1 ORDER BY x DESC"},
		{"cf[Story Points]>3", "cf[Story Points] > 3"},
	}
	for _, c := range cases {
		q, err := Parse(c.in)
		if err != nil {
			t.Fatalf("Parse(%q): %v", c.in, err)
		}
		if got := q.String(); got != c.want {
			t.Errorf("String(Parse(%q)) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestPrinterBuildsParenthesesFromAST(t *testing.T) {
	// Or(a, And(b, c)) prints without parentheses because AND binds tighter.
	q := &Query{Expr: &Or{
		Left:  &Compare{Field: Field{Name: "a"}, Op: OpEq, Value: &Number{Text: "1"}},
		Right: &And{Left: &Compare{Field: Field{Name: "b"}, Op: OpEq, Value: &Number{Text: "2"}}, Right: &Compare{Field: Field{Name: "c"}, Op: OpEq, Value: &Number{Text: "3"}}},
	}}
	if got, want := q.String(), "a = 1 OR b = 2 AND c = 3"; got != want {
		t.Fatalf("String() = %q, want %q", got, want)
	}
	// And(a, Or(b, c)) must keep parentheses.
	q = &Query{Expr: &And{
		Left:  &Compare{Field: Field{Name: "a"}, Op: OpEq, Value: &Number{Text: "1"}},
		Right: &Or{Left: &Compare{Field: Field{Name: "b"}, Op: OpEq, Value: &Number{Text: "2"}}, Right: &Compare{Field: Field{Name: "c"}, Op: OpEq, Value: &Number{Text: "3"}}},
	}}
	if got, want := q.String(), "a = 1 AND (b = 2 OR c = 3)"; got != want {
		t.Fatalf("String() = %q, want %q", got, want)
	}
	if !Equal(q, mustParse(t, q.String())) {
		t.Fatalf("AST built by hand must round trip")
	}
}

func TestEqualIgnoresSpans(t *testing.T) {
	a := mustParse(t, "status = Done")
	b := mustParse(t, "  status   =   Done  ")
	if !Equal(a, b) {
		t.Fatalf("queries differing only in whitespace must be Equal")
	}
	cases := []string{
		"status != Done",
		"status = Closed",
		"status = Done AND a = 1",
		"status = Done OR a = 1",
		"NOT status = Done",
		"status IN (Done)",
		"status IS EMPTY",
		"status = Done ORDER BY a",
		"status = Done ORDER BY a DESC",
		`status = "Done"`,
		"status = 2",
		"status = 2d",
	}
	for _, in := range cases {
		if Equal(a, mustParse(t, in)) {
			t.Errorf("Equal(status = Done, %q) = true, want false", in)
		}
	}
	if Equal(a, nil) || !Equal(nil, nil) {
		t.Fatalf("nil handling wrong")
	}
}
