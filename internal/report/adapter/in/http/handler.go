// Package http exposes the project reports: the catalog, each report as JSON and its table as CSV.
package http

import (
	"bytes"
	"encoding/csv"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/jhonsferg/corelog/internal/platform/apperror"
	"github.com/jhonsferg/corelog/internal/platform/httpresponse"
	appmw "github.com/jhonsferg/corelog/internal/platform/middleware"
	"github.com/jhonsferg/corelog/internal/report/application/port"
	"github.com/jhonsferg/corelog/internal/report/domain"
)

// cacheControl lets the browser reuse a report for a short while; reports are private to the caller.
const cacheControl = "private, max-age=30"

// Handler serves the report endpoints.
type Handler struct {
	service   port.Service
	jwtSecret string
}

// NewHandler builds the handler.
func NewHandler(service port.Service, jwtSecret string) *Handler {
	return &Handler{service: service, jwtSecret: jwtSecret}
}

// RegisterRoutes mounts the report routes below /projects/{project}/reports.
func (h *Handler) RegisterRoutes(r chi.Router) {
	r.Route("/projects/{project}/reports", func(r chi.Router) {
		r.Use(appmw.Authenticate(h.jwtSecret))
		r.Get("/", h.List)
		r.Get("/{kind}", h.Run)
	})
}

type descriptorDTO struct {
	Kind        string   `json:"kind"`
	Title       string   `json:"title"`
	Description string   `json:"description"`
	Params      []string `json:"params"`
}

type pointDTO struct {
	X string  `json:"x"`
	Y float64 `json:"y"`
}

type seriesDTO struct {
	Name   string     `json:"name"`
	Points []pointDTO `json:"points"`
}

type columnDTO struct {
	Key   string `json:"key"`
	Label string `json:"label"`
	Type  string `json:"type"`
}

type tableDTO struct {
	Columns []columnDTO `json:"columns"`
	Rows    [][]any     `json:"rows"`
}

type reportDTO struct {
	Kind        string         `json:"kind"`
	Title       string         `json:"title"`
	GeneratedAt string         `json:"generated_at"`
	Params      map[string]any `json:"params"`
	Series      []seriesDTO    `json:"series"`
	Table       tableDTO       `json:"table"`
	Summary     map[string]any `json:"summary"`
}

func toDTO(res *port.Result) reportDTO {
	rep := res.Report
	dto := reportDTO{Kind: string(rep.Kind), Title: rep.Title, GeneratedAt: res.GeneratedAt.UTC().Format(time.RFC3339),
		Params: res.Params, Series: make([]seriesDTO, 0, len(rep.Series)), Summary: rep.Summary,
		Table: tableDTO{Columns: make([]columnDTO, 0, len(rep.Table.Columns)), Rows: rep.Table.Rows}}
	for _, s := range rep.Series {
		points := make([]pointDTO, 0, len(s.Points))
		for _, p := range s.Points {
			points = append(points, pointDTO{X: p.X, Y: p.Y})
		}
		dto.Series = append(dto.Series, seriesDTO{Name: s.Name, Points: points})
	}
	for _, c := range rep.Table.Columns {
		dto.Table.Columns = append(dto.Table.Columns, columnDTO{Key: c.Key, Label: c.Label, Type: c.Type})
	}
	return dto
}

func actorAndProject(w http.ResponseWriter, r *http.Request) (uuid.UUID, string, uuid.UUID, bool) {
	actorID, ok := appmw.UserIDFromContext(r.Context())
	if !ok {
		httpresponse.Error(w, apperror.New(apperror.KindUnauthorized, "No autenticado"))
		return uuid.Nil, "", uuid.Nil, false
	}
	role, _ := appmw.RoleFromContext(r.Context())
	projectID, err := uuid.Parse(chi.URLParam(r, "project"))
	if err != nil {
		httpresponse.Error(w, apperror.WithCode(apperror.KindNotFound, "project.not_found", "El proyecto no existe"))
		return uuid.Nil, "", uuid.Nil, false
	}
	return actorID, role, projectID, true
}

// List answers GET /projects/{project}/reports.
func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	actorID, role, projectID, ok := actorAndProject(w, r)
	if !ok {
		return
	}
	list, err := h.service.List(r.Context(), actorID, role, projectID)
	if err != nil {
		httpresponse.Error(w, err)
		return
	}
	out := make([]descriptorDTO, 0, len(list))
	for _, d := range list {
		out = append(out, descriptorDTO{Kind: string(d.Kind), Title: d.Title, Description: d.Description, Params: d.Params})
	}
	w.Header().Set("Cache-Control", cacheControl)
	httpresponse.JSON(w, http.StatusOK, out)
}

func rawParams(r *http.Request) domain.RawParams {
	q := r.URL.Query()
	return domain.RawParams{From: q.Get("from"), To: q.Get("to"), GroupBy: q.Get("group_by"), Filter: q.Get("filter"),
		Sprint: q.Get("sprint"), Version: q.Get("version"), Interval: q.Get("interval"), Limit: q.Get("limit")}
}

// Run answers GET /projects/{project}/reports/{kind} and the CSV form {kind}.csv.
func (h *Handler) Run(w http.ResponseWriter, r *http.Request) {
	actorID, role, projectID, ok := actorAndProject(w, r)
	if !ok {
		return
	}
	kind := chi.URLParam(r, "kind")
	asCSV := strings.HasSuffix(kind, ".csv")
	kind = strings.TrimSuffix(kind, ".csv")
	res, err := h.service.Run(r.Context(), actorID, role, projectID, kind, rawParams(r))
	if err != nil {
		httpresponse.Error(w, err)
		return
	}
	w.Header().Set("Cache-Control", cacheControl)
	w.Header().Add("Vary", "Authorization")
	if !asCSV {
		httpresponse.JSON(w, http.StatusOK, toDTO(res))
		return
	}
	body, err := ToCSV(res.Report.Table)
	if err != nil {
		httpresponse.Error(w, err)
		return
	}
	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="informe-%s-%s.csv"`, res.Report.Kind, res.GeneratedAt.UTC().Format("2006-01-02")))
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(body)
}

// ToCSV renders the table with a UTF-8 byte order mark, the Spanish column labels as header and text
// cells that could be read as a formula neutralized with a leading apostrophe.
func ToCSV(t domain.Table) ([]byte, error) {
	var buf bytes.Buffer
	buf.WriteString("\xEF\xBB\xBF")
	w := csv.NewWriter(&buf)
	header := make([]string, len(t.Columns))
	for i, c := range t.Columns {
		header[i] = SafeCell(c.Label)
	}
	if err := w.Write(header); err != nil {
		return nil, err
	}
	for _, row := range t.Rows {
		cells := make([]string, len(row))
		for i, v := range row {
			cells[i] = cellText(v)
		}
		if err := w.Write(cells); err != nil {
			return nil, err
		}
	}
	w.Flush()
	return buf.Bytes(), w.Error()
}

func cellText(v any) string {
	switch x := v.(type) {
	case nil:
		return ""
	case string:
		return SafeCell(x)
	case float64:
		return strconv.FormatFloat(x, 'f', -1, 64)
	case int:
		return strconv.Itoa(x)
	case bool:
		if x {
			return "Sí"
		}
		return "No"
	default:
		return SafeCell(fmt.Sprint(x))
	}
}

// SafeCell prefixes an apostrophe to text that a spreadsheet would run as a formula.
func SafeCell(s string) string {
	if s == "" {
		return s
	}
	switch s[0] {
	case '=', '+', '-', '@', '\t', '\r', '\n':
		return "'" + s
	}
	return s
}
