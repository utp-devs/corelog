package coreql

import (
	"errors"
	"fmt"
	"strings"
	"testing"
)

// grammarCase is one row of the grammar table. A valid row has msg == "".
// An invalid row must fail with a *SyntaxError at byte offset pos whose
// message contains msg.
type grammarCase struct {
	in  string
	pos int
	msg string
}

func ok(in string) grammarCase { return grammarCase{in: in} }

func bad(in string, pos int, msg string) grammarCase {
	return grammarCase{in: in, pos: pos, msg: msg}
}

var handWrittenCases = []grammarCase{
	// empty and blank queries
	ok(""),
	ok("   "),
	ok("\t\n"),

	// simple clauses and value kinds
	ok("status = Done"),
	ok(`status = "In Progress"`),
	ok(`status = 'In Progress'`),
	ok("project = CORE"),
	ok("key = CORE-12"),
	ok("priority = High"),
	ok("status != Done"),
	ok("storyPoints > 3"),
	ok("storyPoints >= 3.5"),
	ok("storyPoints < 10"),
	ok("storyPoints <= 0"),
	ok("summary ~ login"),
	ok(`summary !~ "bug"`),
	ok(`summary ~ ''`),
	ok("labels = backend"),
	ok(`labels = "front end"`),
	ok(`labels = ""`),
	ok("status=Done"),
	ok("status = -5"),
	ok("status = 1.5"),
	ok("status = 5days"),
	ok("status = CORE-12-3"),
	ok("flagged = true"),
	ok("flagged IS EMPTY"),
	ok("type = Bug"),
	ok("issuetype = Story"),
	ok("statusCategory = done"),
	ok("parent = CORE-1"),
	ok("epic = CORE-9"),
	ok("fixVersion = 1.0"),
	ok(`affectedVersion = "2.0 RC"`),
	ok("component = API"),
	ok("watcher = currentUser()"),
	ok("creator = currentUser()"),
	ok("comment ~ x"),
	ok("description !~ y"),
	ok("originalEstimate > 0"),
	ok("remainingEstimate <= 3600"),
	ok("timeSpent > 1.25"),
	ok("votes >= 0"),
	ok("text ~ café"),
	ok(`descripción ~ "niño"`),
	ok(`cf[Story Points] > 3`),
	ok(`cf[10010] = High`),
	ok(`cf[Sprint]= x`),
	ok(`CF[x] = y`),

	// relative dates and functions
	ok("created > -7d"),
	ok("created >= -2w"),
	ok("due < 1M"),
	ok("updated > +3d"),
	ok("created > -1y"),
	ok("created > -3m"),
	ok("created > -3h"),
	ok("created > +0d"),
	ok("created < 2d"),
	ok("due = -10w"),
	ok("created < startOfDay(-1d)"),
	ok("created >= startOfWeek()"),
	ok("created >= endOfMonth()"),
	ok("created = startOfDay(-1d, 2)"),
	ok("created = Done()"),
	ok("assignee = currentUser()"),
	ok(`assignee = membersOf("Equipo A")`),
	ok("assignee = currentUser ()"),
	ok("sprint = openSprints()"),
	ok("start < endOfDay()"),
	ok("resolved >= startOfMonth(-1d)"),
	ok("created >= now()"),

	// IN lists, IS EMPTY, NOT
	ok(`status IN (Done, "In Progress")`),
	ok("status NOT IN (Done)"),
	ok("status IN (Done)"),
	ok("status in (done,open)"),
	ok(`status NOT IN ("A", 'B', C)`),
	ok("a IN (1, 2.5, -3)"),
	ok("a IN (currentUser(), \"x\")"),
	ok("a NOT IN (x)"),
	ok("NOT status IN (Done)"),
	ok("resolution IS EMPTY"),
	ok("resolution IS NOT EMPTY"),
	ok("resolution IS NULL"),
	ok("resolution IS NOT NULL"),
	ok("resolution is empty"),
	ok("assignee IS EMPTY AND reporter = currentUser()"),

	// boolean structure and precedence
	ok("status = Done AND priority = High"),
	ok("status = Done OR priority = High"),
	ok("a = 1 OR b = 2 AND c = 3"),
	ok("a = 1 AND b = 2 OR c = 3"),
	ok("(a = 1 OR b = 2) AND c = 3"),
	ok("a = 1 AND (b = 2 OR c = 3)"),
	ok("((a = 1))"),
	ok("NOT a = 1"),
	ok("NOT (a = 1 OR b = 2)"),
	ok("NOT NOT a = 1"),
	ok("NOT a = 1 AND b = 2"),
	ok("a = 1 AND NOT b = 2"),
	ok("(status = Done)"),
	ok(`project = "CORE" AND (status IN (Done, Closed) OR resolution IS EMPTY)`),
	ok("key != CORE-1 AND key != CORE-2 AND key != CORE-3"),
	ok("summary ~ a OR summary ~ b OR summary ~ c AND priority = High"),
	ok("status = Done and priority = High or key = CORE-1"),
	ok("STATUS = DONE AND Priority = high"),
	ok("status=Done AND key=CORE-1ORDER"),
	ok(`summary ~ "x"ORDER BY created`),

	// ORDER BY
	ok("ORDER BY created"),
	ok("ORDER BY created DESC"),
	ok("ORDER BY created ASC"),
	ok("ORDER BY priority DESC, key ASC, created"),
	ok("status = Done ORDER BY priority DESC, key"),
	ok("order by created desc"),
	ok("status = Done AND key = CORE-1 ORDER BY key ASC, created DESC"),
	ok("ORDER BY cf[Rank]"),

	// strings and escapes
	ok(`summary ~ "caf\"é"`),
	ok(`summary ~ 'it\'s'`),
	ok(`summary = "back\\slash"`),
	ok(`summary = "a\nb"`),
	ok(`summary = "tab\there"`),
	ok(`summary = "AND OR NOT IN"`),
	ok(`summary = "ORDER BY"`),
	ok(`summary = "(paren)"`),
	ok(`summary = "2.0 = 3"`),
	ok(`summary = "unicode ✓"`),

	// misc
	ok("status = Done ORDER BY created"),
	ok("a = 1 ORDER BY x"),
	ok("status = Done"),
	ok("summary = x\tAND\ny = 2"),

	// errors: unexpected or missing tokens
	bad("status =", 8, "fin inesperado"),
	bad("status", 6, "fin inesperado"),
	bad("status = ", 9, "fin inesperado"),
	bad("status = Done AND", 17, "fin inesperado"),
	bad("status = Done )", 14, "token inesperado ')'"),
	bad("(status = Done", 14, "se esperaba ')'"),
	bad("status = Done status", 14, "token inesperado 'status'"),
	bad("status == Done", 8, "token inesperado '='"),
	bad("AND status = Done", 0, "token inesperado 'AND'"),
	bad("status Done", 7, "token inesperado 'Done'; se esperaba un operador"),
	bad("status = Done ORDER created", 20, "token inesperado 'created'; se esperaba BY"),
	bad("status = Done ORDER BY", 22, "fin inesperado"),
	bad("status = Done ORDER BY ,", 23, "token inesperado ','; se esperaba un campo"),
	bad("ORDER BY created,", 17, "fin inesperado"),
	bad("status = Done ORDER BY created ASC DESC", 35, "token inesperado 'DESC'; se esperaba ',' o fin de la consulta"),
	bad("status = Done AND ) ", 18, "token inesperado ')'; se esperaba un campo"),
	bad("status IN Done", 10, "se esperaba '('"),
	bad("status IN ()", 11, "se esperaba un valor"),
	bad("status IN (Done", 15, "fin inesperado"),
	bad("status IN (Done Open)", 16, "se esperaba ',' o ')'"),
	bad("status IN (a,, b)", 13, "token inesperado ','; se esperaba un valor"),
	bad("status NOT Done", 11, "se esperaba IN"),
	bad("status IS Done", 10, "se esperaba EMPTY o NULL"),
	bad("status IS NOT Done", 14, "se esperaba EMPTY o NULL"),
	bad("status IS", 9, "fin inesperado"),
	bad("status IS EMPTY EMPTY", 16, "token inesperado 'EMPTY'"),
	bad("status NOT IN", 13, "fin inesperado"),
	bad("status IN (1 2)", 13, "token inesperado '2'"),
	bad(`status IN ("a" "b")`, 15, "se esperaba ',' o ')'"),
	bad("status = Done OR (", 18, "fin inesperado"),
	bad("(a = 1", 6, "se esperaba ')'"),
	bad("((a = 1)", 8, "se esperaba ')'"),
	bad("a = 1)", 5, "token inesperado ')'"),
	bad("()", 1, "token inesperado ')'; se esperaba un campo"),
	bad("NOT", 3, "fin inesperado"),
	bad("NOT AND a = 1", 4, "token inesperado 'AND'"),
	bad("NOT NOT", 7, "fin inesperado"),
	bad("a = 1 OR", 8, "fin inesperado"),
	bad("a = 1 OR OR b = 2", 9, "token inesperado 'OR'"),
	bad("a = 1 AND AND", 10, "token inesperado 'AND'"),
	bad("a = 1 b = 2", 6, "se esperaba AND, OR, ORDER BY o fin de la consulta"),
	bad("a = 1 ORDER BY", 14, "fin inesperado"),
	bad("status = Done ORDER BY created DESC,", 36, "fin inesperado"),
	bad("status = Done ORDER BY 3", 23, "token inesperado '3'; se esperaba un campo"),
	bad("status = Done ORDER BY ASC", 23, "token inesperado 'ASC'; se esperaba un campo"),
	bad(`status = Done ORDER BY "x"`, 23, "token inesperado '\"x\"'; se esperaba un campo"),
	bad("status = Done ORDER BY created DESC ORDER BY x", 36, "token inesperado 'ORDER'"),
	bad("status = Done ORDER BY created ASC ASC", 35, "token inesperado 'ASC'"),
	bad("status = Done ORDER BY created ;", 31, "se esperaba ',' o fin de la consulta"),
	bad("status = Done AND status", 24, "fin inesperado"),
	bad("status = Done AND priority", 26, "fin inesperado"),
	bad("status = Done AND NOT", 21, "fin inesperado"),
	bad("status = Done AND NOT )", 22, "token inesperado ')'"),
	bad("status = Done AND 5", 18, "token inesperado '5'"),
	bad("status = Done AND ORDER BY x", 18, "token inesperado 'ORDER'; se esperaba un campo, NOT o '('"),
	bad("status = Done AND (status = Done", 32, "se esperaba ')'"),
	bad("status = Done AND (a = 1 OR )", 28, "token inesperado ')'; se esperaba un campo"),
	bad("status = NOT", 9, "token inesperado 'NOT'"),
	bad("status = AND", 9, "token inesperado 'AND'"),
	bad("status = ORDER", 9, "token inesperado 'ORDER'"),
	bad("status = (Done)", 9, "token inesperado '('"),
	bad("status = Done ~", 14, "token inesperado '~'"),
	bad("status = Done = x", 14, "token inesperado '='"),
	bad("status = Done;", 13, "token inesperado ';'"),
	bad("IN (a)", 0, "token inesperado 'IN'"),
	bad(`"status" = Done`, 0, `token inesperado '"status"'`),
	bad("summary ~ \"a\" ~ \"b\"", 14, "token inesperado '~'"),
	bad("status ~~ x", 8, "token inesperado '~'; se esperaba un valor"),
	bad("status = Done\nORDER created", 20, "token inesperado 'created'"),
	bad("status = Done ORDER BY created DESC) ", 35, "token inesperado ')'"),

	// errors: lexical
	bad(`status = "Done`, 9, "cadena sin cerrar"),
	bad("status = 'Done", 9, "cadena sin cerrar"),
	bad(`summary ~ "café`, 10, "cadena sin cerrar"),
	bad(`summary ~ "x\"`, 10, "cadena sin cerrar"),
	bad(`key = "CORE-1`, 6, "cadena sin cerrar"),
	bad("status ! Done", 7, "operador no válido"),
	bad("status = Done & x", 14, "carácter no válido '&'"),
	bad("status = Done #", 14, "carácter no válido '#'"),
	bad("created > 1..2", 11, "carácter no válido '.'"),
	bad("summary ~ \"a\" AND summary !", 26, "operador no válido"),
	bad("cf[Story Points > 3", 0, "campo personalizado sin cerrar"),
	bad("cf[] = 1", 0, "clave de campo personalizado vacía"),
	bad("cf[ ] = 1", 0, "clave de campo personalizado vacía"),
	bad("cf[x", 0, "campo personalizado sin cerrar"),
	bad("status = Done ORDER BY cf[x", 23, "campo personalizado sin cerrar"),
	bad("status = 99999999999999999999999d", 9, "magnitud de fecha relativa fuera de rango"),
	bad("cf[x]", 5, "se esperaba un operador"),
}

// generatedCases builds valid combinations of field names, operators and
// value forms. They exercise the grammar broadly without hand-writing rows.
func generatedCases() []grammarCase {
	fields := []string{
		"project", "key", "issuetype", "type", "status", "statusCategory", "resolution",
		"priority", "assignee", "reporter", "creator", "labels", "component", "fixVersion",
		"affectedVersion", "sprint", "epic", "parent", "summary", "description", "comment",
		"text", "created", "updated", "resolved", "due", "start", "votes", "watcher",
		"flagged", "storyPoints", "originalEstimate", "remainingEstimate", "timeSpent",
		"cf[Team Area]",
	}
	values := []string{
		"Done", `"In Progress"`, "'x y'", "42", "-7d", "CORE-12", "currentUser()",
		`membersOf("team")`, "startOfDay(-1d)", "1.5",
	}
	ops := []string{"=", "!=", "~", "!~", ">", ">=", "<", "<="}
	var out []grammarCase
	for i, f := range fields {
		for j, v := range values {
			op := ops[(i+j)%len(ops)]
			out = append(out, ok(fmt.Sprintf("%s %s %s", f, op, v)))
		}
		out = append(out, ok(f+" IS EMPTY"), ok(f+" IS NOT EMPTY"))
		out = append(out, ok(fmt.Sprintf("%s IN (%s, %s)", f, values[0], values[1])))
		out = append(out, ok(fmt.Sprintf("NOT %s NOT IN (%s)", f, values[i%len(values)])))
		out = append(out, ok(fmt.Sprintf("%s = %s AND (%s = %s OR %s != %s) ORDER BY %s DESC",
			f, values[0], f, values[1], f, values[2], f)))
	}
	return out
}

func allGrammarCases() []grammarCase {
	return append(append([]grammarCase(nil), handWrittenCases...), generatedCases()...)
}

func TestGrammarTable(t *testing.T) {
	cases := allGrammarCases()
	if len(cases) < 200 {
		t.Fatalf("grammar table has %d cases, want at least 200", len(cases))
	}
	for _, c := range cases {
		c := c
		t.Run(fmt.Sprintf("%q", c.in), func(t *testing.T) {
			q, err := Parse(c.in)
			if c.msg == "" {
				if err != nil {
					t.Fatalf("Parse(%q) unexpected error: %v", c.in, err)
				}
				if q == nil {
					t.Fatalf("Parse(%q) returned nil query without error", c.in)
				}
				return
			}
			var se *SyntaxError
			if !errors.As(err, &se) {
				t.Fatalf("Parse(%q) error = %v (%T), want *SyntaxError", c.in, err, err)
			}
			if se.Pos != c.pos {
				t.Errorf("Parse(%q) error pos = %d, want %d (message %q)", c.in, se.Pos, c.pos, se.Message)
			}
			if !strings.Contains(se.Message, c.msg) {
				t.Errorf("Parse(%q) message = %q, want it to contain %q", c.in, se.Message, c.msg)
			}
			if len(se.Expected) == 0 && !strings.HasPrefix(se.Message, "cadena") &&
				!strings.Contains(se.Message, "carácter") && !strings.Contains(se.Message, "operador no") &&
				!strings.Contains(se.Message, "campo personalizado") && !strings.Contains(se.Message, "clave") &&
				!strings.Contains(se.Message, "magnitud") {
				t.Errorf("Parse(%q) error has no expected list", c.in)
			}
		})
	}
}

func TestGrammarRoundTrip(t *testing.T) {
	for _, c := range allGrammarCases() {
		if c.msg != "" {
			continue
		}
		c := c
		t.Run(fmt.Sprintf("%q", c.in), func(t *testing.T) {
			q, err := Parse(c.in)
			if err != nil {
				t.Fatalf("Parse(%q): %v", c.in, err)
			}
			s := q.String()
			q2, err := Parse(s)
			if err != nil {
				t.Fatalf("Parse(String()=%q): %v", s, err)
			}
			if !Equal(q, q2) {
				t.Fatalf("round trip changed the AST:\n in:  %q\n out: %q", c.in, s)
			}
			if q2.String() != s {
				t.Fatalf("String() is not stable: %q -> %q", s, q2.String())
			}
		})
	}
}
