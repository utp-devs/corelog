// Package http exposes the worklog module: worklogs, issue estimates, the personal timesheet and the project
// time summary.
package http

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/jhonsferg/corelog/internal/platform/apperror"
	"github.com/jhonsferg/corelog/internal/platform/httpresponse"
	appmw "github.com/jhonsferg/corelog/internal/platform/middleware"
	"github.com/jhonsferg/corelog/internal/worklog/application/port"
	"github.com/jhonsferg/corelog/internal/worklog/domain"
)

const dateLayout = "2006-01-02"

// Handler serves the worklog routes.
type Handler struct {
	service   port.Service
	jwtSecret string
}

// NewHandler returns the handler; every route requires a valid token.
func NewHandler(service port.Service, jwtSecret string) *Handler {
	return &Handler{service: service, jwtSecret: jwtSecret}
}

// RegisterRoutes mounts the routes on r.
func (h *Handler) RegisterRoutes(r chi.Router) {
	auth := appmw.Authenticate(h.jwtSecret)
	r.Route("/issues/{issue}/worklogs", func(r chi.Router) {
		r.Use(auth)
		r.Get("/", h.List)
		r.Post("/", h.Log)
	})
	r.Route("/issues/{issue}/estimates", func(r chi.Router) {
		r.Use(auth)
		r.Put("/", h.UpdateEstimates)
	})
	r.Route("/worklogs/{id}", func(r chi.Router) {
		r.Use(auth)
		r.Patch("/", h.Update)
		r.Delete("/", h.Delete)
	})
	r.Route("/me/timesheet", func(r chi.Router) {
		r.Use(auth)
		r.Get("/", h.Timesheet)
	})
	r.Route("/projects/{project}/time-summary", func(r chi.Router) {
		r.Use(auth)
		r.Get("/", h.Summary)
	})
}

// requestActor returns the caller id and platform role, or writes 401.
func requestActor(w http.ResponseWriter, r *http.Request) (uuid.UUID, string, bool) {
	actorID, ok := appmw.UserIDFromContext(r.Context())
	if !ok {
		httpresponse.Error(w, apperror.New(apperror.KindUnauthorized, "No autenticado"))
		return uuid.Nil, "", false
	}
	globalRole, _ := appmw.RoleFromContext(r.Context())
	return actorID, globalRole, true
}

// pathID parses a path id. Unparseable ids are reported as not found, like missing resources.
func pathID(w http.ResponseWriter, r *http.Request, name string, notFound error) (uuid.UUID, bool) {
	id, err := uuid.Parse(chi.URLParam(r, name))
	if err != nil {
		httpresponse.Error(w, notFound)
		return uuid.Nil, false
	}
	return id, true
}

// decodeStrict reads a JSON body with unknown fields rejected. An empty body is accepted when optional.
func decodeStrict(r *http.Request, into any, optional bool) error {
	raw, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil {
		return apperror.WithCode(apperror.KindValidation, "validation.failed", "No se pudo leer el cuerpo de la peticion")
	}
	if optional && len(bytes.TrimSpace(raw)) == 0 {
		return nil
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(into); err != nil {
		return apperror.WithCode(apperror.KindValidation, "validation.failed", "El cuerpo de la peticion no es valido")
	}
	if dec.More() {
		return apperror.WithCode(apperror.KindValidation, "validation.failed", "El cuerpo de la peticion no es valido")
	}
	return nil
}

// Request bodies.

type remainingRequest struct {
	Mode         string  `json:"mode"`
	Value        *string `json:"value"`
	ValueSeconds *int64  `json:"value_seconds"`
}

type logRequest struct {
	TimeSpent        *string           `json:"time_spent"`
	TimeSpentSeconds *int64            `json:"time_spent_seconds"`
	StartedAt        *time.Time        `json:"started_at"`
	Description      string            `json:"description"`
	Remaining        *remainingRequest `json:"remaining"`
}

type updateRequest struct {
	TimeSpent        *string    `json:"time_spent"`
	TimeSpentSeconds *int64     `json:"time_spent_seconds"`
	StartedAt        *time.Time `json:"started_at"`
	Description      *string    `json:"description"`
}

// estimateField is one estimate in the request. Absent keys stay nil; null clears the value.
type estimateRequest struct {
	Original  json.RawMessage `json:"original"`
	Remaining json.RawMessage `json:"remaining"`
}

func toDuration(raw json.RawMessage) (*port.Duration, error) {
	if raw == nil {
		return nil, nil
	}
	if string(raw) == "null" {
		return &port.Duration{Clear: true}, nil
	}
	var text string
	if err := json.Unmarshal(raw, &text); err != nil {
		return nil, apperror.WithCode(apperror.KindValidation, domain.CodeDurationInvalid, "La duracion debe ser texto, por ejemplo 1d 2h, o null")
	}
	return &port.Duration{Text: &text}, nil
}

// Response bodies.

type worklogResponse struct {
	ID                     string `json:"id"`
	IssueID                string `json:"issue_id"`
	IssueKey               string `json:"issue_key"`
	AuthorID               string `json:"author_id"`
	AuthorName             string `json:"author_name"`
	StartedAt              string `json:"started_at"`
	TimeSpentSeconds       int64  `json:"time_spent_seconds"`
	TimeSpent              string `json:"time_spent"`
	Description            string `json:"description"`
	RemainingMode          string `json:"remaining_mode"`
	RemainingAdjustSeconds int64  `json:"remaining_adjust_seconds"`
	CreatedAt              string `json:"created_at"`
	UpdatedAt              string `json:"updated_at"`
}

type totalsResponse struct {
	OriginalEstimateSeconds  *int64  `json:"original_estimate_seconds"`
	OriginalEstimate         *string `json:"original_estimate"`
	RemainingEstimateSeconds *int64  `json:"remaining_estimate_seconds"`
	RemainingEstimate        *string `json:"remaining_estimate"`
	TimeSpentSeconds         int64   `json:"time_spent_seconds"`
	TimeSpent                string  `json:"time_spent"`
}

func toTotals(t port.Totals, cfg domain.Config) totalsResponse {
	out := totalsResponse{
		OriginalEstimateSeconds:  t.OriginalSeconds,
		RemainingEstimateSeconds: t.RemainingSeconds,
		TimeSpentSeconds:         t.SpentSeconds,
		TimeSpent:                domain.FormatDuration(t.SpentSeconds, cfg),
	}
	if t.OriginalSeconds != nil {
		s := domain.FormatDuration(*t.OriginalSeconds, cfg)
		out.OriginalEstimate = &s
	}
	if t.RemainingSeconds != nil {
		s := domain.FormatDuration(*t.RemainingSeconds, cfg)
		out.RemainingEstimate = &s
	}
	return out
}

func toWorklog(w domain.Worklog, cfg domain.Config) worklogResponse {
	return worklogResponse{
		ID: w.ID.String(), IssueID: w.IssueID.String(), IssueKey: w.IssueKey,
		AuthorID: w.AuthorID.String(), AuthorName: w.AuthorName,
		StartedAt: w.StartedAt.UTC().Format(time.RFC3339), TimeSpentSeconds: w.TimeSpentSeconds,
		TimeSpent: domain.FormatDuration(w.TimeSpentSeconds, cfg), Description: w.Description,
		RemainingMode: string(w.RemainingMode), RemainingAdjustSeconds: w.RemainingAdjustSeconds,
		CreatedAt: w.CreatedAt.UTC().Format(time.RFC3339), UpdatedAt: w.UpdatedAt.UTC().Format(time.RFC3339),
	}
}

// List returns the worklogs of an issue with its totals.
func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	actorID, globalRole, ok := requestActor(w, r)
	if !ok {
		return
	}
	issueID, ok := pathID(w, r, "issue", domain.ErrIssueNotFound)
	if !ok {
		return
	}
	out, err := h.service.ListIssueWorklogs(r.Context(), actorID, globalRole, issueID)
	if err != nil {
		httpresponse.Error(w, err)
		return
	}
	items := make([]worklogResponse, 0, len(out.Worklogs))
	for _, wl := range out.Worklogs {
		items = append(items, toWorklog(wl, out.Config))
	}
	httpresponse.JSON(w, http.StatusOK, map[string]any{
		"worklogs": items,
		"totals":   toTotals(out.Totals, out.Config),
	})
}

// Log records a worklog on an issue.
func (h *Handler) Log(w http.ResponseWriter, r *http.Request) {
	actorID, globalRole, ok := requestActor(w, r)
	if !ok {
		return
	}
	issueID, ok := pathID(w, r, "issue", domain.ErrIssueNotFound)
	if !ok {
		return
	}
	var body logRequest
	if err := decodeStrict(r, &body, false); err != nil {
		httpresponse.Error(w, err)
		return
	}
	in := port.LogInput{
		TimeSpent:   port.Duration{Text: body.TimeSpent, Seconds: body.TimeSpentSeconds},
		StartedAt:   body.StartedAt,
		Description: body.Description,
		Remaining:   port.RemainingInput{Mode: domain.RemainingAuto},
	}
	if body.Remaining != nil {
		if body.Remaining.Mode != "" {
			in.Remaining.Mode = domain.RemainingMode(body.Remaining.Mode)
		}
		if body.Remaining.Value != nil || body.Remaining.ValueSeconds != nil {
			in.Remaining.Value = &port.Duration{Text: body.Remaining.Value, Seconds: body.Remaining.ValueSeconds}
		}
	}
	out, err := h.service.LogWork(r.Context(), actorID, globalRole, issueID, in)
	if err != nil {
		httpresponse.Error(w, err)
		return
	}
	httpresponse.JSON(w, http.StatusCreated, map[string]any{
		"worklog": toWorklog(*out.Worklog, out.Config),
		"totals":  toTotals(out.Totals, out.Config),
	})
}

// Update changes the time, start or description of a worklog.
func (h *Handler) Update(w http.ResponseWriter, r *http.Request) {
	actorID, globalRole, ok := requestActor(w, r)
	if !ok {
		return
	}
	id, ok := pathID(w, r, "id", domain.ErrWorklogNotFound)
	if !ok {
		return
	}
	var body updateRequest
	if err := decodeStrict(r, &body, false); err != nil {
		httpresponse.Error(w, err)
		return
	}
	in := port.UpdateInput{StartedAt: body.StartedAt, Description: body.Description}
	if body.TimeSpent != nil || body.TimeSpentSeconds != nil {
		in.TimeSpent = &port.Duration{Text: body.TimeSpent, Seconds: body.TimeSpentSeconds}
	}
	out, err := h.service.UpdateWorklog(r.Context(), actorID, globalRole, id, in)
	if err != nil {
		httpresponse.Error(w, err)
		return
	}
	httpresponse.JSON(w, http.StatusOK, map[string]any{
		"worklog": toWorklog(*out.Worklog, out.Config),
		"totals":  toTotals(out.Totals, out.Config),
	})
}

// Delete removes a worklog and gives back its remaining estimate adjust.
func (h *Handler) Delete(w http.ResponseWriter, r *http.Request) {
	actorID, globalRole, ok := requestActor(w, r)
	if !ok {
		return
	}
	id, ok := pathID(w, r, "id", domain.ErrWorklogNotFound)
	if !ok {
		return
	}
	if err := h.service.DeleteWorklog(r.Context(), actorID, globalRole, id); err != nil {
		httpresponse.Error(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// UpdateEstimates sets the original and/or remaining estimate of an issue.
func (h *Handler) UpdateEstimates(w http.ResponseWriter, r *http.Request) {
	actorID, globalRole, ok := requestActor(w, r)
	if !ok {
		return
	}
	issueID, ok := pathID(w, r, "issue", domain.ErrIssueNotFound)
	if !ok {
		return
	}
	var body estimateRequest
	if err := decodeStrict(r, &body, false); err != nil {
		httpresponse.Error(w, err)
		return
	}
	original, err := toDuration(body.Original)
	if err != nil {
		httpresponse.Error(w, err)
		return
	}
	remaining, err := toDuration(body.Remaining)
	if err != nil {
		httpresponse.Error(w, err)
		return
	}
	out, err := h.service.UpdateEstimates(r.Context(), actorID, globalRole, issueID, port.EstimatesInput{Original: original, Remaining: remaining})
	if err != nil {
		httpresponse.Error(w, err)
		return
	}
	httpresponse.JSON(w, http.StatusOK, toTotals(out.Totals, out.Config))
}

type dayEntryResponse struct {
	ID               string `json:"id"`
	IssueID          string `json:"issue_id"`
	IssueKey         string `json:"issue_key"`
	IssueTitle       string `json:"issue_title"`
	AuthorID         string `json:"author_id"`
	StartedAt        string `json:"started_at"`
	TimeSpentSeconds int64  `json:"time_spent_seconds"`
	TimeSpent        string `json:"time_spent"`
	Description      string `json:"description"`
}

// Timesheet returns the worklogs of one user grouped by local day.
func (h *Handler) Timesheet(w http.ResponseWriter, r *http.Request) {
	actorID, globalRole, ok := requestActor(w, r)
	if !ok {
		return
	}
	q := port.TimesheetQuery{From: r.URL.Query().Get("from"), To: r.URL.Query().Get("to")}
	if raw := r.URL.Query().Get("user"); raw != "" {
		id, err := uuid.Parse(raw)
		if err != nil {
			httpresponse.Error(w, apperror.WithCode(apperror.KindValidation, "validation.failed", "El identificador de usuario no es valido"))
			return
		}
		q.UserID = &id
	}
	ts, cfg, err := h.service.Timesheet(r.Context(), actorID, globalRole, q)
	if err != nil {
		httpresponse.Error(w, err)
		return
	}
	days := make([]map[string]any, 0, len(ts.Days))
	for _, day := range ts.Days {
		entries := make([]dayEntryResponse, 0, len(day.Entries))
		for _, e := range day.Entries {
			entries = append(entries, dayEntryResponse{
				ID: e.ID.String(), IssueID: e.IssueID.String(), IssueKey: e.IssueKey, IssueTitle: e.IssueTitle,
				AuthorID: e.AuthorID.String(), StartedAt: e.StartedAt.UTC().Format(time.RFC3339),
				TimeSpentSeconds: e.Seconds, TimeSpent: domain.FormatDuration(e.Seconds, cfg), Description: e.Description,
			})
		}
		days = append(days, map[string]any{
			"date":             day.Date,
			"total_seconds":    day.TotalSeconds,
			"total":            domain.FormatDuration(day.TotalSeconds, cfg),
			"capacity_seconds": day.CapacitySeconds,
			"entries":          entries,
		})
	}
	httpresponse.JSON(w, http.StatusOK, map[string]any{
		"from":          ts.From,
		"to":            ts.To,
		"total_seconds": ts.TotalSeconds,
		"total":         domain.FormatDuration(ts.TotalSeconds, cfg),
		"days":          days,
	})
}

// Summary returns the time of a project grouped by user, issue or day.
func (h *Handler) Summary(w http.ResponseWriter, r *http.Request) {
	actorID, globalRole, ok := requestActor(w, r)
	if !ok {
		return
	}
	projectID, err := uuid.Parse(chi.URLParam(r, "project"))
	if err != nil {
		httpresponse.Error(w, apperror.WithCode(apperror.KindNotFound, "project.not_found", "El proyecto no existe"))
		return
	}
	query := r.URL.Query()
	groupBy := query.Get("group_by")
	if groupBy == "" {
		groupBy = domain.GroupByUser
	}
	sum, cfg, err := h.service.TimeSummary(r.Context(), actorID, globalRole, projectID, port.SummaryQuery{
		From: query.Get("from"), To: query.Get("to"), GroupBy: groupBy,
	})
	if err != nil {
		httpresponse.Error(w, err)
		return
	}
	groups := make([]map[string]any, 0, len(sum.Groups))
	for _, g := range sum.Groups {
		groups = append(groups, map[string]any{
			"key":           g.Key,
			"label":         g.Label,
			"total_seconds": g.TotalSeconds,
			"total":         domain.FormatDuration(g.TotalSeconds, cfg),
			"count":         g.Count,
		})
	}
	httpresponse.JSON(w, http.StatusOK, map[string]any{
		"group_by":      sum.GroupBy,
		"from":          sum.From,
		"to":            sum.To,
		"total_seconds": sum.TotalSeconds,
		"total":         domain.FormatDuration(sum.TotalSeconds, cfg),
		"count":         sum.Count,
		"groups":        groups,
	})
}
