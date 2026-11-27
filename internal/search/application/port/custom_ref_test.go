package port

import "testing"

func TestCustomRef(t *testing.T) {
	cases := map[string]string{
		"CF[Cliente]":       "cf[cliente]",
		"cf[  Story Pts ]":  "cf[story pts]",
		"  Cliente ":        "cliente",
		"cf[abc":            "cf[abc",
		"Estado[Fix]":       "estado[fix]",
		"cf[]":              "cf[]",
		"CF[ÁÉ]":            "cf[áé]",
		"":                  "",
		"cf[a]b]":           "cf[a]b]",
		"\tcf[ x ]\n":       "cf[x]",
		"cf[Nombre Largo ]": "cf[nombre largo]",
	}
	for in, want := range cases {
		if got := CustomRef(in); got != want {
			t.Errorf("CustomRef(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestBuildErrorMessage(t *testing.T) {
	e := &BuildError{Pos: 1, Len: 2, Code: "c", Message: "algo falla"}
	if e.Error() == "" {
		t.Fatal("empty message")
	}
}
