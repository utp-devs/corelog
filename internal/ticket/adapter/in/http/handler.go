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
	"github.com/jhonsferg/corelog/internal/platform/validation"
	"github.com/jhonsferg/corelog/internal/ticket/application/port"
	"github.com/jhonsferg/corelog/internal/ticket/domain"
)

type Handler struct {
	service     port.Service
	jwtSecret   string
	attachments port.AttachmentRepository
	files       port.FileStorage
}

func NewHandler(service port.Service, jwtSecret string) *Handler {
	return &Handler{service: service, jwtSecret: jwtSecret}
}

func (h *Handler) WithAttachments(repo port.AttachmentRepository, files port.FileStorage) *Handler {
	h.attachments, h.files = repo, files
	return h
}

func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	requesterID, ok := appmw.UserIDFromContext(r.Context())
	if !ok {
		httpresponse.Error(w, apperror.New(apperror.KindUnauthorized, "not authenticated"))
		return
	}

	var req createTicketRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpresponse.Error(w, apperror.New(apperror.KindValidation, "invalid request body"))
		return
	}
	if err := validation.Struct(req); err != nil {
		httpresponse.Error(w, apperror.Wrap(apperror.KindValidation, "invalid request body", err))
		return
	}
	globalRole, _ := appmw.RoleFromContext(r.Context())

	ticket, err := h.service.Create(r.Context(), port.CreateInput{
		Title:       req.Title,
		Description: req.Description,
		Priority:    domain.Priority(req.Priority),
		RequesterID: requesterID,
		GlobalRole:  globalRole,
		ProjectID:   parseOptionalUUID(req.ProjectID),
		IssueType:   domain.IssueType(req.IssueType),
		ParentID:    parseOptionalUUID(req.ParentID),
		DueDate:     req.DueDate,
		StoryPoints: req.StoryPoints,
	})
	if err != nil {
		httpresponse.Error(w, err)
		return
	}

	h.writeTicket(w, r, http.StatusCreated, ticket)
}

// writeTicket responde con una incidencia incluyendo el nombre de su solicitante y responsable.
func (h *Handler) writeTicket(w http.ResponseWriter, r *http.Request, status int, ticket *domain.Ticket) {
	names, err := h.service.UserNames(r.Context(), personIDs(ticket))
	if err != nil {
		httpresponse.Error(w, err)
		return
	}
	httpresponse.JSON(w, status, toTicketResponse(ticket, names))
}

func (h *Handler) Get(w http.ResponseWriter, r *http.Request) {
	actorID, ok := appmw.UserIDFromContext(r.Context())
	if !ok {
		httpresponse.Error(w, apperror.New(apperror.KindUnauthorized, "not authenticated"))
		return
	}
	globalRole, _ := appmw.RoleFromContext(r.Context())
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpresponse.Error(w, apperror.New(apperror.KindValidation, "invalid ticket id"))
		return
	}

	ticket, err := h.service.Get(r.Context(), id, actorID, globalRole)
	if err != nil {
		httpresponse.Error(w, err)
		return
	}

	h.writeTicket(w, r, http.StatusOK, ticket)
}

func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	actorID, ok := appmw.UserIDFromContext(r.Context())
	if !ok {
		httpresponse.Error(w, apperror.New(apperror.KindUnauthorized, "not authenticated"))
		return
	}
	globalRole, _ := appmw.RoleFromContext(r.Context())
	q := r.URL.Query()

	filter := port.ListFilter{
		Status:    domain.Status(q.Get("status")),
		Priority:  domain.Priority(q.Get("priority")),
		Query:     q.Get("q"),
		IssueType: domain.IssueType(q.Get("issue_type")),
		Page:      atoiOrDefault(q.Get("page"), 1),
		PageSize:  atoiOrDefault(q.Get("page_size"), 20),
	}

	if labelID := q.Get("label_id"); labelID != "" {
		id, err := uuid.Parse(labelID)
		if err != nil {
			httpresponse.Error(w, apperror.New(apperror.KindValidation, "invalid label id"))
			return
		}
		filter.LabelID = id
	}
	if assigneeID := q.Get("assignee_id"); assigneeID != "" {
		id, err := uuid.Parse(assigneeID)
		if err != nil {
			httpresponse.Error(w, apperror.New(apperror.KindValidation, "invalid assignee id"))
			return
		}
		filter.AssigneeID = &id
	}
	if dueFrom := q.Get("due_from"); dueFrom != "" {
		value, err := time.Parse(time.RFC3339, dueFrom)
		if err != nil {
			httpresponse.Error(w, apperror.New(apperror.KindValidation, "invalid due_from"))
			return
		}
		filter.DueFrom = &value
	}
	if dueTo := q.Get("due_to"); dueTo != "" {
		value, err := time.Parse(time.RFC3339, dueTo)
		if err != nil {
			httpresponse.Error(w, apperror.New(apperror.KindValidation, "invalid due_to"))
			return
		}
		filter.DueTo = &value
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

	httpresponse.Paginated(w, http.StatusOK, toTicketResponses(result.Items, names), httpresponse.PageMeta{
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
		httpresponse.Error(w, apperror.Wrap(apperror.KindValidation, "invalid request body", err))
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

func (h *Handler) UpdateStatus(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpresponse.Error(w, apperror.New(apperror.KindValidation, "invalid ticket id"))
		return
	}

	var req updateStatusRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpresponse.Error(w, apperror.New(apperror.KindValidation, "invalid request body"))
		return
	}
	if err := validation.Struct(req); err != nil {
		httpresponse.Error(w, apperror.Wrap(apperror.KindValidation, "invalid request body", err))
		return
	}

	actorID, ok := appmw.UserIDFromContext(r.Context())
	if !ok {
		httpresponse.Error(w, apperror.New(apperror.KindUnauthorized, "not authenticated"))
		return
	}
	globalRole, _ := appmw.RoleFromContext(r.Context())
	ticket, err := h.service.UpdateStatus(r.Context(), id, actorID, globalRole, domain.Status(req.Status))
	if err != nil {
		httpresponse.Error(w, err)
		return
	}

	h.writeTicket(w, r, http.StatusOK, ticket)
}

func (h *Handler) Assign(w http.ResponseWriter, r *http.Request) {
	assignerID, ok := appmw.UserIDFromContext(r.Context())
	if !ok {
		httpresponse.Error(w, apperror.New(apperror.KindUnauthorized, "not authenticated"))
		return
	}

	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpresponse.Error(w, apperror.New(apperror.KindValidation, "invalid ticket id"))
		return
	}

	var req assignRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpresponse.Error(w, apperror.New(apperror.KindValidation, "invalid request body"))
		return
	}
	if err := validation.Struct(req); err != nil {
		httpresponse.Error(w, apperror.Wrap(apperror.KindValidation, "invalid request body", err))
		return
	}

	assigneeID, err := uuid.Parse(req.AssigneeID)
	if err != nil {
		httpresponse.Error(w, apperror.New(apperror.KindValidation, "invalid assignee id"))
		return
	}

	globalRole, _ := appmw.RoleFromContext(r.Context())
	ticket, err := h.service.Assign(r.Context(), id, assignerID, assigneeID, globalRole)
	if err != nil {
		httpresponse.Error(w, err)
		return
	}

	h.writeTicket(w, r, http.StatusOK, ticket)
}

func (h *Handler) UpdateMetadata(w http.ResponseWriter, r *http.Request) {
	actorID, ok := appmw.UserIDFromContext(r.Context())
	if !ok {
		httpresponse.Error(w, apperror.New(apperror.KindUnauthorized, "not authenticated"))
		return
	}
	globalRole, _ := appmw.RoleFromContext(r.Context())
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpresponse.Error(w, apperror.New(apperror.KindValidation, "invalid ticket id"))
		return
	}
	var req updateMetadataRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpresponse.Error(w, apperror.New(apperror.KindValidation, "invalid request body"))
		return
	}
	if err := validation.Struct(req); err != nil {
		httpresponse.Error(w, apperror.Wrap(apperror.KindValidation, "invalid request body", err))
		return
	}
	ticket, err := h.service.UpdateMetadata(r.Context(), id, actorID, globalRole, port.UpdateMetadataInput{
		IssueType:   domain.IssueType(req.IssueType),
		ParentID:    parseOptionalUUID(req.ParentID),
		DueDate:     req.DueDate,
		StoryPoints: req.StoryPoints,
	})
	if err != nil {
		httpresponse.Error(w, err)
		return
	}
	h.writeTicket(w, r, http.StatusOK, ticket)
}

func (h *Handler) UpdateDetails(w http.ResponseWriter, r *http.Request) {
	actorID, ok := appmw.UserIDFromContext(r.Context())
	if !ok {
		httpresponse.Error(w, apperror.New(apperror.KindUnauthorized, "not authenticated"))
		return
	}
	globalRole, _ := appmw.RoleFromContext(r.Context())
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpresponse.Error(w, apperror.New(apperror.KindValidation, "invalid issue id"))
		return
	}
	var req updateDetailsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpresponse.Error(w, apperror.New(apperror.KindValidation, "invalid request body"))
		return
	}
	if err := validation.Struct(req); err != nil {
		httpresponse.Error(w, apperror.Wrap(apperror.KindValidation, "invalid request body", err))
		return
	}
	ticket, err := h.service.UpdateDetails(r.Context(), id, actorID, globalRole, port.UpdateDetailsInput{
		Title:       req.Title,
		Description: req.Description,
		Priority:    domain.Priority(req.Priority),
		IssueType:   domain.IssueType(req.IssueType),
		ParentID:    parseOptionalUUID(req.ParentID),
		DueDate:     req.DueDate,
		StoryPoints: req.StoryPoints,
	})
	if err != nil {
		httpresponse.Error(w, err)
		return
	}
	h.writeTicket(w, r, http.StatusOK, ticket)
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

func (h *Handler) ListComments(w http.ResponseWriter, r *http.Request) {
	actorID, ok := appmw.UserIDFromContext(r.Context())
	if !ok {
		httpresponse.Error(w, apperror.New(apperror.KindUnauthorized, "not authenticated"))
		return
	}
	globalRole, _ := appmw.RoleFromContext(r.Context())
	issueID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpresponse.Error(w, apperror.New(apperror.KindValidation, "invalid issue id"))
		return
	}
	comments, err := h.service.ListComments(r.Context(), issueID, actorID, globalRole)
	if err != nil {
		httpresponse.Error(w, err)
		return
	}
	httpresponse.JSON(w, http.StatusOK, toCommentResponses(comments))
}

func (h *Handler) CreateComment(w http.ResponseWriter, r *http.Request) {
	issueID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpresponse.Error(w, apperror.New(apperror.KindValidation, "invalid issue id"))
		return
	}
	authorID, ok := appmw.UserIDFromContext(r.Context())
	if !ok {
		httpresponse.Error(w, apperror.New(apperror.KindUnauthorized, "not authenticated"))
		return
	}
	globalRole, _ := appmw.RoleFromContext(r.Context())
	var req createCommentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpresponse.Error(w, apperror.New(apperror.KindValidation, "invalid request body"))
		return
	}
	if err := validation.Struct(req); err != nil {
		httpresponse.Error(w, apperror.Wrap(apperror.KindValidation, "invalid request body", err))
		return
	}
	comment, err := h.service.CreateComment(r.Context(), port.CreateCommentInput{
		IssueID: issueID, AuthorID: authorID, GlobalRole: globalRole, Body: req.Body,
	})
	if err != nil {
		httpresponse.Error(w, err)
		return
	}
	httpresponse.JSON(w, http.StatusCreated, toCommentResponse(comment))
}

func (h *Handler) UpdateComment(w http.ResponseWriter, r *http.Request) {
	commentID, err := uuid.Parse(chi.URLParam(r, "comment_id"))
	if err != nil {
		httpresponse.Error(w, apperror.New(apperror.KindValidation, "invalid comment id"))
		return
	}
	actorID, ok := appmw.UserIDFromContext(r.Context())
	if !ok {
		httpresponse.Error(w, apperror.New(apperror.KindUnauthorized, "not authenticated"))
		return
	}
	globalRole, _ := appmw.RoleFromContext(r.Context())
	var req updateCommentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpresponse.Error(w, apperror.New(apperror.KindValidation, "invalid request body"))
		return
	}
	if err := validation.Struct(req); err != nil {
		httpresponse.Error(w, apperror.Wrap(apperror.KindValidation, "invalid request body", err))
		return
	}
	comment, err := h.service.UpdateComment(r.Context(), commentID, actorID, globalRole, req.Body)
	if err != nil {
		httpresponse.Error(w, err)
		return
	}
	httpresponse.JSON(w, http.StatusOK, toCommentResponse(comment))
}

func (h *Handler) DeleteComment(w http.ResponseWriter, r *http.Request) {
	commentID, err := uuid.Parse(chi.URLParam(r, "comment_id"))
	if err != nil {
		httpresponse.Error(w, apperror.New(apperror.KindValidation, "invalid comment id"))
		return
	}
	actorID, ok := appmw.UserIDFromContext(r.Context())
	if !ok {
		httpresponse.Error(w, apperror.New(apperror.KindUnauthorized, "not authenticated"))
		return
	}
	globalRole, _ := appmw.RoleFromContext(r.Context())
	if err := h.service.DeleteComment(r.Context(), commentID, actorID, globalRole); err != nil {
		httpresponse.Error(w, err)
		return
	}
	httpresponse.JSON(w, http.StatusNoContent, nil)
}

func (h *Handler) ListActivities(w http.ResponseWriter, r *http.Request) {
	actorID, ok := appmw.UserIDFromContext(r.Context())
	if !ok {
		httpresponse.Error(w, apperror.New(apperror.KindUnauthorized, "not authenticated"))
		return
	}
	globalRole, _ := appmw.RoleFromContext(r.Context())
	issueID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpresponse.Error(w, apperror.New(apperror.KindValidation, "invalid issue id"))
		return
	}
	activities, err := h.service.ListActivities(r.Context(), issueID, actorID, globalRole)
	if err != nil {
		httpresponse.Error(w, err)
		return
	}
	httpresponse.JSON(w, http.StatusOK, toActivityResponses(activities))
}

func (h *Handler) ListIssueLinks(w http.ResponseWriter, r *http.Request) {
	actorID, ok := appmw.UserIDFromContext(r.Context())
	if !ok {
		httpresponse.Error(w, apperror.New(apperror.KindUnauthorized, "not authenticated"))
		return
	}
	globalRole, _ := appmw.RoleFromContext(r.Context())
	issueID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpresponse.Error(w, apperror.New(apperror.KindValidation, "invalid issue id"))
		return
	}
	links, err := h.service.ListIssueLinks(r.Context(), issueID, actorID, globalRole)
	if err != nil {
		httpresponse.Error(w, err)
		return
	}
	httpresponse.JSON(w, http.StatusOK, toIssueLinkResponses(links))
}

func (h *Handler) CreateIssueLink(w http.ResponseWriter, r *http.Request) {
	sourceIssueID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpresponse.Error(w, apperror.New(apperror.KindValidation, "invalid issue id"))
		return
	}
	createdBy, ok := appmw.UserIDFromContext(r.Context())
	if !ok {
		httpresponse.Error(w, apperror.New(apperror.KindUnauthorized, "not authenticated"))
		return
	}
	var req createIssueLinkRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpresponse.Error(w, apperror.New(apperror.KindValidation, "invalid request body"))
		return
	}
	if err := validation.Struct(req); err != nil {
		httpresponse.Error(w, apperror.Wrap(apperror.KindValidation, "invalid request body", err))
		return
	}
	targetIssueID, err := uuid.Parse(req.TargetIssueID)
	if err != nil {
		httpresponse.Error(w, apperror.New(apperror.KindValidation, "invalid target issue id"))
		return
	}
	globalRole, _ := appmw.RoleFromContext(r.Context())
	link, err := h.service.CreateIssueLink(r.Context(), sourceIssueID, targetIssueID, createdBy, globalRole, domain.IssueLinkKind(req.Kind))
	if err != nil {
		httpresponse.Error(w, err)
		return
	}
	httpresponse.JSON(w, http.StatusCreated, toIssueLinkResponse(link))
}

func (h *Handler) DeleteIssueLink(w http.ResponseWriter, r *http.Request) {
	issueID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpresponse.Error(w, apperror.New(apperror.KindValidation, "invalid issue id"))
		return
	}
	actorID, ok := appmw.UserIDFromContext(r.Context())
	if !ok {
		httpresponse.Error(w, apperror.New(apperror.KindUnauthorized, "not authenticated"))
		return
	}
	globalRole, _ := appmw.RoleFromContext(r.Context())
	linkID, err := uuid.Parse(chi.URLParam(r, "link_id"))
	if err != nil {
		httpresponse.Error(w, apperror.New(apperror.KindValidation, "invalid issue link id"))
		return
	}
	if err := h.service.DeleteIssueLink(r.Context(), issueID, linkID, actorID, globalRole); err != nil {
		httpresponse.Error(w, err)
		return
	}
	httpresponse.JSON(w, http.StatusNoContent, nil)
}

func (h *Handler) GetWatcherState(w http.ResponseWriter, r *http.Request) {
	issueID, actorID, globalRole, err := watcherRequestContext(r)
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
	issueID, actorID, globalRole, err := watcherRequestContext(r)
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
	issueID, actorID, globalRole, err := watcherRequestContext(r)
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

func watcherRequestContext(r *http.Request) (uuid.UUID, uuid.UUID, string, error) {
	issueID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		return uuid.Nil, uuid.Nil, "", apperror.New(apperror.KindValidation, "invalid issue id")
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
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpresponse.Error(w, apperror.New(apperror.KindValidation, "invalid issue id"))
		return
	}
	var req updateRankRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpresponse.Error(w, apperror.New(apperror.KindValidation, "invalid request body"))
		return
	}
	if err := validation.Struct(req); err != nil {
		httpresponse.Error(w, apperror.Wrap(apperror.KindValidation, "invalid request body", err))
		return
	}
	ticket, err := h.service.UpdateRank(r.Context(), id, actorID, globalRole, req.Rank)
	if err != nil {
		httpresponse.Error(w, err)
		return
	}
	h.writeTicket(w, r, http.StatusOK, ticket)
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
