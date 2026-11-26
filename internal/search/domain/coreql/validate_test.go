package coreql

import (
	"strings"
	"testing"
)

func mustParse(t *testing.T, in string) *Query {
	t.Helper()
	q, err := Parse(in)
	if err != nil {
		t.Fatalf("Parse(%q): %v", in, err)
	}
	return q
}

func messages(errs []ValidationError) string {
	var parts []string
	for _, e := range errs {
		parts = append(parts, e.Message)
	}
	return strings.Join(parts, " | ")
}

func TestValidateSystemCatalog(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string // substring of the expected message; empty means no error
	}{
		{"valid status", "status = Done", ""},
		{"valid text contains", "summary ~ login", ""},
		{"valid date compare", "created > -7d", ""},
		{"valid date function", "created >= startOfDay(-1d)", ""},
		{"valid user function", "assignee = currentUser()", ""},
		{"valid sprint function", "sprint = openSprints()", ""},
		{"valid number compare", "storyPoints >= 3.5", ""},
		{"valid bool", "flagged = true", ""},
		{"valid empty", "resolution IS EMPTY", ""},
		{"valid in list", "status IN (Done, Closed)", ""},
		{"valid order", "status = Done ORDER BY priority DESC, key", ""},
		{"valid empty query", "", ""},
		{"unknown field", "statu = Done", "campo desconocido 'statu'"},
		{"tilt operator on text", "summary > x", "el operador > no es válido para el campo summary"},
		{"contains on status", "status ~ Done", "el operador ~ no es válido para el campo status"},
		{"not contains on date", "created !~ x", "el operador !~ no es válido"},
		{"relative date on number", "storyPoints > -7d", "las fechas relativas solo aplican a campos de fecha"},
		{"number on user", "assignee = 42", "se esperaba un valor de tipo user"},
		{"text not a number", "storyPoints = abc", "se esperaba un número"},
		{"bad bool", "flagged = yes", "se esperaba true o false"},
		{"bad date literal", "due = Tomorrow", "se esperaba una fecha YYYY-MM-DD"},
		{"good date literal", "due = 2026-01-31", ""},
		{"bad datetime literal", "created = 2026-13-45", "se esperaba una fecha y hora"},
		{"good datetime literal", `created > "2026-01-31 10:00"`, ""},
		{"unknown function", "assignee = whoAmI()", "función desconocida 'whoAmI'"},
		{"function wrong arity", "created = startOfDay(-1d, -2d)", "acepta entre 0 y 1 argumentos"},
		{"no-arg function with args", "sprint = openSprints(1)", "acepta 0 argumentos"},
		{"missing required arg", "assignee = membersOf()", "acepta 1 argumentos"},
		{"function returns wrong type", "labels = currentUser()", "no es compatible con el campo labels"},
		{"date function on user field", "assignee = now()", "no es compatible con el campo assignee"},
		{"sprint function on version field", "fixVersion = openSprints()", "no es compatible"},
		{"version function on version field", "fixVersion = releasedVersions()", ""},
		{"relative arg required", "created = startOfDay(Done)", "espera una fecha relativa"},
		{"text arg required", "assignee = membersOf(-1d)", "espera texto como argumento"},
		{"nested function", "assignee = membersOf(currentUser())", "no se admiten funciones anidadas"},
		{"order by text", "ORDER BY summary", ""},
		{"order by description", "ORDER BY description", "no se puede ordenar por el campo description"},
		{"order by labels", "ORDER BY labels", "no se puede ordenar por el campo labels"},
		{"order by watcher", "ORDER BY watcher", "no se puede ordenar por el campo watcher"},
		{"order by comment", "ORDER BY comment DESC", "no se puede ordenar por el campo comment"},
		{"in on bool is still equality", "flagged IN (true, false)", ""},
		{"case-insensitive field", "STATUS = Done", ""},
		{"case-insensitive function", "assignee = CURRENTUSER()", ""},
		{"alias creator", "creator = currentUser()", ""},
		{"alias type", "type = Bug", ""},
	}
	for _, c := range cases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			errs := Validate(mustParse(t, c.in), nil, nil)
			if c.want == "" {
				if len(errs) != 0 {
					t.Fatalf("Validate(%q) = %s, want no errors", c.in, messages(errs))
				}
				return
			}
			if len(errs) == 0 {
				t.Fatalf("Validate(%q) = no errors, want %q", c.in, c.want)
			}
			found := false
			for _, e := range errs {
				if strings.Contains(e.Message, c.want) {
					found = true
				}
			}
			if !found {
				t.Fatalf("Validate(%q) = %s, want message containing %q", c.in, messages(errs), c.want)
			}
		})
	}
}

func TestValidateUnknownFieldSuggestsCloseNames(t *testing.T) {
	errs := Validate(mustParse(t, "statu = Done"), SystemCatalog(), DefaultFunctions())
	if len(errs) != 1 {
		t.Fatalf("errors = %v, want one", messages(errs))
	}
	e := errs[0]
	if e.Pos != 0 || e.Len != len("statu") {
		t.Errorf("position = %d/%d, want 0/5", e.Pos, e.Len)
	}
	if len(e.Suggestions) == 0 || e.Suggestions[0] != "status" {
		t.Errorf("suggestions = %v, want status first", e.Suggestions)
	}
}

func TestValidateSuggestionsAreLimitedAndOrdered(t *testing.T) {
	errs := Validate(mustParse(t, "STATUS2 = x"), SystemCatalog(), nil)
	if len(errs) != 1 {
		t.Fatalf("errors = %v", messages(errs))
	}
	if len(errs[0].Suggestions) == 0 || errs[0].Suggestions[0] != "status" {
		t.Fatalf("suggestions = %v, want status first (case-insensitive distance 1)", errs[0].Suggestions)
	}
	if len(errs[0].Suggestions) > 5 {
		t.Fatalf("suggestions = %d, want at most 5", len(errs[0].Suggestions))
	}
}

func TestValidateOperatorErrorPointsAtOperator(t *testing.T) {
	in := "summary > x"
	errs := Validate(mustParse(t, in), nil, nil)
	if len(errs) != 1 {
		t.Fatalf("errors = %v", messages(errs))
	}
	if errs[0].Pos != strings.Index(in, ">") {
		t.Fatalf("operator error pos = %d, want %d", errs[0].Pos, strings.Index(in, ">"))
	}
}

func TestValidateFunctionErrorPosition(t *testing.T) {
	in := "created = startOfDay(-1d, -2d)"
	errs := Validate(mustParse(t, in), nil, nil)
	if len(errs) != 1 {
		t.Fatalf("errors = %v", messages(errs))
	}
	if errs[0].Pos != strings.Index(in, "startOfDay") {
		t.Fatalf("pos = %d, want function start", errs[0].Pos)
	}
}

func TestValidateCustomCatalogOnTopOfSystem(t *testing.T) {
	custom := NewMapCatalog(
		FieldDef{Name: "cf[Story Points]", Type: TypeNumber, Sortable: true},
		FieldDef{Name: "cf[Team]", Type: TypeEnum},
		FieldDef{Name: "cf[Notes]", Type: TypeText},
	)
	cat := CompositeCatalog{Top: custom, Base: SystemCatalog()}

	cases := map[string]string{
		"cf[Story Points] > 3":           "",
		"CF[story points] = 5":           "",
		"cf[Team] = Alpha":               "",
		"cf[Team] > Alpha":               "el operador > no es válido para el campo cf[Team]",
		"cf[Unknown] = 1":                "campo desconocido 'cf[Unknown]'",
		"cf[Notes] ~ hello":              "",
		"status = Done":                  "",
		"ORDER BY cf[Notes]":             "no se puede ordenar por el campo cf[Notes]",
		"ORDER BY cf[Story Points] DESC": "",
	}
	for in, want := range cases {
		errs := Validate(mustParse(t, in), cat, nil)
		if want == "" {
			if len(errs) != 0 {
				t.Errorf("Validate(%q) = %s, want none", in, messages(errs))
			}
			continue
		}
		if !strings.Contains(messages(errs), want) {
			t.Errorf("Validate(%q) = %s, want %q", in, messages(errs), want)
		}
	}
}

func TestCompositeCatalogLookupAndNames(t *testing.T) {
	custom := NewMapCatalog(FieldDef{Name: "status", Type: TypeText})
	cat := CompositeCatalog{Top: custom, Base: SystemCatalog()}
	def, ok := cat.Lookup("STATUS")
	if !ok || def.Type != TypeText {
		t.Fatalf("top layer should win, got %+v ok=%v", def, ok)
	}
	names := cat.Names()
	count := 0
	for _, n := range names {
		if strings.EqualFold(n, "status") {
			count++
		}
	}
	if count != 1 {
		t.Fatalf("status listed %d times, want once", count)
	}
	if _, ok := (CompositeCatalog{}).Lookup("status"); ok {
		t.Fatalf("empty composite must not resolve names")
	}
}

func TestValidateNilArgumentsUseDefaults(t *testing.T) {
	errs := Validate(mustParse(t, "status = Done"), nil, nil)
	if len(errs) != 0 {
		t.Fatalf("default catalog should accept status: %s", messages(errs))
	}
	if Validate(nil, nil, nil) != nil {
		t.Fatalf("nil query must validate to nil")
	}
}

func TestLevenshtein(t *testing.T) {
	cases := []struct {
		a, b string
		want int
	}{
		{"", "", 0},
		{"abc", "", 3},
		{"statu", "status", 1},
		{"Status", "status", 0},
		{"kitten", "sitting", 3},
		{"prioriti", "priority", 1},
	}
	for _, c := range cases {
		if got := levenshtein(c.a, c.b); got != c.want {
			t.Errorf("levenshtein(%q,%q) = %d, want %d", c.a, c.b, got, c.want)
		}
	}
}
