// Package domain holds the CSV import and export rules: parsing with BOM and delimiter detection, column
// mapping, row validation, sanitization on import and formula-injection protection on export.
package domain

import (
	"bytes"
	"encoding/csv"
	"errors"
	"io"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/jhonsferg/corelog/internal/platform/apperror"
)

// Limits.
const (
	MaxFileBytes    = 10 << 20
	MaxRows         = 20000
	MaxColumns      = 100
	PreviewRows     = 5
	MaxSummary      = 255
	MaxDescription  = 100000
	MaxListItems    = 20
	MaxListItemLen  = 50
	MaxErrorReports = 1000
)

// Import field names usable as mapping keys. Custom fields use "custom:<field name>".
const (
	FieldSummary     = "summary"
	FieldDescription = "description"
	FieldType        = "type"
	FieldPriority    = "priority"
	FieldStatus      = "status"
	FieldAssignee    = "assignee"
	FieldReporter    = "reporter"
	FieldLabels      = "labels"
	FieldComponents  = "components"
	FieldVersions    = "fix_versions"
	FieldDueDate     = "due_date"
	FieldStartDate   = "start_date"
	FieldStoryPoints = "story_points"
	FieldParent      = "parent"
	CustomPrefix     = "custom:"
)

// Job statuses.
const (
	StatusQueued     = "queued"
	StatusValidating = "validating"
	StatusRunning    = "running"
	StatusDone       = "done"
	StatusFailed     = "failed"
)

// Errors.
var (
	ErrFileTooLarge = apperror.WithCode(apperror.KindValidation, "import.file_too_large", "El archivo supera el maximo de 10 MiB")
	ErrNotUTF8      = apperror.WithCode(apperror.KindValidation, "import.invalid_encoding", "El archivo debe estar codificado en UTF-8")
	ErrEmptyFile    = apperror.WithCode(apperror.KindValidation, "import.empty_file", "El archivo no tiene filas de datos")
	ErrTooManyRows  = apperror.WithCode(apperror.KindValidation, "import.too_many_rows", "El archivo supera el maximo de 20000 filas")
	ErrBadCSV       = apperror.WithCode(apperror.KindValidation, "import.invalid_csv", "El archivo no es un CSV valido")
	ErrJobNotFound  = apperror.WithCode(apperror.KindNotFound, "import.job_not_found", "Importacion no encontrada")
	ErrAlreadyStart = apperror.WithCode(apperror.KindConflict, "import.already_started", "La importacion ya se inicio")
	ErrNoReport     = apperror.WithCode(apperror.KindNotFound, "import.no_report", "La importacion no tiene informe de errores")
)

func invalidMapping(msg string) error {
	return apperror.WithCode(apperror.KindValidation, "import.invalid_mapping", "Mapeo no valido: "+msg)
}

// Table is a parsed CSV: the header and the data rows.
type Table struct {
	Header    []string
	Rows      [][]string
	Delimiter rune
}

// ParseCSV decodes a CSV file: UTF-8 with or without BOM, delimiter detected among comma, semicolon, tab and
// pipe, quoted fields with embedded newlines, ragged rows tolerated. Control characters are removed from cells.
func ParseCSV(data []byte) (Table, error) {
	if len(data) > MaxFileBytes {
		return Table{}, ErrFileTooLarge
	}
	data = bytes.TrimPrefix(data, []byte{0xEF, 0xBB, 0xBF})
	if !utf8.Valid(data) {
		return Table{}, ErrNotUTF8
	}
	delim := DetectDelimiter(data)
	r := csv.NewReader(bytes.NewReader(data))
	r.Comma = delim
	r.FieldsPerRecord = -1
	r.LazyQuotes = true
	var records [][]string
	for {
		rec, err := r.Read()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return Table{}, ErrBadCSV
		}
		if len(rec) > MaxColumns {
			return Table{}, ErrBadCSV
		}
		if len(records) > MaxRows+1 {
			return Table{}, ErrTooManyRows
		}
		records = append(records, rec)
	}
	if len(records) < 2 {
		return Table{}, ErrEmptyFile
	}
	t := Table{Delimiter: delim}
	for _, h := range records[0] {
		t.Header = append(t.Header, strings.TrimSpace(Sanitize(h)))
	}
	for _, rec := range records[1:] {
		if blank(rec) {
			continue
		}
		t.Rows = append(t.Rows, rec)
	}
	if len(t.Rows) == 0 {
		return Table{}, ErrEmptyFile
	}
	if len(t.Rows) > MaxRows {
		return Table{}, ErrTooManyRows
	}
	return t, nil
}

func blank(rec []string) bool {
	for _, c := range rec {
		if strings.TrimSpace(c) != "" {
			return false
		}
	}
	return true
}

// DetectDelimiter picks the delimiter that appears most in the first line outside quotes.
func DetectDelimiter(data []byte) rune {
	counts := map[rune]int{}
	inQuotes := false
	for _, b := range data {
		if b == '"' {
			inQuotes = !inQuotes
			continue
		}
		if !inQuotes && (b == '\n' || b == '\r') {
			break
		}
		if !inQuotes {
			switch b {
			case ',', ';', '\t', '|':
				counts[rune(b)]++
			}
		}
	}
	best, bestN := ',', 0
	for _, d := range []rune{',', ';', '\t', '|'} {
		if counts[d] > bestN {
			best, bestN = d, counts[d]
		}
	}
	return best
}

// Sanitize removes control characters (keeping tab, newline and carriage return) and NUL. A leading =, +, - or @
// is kept: imported values are plain text and never evaluated.
func Sanitize(s string) string {
	return strings.Map(func(r rune) rune {
		if r == '\t' || r == '\n' || r == '\r' {
			return r
		}
		if unicode.IsControl(r) || r == utf8.RuneError {
			return -1
		}
		return r
	}, s)
}

// headerAliases lists the header names recognised for each field, lowercased. It covers Jira CSV exports.
var headerAliases = map[string][]string{
	FieldSummary:     {"summary", "resumen", "titulo", "título", "title"},
	FieldDescription: {"description", "descripcion", "descripción"},
	FieldType:        {"issue type", "issuetype", "type", "tipo", "tipo de incidencia"},
	FieldPriority:    {"priority", "prioridad"},
	FieldStatus:      {"status", "estado"},
	FieldAssignee:    {"assignee", "assignee email", "responsable", "asignado", "asignado a"},
	FieldReporter:    {"reporter", "reporter email", "informador", "creador"},
	FieldLabels:      {"labels", "label", "etiquetas"},
	FieldComponents:  {"component/s", "components", "component", "componentes"},
	FieldVersions:    {"fix version/s", "fix versions", "fix version", "versiones"},
	FieldDueDate:     {"due date", "due", "fecha limite", "fecha límite", "vencimiento"},
	FieldStartDate:   {"start date", "fecha de inicio", "inicio"},
	FieldStoryPoints: {"story points", "story point estimate", "custom field (story points)", "puntos", "puntos de historia"},
	FieldParent:      {"parent", "parent id", "parent key", "padre", "incidencia padre"},
}

// SuggestMapping maps recognised headers to fields. Each field gets the first header that matches.
func SuggestMapping(header []string) map[string]string {
	out := map[string]string{}
	for field, aliases := range headerAliases {
	next:
		for _, h := range header {
			low := strings.ToLower(strings.TrimSpace(h))
			for _, a := range aliases {
				if low == a {
					out[field] = h
					break next
				}
			}
		}
	}
	return out
}

// ValidateMapping checks that the mapping targets known fields and existing columns, and maps the summary.
func ValidateMapping(m map[string]string, header []string) error {
	cols := map[string]bool{}
	for _, h := range header {
		cols[h] = true
	}
	if m[FieldSummary] == "" {
		return invalidMapping("la columna de resumen es obligatoria")
	}
	for field, col := range m {
		if _, ok := headerAliases[field]; !ok && !strings.HasPrefix(field, CustomPrefix) {
			return invalidMapping("campo desconocido: " + field)
		}
		if strings.HasPrefix(field, CustomPrefix) && len(field) == len(CustomPrefix) {
			return invalidMapping("campo personalizado sin nombre")
		}
		if col != "" && !cols[col] {
			return invalidMapping("la columna no existe: " + col)
		}
	}
	return nil
}

// Row is one validated import row.
type Row struct {
	Number        int
	Summary       string
	Description   string
	Type          string
	Priority      string
	Status        string
	AssigneeEmail string
	ReporterEmail string
	Labels        []string
	Components    []string
	FixVersions   []string
	DueDate       *time.Time
	StartDate     *time.Time
	StoryPoints   *float64
	ParentKey     string
	Custom        map[string]string
}

// RowError is the validation outcome of a failed row.
type RowError struct {
	Row    int      `json:"row"`
	Errors []string `json:"errors"`
}

func indexes(header []string, name string) []int {
	var out []int
	for i, h := range header {
		if h == name {
			out = append(out, i)
		}
	}
	return out
}

func cell(rec []string, idx []int) string {
	for _, i := range idx {
		if i < len(rec) {
			if v := strings.TrimSpace(Sanitize(rec[i])); v != "" {
				return v
			}
		}
	}
	return ""
}

func cells(rec []string, idx []int) []string {
	var out []string
	for _, i := range idx {
		if i < len(rec) {
			if v := strings.TrimSpace(Sanitize(rec[i])); v != "" {
				out = append(out, v)
			}
		}
	}
	return out
}

var dateLayouts = []string{
	"2006-01-02", time.RFC3339, "2006-01-02 15:04:05", "02/01/2006", "2/1/2006", "02/01/2006 15:04",
	"2/Jan/06 3:04 PM", "02/Jan/06 3:04 PM", "2/Jan/06 15:04", "02/Jan/06", "2/Jan/06", "02/Jan/2006", "2/Jan/2006",
}

// ParseDate reads the date formats of Jira and ISO exports; the time of day is dropped.
func ParseDate(s string) (time.Time, bool) {
	s = strings.TrimSpace(s)
	for _, layout := range dateLayouts {
		if t, err := time.Parse(layout, s); err == nil {
			return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC), true
		}
	}
	return time.Time{}, false
}

// splitList splits a label-like cell on commas, semicolons and spaces-free separators.
func splitList(values []string) []string {
	var out []string
	for _, v := range values {
		for _, p := range strings.FieldsFunc(v, func(r rune) bool { return r == ',' || r == ';' }) {
			if p = strings.TrimSpace(p); p != "" {
				out = append(out, p)
			}
		}
	}
	return out
}

// ParseRow applies the mapping to one record and validates it. number is the 1-based data row number (the
// header is row 0), used in the error report.
func ParseRow(number int, header []string, rec []string, mapping map[string]string) (Row, []string) {
	var errs []string
	row := Row{Number: number, Custom: map[string]string{}}
	get := func(field string) string {
		if col := mapping[field]; col != "" {
			return cell(rec, indexes(header, col))
		}
		return ""
	}
	list := func(field string) []string {
		if col := mapping[field]; col != "" {
			return splitList(cells(rec, indexes(header, col)))
		}
		return nil
	}
	row.Summary = get(FieldSummary)
	switch {
	case row.Summary == "":
		errs = append(errs, "el resumen es obligatorio")
	case utf8.RuneCountInString(row.Summary) > MaxSummary:
		errs = append(errs, "el resumen supera 255 caracteres")
	}
	row.Description = get(FieldDescription)
	if utf8.RuneCountInString(row.Description) > MaxDescription {
		errs = append(errs, "la descripcion es demasiado larga")
	}
	row.Type, row.Priority, row.Status = get(FieldType), get(FieldPriority), get(FieldStatus)
	row.AssigneeEmail, row.ReporterEmail = strings.ToLower(get(FieldAssignee)), strings.ToLower(get(FieldReporter))
	for field, email := range map[string]string{"responsable": row.AssigneeEmail, "informador": row.ReporterEmail} {
		if email != "" && !looksLikeEmail(email) {
			errs = append(errs, field+": se esperaba un correo electronico")
		}
	}
	for name, dst := range map[string]*[]string{FieldLabels: &row.Labels, FieldComponents: &row.Components, FieldVersions: &row.FixVersions} {
		items := list(name)
		if len(items) > MaxListItems {
			errs = append(errs, name+": maximo 20 elementos")
			items = items[:MaxListItems]
		}
		for _, it := range items {
			if utf8.RuneCountInString(it) > MaxListItemLen {
				errs = append(errs, name+": un elemento supera 50 caracteres")
			}
		}
		*dst = items
	}
	for field, dst := range map[string]**time.Time{FieldDueDate: &row.DueDate, FieldStartDate: &row.StartDate} {
		if v := get(field); v != "" {
			if t, ok := ParseDate(v); ok {
				*dst = &t
			} else {
				errs = append(errs, field+": fecha no valida")
			}
		}
	}
	if v := get(FieldStoryPoints); v != "" {
		if p, err := strconv.ParseFloat(strings.ReplaceAll(v, ",", "."), 64); err == nil && p >= 0 && p <= 1000 {
			row.StoryPoints = &p
		} else {
			errs = append(errs, "story_points: numero no valido")
		}
	}
	row.ParentKey = strings.ToUpper(get(FieldParent))
	for field, col := range mapping {
		if strings.HasPrefix(field, CustomPrefix) && col != "" {
			if v := cell(rec, indexes(header, col)); v != "" {
				row.Custom[strings.TrimPrefix(field, CustomPrefix)] = v
			}
		}
	}
	return row, errs
}

func looksLikeEmail(s string) bool {
	at := strings.IndexByte(s, '@')
	return at > 0 && at < len(s)-3 && strings.Contains(s[at:], ".") && !strings.ContainsAny(s, " ,;")
}

// SafeCell protects an exported value against formula injection: a leading =, +, -, @, tab or carriage return
// gets a single quote prefix so spreadsheets read it as text.
func SafeCell(s string) string {
	if s == "" {
		return s
	}
	switch s[0] {
	case '=', '+', '-', '@', '\t', '\r':
		return "'" + s
	}
	return s
}

// WriteCSV writes a header and rows with every cell protected by SafeCell. The file starts with a BOM so
// spreadsheets detect UTF-8.
func WriteCSV(w io.Writer, header []string, rows [][]string) error {
	if _, err := w.Write([]byte{0xEF, 0xBB, 0xBF}); err != nil {
		return err
	}
	cw := csv.NewWriter(w)
	safe := func(cols []string) []string {
		out := make([]string, len(cols))
		for i, c := range cols {
			out[i] = SafeCell(c)
		}
		return out
	}
	if err := cw.Write(safe(header)); err != nil {
		return err
	}
	for _, r := range rows {
		if err := cw.Write(safe(r)); err != nil {
			return err
		}
	}
	cw.Flush()
	return cw.Error()
}
