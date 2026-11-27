package domain

import (
	"fmt"
	"strings"
	"time"

	"github.com/jhonsferg/corelog/internal/platform/apperror"
)

// MaxExportRows bounds the rows of one CSV export.
const MaxExportRows = 10000

// ExportColumn is one column of the CSV export: its whitelisted key and its Spanish header.
type ExportColumn struct {
	Key    string
	Header string
}

// ExportColumns lists every column the export accepts, in the default order.
var ExportColumns = []ExportColumn{
	{Key: "key", Header: "Clave"},
	{Key: "project", Header: "Proyecto"},
	{Key: "summary", Header: "Resumen"},
	{Key: "description", Header: "Descripción"},
	{Key: "type", Header: "Tipo"},
	{Key: "status", Header: "Estado"},
	{Key: "priority", Header: "Prioridad"},
	{Key: "resolution", Header: "Resolución"},
	{Key: "assignee", Header: "Asignado a"},
	{Key: "reporter", Header: "Reportado por"},
	{Key: "created", Header: "Creado"},
	{Key: "updated", Header: "Actualizado"},
	{Key: "due_date", Header: "Fecha límite"},
	{Key: "start_date", Header: "Fecha de inicio"},
	{Key: "story_points", Header: "Puntos"},
	{Key: "labels", Header: "Etiquetas"},
	{Key: "votes", Header: "Votos"},
	{Key: "flagged", Header: "Marcada"},
}

// DefaultExportColumns is the column set of an export that names none.
var DefaultExportColumns = []string{"key", "summary", "type", "status", "priority", "assignee", "reporter", "created", "updated", "due_date"}

var ErrExportColumnInvalid = apperror.WithCode(apperror.KindValidation, "issue.export_column_invalid", "una columna de exportacion no existe o esta repetida")

// ParseExportColumns turns the columns parameter into column keys. An empty parameter selects the defaults.
func ParseExportColumns(raw string) ([]string, error) {
	if strings.TrimSpace(raw) == "" {
		return DefaultExportColumns, nil
	}
	known := make(map[string]bool, len(ExportColumns))
	for _, column := range ExportColumns {
		known[column.Key] = true
	}
	seen := make(map[string]bool)
	var out []string
	for _, part := range strings.Split(raw, ",") {
		key := strings.TrimSpace(part)
		if !known[key] || seen[key] {
			return nil, ErrExportColumnInvalid
		}
		seen[key] = true
		out = append(out, key)
	}
	return out, nil
}

// ExportHeader returns the Spanish header of a column key.
func ExportHeader(key string) string {
	for _, column := range ExportColumns {
		if column.Key == key {
			return column.Header
		}
	}
	return key
}

// SafeCSVCell protects a spreadsheet from formula injection: a value that starts like a formula gets a leading
// apostrophe, so it is read as text.
func SafeCSVCell(value string) string {
	if value == "" {
		return value
	}
	if strings.ContainsRune("=+-@\t\r", rune(value[0])) {
		return "'" + value
	}
	return value
}

// ExportFileName is the download name of an export made at now.
func ExportFileName(now time.Time) string {
	return fmt.Sprintf("incidencias-%s.csv", now.UTC().Format("2006-01-02"))
}
