package domain

import (
	"testing"
	"time"

	"github.com/jhonsferg/corelog/internal/platform/apperror"
)

func TestSafeCSVCellPrefixesFormulas(t *testing.T) {
	cases := map[string]string{
		"=SUM(A1)":    "'=SUM(A1)",
		"+1":          "'+1",
		"-2":          "'-2",
		"@cmd":        "'@cmd",
		"\tcell":      "'\tcell",
		"\rcell":      "'\rcell",
		"normal":      "normal",
		"":            "",
		"a=b":         "a=b",
		"Descripción": "Descripción",
	}
	for in, want := range cases {
		if got := SafeCSVCell(in); got != want {
			t.Errorf("SafeCSVCell(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestParseExportColumns(t *testing.T) {
	if cols, err := ParseExportColumns(""); err != nil || len(cols) != len(DefaultExportColumns) {
		t.Fatalf("default columns = %v, %v", cols, err)
	}
	cols, err := ParseExportColumns("key, summary,labels")
	if err != nil || len(cols) != 3 || cols[1] != "summary" {
		t.Fatalf("columns = %v, %v", cols, err)
	}
	for _, bad := range []string{"password", "key,key", "key,,summary", "key;drop"} {
		if _, err := ParseExportColumns(bad); apperror.CodeOf(err) != "issue.export_column_invalid" {
			t.Errorf("ParseExportColumns(%q) = %v, want invalid column", bad, err)
		}
	}
}

func TestExportHeadersAreSpanish(t *testing.T) {
	if got := ExportHeader("key"); got != "Clave" {
		t.Fatalf("header of key = %q", got)
	}
	if got := ExportHeader("due_date"); got != "Fecha límite" {
		t.Fatalf("header of due_date = %q", got)
	}
}

func TestExportFileNameUsesUTCDate(t *testing.T) {
	at := time.Date(2026, 11, 27, 23, 30, 0, 0, time.FixedZone("UTC-5", -5*3600))
	if got := ExportFileName(at); got != "incidencias-2026-11-28.csv" {
		t.Fatalf("file name = %q", got)
	}
}
