// Package http exposes the CSV import and the project exports.
package http

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/jhonsferg/corelog/internal/importexport/application/port"
	"github.com/jhonsferg/corelog/internal/importexport/domain"
	"github.com/jhonsferg/corelog/internal/platform/apperror"
	"github.com/jhonsferg/corelog/internal/platform/httpresponse"
	appmw "github.com/jhonsferg/corelog/internal/platform/middleware"
)

// multipartOverhead is the room above the file cap for the multipart envelope.
const multipartOverhead = 1 << 20

// Handler serves the import and export routes of a project.
type Handler struct {
	service   port.Service
	jwtSecret string
}

// NewHandler returns the handler.
func NewHandler(service port.Service, jwtSecret string) *Handler {
	return &Handler{service: service, jwtSecret: jwtSecret}
}

// RegisterRoutes mounts the routes with their full paths.
func (h *Handler) RegisterRoutes(r chi.Router) {
	r.Route("/projects/{project}/import", func(r chi.Router) {
		r.Use(appmw.Authenticate(h.jwtSecret))
		r.Post("/csv", h.Upload)
		r.Post("/csv/{job}/start", h.Start)
		r.Get("/jobs", h.Jobs)
		r.Get("/jobs/{job}", h.Job)
		r.Get("/jobs/{job}/errors.csv", h.Errors)
	})
	r.Route("/projects/{project}/export", func(r chi.Router) {
		r.Use(appmw.Authenticate(h.jwtSecret))
		r.Get("/issues.csv", h.ExportIssues)
		r.Get("/project.json", h.ExportProject)
	})
}

var errProject = apperror.WithCode(apperror.KindNotFound, "project.not_found", "Proyecto no encontrado")

func caller(w http.ResponseWriter, r *http.Request) (uuid.UUID, string, uuid.UUID, bool) {
	actor, ok := appmw.UserIDFromContext(r.Context())
	if !ok {
		httpresponse.Error(w, apperror.New(apperror.KindUnauthorized, "No autenticado"))
		return uuid.Nil, "", uuid.Nil, false
	}
	role, _ := appmw.RoleFromContext(r.Context())
	project, err := uuid.Parse(chi.URLParam(r, "project"))
	if err != nil {
		httpresponse.Error(w, errProject)
		return uuid.Nil, "", uuid.Nil, false
	}
	return actor, role, project, true
}

type previewDTO struct {
	JobID            uuid.UUID         `json:"job_id"`
	Columns          []string          `json:"columns"`
	SampleRows       [][]string        `json:"sample_rows"`
	SuggestedMapping map[string]string `json:"suggested_mapping"`
	TotalRows        int               `json:"total_rows"`
	Delimiter        string            `json:"delimiter"`
}

type jobDTO struct {
	ID            uuid.UUID         `json:"id"`
	ProjectID     uuid.UUID         `json:"project_id"`
	UserID        uuid.UUID         `json:"user_id"`
	Source        string            `json:"source"`
	Status        string            `json:"status"`
	TotalRows     int               `json:"total_rows"`
	ProcessedRows int               `json:"processed_rows"`
	CreatedRows   int               `json:"created_rows"`
	ErrorRows     int               `json:"error_rows"`
	DryRun        bool              `json:"dry_run"`
	FileName      string            `json:"file_name"`
	Mapping       map[string]string `json:"mapping"`
	CreatedAt     time.Time         `json:"created_at"`
	FinishedAt    *time.Time        `json:"finished_at"`
}

func toJobDTO(j port.Job) jobDTO {
	if j.Mapping == nil {
		j.Mapping = map[string]string{}
	}
	return jobDTO{ID: j.ID, ProjectID: j.ProjectID, UserID: j.UserID, Source: j.Source, Status: j.Status, TotalRows: j.TotalRows,
		ProcessedRows: j.ProcessedRows, CreatedRows: j.CreatedRows, ErrorRows: j.ErrorRows, DryRun: j.DryRun, FileName: j.FileName,
		Mapping: j.Mapping, CreatedAt: j.CreatedAt, FinishedAt: j.FinishedAt}
}

// Upload receives the CSV (multipart field "file") and returns the preview.
func (h *Handler) Upload(w http.ResponseWriter, r *http.Request) {
	actor, role, project, ok := caller(w, r)
	if !ok {
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, domain.MaxFileBytes+multipartOverhead)
	file, header, err := r.FormFile("file")
	if err != nil {
		var tooBig *http.MaxBytesError
		if errorsAs(err, &tooBig) {
			httpresponse.Error(w, domain.ErrFileTooLarge)
			return
		}
		httpresponse.Error(w, apperror.WithCode(apperror.KindValidation, "import.file_required", "Adjunta un archivo CSV en el campo \"file\""))
		return
	}
	defer func() { _ = file.Close() }()
	data, err := io.ReadAll(io.LimitReader(file, domain.MaxFileBytes+1))
	if err != nil {
		httpresponse.Error(w, domain.ErrBadCSV)
		return
	}
	pv, err := h.service.Upload(r.Context(), actor, role, project, header.Filename, data)
	if err != nil {
		httpresponse.Error(w, err)
		return
	}
	httpresponse.JSON(w, http.StatusCreated, previewDTO{JobID: pv.JobID, Columns: pv.Columns, SampleRows: pv.SampleRows,
		SuggestedMapping: pv.SuggestedMapping, TotalRows: pv.TotalRows, Delimiter: pv.Delimiter})
}

type startRequest struct {
	Mapping map[string]string `json:"mapping"`
	DryRun  bool              `json:"dry_run"`
}

// Start fixes the mapping and queues the job.
func (h *Handler) Start(w http.ResponseWriter, r *http.Request) {
	actor, role, project, ok := caller(w, r)
	if !ok {
		return
	}
	jobID, err := uuid.Parse(chi.URLParam(r, "job"))
	if err != nil {
		httpresponse.Error(w, domain.ErrJobNotFound)
		return
	}
	var req startRequest
	if err := decodeStrict(r, &req); err != nil {
		httpresponse.Error(w, err)
		return
	}
	job, err := h.service.Start(r.Context(), actor, role, project, jobID, req.Mapping, req.DryRun)
	if err != nil {
		httpresponse.Error(w, err)
		return
	}
	httpresponse.JSON(w, http.StatusAccepted, toJobDTO(job))
}

// Jobs lists the import jobs of the project.
func (h *Handler) Jobs(w http.ResponseWriter, r *http.Request) {
	actor, role, project, ok := caller(w, r)
	if !ok {
		return
	}
	jobs, err := h.service.Jobs(r.Context(), actor, role, project)
	if err != nil {
		httpresponse.Error(w, err)
		return
	}
	out := make([]jobDTO, len(jobs))
	for i, j := range jobs {
		out[i] = toJobDTO(j)
	}
	httpresponse.JSON(w, http.StatusOK, out)
}

// Job returns one job with its progress and the first row errors.
func (h *Handler) Job(w http.ResponseWriter, r *http.Request) {
	actor, role, project, ok := caller(w, r)
	if !ok {
		return
	}
	jobID, err := uuid.Parse(chi.URLParam(r, "job"))
	if err != nil {
		httpresponse.Error(w, domain.ErrJobNotFound)
		return
	}
	job, err := h.service.GetJob(r.Context(), actor, role, project, jobID)
	if err != nil {
		httpresponse.Error(w, err)
		return
	}
	errs := job.Errors
	if len(errs) > 50 {
		errs = errs[:50]
	}
	if errs == nil {
		errs = []domain.RowError{}
	}
	httpresponse.JSON(w, http.StatusOK, struct {
		jobDTO
		Errors []domain.RowError `json:"errors"`
	}{toJobDTO(job), errs})
}

// Errors downloads the row error report as CSV.
func (h *Handler) Errors(w http.ResponseWriter, r *http.Request) {
	actor, role, project, ok := caller(w, r)
	if !ok {
		return
	}
	jobID, err := uuid.Parse(chi.URLParam(r, "job"))
	if err != nil {
		httpresponse.Error(w, domain.ErrJobNotFound)
		return
	}
	var buf bytes.Buffer
	if err := h.service.ErrorReport(r.Context(), actor, role, project, jobID, &buf); err != nil {
		httpresponse.Error(w, err)
		return
	}
	writeCSV(w, "errores-importacion.csv", buf.Bytes())
}

// ExportIssues downloads the issues of the project as CSV (?filter= CoreQL).
func (h *Handler) ExportIssues(w http.ResponseWriter, r *http.Request) {
	actor, role, project, ok := caller(w, r)
	if !ok {
		return
	}
	var buf bytes.Buffer
	key, err := h.service.ExportIssues(r.Context(), actor, role, project, r.URL.Query().Get("filter"), &buf)
	if err != nil {
		httpresponse.Error(w, err)
		return
	}
	writeCSV(w, key+"-incidencias.csv", buf.Bytes())
}

// ExportProject downloads the full backup of the project as JSON.
func (h *Handler) ExportProject(w http.ResponseWriter, r *http.Request) {
	actor, role, project, ok := caller(w, r)
	if !ok {
		return
	}
	dump, key, err := h.service.ExportProject(r.Context(), actor, role, project)
	if err != nil {
		httpresponse.Error(w, err)
		return
	}
	raw, err := json.Marshal(dump)
	if err != nil {
		httpresponse.Error(w, apperror.Wrap(apperror.KindInternal, "no se pudo codificar la copia", err))
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Content-Disposition", `attachment; filename="`+key+`-proyecto.json"`)
	w.Header().Set("Cache-Control", "no-store")
	_, _ = w.Write(raw)
}

func writeCSV(w http.ResponseWriter, name string, body []byte) {
	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", `attachment; filename="`+name+`"`)
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	_, _ = w.Write(body)
}

func decodeStrict(r *http.Request, into any) error {
	raw, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil {
		return apperror.WithCode(apperror.KindValidation, "validation.failed", "No se pudo leer el cuerpo de la peticion")
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(into); err != nil || dec.More() {
		return apperror.WithCode(apperror.KindValidation, "validation.failed", "El cuerpo de la peticion no es valido")
	}
	return nil
}

// errorsAs is errors.As without importing errors in the middle of the file.
func errorsAs(err error, target any) bool { return errors.As(err, target) }
