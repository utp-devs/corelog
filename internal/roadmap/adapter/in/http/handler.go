// Package http exposes the roadmap routes.
package http

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/jhonsferg/corelog/internal/platform/apperror"
	"github.com/jhonsferg/corelog/internal/platform/httpresponse"
	appmw "github.com/jhonsferg/corelog/internal/platform/middleware"
	"github.com/jhonsferg/corelog/internal/roadmap/application/port"
	"github.com/jhonsferg/corelog/internal/roadmap/domain"
)

const dateLayout = "2006-01-02"

// Handler serves the roadmap of a project.
type Handler struct {
	service   port.Service
	jwtSecret string
}

// NewHandler builds the handler.
func NewHandler(service port.Service, jwtSecret string) *Handler {
	return &Handler{service: service, jwtSecret: jwtSecret}
}

// RegisterRoutes mounts the roadmap routes. Authentication is the only middleware; authorization is in the use case.
func (h *Handler) RegisterRoutes(r chi.Router) {
	r.Route("/projects/{id}/roadmap", func(r chi.Router) {
		r.Use(appmw.Authenticate(h.jwtSecret))
		r.Get("/", h.Get)
		r.Get("/dependencies", h.Dependencies)
		r.Put("/rank", h.Rank)
	})
}

type progressResponse struct {
	Total       int     `json:"total"`
	Todo        int     `json:"todo"`
	InProgress  int     `json:"in_progress"`
	Done        int     `json:"done"`
	Percent     int     `json:"percent"`
	PointsTotal float64 `json:"points_total"`
	PointsDone  float64 `json:"points_done"`
}

type barResponse struct {
	ID             string           `json:"id"`
	Kind           string           `json:"kind"`
	Key            string           `json:"key,omitempty"`
	Title          string           `json:"title"`
	Status         string           `json:"status"`
	StatusCategory string           `json:"status_category,omitempty"`
	Assignee       string           `json:"assignee,omitempty"`
	Start          *string          `json:"start"`
	End            *string          `json:"end"`
	StartSource    domain.Source    `json:"start_source"`
	EndSource      domain.Source    `json:"end_source"`
	Dated          bool             `json:"dated"`
	Complete       bool             `json:"complete"`
	Progress       progressResponse `json:"progress"`
}

type dependencyResponse struct {
	From string `json:"from"`
	To   string `json:"to"`
	Kind string `json:"kind"`
}

type roadmapResponse struct {
	From         *string              `json:"from"`
	To           *string              `json:"to"`
	GroupBy      string               `json:"group_by"`
	Bars         []barResponse        `json:"bars"`
	Versions     []barResponse        `json:"versions"`
	Sprints      []barResponse        `json:"sprints"`
	Dependencies []dependencyResponse `json:"dependencies"`
}

func date(t *time.Time) *string {
	if t == nil {
		return nil
	}
	s := t.Format(dateLayout)
	return &s
}

func toBars(bars []domain.Bar) []barResponse {
	out := make([]barResponse, len(bars))
	for i, b := range bars {
		out[i] = barResponse{
			ID: b.ID.String(), Kind: b.Kind, Key: b.Key, Title: b.Title, Status: b.Status, StatusCategory: b.StatusCategory, Assignee: b.Assignee,
			Start: date(b.Start), End: date(b.End), StartSource: b.StartSource, EndSource: b.EndSource, Dated: b.Dated, Complete: b.Complete,
			Progress: progressResponse{Total: b.Progress.Total, Todo: b.Progress.Todo, InProgress: b.Progress.InProgress, Done: b.Progress.Done,
				Percent: b.Progress.Percent, PointsTotal: b.Progress.PointsTotal, PointsDone: b.Progress.PointsDone},
		}
	}
	return out
}

func toDeps(deps []domain.Dependency) []dependencyResponse {
	out := make([]dependencyResponse, len(deps))
	for i, d := range deps {
		out[i] = dependencyResponse{From: d.From.String(), To: d.To.String(), Kind: d.Kind}
	}
	return out
}

func actor(w http.ResponseWriter, r *http.Request) (uuid.UUID, string, uuid.UUID, bool) {
	id, ok := appmw.UserIDFromContext(r.Context())
	if !ok {
		httpresponse.Error(w, apperror.New(apperror.KindUnauthorized, "not authenticated"))
		return uuid.Nil, "", uuid.Nil, false
	}
	role, _ := appmw.RoleFromContext(r.Context())
	project, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpresponse.Error(w, apperror.WithCode(apperror.KindNotFound, "project.not_found", "proyecto no encontrado"))
		return uuid.Nil, "", uuid.Nil, false
	}
	return id, role, project, true
}

func parseDate(value, name string) (*time.Time, error) {
	if value == "" {
		return nil, nil
	}
	t, err := time.Parse(dateLayout, value)
	if err != nil {
		return nil, apperror.WithCode(apperror.KindValidation, domain.CodeInvalidRange, name+" debe usar el formato AAAA-MM-DD")
	}
	return &t, nil
}

// Get returns the roadmap: from, to (AAAA-MM-DD, inclusive), group_by (epic, version, sprint), filter (all, open,
// done) and q (text in key or title).
func (h *Handler) Get(w http.ResponseWriter, r *http.Request) {
	actorID, role, project, ok := actor(w, r)
	if !ok {
		return
	}
	query := r.URL.Query()
	from, err := parseDate(query.Get("from"), "from")
	if err != nil {
		httpresponse.Error(w, err)
		return
	}
	to, err := parseDate(query.Get("to"), "to")
	if err != nil {
		httpresponse.Error(w, err)
		return
	}
	group, err := domain.ParseGroup(query.Get("group_by"))
	if err != nil {
		httpresponse.Error(w, err)
		return
	}
	filter, err := domain.ParseFilter(query.Get("filter"))
	if err != nil {
		httpresponse.Error(w, err)
		return
	}
	text := strings.TrimSpace(query.Get("q"))
	if len([]rune(text)) > 200 {
		httpresponse.Error(w, apperror.WithCode(apperror.KindValidation, domain.CodeInvalidQuery, "q admite como máximo 200 caracteres"))
		return
	}
	result, err := h.service.Roadmap(r.Context(), actorID, role, project, domain.Query{From: from, To: to, Group: group, Filter: filter, Text: text})
	if err != nil {
		httpresponse.Error(w, err)
		return
	}
	httpresponse.JSON(w, http.StatusOK, roadmapResponse{
		From: date(result.From), To: date(result.To), GroupBy: string(result.Group), Bars: toBars(result.Bars),
		Versions: toBars(result.Versions), Sprints: toBars(result.Sprints), Dependencies: toDeps(result.Dependencies),
	})
}

// Dependencies lists the blocks links between epics of the project.
func (h *Handler) Dependencies(w http.ResponseWriter, r *http.Request) {
	actorID, role, project, ok := actor(w, r)
	if !ok {
		return
	}
	deps, err := h.service.Dependencies(r.Context(), actorID, role, project)
	if err != nil {
		httpresponse.Error(w, err)
		return
	}
	httpresponse.JSON(w, http.StatusOK, toDeps(deps))
}

type rankRequest struct {
	EpicIDs *[]string `json:"epic_ids"`
}

// Rank stores the manual order of the epics.
func (h *Handler) Rank(w http.ResponseWriter, r *http.Request) {
	actorID, role, project, ok := actor(w, r)
	if !ok {
		return
	}
	var req rankRequest
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, 256<<10))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&req); err != nil || req.EpicIDs == nil {
		httpresponse.Error(w, apperror.WithCode(apperror.KindValidation, "validation.failed", "el cuerpo de la petición no es válido: epic_ids es obligatorio"))
		return
	}
	if _, err := dec.Token(); err != io.EOF {
		httpresponse.Error(w, apperror.WithCode(apperror.KindValidation, "validation.failed", "el cuerpo de la petición no es válido"))
		return
	}
	ids := make([]uuid.UUID, 0, len(*req.EpicIDs))
	for _, raw := range *req.EpicIDs {
		id, err := uuid.Parse(raw)
		if err != nil {
			httpresponse.Error(w, apperror.WithCode(apperror.KindValidation, domain.CodeInvalidRank, "epic_ids contiene un id no válido"))
			return
		}
		ids = append(ids, id)
	}
	if err := h.service.Rank(r.Context(), actorID, role, project, ids); err != nil {
		httpresponse.Error(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
