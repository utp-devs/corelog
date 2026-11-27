package http

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/jhonsferg/corelog/internal/platform/apperror"
	"github.com/jhonsferg/corelog/internal/platform/httpresponse"
	appmw "github.com/jhonsferg/corelog/internal/platform/middleware"
	"github.com/jhonsferg/corelog/internal/sprint/application/port"
	"github.com/jhonsferg/corelog/internal/sprint/domain"
)

const dateLayout = "2006-01-02"

type Handler struct {
	service   port.Service
	jwtSecret string
}

func NewHandler(service port.Service, jwtSecret string) *Handler {
	return &Handler{service: service, jwtSecret: jwtSecret}
}

func (h *Handler) RegisterRoutes(r chi.Router) {
	r.Route("/sprints", func(r chi.Router) {
		r.Use(appmw.Authenticate(h.jwtSecret))
		r.Get("/", h.List)
		r.Post("/", h.Create)
		r.Get("/velocity", h.Velocity)
		r.Route("/{id}", func(r chi.Router) {
			r.Get("/", h.Get)
			r.Patch("/", h.Update)
			r.Delete("/", h.Delete)
			r.Post("/start", h.Start)
			r.Post("/close", h.Close)
			r.Post("/complete", h.Complete)
			r.Get("/report", h.Report)
			r.Get("/issues", h.ListIssues)
			r.Put("/issues/{issue_id}", h.AssignIssue)
			r.Delete("/issues/{issue_id}", h.RemoveIssue)
			r.Get("/burndown", h.Burndown)
		})
	})
	r.Route("/projects/{id}/backlog", func(r chi.Router) {
		r.Use(appmw.Authenticate(h.jwtSecret))
		r.Get("/", h.Backlog)
	})
	r.With(appmw.Authenticate(h.jwtSecret)).Put("/backlog/rank", h.Rank)
}

type sprintResponse struct {
	ID              string   `json:"id"`
	ProjectID       string   `json:"project_id"`
	BoardID         *string  `json:"board_id"`
	Name            string   `json:"name"`
	Goal            string   `json:"goal"`
	Status          string   `json:"status"`
	StartDate       *string  `json:"start_date"`
	EndDate         *string  `json:"end_date"`
	StartedAt       *string  `json:"started_at"`
	CompletedAt     *string  `json:"completed_at"`
	ClosedAt        *string  `json:"closed_at"`
	CommittedPoints *float64 `json:"committed_points"`
	CompletedPoints *float64 `json:"completed_points"`
}

type createSprintRequest struct {
	Name      string  `json:"name"`
	Goal      string  `json:"goal"`
	StartDate *string `json:"start_date"`
	EndDate   *string `json:"end_date"`
}

// optionalDate distingue un campo ausente, null (borrar) y una fecha.
type optionalDate struct {
	set   bool
	value *string
}

func (o *optionalDate) UnmarshalJSON(b []byte) error {
	o.set = true
	if string(b) == "null" {
		return nil
	}
	var s string
	if err := json.Unmarshal(b, &s); err != nil {
		return err
	}
	o.value = &s
	return nil
}

type updateSprintRequest struct {
	Name      *string      `json:"name"`
	Goal      *string      `json:"goal"`
	StartDate optionalDate `json:"start_date"`
	EndDate   optionalDate `json:"end_date"`
}

type startSprintRequest struct {
	Name      *string `json:"name"`
	Goal      *string `json:"goal"`
	StartDate *string `json:"start_date"`
	EndDate   *string `json:"end_date"`
	Force     bool    `json:"force"`
}

type completeSprintRequest struct {
	Destination    string     `json:"destination"`
	Unfinished     string     `json:"unfinished"`
	TargetSprintID *uuid.UUID `json:"target_sprint_id"`
	NewSprintName  string     `json:"new_sprint_name"`
}

type issueResponse struct {
	ID          string   `json:"id"`
	Title       string   `json:"title"`
	Status      string   `json:"status"`
	Priority    string   `json:"priority"`
	StoryPoints *float64 `json:"story_points"`
	ClosedAt    *string  `json:"closed_at"`
}

type burndownResponse struct {
	Date      string  `json:"date"`
	Ideal     float64 `json:"ideal"`
	Remaining float64 `json:"remaining"`
}

type velocityResponse struct {
	SprintID string  `json:"sprint_id"`
	Name     string  `json:"name"`
	Points   float64 `json:"points"`
}

func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	actorID, globalRole, projectID, ok := h.requestContext(w, r)
	if !ok {
		return
	}
	sprints, err := h.service.List(r.Context(), actorID, globalRole, projectID)
	if err != nil {
		httpresponse.Error(w, err)
		return
	}
	response := make([]sprintResponse, 0, len(sprints))
	for _, sprint := range sprints {
		response = append(response, toSprintResponse(sprint))
	}
	httpresponse.JSON(w, http.StatusOK, response)
}

func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	actorID, globalRole, projectID, ok := h.requestContext(w, r)
	if !ok {
		return
	}
	var body createSprintRequest
	if err := decodeStrict(r, &body, false); err != nil {
		httpresponse.Error(w, err)
		return
	}
	start, err := parseDate(body.StartDate)
	if err != nil {
		httpresponse.Error(w, err)
		return
	}
	end, err := parseDate(body.EndDate)
	if err != nil {
		httpresponse.Error(w, err)
		return
	}
	sprint, err := h.service.Create(r.Context(), actorID, globalRole, projectID, body.Name, body.Goal, start, end)
	if err != nil {
		httpresponse.Error(w, err)
		return
	}
	httpresponse.JSON(w, http.StatusCreated, toSprintResponse(sprint))
}

func (h *Handler) Get(w http.ResponseWriter, r *http.Request) {
	actorID, globalRole, projectID, sprintID, ok := h.sprintContext(w, r)
	if !ok {
		return
	}
	sprint, err := h.service.Get(r.Context(), actorID, globalRole, projectID, sprintID)
	if err != nil {
		httpresponse.Error(w, err)
		return
	}
	httpresponse.JSON(w, http.StatusOK, toSprintResponse(sprint))
}

func (h *Handler) Update(w http.ResponseWriter, r *http.Request) {
	actorID, globalRole, projectID, sprintID, ok := h.sprintContext(w, r)
	if !ok {
		return
	}
	var body updateSprintRequest
	if err := decodeStrict(r, &body, false); err != nil {
		httpresponse.Error(w, err)
		return
	}
	patch := domain.Patch{Name: body.Name, Goal: body.Goal}
	var err error
	if patch.StartDate, err = patchDate(body.StartDate); err != nil {
		httpresponse.Error(w, err)
		return
	}
	if patch.EndDate, err = patchDate(body.EndDate); err != nil {
		httpresponse.Error(w, err)
		return
	}
	sprint, err := h.service.Update(r.Context(), actorID, globalRole, projectID, sprintID, patch)
	if err != nil {
		httpresponse.Error(w, err)
		return
	}
	httpresponse.JSON(w, http.StatusOK, toSprintResponse(sprint))
}

func (h *Handler) Delete(w http.ResponseWriter, r *http.Request) {
	actorID, globalRole, projectID, sprintID, ok := h.sprintContext(w, r)
	if !ok {
		return
	}
	if err := h.service.Delete(r.Context(), actorID, globalRole, projectID, sprintID); err != nil {
		httpresponse.Error(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) Start(w http.ResponseWriter, r *http.Request) {
	actorID, globalRole, projectID, sprintID, ok := h.sprintContext(w, r)
	if !ok {
		return
	}
	var body startSprintRequest
	if err := decodeStrict(r, &body, true); err != nil {
		httpresponse.Error(w, err)
		return
	}
	start, err := parseDate(body.StartDate)
	if err != nil {
		httpresponse.Error(w, err)
		return
	}
	end, err := parseDate(body.EndDate)
	if err != nil {
		httpresponse.Error(w, err)
		return
	}
	sprint, err := h.service.Start(r.Context(), actorID, globalRole, projectID, sprintID, port.StartInput{
		Name: body.Name, Goal: body.Goal, StartDate: start, EndDate: end, Force: body.Force,
	})
	if err != nil {
		httpresponse.Error(w, err)
		return
	}
	httpresponse.JSON(w, http.StatusOK, toSprintResponse(sprint))
}

// Close cierra el sprint con la forma antigua (unfinished/target_sprint_id) y devuelve solo el sprint.
func (h *Handler) Close(w http.ResponseWriter, r *http.Request) {
	result, ok := h.completeSprint(w, r)
	if !ok {
		return
	}
	httpresponse.JSON(w, http.StatusOK, toSprintResponse(result.Sprint))
}

type completeResponse struct {
	Sprint          sprintResponse  `json:"sprint"`
	NextSprint      *sprintResponse `json:"next_sprint"`
	CompletedIssues int             `json:"completed_issues"`
	MovedIssues     int             `json:"moved_issues"`
	CompletedPoints float64         `json:"completed_points"`
	CommittedPoints float64         `json:"committed_points"`
}

// Complete cierra el sprint y devuelve el resumen del cierre.
func (h *Handler) Complete(w http.ResponseWriter, r *http.Request) {
	result, ok := h.completeSprint(w, r)
	if !ok {
		return
	}
	response := completeResponse{
		Sprint: toSprintResponse(result.Sprint), CompletedIssues: result.CompletedIssues, MovedIssues: result.MovedIssues,
		CompletedPoints: result.CompletedPoints, CommittedPoints: result.CommittedPoints,
	}
	if result.NextSprint != nil {
		next := toSprintResponse(result.NextSprint)
		response.NextSprint = &next
	}
	httpresponse.JSON(w, http.StatusOK, response)
}

func (h *Handler) completeSprint(w http.ResponseWriter, r *http.Request) (*port.CompleteResult, bool) {
	actorID, globalRole, projectID, sprintID, ok := h.sprintContext(w, r)
	if !ok {
		return nil, false
	}
	var body completeSprintRequest
	if err := decodeStrict(r, &body, true); err != nil {
		httpresponse.Error(w, err)
		return nil, false
	}
	result, err := h.service.Complete(r.Context(), actorID, globalRole, projectID, sprintID, port.CompleteInput{
		Destination: body.Destination, Unfinished: body.Unfinished, TargetSprintID: body.TargetSprintID, NewSprintName: body.NewSprintName,
	})
	if err != nil {
		httpresponse.Error(w, err)
		return nil, false
	}
	return result, true
}

type reportIssueResponse struct {
	IssueID         string   `json:"issue_id"`
	Key             string   `json:"key"`
	Summary         string   `json:"summary"`
	Points          *float64 `json:"points"`
	StatusCategory  string   `json:"status_category"`
	AddedAfterStart bool     `json:"added_after_start"`
	RemovedAt       *string  `json:"removed_at,omitempty"`
	RemovedKind     string   `json:"removed_kind,omitempty"`
}

type burndownDayResponse struct {
	Date      string   `json:"date"`
	Ideal     *float64 `json:"ideal"`
	Remaining float64  `json:"remaining"`
	Scope     float64  `json:"scope"`
	Completed float64  `json:"completed"`
}

type reportResponse struct {
	Sprint           sprintResponse        `json:"sprint"`
	CommittedPoints  float64               `json:"committed_points"`
	CompletedPoints  float64               `json:"completed_points"`
	IncompletePoints float64               `json:"incomplete_points"`
	AddedPoints      float64               `json:"added_points"`
	RemovedPoints    float64               `json:"removed_points"`
	Completed        []reportIssueResponse `json:"completed"`
	Incomplete       []reportIssueResponse `json:"incomplete"`
	Removed          []reportIssueResponse `json:"removed"`
	AddedAfterStart  []reportIssueResponse `json:"added_after_start"`
	Burndown         []burndownDayResponse `json:"burndown"`
	Velocity         []velocityResponse    `json:"velocity"`
}

func (h *Handler) Report(w http.ResponseWriter, r *http.Request) {
	actorID, globalRole, projectID, sprintID, ok := h.sprintContext(w, r)
	if !ok {
		return
	}
	report, err := h.service.Report(r.Context(), actorID, globalRole, projectID, sprintID)
	if err != nil {
		httpresponse.Error(w, err)
		return
	}
	response := reportResponse{
		Sprint: toSprintResponse(report.Sprint), CommittedPoints: report.CommittedPoints, CompletedPoints: report.CompletedPoints,
		IncompletePoints: report.IncompletePoints, AddedPoints: report.AddedPoints, RemovedPoints: report.RemovedPoints,
		Completed: toReportIssues(report.Completed), Incomplete: toReportIssues(report.Incomplete),
		Removed: toReportIssues(report.Removed), AddedAfterStart: toReportIssues(report.AddedAfterStart),
		Burndown: make([]burndownDayResponse, 0, len(report.Burndown)), Velocity: make([]velocityResponse, 0, len(report.Velocity)),
	}
	for _, d := range report.Burndown {
		response.Burndown = append(response.Burndown, burndownDayResponse{
			Date: d.Date.Format(dateLayout), Ideal: d.Ideal, Remaining: d.Remaining, Scope: d.Scope, Completed: d.Completed,
		})
	}
	for _, v := range report.Velocity {
		response.Velocity = append(response.Velocity, velocityResponse{SprintID: v.SprintID.String(), Name: v.Name, Points: v.Points})
	}
	httpresponse.JSON(w, http.StatusOK, response)
}

func toReportIssues(items []domain.ReportIssue) []reportIssueResponse {
	out := make([]reportIssueResponse, 0, len(items))
	for _, i := range items {
		item := reportIssueResponse{IssueID: i.IssueID.String(), Key: i.Key, Summary: i.Summary, Points: i.Points,
			StatusCategory: i.StatusCategory, AddedAfterStart: i.AddedAfterStart, RemovedKind: string(i.RemovedKind)}
		if i.RemovedAt != nil {
			v := i.RemovedAt.UTC().Format(time.RFC3339)
			item.RemovedAt = &v
		}
		out = append(out, item)
	}
	return out
}

func (h *Handler) ListIssues(w http.ResponseWriter, r *http.Request) {
	actorID, globalRole, projectID, sprintID, ok := h.sprintContext(w, r)
	if !ok {
		return
	}
	refs, err := h.service.ListIssues(r.Context(), actorID, globalRole, projectID, sprintID)
	if err != nil {
		httpresponse.Error(w, err)
		return
	}
	response := make([]issueResponse, 0, len(refs))
	for _, ref := range refs {
		entry := issueResponse{ID: ref.ID.String(), Title: ref.Title, Status: ref.Status, Priority: ref.Priority, StoryPoints: ref.StoryPoints}
		if ref.ClosedAt != nil {
			closed := ref.ClosedAt.Format(time.RFC3339)
			entry.ClosedAt = &closed
		}
		response = append(response, entry)
	}
	httpresponse.JSON(w, http.StatusOK, response)
}

func (h *Handler) AssignIssue(w http.ResponseWriter, r *http.Request) {
	actorID, globalRole, projectID, sprintID, ok := h.sprintContext(w, r)
	if !ok {
		return
	}
	issueID, err := uuid.Parse(chi.URLParam(r, "issue_id"))
	if err != nil {
		httpresponse.Error(w, apperror.WithCode(apperror.KindValidation, "validation.failed", "Identificador de incidencia no valido"))
		return
	}
	if err := h.service.AssignIssue(r.Context(), actorID, globalRole, projectID, sprintID, issueID); err != nil {
		httpresponse.Error(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) RemoveIssue(w http.ResponseWriter, r *http.Request) {
	actorID, globalRole, projectID, sprintID, ok := h.sprintContext(w, r)
	if !ok {
		return
	}
	issueID, err := uuid.Parse(chi.URLParam(r, "issue_id"))
	if err != nil {
		httpresponse.Error(w, apperror.WithCode(apperror.KindValidation, "validation.failed", "Identificador de incidencia no valido"))
		return
	}
	if err := h.service.RemoveIssue(r.Context(), actorID, globalRole, projectID, sprintID, issueID); err != nil {
		httpresponse.Error(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) Burndown(w http.ResponseWriter, r *http.Request) {
	actorID, globalRole, projectID, sprintID, ok := h.sprintContext(w, r)
	if !ok {
		return
	}
	points, err := h.service.Burndown(r.Context(), actorID, globalRole, projectID, sprintID)
	if err != nil {
		httpresponse.Error(w, err)
		return
	}
	response := make([]burndownResponse, 0, len(points))
	for _, point := range points {
		response = append(response, burndownResponse{Date: point.Date.Format(dateLayout), Ideal: point.Ideal, Remaining: point.Remaining})
	}
	httpresponse.JSON(w, http.StatusOK, response)
}

func (h *Handler) Velocity(w http.ResponseWriter, r *http.Request) {
	actorID, globalRole, projectID, ok := h.requestContext(w, r)
	if !ok {
		return
	}
	limit := 0
	if raw := r.URL.Query().Get("limit"); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil {
			httpresponse.Error(w, apperror.WithCode(apperror.KindValidation, "validation.failed", "El limite no es valido"))
			return
		}
		limit = parsed
	}
	points, err := h.service.Velocity(r.Context(), actorID, globalRole, projectID, limit)
	if err != nil {
		httpresponse.Error(w, err)
		return
	}
	response := make([]velocityResponse, 0, len(points))
	for _, point := range points {
		response = append(response, velocityResponse{SprintID: point.SprintID.String(), Name: point.Name, Points: point.Points})
	}
	httpresponse.JSON(w, http.StatusOK, response)
}

// requestContext lee el actor, su rol global y el proyecto indicado en project_id.
func (h *Handler) requestContext(w http.ResponseWriter, r *http.Request) (uuid.UUID, string, uuid.UUID, bool) {
	actorID, ok := appmw.UserIDFromContext(r.Context())
	if !ok {
		httpresponse.Error(w, apperror.New(apperror.KindUnauthorized, "No autenticado"))
		return uuid.Nil, "", uuid.Nil, false
	}
	globalRole, _ := appmw.RoleFromContext(r.Context())
	projectID, err := uuid.Parse(r.URL.Query().Get("project_id"))
	if err != nil {
		httpresponse.Error(w, apperror.WithCode(apperror.KindValidation, "validation.failed", "El identificador de proyecto no es valido"))
		return uuid.Nil, "", uuid.Nil, false
	}
	return actorID, globalRole, projectID, true
}

// sprintContext lee el sprint de la ruta y su proyecto: el de project_id si viene y, si no, el del propio
// sprint. Un sprint inexistente responde 404 igual que uno de otro proyecto.
func (h *Handler) sprintContext(w http.ResponseWriter, r *http.Request) (uuid.UUID, string, uuid.UUID, uuid.UUID, bool) {
	actorID, ok := appmw.UserIDFromContext(r.Context())
	if !ok {
		httpresponse.Error(w, apperror.New(apperror.KindUnauthorized, "No autenticado"))
		return uuid.Nil, "", uuid.Nil, uuid.Nil, false
	}
	globalRole, _ := appmw.RoleFromContext(r.Context())
	sprintID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpresponse.Error(w, apperror.WithCode(apperror.KindValidation, "validation.failed", "El identificador de sprint no es valido"))
		return uuid.Nil, "", uuid.Nil, uuid.Nil, false
	}
	var projectID uuid.UUID
	if raw := r.URL.Query().Get("project_id"); raw != "" {
		if projectID, err = uuid.Parse(raw); err != nil {
			httpresponse.Error(w, apperror.WithCode(apperror.KindValidation, "validation.failed", "El identificador de proyecto no es valido"))
			return uuid.Nil, "", uuid.Nil, uuid.Nil, false
		}
	} else if projectID, err = h.service.ResolveSprintProject(r.Context(), sprintID); err != nil {
		httpresponse.Error(w, err)
		return uuid.Nil, "", uuid.Nil, uuid.Nil, false
	}
	return actorID, globalRole, projectID, sprintID, true
}

// decodeStrict lee el cuerpo rechazando campos desconocidos. Con optional, un cuerpo vacio es valido.
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
		_ = err
		return apperror.WithCode(apperror.KindValidation, "validation.failed", "El cuerpo de la peticion no es valido")
	}
	return nil
}

func parseDate(raw *string) (*time.Time, error) {
	if raw == nil || *raw == "" {
		return nil, nil
	}
	parsed, err := time.Parse(dateLayout, *raw)
	if err != nil {
		return nil, apperror.WithCode(apperror.KindValidation, "validation.failed", "Las fechas deben tener el formato AAAA-MM-DD")
	}
	return &parsed, nil
}

// patchDate traduce un campo de fecha del PATCH: ausente = sin cambio (nil), null = borrar (fecha cero).
func patchDate(o optionalDate) (*time.Time, error) {
	if !o.set {
		return nil, nil
	}
	if o.value == nil || *o.value == "" {
		return &time.Time{}, nil
	}
	return parseDate(o.value)
}

func toSprintResponse(sprint *domain.Sprint) sprintResponse {
	response := sprintResponse{
		ID: sprint.ID.String(), ProjectID: sprint.ProjectID.String(), Name: sprint.Name,
		Goal: sprint.Goal, Status: string(sprint.Status),
		CommittedPoints: sprint.CommittedPoints, CompletedPoints: sprint.CompletedPoints,
	}
	if sprint.BoardID != nil {
		v := sprint.BoardID.String()
		response.BoardID = &v
	}
	date := func(t *time.Time) *string {
		if t == nil {
			return nil
		}
		v := t.Format(dateLayout)
		return &v
	}
	stamp := func(t *time.Time) *string {
		if t == nil {
			return nil
		}
		v := t.UTC().Format(time.RFC3339)
		return &v
	}
	response.StartDate, response.EndDate = date(sprint.StartDate), date(sprint.EndDate)
	response.StartedAt, response.CompletedAt, response.ClosedAt = stamp(sprint.StartedAt), stamp(sprint.CompletedAt), stamp(sprint.ClosedAt)
	return response
}
