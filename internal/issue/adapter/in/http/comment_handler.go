package http

import (
	"bytes"
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
)

// maxCommentBody bounds a comment request in bytes. The 20000 character limit of the body is enforced by the domain.
const maxCommentBody = 128 << 10

// commentActor reads the authenticated caller of a comment request.
func commentActor(r *http.Request) (port.CommentActor, error) {
	id, ok := appmw.UserIDFromContext(r.Context())
	if !ok {
		return port.CommentActor{}, apperror.New(apperror.KindUnauthorized, "not authenticated")
	}
	globalRole, _ := appmw.RoleFromContext(r.Context())
	return port.CommentActor{ID: id, GlobalRole: globalRole}, nil
}

// commentService returns the comment service or an internal error when the module is not wired.
func (h *Handler) commentService() (port.CommentService, error) {
	if h.comments == nil {
		return nil, apperror.New(apperror.KindInternal, "comments are not available")
	}
	return h.comments, nil
}

// decodeCommentBody reads a strict JSON body: unknown fields and trailing data are rejected.
func decodeCommentBody(w http.ResponseWriter, r *http.Request, dst any) error {
	raw, err := io.ReadAll(http.MaxBytesReader(w, r.Body, maxCommentBody))
	if err != nil {
		return apperror.New(apperror.KindValidation, "invalid request body")
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(dst); err != nil {
		return apperror.New(apperror.KindValidation, "invalid request body")
	}
	if decoder.More() {
		return apperror.New(apperror.KindValidation, "invalid request body")
	}
	return nil
}

func parseCommentID(r *http.Request) (uuid.UUID, error) {
	id, err := uuid.Parse(chi.URLParam(r, "comment_id"))
	if err != nil {
		return uuid.Nil, apperror.New(apperror.KindValidation, "invalid comment id")
	}
	return id, nil
}

// ListComments serves GET /issues/{id}/comments: root comments with their replies, oldest first unless sort=desc.
// Query: sort=asc|desc, cursor, limit (1..100, default 50). The envelope is {data: threads, meta: {limit,
// next_cursor}}.
func (h *Handler) ListComments(w http.ResponseWriter, r *http.Request) {
	actor, err := commentActor(r)
	if err != nil {
		httpresponse.Error(w, err)
		return
	}
	service, err := h.commentService()
	if err != nil {
		httpresponse.Error(w, err)
		return
	}
	issueID, err := h.issueRef(r, "id")
	if err != nil {
		httpresponse.Error(w, err)
		return
	}
	query := r.URL.Query()
	input := port.ThreadPageInput{Sort: query.Get("sort"), Cursor: query.Get("cursor")}
	if raw := query.Get("limit"); raw != "" {
		limit, err := strconv.Atoi(raw)
		if err != nil {
			httpresponse.Error(w, domain.ErrCommentLimitInvalid)
			return
		}
		input.Limit, input.LimitGiven = limit, true
	}
	page, err := service.ListThreads(r.Context(), actor, issueID, input)
	if err != nil {
		httpresponse.Error(w, err)
		return
	}
	limit, err := domain.NormalizeCommentLimit(input.Limit, input.LimitGiven)
	if err != nil {
		httpresponse.Error(w, err)
		return
	}
	threads := make([]commentResponse, 0, len(page.Threads))
	for _, view := range page.Threads {
		threads = append(threads, toThreadResponse(view))
	}
	meta := commentPageMeta{Limit: limit}
	if page.NextCursor != "" {
		cursor := page.NextCursor
		meta.NextCursor = &cursor
	}
	writeCommentPage(w, threads, meta)
}

// writeCommentPage writes the thread list, where data is the array of threads and meta sits beside it.
func writeCommentPage(w http.ResponseWriter, data []commentResponse, meta commentPageMeta) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(struct {
		Data []commentResponse `json:"data"`
		Meta commentPageMeta   `json:"meta"`
	}{Data: data, Meta: meta})
}

// CreateComment serves POST /issues/{id}/comments with body, optional parent_id, visibility and internal_role.
func (h *Handler) CreateComment(w http.ResponseWriter, r *http.Request) {
	actor, err := commentActor(r)
	if err != nil {
		httpresponse.Error(w, err)
		return
	}
	service, err := h.commentService()
	if err != nil {
		httpresponse.Error(w, err)
		return
	}
	issueID, err := h.issueRef(r, "id")
	if err != nil {
		httpresponse.Error(w, err)
		return
	}
	var req commentCreateRequest
	if err := decodeCommentBody(w, r, &req); err != nil {
		httpresponse.Error(w, err)
		return
	}
	input := port.CommentCreateInput{
		IssueID: issueID, Body: req.Body, Visibility: req.Visibility, InternalRole: req.InternalRole,
	}
	if req.ParentID != nil {
		parent, err := uuid.Parse(*req.ParentID)
		if err != nil {
			httpresponse.Error(w, domain.ErrCommentParentInvalid)
			return
		}
		input.ParentID = &parent
	}
	result, err := service.CreateComment(r.Context(), actor, input)
	if err != nil {
		httpresponse.Error(w, err)
		return
	}
	httpresponse.JSON(w, http.StatusCreated, toCommentWriteResponse(result))
}

// UpdateComment serves PATCH /comments/{id} and the legacy PATCH /issues/{id}/comments/{comment_id}.
func (h *Handler) UpdateComment(w http.ResponseWriter, r *http.Request) {
	actor, err := commentActor(r)
	if err != nil {
		httpresponse.Error(w, err)
		return
	}
	service, err := h.commentService()
	if err != nil {
		httpresponse.Error(w, err)
		return
	}
	commentID, err := parseCommentID(r)
	if err != nil {
		httpresponse.Error(w, err)
		return
	}
	var req commentUpdateRequest
	if err := decodeCommentBody(w, r, &req); err != nil {
		httpresponse.Error(w, err)
		return
	}
	result, err := service.UpdateComment(r.Context(), actor, commentID, req.Body)
	if err != nil {
		httpresponse.Error(w, err)
		return
	}
	httpresponse.JSON(w, http.StatusOK, toCommentWriteResponse(result))
}

// DeleteComment serves DELETE /comments/{id} and the legacy DELETE /issues/{id}/comments/{comment_id}.
func (h *Handler) DeleteComment(w http.ResponseWriter, r *http.Request) {
	actor, err := commentActor(r)
	if err != nil {
		httpresponse.Error(w, err)
		return
	}
	service, err := h.commentService()
	if err != nil {
		httpresponse.Error(w, err)
		return
	}
	commentID, err := parseCommentID(r)
	if err != nil {
		httpresponse.Error(w, err)
		return
	}
	if err := service.DeleteComment(r.Context(), actor, commentID); err != nil {
		httpresponse.Error(w, err)
		return
	}
	httpresponse.JSON(w, http.StatusNoContent, nil)
}

// ListCommentRevisions serves GET /comments/{id}/revisions: the previous bodies, newest first.
func (h *Handler) ListCommentRevisions(w http.ResponseWriter, r *http.Request) {
	actor, err := commentActor(r)
	if err != nil {
		httpresponse.Error(w, err)
		return
	}
	service, err := h.commentService()
	if err != nil {
		httpresponse.Error(w, err)
		return
	}
	commentID, err := parseCommentID(r)
	if err != nil {
		httpresponse.Error(w, err)
		return
	}
	revisions, err := service.ListRevisions(r.Context(), actor, commentID)
	if err != nil {
		httpresponse.Error(w, err)
		return
	}
	httpresponse.JSON(w, http.StatusOK, toRevisionResponses(revisions))
}

// PutCommentReaction serves PUT /comments/{id}/reactions/{emoji}. Repeating it leaves the same state.
func (h *Handler) PutCommentReaction(w http.ResponseWriter, r *http.Request) {
	h.changeReaction(w, r, true)
}

// DeleteCommentReaction serves DELETE /comments/{id}/reactions/{emoji}.
func (h *Handler) DeleteCommentReaction(w http.ResponseWriter, r *http.Request) {
	h.changeReaction(w, r, false)
}

func (h *Handler) changeReaction(w http.ResponseWriter, r *http.Request, on bool) {
	actor, err := commentActor(r)
	if err != nil {
		httpresponse.Error(w, err)
		return
	}
	service, err := h.commentService()
	if err != nil {
		httpresponse.Error(w, err)
		return
	}
	commentID, err := parseCommentID(r)
	if err != nil {
		httpresponse.Error(w, err)
		return
	}
	reactions, err := service.SetReaction(r.Context(), actor, commentID, chi.URLParam(r, "emoji"), on)
	if err != nil {
		httpresponse.Error(w, err)
		return
	}
	httpresponse.JSON(w, http.StatusOK, toReactionsResponse(reactions))
}
