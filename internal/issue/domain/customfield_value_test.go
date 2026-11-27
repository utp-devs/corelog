package domain

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/google/uuid"
)

func fieldOfType(t FieldType, options ...FieldOption) *CustomField {
	return &CustomField{ID: uuid.New(), Name: "campo", Type: t, Options: options}
}

func TestNormalizeFieldValueMatrix(t *testing.T) {
	min, max := 1.0, 10.0
	number := &CustomField{ID: uuid.New(), Name: "n", Type: FieldNumber, NumberMin: &min, NumberMax: &max}
	sel := fieldOfType(FieldSelect, FieldOption{ID: "o1", Label: "Alta"}, FieldOption{ID: "o2", Label: "Vieja", Archived: true})
	multi := fieldOfType(FieldMultiSelect, FieldOption{ID: "o1", Label: "Alta"}, FieldOption{ID: "o2", Label: "Media"})
	userID := uuid.NewString()
	cases := []struct {
		name    string
		field   *CustomField
		raw     string
		want    any
		wantErr bool
	}{
		{"text trims", fieldOfType(FieldText), `"  hola  "`, "hola", false},
		{"text too long", fieldOfType(FieldText), `"` + strings.Repeat("a", 256) + `"`, nil, true},
		{"text wrong json type", fieldOfType(FieldText), `12`, nil, true},
		{"text empty clears", fieldOfType(FieldText), `"   "`, nil, false},
		{"paragraph ok", fieldOfType(FieldParagraph), `"parrafo"`, "parrafo", false},
		{"paragraph too long", fieldOfType(FieldParagraph), `"` + strings.Repeat("a", 20001) + `"`, nil, true},
		{"number ok", number, `5.5`, 5.5, false},
		{"number below min", number, `0`, nil, true},
		{"number above max", number, `11`, nil, true},
		{"number not a number", number, `"5"`, nil, true},
		{"date ok", fieldOfType(FieldDate), `"2026-11-27"`, "2026-11-27", false},
		{"date bad format", fieldOfType(FieldDate), `"27/11/2026"`, nil, true},
		{"date impossible day", fieldOfType(FieldDate), `"2026-02-30"`, nil, true},
		{"datetime converted to UTC", fieldOfType(FieldDateTime), `"2026-11-27T10:00:00+02:00"`, "2026-11-27T08:00:00Z", false},
		{"datetime bad", fieldOfType(FieldDateTime), `"2026-11-27 10:00"`, nil, true},
		{"select known", sel, `"o1"`, "o1", false},
		{"select archived", sel, `"o2"`, nil, true},
		{"select unknown", sel, `"zz"`, nil, true},
		{"multiselect set", multi, `["o1","o2"]`, []string{"o1", "o2"}, false},
		{"multiselect duplicate", multi, `["o1","o1"]`, nil, true},
		{"multiselect unknown option", multi, `["o1","zz"]`, nil, true},
		{"multiselect empty clears", multi, `[]`, nil, false},
		{"checkbox true", fieldOfType(FieldCheckbox), `true`, true, false},
		{"checkbox string", fieldOfType(FieldCheckbox), `"true"`, nil, true},
		{"user uuid", fieldOfType(FieldUser), `"` + userID + `"`, userID, false},
		{"user not uuid", fieldOfType(FieldUser), `"ana"`, nil, true},
		{"url https", fieldOfType(FieldURL), `"https://example.com/a"`, "https://example.com/a", false},
		{"url ftp rejected", fieldOfType(FieldURL), `"ftp://example.com"`, nil, true},
		{"url javascript rejected", fieldOfType(FieldURL), `"javascript:alert(1)"`, nil, true},
		{"url without host", fieldOfType(FieldURL), `"https://"`, nil, true},
		{"labels dedupe and trim", fieldOfType(FieldLabels), `[" a ","b","a"]`, []string{"a", "b"}, false},
		{"labels empty label", fieldOfType(FieldLabels), `["  "]`, nil, true},
		{"labels too long label", fieldOfType(FieldLabels), `["` + strings.Repeat("x", 41) + `"]`, nil, true},
		{"labels too many", fieldOfType(FieldLabels), labelsJSON(21), nil, true},
		{"null clears", fieldOfType(FieldText), `null`, nil, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := NormalizeFieldValue(tc.field, json.RawMessage(tc.raw))
			if (err != nil) != tc.wantErr {
				t.Fatalf("err = %v, wantErr %v", err, tc.wantErr)
			}
			if tc.wantErr {
				return
			}
			if !sameValue(got, tc.want) {
				t.Errorf("value = %#v, want %#v", got, tc.want)
			}
		})
	}
}

func labelsJSON(n int) string {
	items := make([]string, 0, n)
	for i := 0; i < n; i++ {
		items = append(items, `"l`+strings.Repeat("x", i%3)+string(rune('a'+i%26))+strings.Repeat("y", i/26)+`"`)
	}
	return "[" + strings.Join(items, ",") + "]"
}

// sameValue compares normalized values, which are strings, floats, bools or string slices.
func sameValue(got, want any) bool {
	a, _ := json.Marshal(got)
	b, _ := json.Marshal(want)
	return string(a) == string(b)
}

func TestValidateDefinition(t *testing.T) {
	min, max := 5.0, 1.0
	cases := []struct {
		name    string
		field   *CustomField
		wantErr bool
	}{
		{"select with options", &CustomField{Name: "x", Type: FieldSelect, Options: []FieldOption{{ID: "a", Label: "A"}}}, false},
		{"text with options", &CustomField{Name: "x", Type: FieldText, Options: []FieldOption{{ID: "a", Label: "A"}}}, true},
		{"duplicate label ignoring case", &CustomField{Name: "x", Type: FieldSelect, Options: []FieldOption{{ID: "a", Label: "A"}, {ID: "b", Label: "a"}}}, true},
		{"bad palette color", &CustomField{Name: "x", Type: FieldSelect, Options: []FieldOption{{ID: "a", Label: "A", Color: "magenta"}}}, true},
		{"number bounds inverted", &CustomField{Name: "x", Type: FieldNumber, NumberMin: &min, NumberMax: &max}, true},
		{"bounds on text", &CustomField{Name: "x", Type: FieldText, NumberMin: &min}, true},
		{"unknown type", &CustomField{Name: "x", Type: FieldType("color")}, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if err := ValidateDefinition(tc.field); (err != nil) != tc.wantErr {
				t.Fatalf("err = %v, wantErr %v", err, tc.wantErr)
			}
		})
	}
}

func TestMergeOptionsAssignsIDsAndReportsRemoved(t *testing.T) {
	existing := []FieldOption{{ID: "a", Label: "A"}, {ID: "b", Label: "B"}}
	next := 0
	newID := func() string { next++; return "new" + string(rune('0'+next)) }

	kept, removed, err := MergeOptions(existing, []OptionInput{{ID: "b", Label: "B2"}, {Label: "C"}}, newID)
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if len(kept) != 2 || kept[0].ID != "b" || kept[0].Label != "B2" || kept[1].ID != "new1" || kept[1].Position != 1 {
		t.Errorf("kept = %+v", kept)
	}
	if len(removed) != 1 || removed[0].ID != "a" {
		t.Errorf("removed = %+v, want option a", removed)
	}
	if _, _, err := MergeOptions(existing, []OptionInput{{ID: "zz", Label: "Z"}}, newID); err == nil {
		t.Error("unknown option id must be rejected")
	}
	if _, _, err := MergeOptions(existing, []OptionInput{{ID: "a", Label: "A"}, {ID: "a", Label: "A"}}, newID); err == nil {
		t.Error("repeated option id must be rejected")
	}
}
