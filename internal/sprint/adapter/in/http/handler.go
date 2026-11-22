package http

import (
	"encoding/json"
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
			r.Post("/start", h.Start)
			r.Post("/close", h.Close)
			r.Get("/issues", h.ListIssues)
			r.Put("/issues/{issue_id}", h.AssignIssue)
			r.Delete("/issues/{issue_id}", h.RemoveIssue)
			r.Get("/burndown", h.Burndown)
		})
	})
}

type sprintResponse struct {
	ID        string  `json:"id"`
	ProjectID string  `json:"project_id"`
	Name      string  `json:"name"`
	Goal      string  `json:"goal"`
	Status    string  `json:"status"`
	StartDate *string `json:"start_date"`
	EndDate   *string `json:"end_date"`
	ClosedAt  *string `json:"closed_at"`
}

type createSprintRequest struct {
	Name      string  `json:"name"`
	Goal      string  `json:"goal"`
	StartDate *string `json:"start_date"`
	EndDate   *string `json:"end_date"`
}

type closeSprintRequest struct {
	Unfinished     string     `json:"unfinished"`
	TargetSprintID *uuid.UUID `json:"target_sprint_id"`
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
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		httpresponse.Error(w, apperror.New(apperror.KindValidation, "invalid request body"))
		return
	}
	start, end, err := parseDates(body.StartDate, body.EndDate)
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

func (h *Handler) Start(w http.ResponseWriter, r *http.Request) {
	actorID, globalRole, projectID, sprintID, ok := h.sprintContext(w, r)
	if !ok {
		return
	}
	sprint, err := h.service.Start(r.Context(), actorID, globalRole, projectID, sprintID)
	if err != nil {
		httpresponse.Error(w, err)
		return
	}
	httpresponse.JSON(w, http.StatusOK, toSprintResponse(sprint))
}

func (h *Handler) Close(w http.ResponseWriter, r *http.Request) {
	actorID, globalRole, projectID, sprintID, ok := h.sprintContext(w, r)
	if !ok {
		return
	}
	var body closeSprintRequest
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		httpresponse.Error(w, apperror.New(apperror.KindValidation, "invalid request body"))
		return
	}
	sprint, err := h.service.Close(r.Context(), actorID, globalRole, projectID, sprintID, port.CloseInput{
		Unfinished: body.Unfinished, TargetSprintID: body.TargetSprintID,
	})
	if err != nil {
		httpresponse.Error(w, err)
		return
	}
	httpresponse.JSON(w, http.StatusOK, toSprintResponse(sprint))
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
		httpresponse.Error(w, apperror.New(apperror.KindValidation, "invalid issue id"))
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
		httpresponse.Error(w, apperror.New(apperror.KindValidation, "invalid issue id"))
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
			httpresponse.Error(w, apperror.New(apperror.KindValidation, "invalid limit"))
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
		httpresponse.Error(w, apperror.New(apperror.KindUnauthorized, "not authenticated"))
		return uuid.Nil, "", uuid.Nil, false
	}
	globalRole, _ := appmw.RoleFromContext(r.Context())
	projectID, err := uuid.Parse(r.URL.Query().Get("project_id"))
	if err != nil {
		httpresponse.Error(w, apperror.New(apperror.KindValidation, "invalid project id"))
		return uuid.Nil, "", uuid.Nil, false
	}
	return actorID, globalRole, projectID, true
}

// sprintContext añade el identificador del sprint de la ruta al contexto de la peticion.
func (h *Handler) sprintContext(w http.ResponseWriter, r *http.Request) (uuid.UUID, string, uuid.UUID, uuid.UUID, bool) {
	actorID, globalRole, projectID, ok := h.requestContext(w, r)
	if !ok {
		return uuid.Nil, "", uuid.Nil, uuid.Nil, false
	}
	sprintID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpresponse.Error(w, apperror.New(apperror.KindValidation, "invalid sprint id"))
		return uuid.Nil, "", uuid.Nil, uuid.Nil, false
	}
	return actorID, globalRole, projectID, sprintID, true
}

func parseDates(start, end *string) (*time.Time, *time.Time, error) {
	parse := func(raw *string) (*time.Time, error) {
		if raw == nil || *raw == "" {
			return nil, nil
		}
		parsed, err := time.Parse(dateLayout, *raw)
		if err != nil {
			return nil, apperror.New(apperror.KindValidation, "dates must use the format YYYY-MM-DD")
		}
		return &parsed, nil
	}
	startDate, err := parse(start)
	if err != nil {
		return nil, nil, err
	}
	endDate, err := parse(end)
	if err != nil {
		return nil, nil, err
	}
	return startDate, endDate, nil
}

func toSprintResponse(sprint *domain.Sprint) sprintResponse {
	response := sprintResponse{
		ID: sprint.ID.String(), ProjectID: sprint.ProjectID.String(), Name: sprint.Name,
		Goal: sprint.Goal, Status: string(sprint.Status),
	}
	if sprint.StartDate != nil {
		value := sprint.StartDate.Format(dateLayout)
		response.StartDate = &value
	}
	if sprint.EndDate != nil {
		value := sprint.EndDate.Format(dateLayout)
		response.EndDate = &value
	}
	if sprint.ClosedAt != nil {
		value := sprint.ClosedAt.Format(time.RFC3339)
		response.ClosedAt = &value
	}
	return response
}
