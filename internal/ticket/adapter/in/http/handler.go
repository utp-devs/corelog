package http

import (
	"encoding/json"
	"net/http"
	"strconv"

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
	service   port.Service
	jwtSecret string
}

func NewHandler(service port.Service, jwtSecret string) *Handler {
	return &Handler{service: service, jwtSecret: jwtSecret}
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

	ticket, err := h.service.Create(r.Context(), port.CreateInput{
		Title:       req.Title,
		Description: req.Description,
		Priority:    domain.Priority(req.Priority),
		RequesterID: requesterID,
		IssueType:   domain.IssueType(req.IssueType),
		ParentID:    parseOptionalUUID(req.ParentID),
		DueDate:     req.DueDate,
		StoryPoints: req.StoryPoints,
	})
	if err != nil {
		httpresponse.Error(w, err)
		return
	}

	httpresponse.JSON(w, http.StatusCreated, toTicketResponse(ticket))
}

func (h *Handler) Get(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpresponse.Error(w, apperror.New(apperror.KindValidation, "invalid ticket id"))
		return
	}

	ticket, err := h.service.Get(r.Context(), id)
	if err != nil {
		httpresponse.Error(w, err)
		return
	}

	httpresponse.JSON(w, http.StatusOK, toTicketResponse(ticket))
}

func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()

	filter := port.ListFilter{
		Status:   domain.Status(q.Get("status")),
		Priority: domain.Priority(q.Get("priority")),
		Page:     atoiOrDefault(q.Get("page"), 1),
		PageSize: atoiOrDefault(q.Get("page_size"), 20),
	}

	tickets, err := h.service.List(r.Context(), filter)
	if err != nil {
		httpresponse.Error(w, err)
		return
	}

	httpresponse.JSON(w, http.StatusOK, toTicketResponses(tickets))
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

	actorRole, _ := appmw.RoleFromContext(r.Context())
	ticket, err := h.service.UpdateStatus(r.Context(), id, domain.Status(req.Status), actorRole)
	if err != nil {
		httpresponse.Error(w, err)
		return
	}

	httpresponse.JSON(w, http.StatusOK, toTicketResponse(ticket))
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

	ticket, err := h.service.Assign(r.Context(), id, assignerID, assigneeID)
	if err != nil {
		httpresponse.Error(w, err)
		return
	}

	httpresponse.JSON(w, http.StatusOK, toTicketResponse(ticket))
}

func (h *Handler) UpdateMetadata(w http.ResponseWriter, r *http.Request) {
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
	ticket, err := h.service.UpdateMetadata(r.Context(), id, port.UpdateMetadataInput{
		IssueType:   domain.IssueType(req.IssueType),
		ParentID:    parseOptionalUUID(req.ParentID),
		DueDate:     req.DueDate,
		StoryPoints: req.StoryPoints,
	})
	if err != nil {
		httpresponse.Error(w, err)
		return
	}
	httpresponse.JSON(w, http.StatusOK, toTicketResponse(ticket))
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
	issueID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpresponse.Error(w, apperror.New(apperror.KindValidation, "invalid issue id"))
		return
	}
	comments, err := h.service.ListComments(r.Context(), issueID)
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
		IssueID: issueID, AuthorID: authorID, Body: req.Body,
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
	var req updateCommentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpresponse.Error(w, apperror.New(apperror.KindValidation, "invalid request body"))
		return
	}
	if err := validation.Struct(req); err != nil {
		httpresponse.Error(w, apperror.Wrap(apperror.KindValidation, "invalid request body", err))
		return
	}
	comment, err := h.service.UpdateComment(r.Context(), commentID, actorID, req.Body)
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
	if err := h.service.DeleteComment(r.Context(), commentID, actorID); err != nil {
		httpresponse.Error(w, err)
		return
	}
	httpresponse.JSON(w, http.StatusNoContent, nil)
}

func (h *Handler) ListActivities(w http.ResponseWriter, r *http.Request) {
	issueID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpresponse.Error(w, apperror.New(apperror.KindValidation, "invalid issue id"))
		return
	}
	activities, err := h.service.ListActivities(r.Context(), issueID)
	if err != nil {
		httpresponse.Error(w, err)
		return
	}
	httpresponse.JSON(w, http.StatusOK, toActivityResponses(activities))
}

func (h *Handler) ListIssueLinks(w http.ResponseWriter, r *http.Request) {
	issueID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpresponse.Error(w, apperror.New(apperror.KindValidation, "invalid issue id"))
		return
	}
	links, err := h.service.ListIssueLinks(r.Context(), issueID)
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
	link, err := h.service.CreateIssueLink(r.Context(), sourceIssueID, targetIssueID, createdBy, domain.IssueLinkKind(req.Kind))
	if err != nil {
		httpresponse.Error(w, err)
		return
	}
	httpresponse.JSON(w, http.StatusCreated, toIssueLinkResponse(link))
}

func (h *Handler) DeleteIssueLink(w http.ResponseWriter, r *http.Request) {
	linkID, err := uuid.Parse(chi.URLParam(r, "link_id"))
	if err != nil {
		httpresponse.Error(w, apperror.New(apperror.KindValidation, "invalid issue link id"))
		return
	}
	if err := h.service.DeleteIssueLink(r.Context(), linkID); err != nil {
		httpresponse.Error(w, err)
		return
	}
	httpresponse.JSON(w, http.StatusNoContent, nil)
}

func (h *Handler) UpdateRank(w http.ResponseWriter, r *http.Request) {
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
	ticket, err := h.service.UpdateRank(r.Context(), id, req.Rank)
	if err != nil {
		httpresponse.Error(w, err)
		return
	}
	httpresponse.JSON(w, http.StatusOK, toTicketResponse(ticket))
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
