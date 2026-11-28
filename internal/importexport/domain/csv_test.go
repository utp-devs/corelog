package domain

import (
	"bytes"
	"strings"
	"testing"
	"time"
)

func TestParseCSVDelimitersBOMAndQuotes(t *testing.T) {
	cases := []struct {
		name, in string
		delim    rune
		rows     int
		header   []string
	}{
		{"comma", "a,b\n1,2\n3,4\n", ',', 2, []string{"a", "b"}},
		{"semicolon with BOM", "\xEF\xBB\xBFa;b\n1;2\n", ';', 1, []string{"a", "b"}},
		{"tab", "a\tb\n1\t2\n", '\t', 1, []string{"a", "b"}},
		{"pipe", "a|b|c\n1|2|3\n", '|', 1, []string{"a", "b", "c"}},
		{"quoted delimiter and newline", "a,b\n\"x,1\",\"line1\nline2\"\n", ',', 1, []string{"a", "b"}},
		{"CRLF", "a,b\r\n1,2\r\n", ',', 1, []string{"a", "b"}},
		{"ragged rows and blank lines", "a,b,c\n1,2\n\n,,\n4,5,6,7\n", ',', 2, []string{"a", "b", "c"}},
		{"delimiter inside quoted header", "\"a;b\",c\n1,2\n", ',', 1, []string{"a;b", "c"}},
		{"control characters are removed", "a,b\x00\x07\n1,2\n", ',', 1, []string{"a", "b"}},
	}
	for _, c := range cases {
		table, err := ParseCSV([]byte(c.in))
		if err != nil {
			t.Errorf("%s: %v", c.name, err)
			continue
		}
		if table.Delimiter != c.delim || len(table.Rows) != c.rows || strings.Join(table.Header, "|") != strings.Join(c.header, "|") {
			t.Errorf("%s: delim %q rows %d header %v", c.name, table.Delimiter, len(table.Rows), table.Header)
		}
	}
	table, _ := ParseCSV([]byte("a,b\n\"x,1\",\"line1\nline2\"\n"))
	if table.Rows[0][0] != "x,1" || table.Rows[0][1] != "line1\nline2" {
		t.Fatalf("quoted cells: %q", table.Rows[0])
	}
}

func TestParseCSVRejections(t *testing.T) {
	wide := strings.Repeat("c,", MaxColumns) + "c\n1\n"
	many := "a\n" + strings.Repeat("x\n", MaxRows+1)
	cases := map[string]struct {
		in   []byte
		code string
	}{
		"invalid utf8":  {[]byte("a,b\n\xff\xfe,1\n"), "import.invalid_encoding"},
		"empty":         {nil, "import.empty_file"},
		"header only":   {[]byte("a,b\n"), "import.empty_file"},
		"only blanks":   {[]byte("a,b\n,\n  ,  \n"), "import.empty_file"},
		"too big":       {bytes.Repeat([]byte("x"), MaxFileBytes+1), "import.file_too_large"},
		"too many rows": {[]byte(many), "import.too_many_rows"},
		"too wide":      {[]byte(wide), "import.invalid_csv"},
	}
	for name, c := range cases {
		_, err := ParseCSV(c.in)
		if err == nil || !strings.Contains(err.Error(), "") {
			t.Errorf("%s: accepted", name)
			continue
		}
		if got := codeOf(err); got != c.code {
			t.Errorf("%s: code %q, want %q", name, got, c.code)
		}
	}
}

func codeOf(err error) string {
	type coder interface{ error }
	for _, e := range []error{ErrFileTooLarge, ErrNotUTF8, ErrEmptyFile, ErrTooManyRows, ErrBadCSV} {
		if err == e {
			switch e {
			case ErrFileTooLarge:
				return "import.file_too_large"
			case ErrNotUTF8:
				return "import.invalid_encoding"
			case ErrEmptyFile:
				return "import.empty_file"
			case ErrTooManyRows:
				return "import.too_many_rows"
			default:
				return "import.invalid_csv"
			}
		}
	}
	return ""
}

func FuzzParseCSV(f *testing.F) {
	for _, seed := range []string{"a,b\n1,2\n", "\xEF\xBB\xBFa;b\n\"x\ny\";2\n", "a\tb\n=1+1\t@x\n", "\"unterminated"} {
		f.Add([]byte(seed))
	}
	f.Fuzz(func(t *testing.T, data []byte) {
		table, err := ParseCSV(data)
		if err != nil {
			return
		}
		if len(table.Rows) == 0 || len(table.Header) == 0 {
			t.Fatalf("a parsed table needs a header and rows: %+v", table)
		}
	})
}

func TestSuggestMapping(t *testing.T) {
	jira := []string{"Summary", "Issue key", "Issue Type", "Status", "Priority", "Assignee", "Reporter", "Created", "Labels", "Labels",
		"Description", "Parent id", "Custom field (Story Points)", "Due Date", "Component/s", "Fix Version/s", "Sprint", "Severidad"}
	got := SuggestMapping(jira)
	want := map[string]string{
		FieldSummary: "Summary", FieldType: "Issue Type", FieldStatus: "Status", FieldPriority: "Priority", FieldAssignee: "Assignee",
		FieldReporter: "Reporter", FieldLabels: "Labels", FieldDescription: "Description", FieldParent: "Parent id",
		FieldStoryPoints: "Custom field (Story Points)", FieldDueDate: "Due Date", FieldComponents: "Component/s", FieldVersions: "Fix Version/s",
	}
	for k, v := range want {
		if got[k] != v {
			t.Errorf("%s = %q, want %q", k, got[k], v)
		}
	}
	if _, ok := got["sprint"]; ok {
		t.Error("unrecognized columns are not mapped")
	}
	spanish := SuggestMapping([]string{"Resumen", "Tipo de incidencia", "Estado", "Prioridad", "Responsable", "Etiquetas", "Fecha límite"})
	if spanish[FieldSummary] != "Resumen" || spanish[FieldType] != "Tipo de incidencia" || spanish[FieldDueDate] != "Fecha límite" || spanish[FieldLabels] != "Etiquetas" {
		t.Errorf("spanish headers: %v", spanish)
	}
	if len(SuggestMapping([]string{"foo", "bar"})) != 0 {
		t.Error("nothing to suggest")
	}
}

func TestValidateMapping(t *testing.T) {
	header := []string{"Summary", "Severidad"}
	if err := ValidateMapping(map[string]string{FieldSummary: "Summary", "custom:Severidad": "Severidad"}, header); err != nil {
		t.Fatal(err)
	}
	bad := map[string]map[string]string{
		"no summary":      {FieldDescription: "Summary"},
		"unknown column":  {FieldSummary: "Nope"},
		"unknown field":   {FieldSummary: "Summary", "password": "Summary"},
		"nameless custom": {FieldSummary: "Summary", "custom:": "Summary"},
	}
	for name, m := range bad {
		if err := ValidateMapping(m, header); err == nil {
			t.Errorf("%s accepted", name)
		}
	}
}

func TestParseRow(t *testing.T) {
	header := []string{"Resumen", "Desc", "Tipo", "Prio", "Estado", "Resp", "Inf", "Et", "Et", "Comp", "Ver", "Vence", "Inicio", "SP", "Padre", "Sev"}
	mapping := map[string]string{FieldSummary: "Resumen", FieldDescription: "Desc", FieldType: "Tipo", FieldPriority: "Prio", FieldStatus: "Estado",
		FieldAssignee: "Resp", FieldReporter: "Inf", FieldLabels: "Et", FieldComponents: "Comp", FieldVersions: "Ver", FieldDueDate: "Vence",
		FieldStartDate: "Inicio", FieldStoryPoints: "SP", FieldParent: "Padre", "custom:Severidad": "Sev"}
	rec := []string{"  =cmd|' /C calc'!A0 ", "texto\x00", "Bug", "High", "Done", " Ana@Example.test ", "bob@example.test", "a, b", "c", "Core; UI", "1.0",
		"2/Jan/26 3:04 PM", "2026-01-05", "3,5", "core-12", "Alta"}
	row, errs := ParseRow(7, header, rec, mapping)
	if len(errs) != 0 {
		t.Fatalf("errors %v", errs)
	}
	if row.Summary != "=cmd|' /C calc'!A0" || row.Description != "texto" || row.AssigneeEmail != "ana@example.test" || row.ParentKey != "CORE-12" {
		t.Fatalf("row %+v", row)
	}
	if strings.Join(row.Labels, ",") != "a,b,c" || strings.Join(row.Components, ",") != "Core,UI" || row.StoryPoints == nil || *row.StoryPoints != 3.5 {
		t.Fatalf("lists/points %+v", row)
	}
	if row.DueDate == nil || !row.DueDate.Equal(time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC)) || row.StartDate == nil || row.Custom["Severidad"] != "Alta" || row.Number != 7 {
		t.Fatalf("dates/custom %+v", row)
	}

	bad := [][]string{
		{"", "", "", "", "", "", "", "", "", "", "", "", "", "", "", ""},                       // no summary
		{strings.Repeat("x", 256), "", "", "", "", "", "", "", "", "", "", "", "", "", "", ""}, // long summary
		{"ok", "", "", "", "", "not-an-email", "", "", "", "", "", "", "", "", "", ""},
		{"ok", "", "", "", "", "", "", "", "", "", "", "31/31/2026", "", "", "", ""},
		{"ok", "", "", "", "", "", "", "", "", "", "", "", "", "abc", "", ""},
		{"ok", "", "", "", "", "", "", "", "", "", "", "", "", "-1", "", ""},
		{"ok", "", "", "", "", "", "", strings.Repeat("l,", 25), "", "", "", "", "", "", "", ""},
		{"ok", "", "", "", "", "", "", strings.Repeat("l", 51), "", "", "", "", "", "", "", ""},
	}
	for i, rec := range bad {
		if _, errs := ParseRow(i+1, header, rec, mapping); len(errs) == 0 {
			t.Errorf("bad row %d accepted", i)
		}
	}
	// Short records are tolerated.
	if row, errs := ParseRow(1, header, []string{"solo resumen"}, mapping); len(errs) != 0 || row.Summary != "solo resumen" {
		t.Errorf("short record: %v %+v", errs, row)
	}
	if _, ok := ParseDate("nope"); ok {
		t.Error("bad date parsed")
	}
}

func TestFormulaInjectionProtection(t *testing.T) {
	cases := map[string]string{
		"=1+1": "'=1+1", "+cmd": "'+cmd", "-2+3": "'-2+3", "@SUM(A1)": "'@SUM(A1)", "\tx": "'\tx", "\rx": "'\rx",
		"normal": "normal", "": "", "a=b": "a=b", "'=1": "'=1",
	}
	for in, want := range cases {
		if got := SafeCell(in); got != want {
			t.Errorf("SafeCell(%q) = %q, want %q", in, got, want)
		}
	}
	var buf bytes.Buffer
	if err := WriteCSV(&buf, []string{"=h"}, [][]string{{"=HYPERLINK(\"x\")"}, {"ok, with comma"}, {"line\nbreak"}}); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if !strings.HasPrefix(out, "\xEF\xBB\xBF") || strings.Contains(out, "\n=") || strings.Contains(out, ",=") || !strings.Contains(out, "'=HYPERLINK") {
		t.Fatalf("csv %q", out)
	}
	table, err := ParseCSV(buf.Bytes())
	if err != nil || len(table.Rows) != 3 || table.Rows[2][0] != "line\nbreak" {
		t.Fatalf("round trip: %v %+v", err, table)
	}
}

func TestSanitize(t *testing.T) {
	if got := Sanitize("a\x00b\x1bc​d\te\nf\r"); got != "abc​d\te\nf\r" {
		t.Fatalf("Sanitize = %q", got)
	}
	if got := Sanitize("=+-@ kept"); got != "=+-@ kept" {
		t.Fatalf("leading characters must be kept as text: %q", got)
	}
}
