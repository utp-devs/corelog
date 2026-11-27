// Package http exposes the note endpoints. Every route requires authentication; ownership is enforced by the
// use case, so other users (administrators included) receive 404 for notes they do not own.
package http

import (
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/jhonsferg/corelog/internal/note/application/port"
	"github.com/jhonsferg/corelog/internal/note/domain"
	"github.com/jhonsferg/corelog/internal/platform/apperror"
	"github.com/jhonsferg/corelog/internal/platform/httpresponse"
	appmw "github.com/jhonsferg/corelog/internal/platform/middleware"
	"github.com/jhonsferg/corelog/internal/platform/pagination"
)

// maxBodyBytes bounds a request body; a full 20000-character body with JSON escaping fits comfortably.
const maxBodyBytes = 1 << 20

type Handler struct {
	service   port.Service
	jwtSecret string
}

func NewHandler(service port.Service, jwtSecret string) *Handler {
	return &Handler{service: service, jwtSecret: jwtSecret}
}

func (h *Handler) RegisterRoutes(r chi.Router) {
	r.Route("/notes", func(r chi.Router) {
		r.Use(appmw.Authenticate(h.jwtSecret))
		r.Get("/", h.List)
		r.Post("/", h.Create)
		r.Get("/tags", h.ListTags)
		r.Get("/{id}", h.Get)
		r.Patch("/{id}", h.Update)
		r.Delete("/{id}", h.Delete)
		r.Post("/{id}/pin", h.Pin)
		r.Post("/{id}/unpin", h.Unpin)
	})
}

// noteResponse is the public shape of a note.
type noteResponse struct {
	ID        string   `json:"id"`
	Title     string   `json:"title"`
	Body      string   `json:"body"`
	Color     string   `json:"color"`
	Pinned    bool     `json:"pinned"`
	Tags      []string `json:"tags"`
	IssueID   *string  `json:"issue_id"`
	ProjectID *string  `json:"project_id"`
	CreatedAt string   `json:"created_at"`
	UpdatedAt string   `json:"updated_at"`
}

// createRequest is the body of POST /notes. Unknown fields are rejected.
type createRequest struct {
	Title     string   `json:"title"`
	Body      string   `json:"body"`
	Color     string   `json:"color"`
	Tags      []string `json:"tags"`
	Pinned    bool     `json:"pinned"`
	IssueID   *string  `json:"issue_id"`
	ProjectID *string  `json:"project_id"`
}

// linkField records whether a JSON key was sent, so that an explicit null or empty string clears the link
// while an absent key leaves it unchanged.
type linkField struct {
	present bool
	value   *string
}

func (l *linkField) UnmarshalJSON(raw []byte) error {
	l.present = true
	if string(raw) == "null" {
		return nil
	}
	var s string
	if err := json.Unmarshal(raw, &s); err != nil {
		return err
	}
	l.value = &s
	return nil
}

// patchRequest is the body of PATCH /notes/{id}. Every field is optional.
type patchRequest struct {
	Title     *string   `json:"title"`
	Body      *string   `json:"body"`
	Color     *string   `json:"color"`
	Tags      *[]string `json:"tags"`
	IssueID   linkField `json:"issue_id"`
	ProjectID linkField `json:"project_id"`
}

type tagResponse struct {
	Tag   string `json:"tag"`
	Count int    `json:"count"`
}

type listMeta struct {
	Limit      int     `json:"limit"`
	NextCursor *string `json:"next_cursor"`
}

// Create handles POST /notes.
func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	userID, ok := currentUser(w, r)
	if !ok {
		return
	}
	var req createRequest
	if !decodeStrict(w, r, &req) {
		return
	}
	issueID, err := parseOptionalID("issue_id", req.IssueID)
	if err != nil {
		httpresponse.Error(w, err)
		return
	}
	projectID, err := parseOptionalID("project_id", req.ProjectID)
	if err != nil {
		httpresponse.Error(w, err)
		return
	}
	note, err := h.service.Create(r.Context(), userID, domain.Fields{
		Title: req.Title, Body: req.Body, Color: req.Color, Tags: req.Tags, Pinned: req.Pinned,
		IssueID: issueID, ProjectID: projectID,
	})
	if err != nil {
		httpresponse.Error(w, err)
		return
	}
	httpresponse.JSON(w, http.StatusCreated, toNoteResponse(note))
}

// List handles GET /notes with the filters q, tag, pinned, issue, project, cursor and limit.
func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	userID, ok := currentUser(w, r)
	if !ok {
		return
	}
	query, err := parseListQuery(r)
	if err != nil {
		httpresponse.Error(w, err)
		return
	}
	page, err := h.service.List(r.Context(), userID, query)
	if err != nil {
		httpresponse.Error(w, err)
		return
	}
	meta := listMeta{Limit: query.Limit}
	if page.NextCursor != "" {
		cursor := page.NextCursor
		meta.NextCursor = &cursor
	}
	writeList(w, toNoteResponses(page.Items), meta)
}

// ListTags handles GET /notes/tags.
func (h *Handler) ListTags(w http.ResponseWriter, r *http.Request) {
	userID, ok := currentUser(w, r)
	if !ok {
		return
	}
	tags, err := h.service.ListTags(r.Context(), userID)
	if err != nil {
		httpresponse.Error(w, err)
		return
	}
	out := make([]tagResponse, 0, len(tags))
	for _, t := range tags {
		out = append(out, tagResponse{Tag: t.Tag, Count: t.Count})
	}
	httpresponse.JSON(w, http.StatusOK, out)
}

// Get handles GET /notes/{id}.
func (h *Handler) Get(w http.ResponseWriter, r *http.Request) {
	userID, id, ok := targetContext(w, r)
	if !ok {
		return
	}
	note, err := h.service.Get(r.Context(), userID, id)
	if err != nil {
		httpresponse.Error(w, err)
		return
	}
	httpresponse.JSON(w, http.StatusOK, toNoteResponse(note))
}

// Update handles PATCH /notes/{id}.
func (h *Handler) Update(w http.ResponseWriter, r *http.Request) {
	userID, id, ok := targetContext(w, r)
	if !ok {
		return
	}
	var req patchRequest
	if !decodeStrict(w, r, &req) {
		return
	}
	patch := domain.Patch{Title: req.Title, Body: req.Body, Color: req.Color, Tags: req.Tags}
	var err error
	if req.IssueID.present {
		if patch.IssueID, err = parseOptionalID("issue_id", req.IssueID.value); err != nil {
			httpresponse.Error(w, err)
			return
		}
		patch.ClearIssue = patch.IssueID == nil
	}
	if req.ProjectID.present {
		if patch.ProjectID, err = parseOptionalID("project_id", req.ProjectID.value); err != nil {
			httpresponse.Error(w, err)
			return
		}
		patch.ClearProject = patch.ProjectID == nil
	}
	note, err := h.service.Update(r.Context(), userID, id, patch)
	if err != nil {
		httpresponse.Error(w, err)
		return
	}
	httpresponse.JSON(w, http.StatusOK, toNoteResponse(note))
}

// Delete handles DELETE /notes/{id} (soft delete).
func (h *Handler) Delete(w http.ResponseWriter, r *http.Request) {
	userID, id, ok := targetContext(w, r)
	if !ok {
		return
	}
	if err := h.service.Delete(r.Context(), userID, id); err != nil {
		httpresponse.Error(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// Pin handles POST /notes/{id}/pin. Idempotent; answers the note.
func (h *Handler) Pin(w http.ResponseWriter, r *http.Request) {
	h.setPinned(w, r, true)
}

// Unpin handles POST /notes/{id}/unpin. Idempotent; answers the note.
func (h *Handler) Unpin(w http.ResponseWriter, r *http.Request) {
	h.setPinned(w, r, false)
}

func (h *Handler) setPinned(w http.ResponseWriter, r *http.Request, pinned bool) {
	userID, id, ok := targetContext(w, r)
	if !ok {
		return
	}
	note, err := h.service.SetPinned(r.Context(), userID, id, pinned)
	if err != nil {
		httpresponse.Error(w, err)
		return
	}
	httpresponse.JSON(w, http.StatusOK, toNoteResponse(note))
}

// parseListQuery reads and validates the list filters. Invalid values are 422 with note.invalid.
func parseListQuery(r *http.Request) (port.ListQuery, error) {
	q := r.URL.Query()
	query := port.ListQuery{Query: q.Get("q"), Tag: q.Get("tag")}
	if raw := q.Get("pinned"); raw != "" {
		pinned, err := strconv.ParseBool(raw)
		if err != nil {
			return port.ListQuery{}, invalidParam("pinned", "debe ser true o false")
		}
		query.Pinned = &pinned
	}
	if raw := q.Get("issue"); raw != "" {
		id, err := uuid.Parse(raw)
		if err != nil {
			return port.ListQuery{}, invalidParam("issue", "identificador no valido")
		}
		query.IssueID = &id
	}
	if raw := q.Get("project"); raw != "" {
		id, err := uuid.Parse(raw)
		if err != nil {
			return port.ListQuery{}, invalidParam("project", "identificador no valido")
		}
		query.ProjectID = &id
	}
	if raw := q.Get("limit"); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil {
			return port.ListQuery{}, invalidParam("limit", "debe ser un entero")
		}
		query.Limit = n
	}
	if raw := q.Get("cursor"); raw != "" {
		var pos port.Position
		if err := pagination.DecodeCursor(raw, &pos); err != nil {
			return port.ListQuery{}, invalidParam("cursor", "cursor no valido")
		}
		query.After = &pos
	}
	return query, nil
}

func writeList(w http.ResponseWriter, data []noteResponse, meta listMeta) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(struct {
		Data []noteResponse `json:"data"`
		Meta listMeta       `json:"meta"`
	}{Data: data, Meta: meta})
}

func currentUser(w http.ResponseWriter, r *http.Request) (uuid.UUID, bool) {
	userID, ok := appmw.UserIDFromContext(r.Context())
	if !ok {
		httpresponse.Error(w, apperror.New(apperror.KindUnauthorized, "no autenticado"))
		return uuid.Nil, false
	}
	return userID, true
}

// targetContext reads the actor and the note id from the route. A malformed id cannot match a note: 404.
func targetContext(w http.ResponseWriter, r *http.Request) (uuid.UUID, uuid.UUID, bool) {
	userID, ok := currentUser(w, r)
	if !ok {
		return uuid.Nil, uuid.Nil, false
	}
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpresponse.Error(w, domain.ErrNotFound)
		return uuid.Nil, uuid.Nil, false
	}
	return userID, id, true
}

// decodeStrict reads one JSON object, rejecting unknown fields and trailing data. It writes the error response
// and returns false when the body is not valid.
func decodeStrict(w http.ResponseWriter, r *http.Request, dst any) bool {
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxBodyBytes))
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		httpresponse.Error(w, apperror.WithCode(apperror.KindValidation, "note.invalid", "cuerpo de la peticion no valido"))
		return false
	}
	if _, err := dec.Token(); err != io.EOF {
		httpresponse.Error(w, apperror.WithCode(apperror.KindValidation, "note.invalid", "cuerpo de la peticion no valido"))
		return false
	}
	return true
}

// parseOptionalID turns an optional identifier from the body into a uuid. Null and the empty string mean none.
func parseOptionalID(field string, raw *string) (*uuid.UUID, error) {
	if raw == nil || *raw == "" {
		return nil, nil
	}
	id, err := uuid.Parse(*raw)
	if err != nil {
		return nil, invalidParam(field, "identificador no valido")
	}
	return &id, nil
}

func invalidParam(field, message string) error {
	return apperror.WithCode(apperror.KindValidation, "note.invalid", "datos de la nota no validos").
		WithDetails(apperror.FieldError{Field: field, Message: message})
}

func toNoteResponse(n domain.Note) noteResponse {
	return noteResponse{
		ID: n.ID.String(), Title: n.Title, Body: n.Body, Color: n.Color, Pinned: n.Pinned,
		Tags: nonNilTags(n.Tags), IssueID: uuidPtrString(n.IssueID), ProjectID: uuidPtrString(n.ProjectID),
		CreatedAt: n.CreatedAt.UTC().Format(time.RFC3339), UpdatedAt: n.UpdatedAt.UTC().Format(time.RFC3339),
	}
}

func toNoteResponses(notes []domain.Note) []noteResponse {
	out := make([]noteResponse, 0, len(notes))
	for _, n := range notes {
		out = append(out, toNoteResponse(n))
	}
	return out
}

func nonNilTags(tags []string) []string {
	if tags == nil {
		return []string{}
	}
	return tags
}

func uuidPtrString(id *uuid.UUID) *string {
	if id == nil {
		return nil
	}
	s := id.String()
	return &s
}
