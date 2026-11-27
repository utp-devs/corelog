package http

import (
	"encoding/json"
	"io"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/jhonsferg/corelog/internal/issue/application/port"
	"github.com/jhonsferg/corelog/internal/issue/domain"
	"github.com/jhonsferg/corelog/internal/platform/apperror"
	"github.com/jhonsferg/corelog/internal/platform/httpresponse"
	appmw "github.com/jhonsferg/corelog/internal/platform/middleware"
	"github.com/jhonsferg/corelog/internal/platform/validation"
)

type Handler struct {
	service     port.Service
	jwtSecret   string
	attachments port.AttachmentRepository
	files       port.FileStorage
	fieldScope  port.CustomFieldScopeReader
	ops         port.IssueOperations
	transitions port.TransitionService
	comments    port.CommentService
}

// WithComments fixes the comment service behind the comment routes.
func (h *Handler) WithComments(comments port.CommentService) *Handler {
	h.comments = comments
	return h
}

// WithFieldScope fixes the reader that lists the custom fields of an issue in its responses.
func (h *Handler) WithFieldScope(scope port.CustomFieldScopeReader) *Handler {
	h.fieldScope = scope
	return h
}

func NewHandler(service port.Service, jwtSecret string) *Handler {
	return &Handler{service: service, jwtSecret: jwtSecret}
}

func (h *Handler) WithAttachments(repo port.AttachmentRepository, files port.FileStorage) *Handler {
	h.attachments, h.files = repo, files
	return h
}

func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	reporterID, ok := appmw.UserIDFromContext(r.Context())
	if !ok {
		httpresponse.Error(w, apperror.New(apperror.KindUnauthorized, "not authenticated"))
		return
	}

	var req createIssueRequest
	raw, err := io.ReadAll(http.MaxBytesReader(w, r.Body, maxIssueBody))
	if err != nil {
		httpresponse.Error(w, apperror.New(apperror.KindValidation, "invalid request body"))
		return
	}
	if err := json.Unmarshal(raw, &req); err != nil {
		httpresponse.Error(w, apperror.New(apperror.KindValidation, "invalid request body"))
		return
	}
	var sent map[string]json.RawMessage
	_ = json.Unmarshal(raw, &sent)
	if err := validation.Struct(req); err != nil {
		httpresponse.Error(w, validation.Failed(req, err))
		return
	}
	components, err := parseComponentIDs(req.Components)
	if err != nil {
		httpresponse.Error(w, err)
		return
	}
	globalRole, _ := appmw.RoleFromContext(r.Context())

	issue, err := h.service.Create(r.Context(), port.CreateInput{
		Components:  components,
		Summary:     req.Summary,
		Description: req.Description,
		Priority:    domain.Priority(req.Priority),
		ReporterID:  reporterID,
		GlobalRole:  globalRole,
		ProjectID:   parseOptionalUUID(req.ProjectID),
		IssueType:   domain.IssueType(req.IssueType),
		IssueTypeID: parseOptionalUUID(req.IssueTypeID),
		PriorityID:  parseOptionalUUID(req.PriorityID),
		ParentID:    parseOptionalUUID(req.ParentID),
		DueDate:     req.DueDate.value(),
		StoryPoints: req.StoryPoints,
		AssigneeID:  parseOptionalUUID(req.AssigneeID),
		StartDate:   req.StartDate.value(),

		OriginalEstimateSeconds:  req.OriginalEstimateSeconds,
		RemainingEstimateSeconds: req.RemainingEstimateSeconds,
		Provided:                 providedSystemFields(sent),
		Custom:                   req.Custom,
	})
	if err != nil {
		httpresponse.Error(w, err)
		return
	}

	h.writeIssue(w, r, http.StatusCreated, issue)
}

// writeIssue responde con una incidencia incluyendo el nombre de su solicitante y responsable.
func (h *Handler) writeIssue(w http.ResponseWriter, r *http.Request, status int, issue *domain.Issue) {
	resp, err := h.buildIssueResponse(r, issue)
	if err != nil {
		httpresponse.Error(w, err)
		return
	}
	httpresponse.JSON(w, status, resp)
}

// buildIssueResponse builds the response of one issue with person names and its custom fields.
func (h *Handler) buildIssueResponse(r *http.Request, issue *domain.Issue) (issueResponse, error) {
	names, err := h.service.UserNames(r.Context(), personIDs(issue))
	if err != nil {
		return issueResponse{}, err
	}
	resp := toIssueResponse(issue, names)
	resp.Custom = issue.Custom
	if resp.Custom == nil {
		resp.Custom = map[string]any{}
	}
	resp.CustomFields = []issueCustomFieldResponse{}
	if h.fieldScope != nil {
		scope, err := h.fieldScope.ScopeFor(r.Context(), issue.ProjectID, issue.IssueTypeID)
		if err != nil {
			return issueResponse{}, err
		}
		resp.CustomFields = toIssueCustomFields(scope)
	}
	return resp, nil
}

func (h *Handler) Get(w http.ResponseWriter, r *http.Request) {
	actorID, ok := appmw.UserIDFromContext(r.Context())
	if !ok {
		httpresponse.Error(w, apperror.New(apperror.KindUnauthorized, "not authenticated"))
		return
	}
	globalRole, _ := appmw.RoleFromContext(r.Context())
	id, err := h.issueRef(r, "id")
	if err != nil {
		httpresponse.Error(w, err)
		return
	}

	view, err := h.service.GetView(r.Context(), id, actorID, globalRole)
	if err != nil {
		httpresponse.Error(w, err)
		return
	}
	resp, err := h.buildIssueResponse(r, view.Issue)
	if err != nil {
		httpresponse.Error(w, err)
		return
	}
	httpresponse.JSON(w, http.StatusOK, toIssueViewResponse(resp, view))
}

// unassignedFilterValue is the assignee_id query value that lists issues without an assignee.
const unassignedFilterValue = "none"

func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	actorID, ok := appmw.UserIDFromContext(r.Context())
	if !ok {
		httpresponse.Error(w, apperror.New(apperror.KindUnauthorized, "not authenticated"))
		return
	}
	globalRole, _ := appmw.RoleFromContext(r.Context())
	filter, err := parseIssueFilter(r.URL.Query())
	if err != nil {
		httpresponse.Error(w, err)
		return
	}

	result, err := h.service.List(r.Context(), actorID, globalRole, filter)
	if err != nil {
		httpresponse.Error(w, err)
		return
	}

	names, err := h.service.UserNames(r.Context(), personIDs(result.Items...))
	if err != nil {
		httpresponse.Error(w, err)
		return
	}

	httpresponse.Paginated(w, http.StatusOK, toIssueResponses(result.Items, names), httpresponse.PageMeta{
		Page:     result.Page,
		PageSize: result.PageSize,
		Total:    result.Total,
	})
}

func (h *Handler) ListSavedFilters(w http.ResponseWriter, r *http.Request) {
	ownerID, ok := appmw.UserIDFromContext(r.Context())
	if !ok {
		httpresponse.Error(w, apperror.New(apperror.KindUnauthorized, "not authenticated"))
		return
	}
	filters, err := h.service.ListSavedFilters(r.Context(), ownerID)
	if err != nil {
		httpresponse.Error(w, err)
		return
	}
	response := make([]savedFilterResponse, 0, len(filters))
	for _, filter := range filters {
		response = append(response, toSavedFilterResponse(filter))
	}
	httpresponse.JSON(w, http.StatusOK, response)
}

func (h *Handler) CreateSavedFilter(w http.ResponseWriter, r *http.Request) {
	ownerID, ok := appmw.UserIDFromContext(r.Context())
	if !ok {
		httpresponse.Error(w, apperror.New(apperror.KindUnauthorized, "not authenticated"))
		return
	}
	var req createSavedFilterRequest
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&req); err != nil {
		httpresponse.Error(w, apperror.New(apperror.KindValidation, "invalid request body"))
		return
	}
	if err := validation.Struct(req); err != nil {
		httpresponse.Error(w, validation.Failed(req, err))
		return
	}
	filter, err := h.service.CreateSavedFilter(r.Context(), ownerID, req.Name, *req.Criteria)
	if err != nil {
		httpresponse.Error(w, err)
		return
	}
	httpresponse.JSON(w, http.StatusCreated, toSavedFilterResponse(filter))
}

func (h *Handler) DeleteSavedFilter(w http.ResponseWriter, r *http.Request) {
	ownerID, ok := appmw.UserIDFromContext(r.Context())
	if !ok {
		httpresponse.Error(w, apperror.New(apperror.KindUnauthorized, "not authenticated"))
		return
	}
	filterID, err := uuid.Parse(chi.URLParam(r, "filter_id"))
	if err != nil {
		httpresponse.Error(w, apperror.New(apperror.KindValidation, "invalid saved filter id"))
		return
	}
	if err := h.service.DeleteSavedFilter(r.Context(), ownerID, filterID); err != nil {
		httpresponse.Error(w, err)
		return
	}
	httpresponse.JSON(w, http.StatusNoContent, nil)
}

// Assign asigna la incidencia a assignee_id, o la desasigna cuando assignee_id es null.
func (h *Handler) Assign(w http.ResponseWriter, r *http.Request) {
	assignerID, ok := appmw.UserIDFromContext(r.Context())
	if !ok {
		httpresponse.Error(w, apperror.New(apperror.KindUnauthorized, "not authenticated"))
		return
	}

	id, err := h.issueRef(r, "id")
	if err != nil {
		httpresponse.Error(w, err)
		return
	}

	var req assignRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpresponse.Error(w, apperror.New(apperror.KindValidation, "invalid request body"))
		return
	}
	if !req.AssigneeID.Present {
		httpresponse.Error(w, apperror.WithCode(apperror.KindValidation, validation.CodeFailed, "invalid request body").
			WithDetails(apperror.FieldError{Field: "assignee_id", Message: "es obligatorio"}))
		return
	}

	globalRole, _ := appmw.RoleFromContext(r.Context())
	var issue *domain.Issue
	if req.AssigneeID.Value == nil {
		issue, err = h.service.Unassign(r.Context(), id, assignerID, globalRole)
	} else {
		if err := validation.Struct(assignedUserValidation{AssigneeID: *req.AssigneeID.Value}); err != nil {
			httpresponse.Error(w, validation.Failed(assignedUserValidation{}, err))
			return
		}
		assigneeID := uuid.MustParse(*req.AssigneeID.Value)
		issue, err = h.service.Assign(r.Context(), id, assignerID, assigneeID, globalRole)
	}
	if err != nil {
		httpresponse.Error(w, err)
		return
	}

	h.writeIssue(w, r, http.StatusOK, issue)
}

func (h *Handler) UpdateMetadata(w http.ResponseWriter, r *http.Request) {
	actorID, ok := appmw.UserIDFromContext(r.Context())
	if !ok {
		httpresponse.Error(w, apperror.New(apperror.KindUnauthorized, "not authenticated"))
		return
	}
	globalRole, _ := appmw.RoleFromContext(r.Context())
	id, err := h.issueRef(r, "id")
	if err != nil {
		httpresponse.Error(w, err)
		return
	}
	var req updateMetadataRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpresponse.Error(w, apperror.New(apperror.KindValidation, "invalid request body"))
		return
	}
	if err := validation.Struct(req); err != nil {
		httpresponse.Error(w, validation.Failed(req, err))
		return
	}
	issue, err := h.service.UpdateMetadata(r.Context(), id, actorID, globalRole, port.UpdateMetadataInput{
		IssueType:   domain.IssueType(req.IssueType),
		IssueTypeID: parseOptionalUUID(req.IssueTypeID),
		ParentID:    parseOptionalUUID(req.ParentID),
		DueDate:     req.DueDate.value(),
		StoryPoints: req.StoryPoints,
	})
	if err != nil {
		httpresponse.Error(w, err)
		return
	}
	h.writeIssue(w, r, http.StatusOK, issue)
}

func (h *Handler) UpdateDetails(w http.ResponseWriter, r *http.Request) {
	actorID, ok := appmw.UserIDFromContext(r.Context())
	if !ok {
		httpresponse.Error(w, apperror.New(apperror.KindUnauthorized, "not authenticated"))
		return
	}
	globalRole, _ := appmw.RoleFromContext(r.Context())
	id, err := h.issueRef(r, "id")
	if err != nil {
		httpresponse.Error(w, err)
		return
	}
	var req updateDetailsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpresponse.Error(w, apperror.New(apperror.KindValidation, "invalid request body"))
		return
	}
	if err := validation.Struct(req); err != nil {
		httpresponse.Error(w, validation.Failed(req, err))
		return
	}
	issue, err := h.service.UpdateDetails(r.Context(), id, actorID, globalRole, port.UpdateDetailsInput{
		Summary:     req.Summary,
		Description: req.Description,
		Priority:    domain.Priority(req.Priority),
		PriorityID:  parseOptionalUUID(req.PriorityID),
		IssueType:   domain.IssueType(req.IssueType),
		IssueTypeID: parseOptionalUUID(req.IssueTypeID),
		ParentID:    parseOptionalUUID(req.ParentID),
		DueDate:     req.DueDate.value(),
		StoryPoints: req.StoryPoints,
		Custom:      req.Custom,
	})
	if err != nil {
		httpresponse.Error(w, err)
		return
	}
	h.writeIssue(w, r, http.StatusOK, issue)
}

func parseOptionalUUID(value string) *uuid.UUID {
	if value == "" {
		return nil
	}
	id, err := uuid.Parse(value)
	if err != nil {
		return nil
	}
	return &id
}

func (h *Handler) ListActivities(w http.ResponseWriter, r *http.Request) {
	actorID, ok := appmw.UserIDFromContext(r.Context())
	if !ok {
		httpresponse.Error(w, apperror.New(apperror.KindUnauthorized, "not authenticated"))
		return
	}
	globalRole, _ := appmw.RoleFromContext(r.Context())
	issueID, err := h.issueRef(r, "id")
	if err != nil {
		httpresponse.Error(w, err)
		return
	}
	activities, err := h.service.ListActivities(r.Context(), issueID, actorID, globalRole)
	if err != nil {
		httpresponse.Error(w, err)
		return
	}
	httpresponse.JSON(w, http.StatusOK, toActivityResponses(activities))
}

func (h *Handler) GetWatcherState(w http.ResponseWriter, r *http.Request) {
	issueID, actorID, globalRole, err := h.watcherRequestContext(r)
	if err != nil {
		httpresponse.Error(w, err)
		return
	}
	state, err := h.service.GetWatcherState(r.Context(), issueID, actorID, globalRole)
	if err != nil {
		httpresponse.Error(w, err)
		return
	}
	httpresponse.JSON(w, http.StatusOK, watcherStateResponse{IsWatching: state.IsWatching, Count: state.Count})
}

func (h *Handler) WatchIssue(w http.ResponseWriter, r *http.Request) {
	issueID, actorID, globalRole, err := h.watcherRequestContext(r)
	if err != nil {
		httpresponse.Error(w, err)
		return
	}
	state, err := h.service.WatchIssue(r.Context(), issueID, actorID, globalRole)
	if err != nil {
		httpresponse.Error(w, err)
		return
	}
	httpresponse.JSON(w, http.StatusOK, watcherStateResponse{IsWatching: state.IsWatching, Count: state.Count})
}

func (h *Handler) UnwatchIssue(w http.ResponseWriter, r *http.Request) {
	issueID, actorID, globalRole, err := h.watcherRequestContext(r)
	if err != nil {
		httpresponse.Error(w, err)
		return
	}
	state, err := h.service.UnwatchIssue(r.Context(), issueID, actorID, globalRole)
	if err != nil {
		httpresponse.Error(w, err)
		return
	}
	httpresponse.JSON(w, http.StatusOK, watcherStateResponse{IsWatching: state.IsWatching, Count: state.Count})
}

func (h *Handler) watcherRequestContext(r *http.Request) (uuid.UUID, uuid.UUID, string, error) {
	issueID, err := h.issueRef(r, "id")
	if err != nil {
		return uuid.Nil, uuid.Nil, "", err
	}
	actorID, ok := appmw.UserIDFromContext(r.Context())
	if !ok {
		return uuid.Nil, uuid.Nil, "", apperror.New(apperror.KindUnauthorized, "not authenticated")
	}
	globalRole, _ := appmw.RoleFromContext(r.Context())
	return issueID, actorID, globalRole, nil
}

func (h *Handler) UpdateRank(w http.ResponseWriter, r *http.Request) {
	actorID, ok := appmw.UserIDFromContext(r.Context())
	if !ok {
		httpresponse.Error(w, apperror.New(apperror.KindUnauthorized, "not authenticated"))
		return
	}
	globalRole, _ := appmw.RoleFromContext(r.Context())
	id, err := h.issueRef(r, "id")
	if err != nil {
		httpresponse.Error(w, err)
		return
	}
	var req updateRankRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpresponse.Error(w, apperror.New(apperror.KindValidation, "invalid request body"))
		return
	}
	if err := validation.Struct(req); err != nil {
		httpresponse.Error(w, validation.Failed(req, err))
		return
	}
	issue, err := h.service.UpdateRank(r.Context(), id, actorID, globalRole, req.Rank)
	if err != nil {
		httpresponse.Error(w, err)
		return
	}
	h.writeIssue(w, r, http.StatusOK, issue)
}

func atoiOrDefault(s string, fallback int) int {
	if s == "" {
		return fallback
	}
	v, err := strconv.Atoi(s)
	if err != nil {
		return fallback
	}
	return v
}

type backlogItemResponse struct {
	ID          string   `json:"id"`
	ProjectID   string   `json:"project_id"`
	Summary     string   `json:"summary"`
	Type        string   `json:"type"`
	Status      string   `json:"status"`
	Priority    string   `json:"priority"`
	Rank        string   `json:"rank"`
	AssigneeID  *string  `json:"assignee_id,omitempty"`
	StoryPoints *float64 `json:"story_points,omitempty"`
}

type backlogMoveRequest struct {
	Position int `json:"position"`
}

// ListBacklog devuelve las incidencias del proyecto indicado en project_id, ordenadas por rank.
func (h *Handler) ListBacklog(w http.ResponseWriter, r *http.Request) {
	actorID, ok := appmw.UserIDFromContext(r.Context())
	if !ok {
		httpresponse.Error(w, apperror.New(apperror.KindUnauthorized, "not authenticated"))
		return
	}
	globalRole, _ := appmw.RoleFromContext(r.Context())
	projectID, err := uuid.Parse(r.URL.Query().Get("project_id"))
	if err != nil {
		httpresponse.Error(w, apperror.New(apperror.KindValidation, "invalid project id"))
		return
	}
	items, err := h.service.Backlog(r.Context(), actorID, globalRole, projectID)
	if err != nil {
		httpresponse.Error(w, err)
		return
	}
	httpresponse.JSON(w, http.StatusOK, toBacklogResponse(items))
}

// MoveInBacklog coloca una incidencia en la posicion indicada del backlog y devuelve el backlog nuevo.
func (h *Handler) MoveInBacklog(w http.ResponseWriter, r *http.Request) {
	actorID, ok := appmw.UserIDFromContext(r.Context())
	if !ok {
		httpresponse.Error(w, apperror.New(apperror.KindUnauthorized, "not authenticated"))
		return
	}
	globalRole, _ := appmw.RoleFromContext(r.Context())
	projectID, err := uuid.Parse(r.URL.Query().Get("project_id"))
	if err != nil {
		httpresponse.Error(w, apperror.New(apperror.KindValidation, "invalid project id"))
		return
	}
	issueID, err := h.issueRef(r, "issue_id")
	if err != nil {
		httpresponse.Error(w, err)
		return
	}
	var body backlogMoveRequest
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		httpresponse.Error(w, apperror.New(apperror.KindValidation, "invalid request body"))
		return
	}
	items, err := h.service.MoveInBacklog(r.Context(), actorID, globalRole, projectID, issueID, body.Position)
	if err != nil {
		httpresponse.Error(w, err)
		return
	}
	httpresponse.JSON(w, http.StatusOK, toBacklogResponse(items))
}

func toBacklogResponse(items []*domain.Issue) []backlogItemResponse {
	response := make([]backlogItemResponse, 0, len(items))
	for _, item := range items {
		entry := backlogItemResponse{
			ID: item.ID.String(), ProjectID: item.ProjectID.String(), Summary: item.Summary,
			Type: string(item.IssueType), Status: string(item.Status), Priority: string(item.Priority),
			Rank: item.Rank, StoryPoints: item.StoryPoints,
		}
		if item.AssigneeID != nil {
			assignee := item.AssigneeID.String()
			entry.AssigneeID = &assignee
		}
		response = append(response, entry)
	}
	return response
}

// issueRef resolves an issue path segment that is either a UUID or a human key (CORE-12). A key is looked up
// among the issues the caller can see; organization_id in the query narrows it when the key exists in several
// organizations. Anything else is a validation error.
func (h *Handler) issueRef(r *http.Request, param string) (uuid.UUID, error) {
	segment := chi.URLParam(r, param)
	if id, err := uuid.Parse(segment); err == nil {
		return id, nil
	}
	if _, _, ok := domain.ParseKey(segment); !ok {
		return uuid.Nil, domain.ErrInvalidIssueRef
	}
	actorID, ok := appmw.UserIDFromContext(r.Context())
	if !ok {
		return uuid.Nil, apperror.New(apperror.KindUnauthorized, "not authenticated")
	}
	globalRole, _ := appmw.RoleFromContext(r.Context())
	orgID := uuid.Nil
	if value := r.URL.Query().Get("organization_id"); value != "" {
		parsed, err := uuid.Parse(value)
		if err != nil {
			return uuid.Nil, apperror.WithCode(apperror.KindValidation, "issue.organization_invalid", "organization_id debe ser un UUID valido")
		}
		orgID = parsed
	}
	return h.service.ResolveKey(r.Context(), actorID, globalRole, orgID, segment)
}
