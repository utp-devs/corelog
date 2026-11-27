package domain

import (
	"encoding/json"
	"testing"

	"github.com/google/uuid"
)

func ctxFor(project, issueType *uuid.UUID, required bool, def any) FieldContext {
	return FieldContext{ProjectID: project, IssueTypeID: issueType, Required: required, DefaultValue: def}
}

func TestBuildScopeResolvesContextsByProjectAndType(t *testing.T) {
	projectA, projectB := uuid.New(), uuid.New()
	taskType, bugType := uuid.New(), uuid.New()

	onlyA := &CustomField{ID: uuid.New(), Name: "solo A", Type: FieldText, Position: 1,
		Contexts: []FieldContext{ctxFor(&projectA, nil, false, nil)}}
	taskOnly := &CustomField{ID: uuid.New(), Name: "solo tarea", Type: FieldText, Position: 0,
		Contexts: []FieldContext{ctxFor(nil, &taskType, true, nil)}}
	orphan := &CustomField{ID: uuid.New(), Name: "sin contexto", Type: FieldText, Position: 2}
	allWithDefault := &CustomField{ID: uuid.New(), Name: "todo", Type: FieldNumber, Position: 3,
		Contexts: []FieldContext{
			ctxFor(nil, nil, false, 1.0),
			ctxFor(&projectA, &taskType, false, 7.0),
		}}

	scope := BuildScope([]*CustomField{onlyA, taskOnly, orphan, allWithDefault}, nil, projectA, taskType)
	if len(scope) != 3 {
		t.Fatalf("scope has %d fields, want 3 (orphan excluded): %+v", len(scope), scope)
	}
	if scope[0].Field != taskOnly || !scope[0].Required {
		t.Errorf("first field = %s required %v, want solo tarea required", scope[0].Field.Name, scope[0].Required)
	}
	if scope[2].Field != allWithDefault || scope[2].DefaultValue != 7.0 {
		t.Errorf("most specific context must give the default, got %v", scope[2].DefaultValue)
	}

	scope = BuildScope([]*CustomField{onlyA, taskOnly, allWithDefault}, nil, projectB, bugType)
	if len(scope) != 1 || scope[0].Field != allWithDefault || scope[0].DefaultValue != 1.0 {
		t.Errorf("project B bug scope = %+v, want only the global field with default 1", scope)
	}
}

func TestBuildScopeLayoutRequiredMakesFieldRequired(t *testing.T) {
	project, issueType := uuid.New(), uuid.New()
	field := &CustomField{ID: uuid.New(), Name: "a", Type: FieldText, Contexts: []FieldContext{ctxFor(nil, nil, false, nil)}}
	scope := BuildScope([]*CustomField{field}, map[uuid.UUID]bool{field.ID: true}, project, issueType)
	if len(scope) != 1 || !scope[0].Required {
		t.Fatalf("layout required flag must apply, got %+v", scope)
	}
}

func TestApplyCustomValuesRequiredAndDefaults(t *testing.T) {
	required := &CustomField{ID: uuid.New(), Name: "obligatorio", Type: FieldText}
	withDefault := &CustomField{ID: uuid.New(), Name: "con defecto", Type: FieldNumber}
	scope := []ScopedField{
		{Field: required, Required: true},
		{Field: withDefault, DefaultValue: 3.0},
	}
	key := required.ID.String()
	defKey := withDefault.ID.String()

	// Create without the required value fails, and reports it per field.
	res := ApplyCustomValues(CustomWrite{Scope: scope, Create: true, RequireAll: true})
	if len(res.Problems) != 1 || res.Problems[0].FieldID != key {
		t.Fatalf("problems = %+v, want the required field only", res.Problems)
	}

	// Create with the value applies the default to the field the input does not mention.
	res = ApplyCustomValues(CustomWrite{Scope: scope, Create: true, RequireAll: true,
		Input: map[string]json.RawMessage{key: json.RawMessage(`"x"`)}})
	if len(res.Problems) != 0 {
		t.Fatalf("unexpected problems %+v", res.Problems)
	}
	if res.Values[defKey] != 3.0 || res.Values[key] != "x" {
		t.Errorf("values = %+v, want default 3 and x", res.Values)
	}

	// Update that does not name the required field keeps a legacy issue editable.
	res = ApplyCustomValues(CustomWrite{Scope: scope, Current: map[string]any{}, Input: map[string]json.RawMessage{defKey: json.RawMessage(`4`)}})
	if len(res.Problems) != 0 {
		t.Errorf("legacy update must not check unnamed required fields, got %+v", res.Problems)
	}

	// Update that clears the required field fails.
	res = ApplyCustomValues(CustomWrite{Scope: scope, Current: map[string]any{key: "x"}, Input: map[string]json.RawMessage{key: json.RawMessage(`null`)}})
	if len(res.Problems) != 1 {
		t.Errorf("clearing a required field must fail, got %+v", res.Problems)
	}
}

func TestApplyCustomValuesRejectsOutOfScopeAndKeepsOthers(t *testing.T) {
	inScope := &CustomField{ID: uuid.New(), Name: "en alcance", Type: FieldCheckbox}
	scope := []ScopedField{{Field: inScope}}
	stranger := uuid.NewString()
	current := map[string]any{stranger: "previo", inScope.ID.String(): true}

	res := ApplyCustomValues(CustomWrite{Scope: scope, Current: current, Input: map[string]json.RawMessage{stranger: json.RawMessage(`"nuevo"`)}})
	if len(res.Problems) != 1 {
		t.Fatalf("out of scope value must be a problem, got %+v", res.Problems)
	}
	if res.Values[stranger] != "previo" {
		t.Errorf("rejected input must not change stored value, got %v", res.Values[stranger])
	}
	res = ApplyCustomValues(CustomWrite{Scope: scope, Current: current, Input: map[string]json.RawMessage{inScope.ID.String(): json.RawMessage(`false`)}})
	if len(res.Problems) != 0 || res.Values[stranger] != "previo" || res.Values[inScope.ID.String()] != false {
		t.Errorf("update must keep unrelated values, got %+v", res.Values)
	}
}

func TestPruneCustomValuesDropsOutOfScope(t *testing.T) {
	kept := &CustomField{ID: uuid.New(), Name: "k", Type: FieldText}
	out := map[string]any{kept.ID.String(): "a", uuid.NewString(): "b"}
	got := PruneCustomValues(out, []ScopedField{{Field: kept}})
	if len(got) != 1 || got[kept.ID.String()] != "a" {
		t.Errorf("pruned = %+v", got)
	}
}

func TestResolveLayoutOrdersStoredThenDefaults(t *testing.T) {
	a := &CustomField{ID: uuid.New(), Name: "a", Type: FieldText, Position: 0}
	b := &CustomField{ID: uuid.New(), Name: "b", Type: FieldText, Position: 1}
	scope := []ScopedField{{Field: a}, {Field: b, Required: true}}

	stored := []LayoutItem{
		{Kind: LayoutCustom, FieldID: b.ID},
		{Kind: LayoutSystem, Key: "description"},
		{Kind: LayoutCustom, FieldID: uuid.New()}, // not in scope: dropped
		{Kind: LayoutSystem, Key: "summary", Required: true},
	}
	items := ResolveLayout(stored, scope)

	if items[0].Kind != LayoutCustom || items[0].FieldID != b.ID || !items[0].Required {
		t.Errorf("first entry = %+v, want custom b first and required", items[0])
	}
	if items[1].Key != "description" || items[2].Key != "summary" {
		t.Errorf("stored system order not kept: %+v", items[:3])
	}
	// Missing system fields follow in default order, then the custom field not stored.
	var keys []string
	for _, item := range items[3:] {
		if item.Kind == LayoutSystem {
			keys = append(keys, item.Key)
		}
	}
	if len(keys) != len(SystemFieldKeys)-2 || keys[0] != "priority" {
		t.Errorf("missing system keys = %v", keys)
	}
	last := items[len(items)-1]
	if last.Kind != LayoutCustom || last.FieldID != a.ID {
		t.Errorf("unstored in-scope field must be appended, last = %+v", last)
	}
	if len(items) != len(SystemFieldKeys)+2 {
		t.Errorf("layout has %d entries, want %d", len(items), len(SystemFieldKeys)+2)
	}
}

func TestResolveLayoutWithoutStoredUsesDefaults(t *testing.T) {
	items := ResolveLayout(nil, nil)
	if len(items) != len(SystemFieldKeys) {
		t.Fatalf("default layout has %d entries", len(items))
	}
	for index, item := range items {
		if item.Key != SystemFieldKeys[index] {
			t.Errorf("entry %d = %s, want %s", index, item.Key, SystemFieldKeys[index])
		}
		if item.Required != (item.Key == "summary") {
			t.Errorf("required flag of %s = %v", item.Key, item.Required)
		}
	}
}

func TestValidateLayoutProblems(t *testing.T) {
	known := uuid.New()
	ok := ValidateLayout([]LayoutItem{{Kind: LayoutSystem, Key: "summary"}, {Kind: LayoutCustom, FieldID: known}}, map[uuid.UUID]bool{known: true})
	if len(ok) != 0 {
		t.Fatalf("valid layout reported problems: %+v", ok)
	}
	bad := ValidateLayout([]LayoutItem{
		{Kind: LayoutSystem, Key: "nope"},
		{Kind: LayoutSystem, Key: "summary"},
		{Kind: LayoutSystem, Key: "summary"},
		{Kind: LayoutCustom, FieldID: uuid.New()},
		{Kind: LayoutKind("other")},
	}, map[uuid.UUID]bool{known: true})
	if len(bad) != 4 {
		t.Errorf("problems = %d, want 4: %+v", len(bad), bad)
	}
}
