package coreql

import "testing"

// suggestLabels returns the labels of Suggest for input with the cursor at the end.
func suggestLabels(input string) map[string]bool {
	out := map[string]bool{}
	for _, s := range Suggest(input, RuneOffset(input, len(input)), nil) {
		out[s.Label] = true
	}
	return out
}

func TestSuggestExpressionStartOffersFields(t *testing.T) {
	got := suggestLabels("")
	for _, want := range []string{"status", "project", "summary", "NOT", "ORDER BY"} {
		if !got[want] {
			t.Errorf("empty input: missing %q", want)
		}
	}
	if got := suggestLabels("sta"); !got["status"] || !got["statusCategory"] || got["project"] {
		t.Errorf("prefix sta: got %v", got)
	}
}

func TestSuggestPrefixIsCaseInsensitive(t *testing.T) {
	got := suggestLabels("STA")
	if !got["status"] {
		t.Fatalf("STA should match status: %v", got)
	}
}

func TestSuggestOperatorsDependOnFieldType(t *testing.T) {
	status := suggestLabels("status ")
	if !status["="] || !status["!="] || status["~"] || status[">"] {
		t.Errorf("status operators = %v, want = and != only", status)
	}
	text := suggestLabels("summary ")
	if !text["~"] || !text["!~"] || text[">"] {
		t.Errorf("summary operators = %v, want ~ and !~ but not >", text)
	}
	date := suggestLabels("created ")
	if !date[">"] || !date[">="] || date["~"] {
		t.Errorf("created operators = %v, want comparisons", date)
	}
	if !status["IS"] || !status["IN"] || !status["NOT IN"] {
		t.Errorf("operator context should offer IS, IN and NOT IN: %v", status)
	}
}

func TestSuggestValuesOfferFunctionsForFieldType(t *testing.T) {
	dates := suggestLabels("created > ")
	if !dates["now()"] || !dates["startOfDay()"] || dates["currentUser()"] {
		t.Errorf("date values = %v", dates)
	}
	users := suggestLabels("assignee = ")
	if !users["currentUser()"] || !users["membersOf()"] || users["now()"] {
		t.Errorf("user values = %v", users)
	}
	sprints := suggestLabels("sprint = ")
	if !sprints["openSprints()"] || !sprints["closedSprints()"] {
		t.Errorf("sprint values = %v", sprints)
	}
}

func TestSuggestAfterValueOffersConnectives(t *testing.T) {
	got := suggestLabels("status = Done ")
	for _, want := range []string{"AND", "OR", "ORDER BY"} {
		if !got[want] {
			t.Errorf("after value: missing %q in %v", want, got)
		}
	}
}

func TestSuggestAfterConnectiveOffersFields(t *testing.T) {
	got := suggestLabels("status = Done AND ")
	if !got["priority"] || !got["NOT"] {
		t.Errorf("after AND: %v", got)
	}
}

func TestSuggestNotOffersIN(t *testing.T) {
	got := suggestLabels("status NOT")
	if !got["NOT IN"] || len(got) != 1 {
		t.Errorf("status NOT: %v, want only NOT IN", got)
	}
}

func TestSuggestIsOffersEmptyAndNull(t *testing.T) {
	got := suggestLabels("resolution IS ")
	if !got["EMPTY"] || !got["NULL"] || !got["NOT"] {
		t.Errorf("IS: %v", got)
	}
}

func TestSuggestAfterOrderByOffersSortableFields(t *testing.T) {
	got := suggestLabels("status = Done ORDER BY ")
	if !got["created"] || !got["priority"] || !got["summary"] {
		t.Errorf("ORDER BY: missing sortable fields in %v", got)
	}
	if got["labels"] || got["watcher"] || got["description"] {
		t.Errorf("ORDER BY must not offer non-sortable fields: %v", got)
	}
	dirs := suggestLabels("status = Done ORDER BY created ")
	if !dirs["ASC"] || !dirs["DESC"] || dirs["created"] {
		t.Errorf("direction suggestions = %v", dirs)
	}
}

func TestSuggestRespectsCursorInsideQuery(t *testing.T) {
	input := "status = Done AND priority = High"
	// Cursor right after the word AND (rune offset 17): the partial token is
	// AND, and the context before it is a complete value, so only AND matches.
	got := Suggest(input, 17, nil)
	if len(got) != 1 || got[0].Label != "AND" {
		t.Fatalf("cursor after AND = %v, want exactly AND", got)
	}
}

func TestSuggestWithUTF8Cursor(t *testing.T) {
	input := `summary ~ "canción" AND sta`
	// Cursor at the end, counted in runes.
	got := Suggest(input, RuneOffset(input, len(input)), nil)
	found := false
	for _, s := range got {
		if s.Label == "status" {
			found = true
		}
	}
	if !found {
		t.Fatalf("status not suggested after a UTF-8 string: %v", got)
	}
}

func TestSuggestInsideStringReturnsNothing(t *testing.T) {
	if got := Suggest(`summary = "sta`, RuneOffset(`summary = "sta`, len(`summary = "sta`)), nil); len(got) != 0 {
		t.Fatalf("inside an open string nothing should be suggested: %v", got)
	}
}

func TestSuggestCustomCatalog(t *testing.T) {
	cat := CompositeCatalog{
		Top:  NewMapCatalog(FieldDef{Name: "cf[Team]", Type: TypeEnum, Sortable: true}),
		Base: SystemCatalog(),
	}
	got := Suggest("cf", 2, cat)
	found := false
	for _, s := range got {
		if s.Label == "cf[Team]" && s.Kind == SuggestField {
			found = true
		}
	}
	if !found {
		t.Fatalf("custom field not suggested: %v", got)
	}
}
