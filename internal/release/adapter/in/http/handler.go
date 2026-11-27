// Package http exposes the versions and release notes of a project.
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
	"github.com/jhonsferg/corelog/internal/release/application/port"
	"github.com/jhonsferg/corelog/internal/release/application/usecase"
	"github.com/jhonsferg/corelog/internal/release/domain"
)

// maxBody bounds the body of version writes.
const maxBody = 1 << 20

// Handler serves the version routes.
type Handler struct {
	service   port.Service
	projects  port.ProjectKeys
	jwtSecret string
}

// NewHandler builds the handler. projects resolves project keys in the {id} segment.
func NewHandler(service port.Service, projects port.ProjectKeys, jwtSecret string) *Handler {
	return &Handler{service: service, projects: projects, jwtSecret: jwtSecret}
}

// RegisterRoutes mounts the version routes. Authentication is the only middleware; authorization is in the use case.
func (h *Handler) RegisterRoutes(r chi.Router) {
	r.Route("/projects/{id}/versions", func(r chi.Router) {
		r.Use(appmw.Authenticate(h.jwtSecret))
		r.Get("/", h.List)
		r.Post("/", h.Create)
	})
	r.Route("/versions/{id}", func(r chi.Router) {
		r.Use(appmw.Authenticate(h.jwtSecret))
		r.Get("/", h.Get)
		r.Patch("/", h.Update)
		r.Delete("/", h.Delete)
		r.Post("/release", h.Release)
		r.Post("/archive", h.Archive)
		r.Get("/notes", h.Notes)
	})
}

type progressResponse struct {
	Issues issueProgressResponse  `json:"issues"`
	Points pointsProgressResponse `json:"points"`
}

type issueProgressResponse struct {
	Total      int            `json:"total"`
	Done       int            `json:"done"`
	ByCategory map[string]int `json:"by_category"`
}

type pointsProgressResponse struct {
	Total float64 `json:"total"`
	Done  float64 `json:"done"`
}

type versionResponse struct {
	ID          string           `json:"id"`
	ProjectID   string           `json:"project_id"`
	Name        string           `json:"name"`
	Description string           `json:"description"`
	StartDate   *string          `json:"start_date"`
	ReleaseDate *string          `json:"release_date"`
	Status      string           `json:"status"`
	ReleasedAt  *time.Time       `json:"released_at"`
	ArchivedAt  *time.Time       `json:"archived_at"`
	IssuesTotal int              `json:"issues_total"`
	IssuesDone  int              `json:"issues_done"`
	Progress    progressResponse `json:"progress"`
	CreatedAt   time.Time        `json:"created_at"`
	UpdatedAt   time.Time        `json:"updated_at"`
}

func toVersionResponse(v *domain.Version) versionResponse {
	byCategory := v.Progress.ByCategory
	if byCategory == nil {
		byCategory = map[string]int{domain.CategoryTodo: 0, domain.CategoryInProgress: 0, domain.CategoryDone: 0}
	}
	return versionResponse{
		ID: v.ID.String(), ProjectID: v.ProjectID.String(), Name: v.Name, Description: v.Description,
		StartDate: dateString(v.StartDate), ReleaseDate: dateString(v.ReleaseDate), Status: string(v.Status),
		ReleasedAt: v.ReleasedAt, ArchivedAt: v.ArchivedAt,
		IssuesTotal: v.Progress.IssuesTotal, IssuesDone: v.Progress.IssuesDone,
		Progress: progressResponse{
			Issues: issueProgressResponse{Total: v.Progress.IssuesTotal, Done: v.Progress.IssuesDone, ByCategory: byCategory},
			Points: pointsProgressResponse{Total: v.Progress.PointsTotal, Done: v.Progress.PointsDone},
		},
		CreatedAt: v.CreatedAt, UpdatedAt: v.UpdatedAt,
	}
}

func dateString(t *time.Time) *string {
	if t == nil {
		return nil
	}
	s := t.Format(domain.DateLayout)
	return &s
}

type createVersionRequest struct {
	Name        string  `json:"name"`
	Description string  `json:"description"`
	StartDate   *string `json:"start_date"`
	ReleaseDate *string `json:"release_date"`
}

// optionalDate tells an absent date (not set), null (clear) and a value apart.
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

type updateVersionRequest struct {
	Name        *string      `json:"name"`
	Description *string      `json:"description"`
	StartDate   optionalDate `json:"start_date"`
	ReleaseDate optionalDate `json:"release_date"`
	Status      *string      `json:"status"`
}

type releaseVersionRequest struct {
	ReleaseDate      *string         `json:"release_date"`
	MoveUnresolvedTo json.RawMessage `json:"move_unresolved_to"`
}

type releaseResponse struct {
	Version          versionResponse  `json:"version"`
	MovedIssues      int              `json:"moved_issues"`
	UnresolvedIssues int              `json:"unresolved_issues"`
	NextVersion      *versionResponse `json:"next_version"`
}

type noteIssueResponse struct {
	ID      string `json:"id"`
	Key     string `json:"key"`
	Summary string `json:"summary"`
	Done    bool   `json:"done"`
}

type noteGroupResponse struct {
	Title  string              `json:"title"`
	Issues []noteIssueResponse `json:"issues"`
}

type notesResponse struct {
	VersionID   string              `json:"version_id"`
	VersionName string              `json:"version_name"`
	Status      string              `json:"status"`
	ReleaseDate *string             `json:"release_date"`
	IssueCount  int                 `json:"issue_count"`
	Markdown    string              `json:"markdown"`
	Groups      []noteGroupResponse `json:"groups"`
}

func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	actorID, globalRole, projectID, ok := h.projectContext(w, r)
	if !ok {
		return
	}
	var filter port.ListFilter
	if raw := r.URL.Query().Get("status"); raw != "" {
		status := domain.Status(raw)
		if !status.Valid() {
			httpresponse.Error(w, domain.ErrVersionStatusInvalid)
			return
		}
		filter.Status = &status
	}
	versions, err := h.service.List(r.Context(), actorID, globalRole, projectID, filter)
	if err != nil {
		httpresponse.Error(w, err)
		return
	}
	out := make([]versionResponse, 0, len(versions))
	for _, v := range versions {
		out = append(out, toVersionResponse(v))
	}
	httpresponse.JSON(w, http.StatusOK, out)
}

func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	actorID, globalRole, projectID, ok := h.projectContext(w, r)
	if !ok {
		return
	}
	var body createVersionRequest
	if err := decodeStrict(r, &body, false); err != nil {
		httpresponse.Error(w, err)
		return
	}
	start, err := parseDate(body.StartDate)
	if err != nil {
		httpresponse.Error(w, err)
		return
	}
	release, err := parseDate(body.ReleaseDate)
	if err != nil {
		httpresponse.Error(w, err)
		return
	}
	v, err := h.service.Create(r.Context(), actorID, globalRole, projectID, port.CreateInput{
		Name: body.Name, Description: body.Description, StartDate: start, ReleaseDate: release,
	})
	if err != nil {
		httpresponse.Error(w, err)
		return
	}
	httpresponse.JSON(w, http.StatusCreated, toVersionResponse(v))
}

func (h *Handler) Get(w http.ResponseWriter, r *http.Request) {
	actorID, globalRole, id, ok := versionContext(w, r)
	if !ok {
		return
	}
	v, err := h.service.Get(r.Context(), actorID, globalRole, id)
	if err != nil {
		httpresponse.Error(w, err)
		return
	}
	httpresponse.JSON(w, http.StatusOK, toVersionResponse(v))
}

func (h *Handler) Update(w http.ResponseWriter, r *http.Request) {
	actorID, globalRole, id, ok := versionContext(w, r)
	if !ok {
		return
	}
	var body updateVersionRequest
	if err := decodeStrict(r, &body, false); err != nil {
		httpresponse.Error(w, err)
		return
	}
	patch := domain.VersionPatch{Name: body.Name, Description: body.Description}
	var err error
	if patch.StartDateSet, patch.StartDate, err = patchDate(body.StartDate); err != nil {
		httpresponse.Error(w, err)
		return
	}
	if patch.ReleaseDateSet, patch.ReleaseDate, err = patchDate(body.ReleaseDate); err != nil {
		httpresponse.Error(w, err)
		return
	}
	if body.Status != nil {
		status := domain.Status(*body.Status)
		if !status.Valid() {
			httpresponse.Error(w, domain.ErrVersionStatusInvalid)
			return
		}
		patch.Status = &status
	}
	v, err := h.service.Update(r.Context(), actorID, globalRole, id, patch)
	if err != nil {
		httpresponse.Error(w, err)
		return
	}
	httpresponse.JSON(w, http.StatusOK, toVersionResponse(v))
}

func (h *Handler) Delete(w http.ResponseWriter, r *http.Request) {
	actorID, globalRole, id, ok := versionContext(w, r)
	if !ok {
		return
	}
	var moveTo *uuid.UUID
	if raw := r.URL.Query().Get("move_to"); raw != "" {
		parsed, err := uuid.Parse(raw)
		if err != nil {
			httpresponse.Error(w, domain.ErrVersionMoveTarget)
			return
		}
		moveTo = &parsed
	}
	if err := h.service.Delete(r.Context(), actorID, globalRole, id, moveTo); err != nil {
		httpresponse.Error(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) Release(w http.ResponseWriter, r *http.Request) {
	actorID, globalRole, id, ok := versionContext(w, r)
	if !ok {
		return
	}
	var body releaseVersionRequest
	if err := decodeStrict(r, &body, true); err != nil {
		httpresponse.Error(w, err)
		return
	}
	releaseDate, err := parseDate(body.ReleaseDate)
	if err != nil {
		httpresponse.Error(w, err)
		return
	}
	move, err := parseMoveTarget(body.MoveUnresolvedTo)
	if err != nil {
		httpresponse.Error(w, err)
		return
	}
	result, err := h.service.Release(r.Context(), actorID, globalRole, id, port.ReleaseInput{ReleaseDate: releaseDate, Move: move})
	if err != nil {
		httpresponse.Error(w, err)
		return
	}
	response := releaseResponse{
		Version: toVersionResponse(result.Version), MovedIssues: result.MovedIssues, UnresolvedIssues: result.UnresolvedIssues,
	}
	if result.NextVersion != nil {
		next := toVersionResponse(result.NextVersion)
		response.NextVersion = &next
	}
	httpresponse.JSON(w, http.StatusOK, response)
}

func (h *Handler) Archive(w http.ResponseWriter, r *http.Request) {
	actorID, globalRole, id, ok := versionContext(w, r)
	if !ok {
		return
	}
	v, err := h.service.Archive(r.Context(), actorID, globalRole, id)
	if err != nil {
		httpresponse.Error(w, err)
		return
	}
	httpresponse.JSON(w, http.StatusOK, toVersionResponse(v))
}

func (h *Handler) Notes(w http.ResponseWriter, r *http.Request) {
	actorID, globalRole, id, ok := versionContext(w, r)
	if !ok {
		return
	}
	notes, err := h.service.Notes(r.Context(), actorID, globalRole, id)
	if err != nil {
		httpresponse.Error(w, err)
		return
	}
	groups := make([]noteGroupResponse, 0, len(notes.Groups))
	for _, group := range notes.Groups {
		items := make([]noteIssueResponse, 0, len(group.Issues))
		for _, item := range group.Issues {
			items = append(items, noteIssueResponse{ID: item.ID.String(), Key: item.Key, Summary: item.Summary, Done: item.Done})
		}
		groups = append(groups, noteGroupResponse{Title: group.Title, Issues: items})
	}
	var releaseDate *string
	if notes.ReleaseDate != "" {
		releaseDate = &notes.ReleaseDate
	}
	count := 0
	for _, group := range notes.Groups {
		count += len(group.Issues)
	}
	httpresponse.JSON(w, http.StatusOK, notesResponse{
		VersionID: notes.VersionID.String(), VersionName: notes.VersionName, Status: string(notes.Status),
		ReleaseDate: releaseDate, IssueCount: count, Markdown: notes.Markdown, Groups: groups,
	})
}

// projectContext reads the caller and the project of the {id} segment, a UUID or a key.
func (h *Handler) projectContext(w http.ResponseWriter, r *http.Request) (uuid.UUID, string, uuid.UUID, bool) {
	actorID, ok := appmw.UserIDFromContext(r.Context())
	if !ok {
		httpresponse.Error(w, apperror.New(apperror.KindUnauthorized, "No autenticado"))
		return uuid.Nil, "", uuid.Nil, false
	}
	globalRole, _ := appmw.RoleFromContext(r.Context())
	segment := chi.URLParam(r, "id")
	if id, err := uuid.Parse(segment); err == nil {
		return actorID, globalRole, id, true
	}
	var orgID uuid.UUID
	if raw := r.URL.Query().Get("organization_id"); raw != "" {
		parsed, err := uuid.Parse(raw)
		if err != nil {
			httpresponse.Error(w, apperror.WithCode(apperror.KindValidation, "validation.failed", "El identificador de organización no es valido"))
			return uuid.Nil, "", uuid.Nil, false
		}
		orgID = parsed
	}
	projectID, err := h.projects.ResolveProjectKey(r.Context(), actorID, globalRole, orgID, segment)
	if err != nil {
		httpresponse.Error(w, err)
		return uuid.Nil, "", uuid.Nil, false
	}
	return actorID, globalRole, projectID, true
}

// versionContext reads the caller and the version id of the route.
func versionContext(w http.ResponseWriter, r *http.Request) (uuid.UUID, string, uuid.UUID, bool) {
	actorID, ok := appmw.UserIDFromContext(r.Context())
	if !ok {
		httpresponse.Error(w, apperror.New(apperror.KindUnauthorized, "No autenticado"))
		return uuid.Nil, "", uuid.Nil, false
	}
	globalRole, _ := appmw.RoleFromContext(r.Context())
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpresponse.Error(w, domain.ErrVersionNotFound)
		return uuid.Nil, "", uuid.Nil, false
	}
	return actorID, globalRole, id, true
}

// decodeStrict reads the body rejecting unknown fields. With optional, an empty body is valid.
func decodeStrict(r *http.Request, into any, optional bool) error {
	raw, err := io.ReadAll(io.LimitReader(r.Body, maxBody))
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
	return nil
}

func parseDate(raw *string) (*time.Time, error) {
	if raw == nil || *raw == "" {
		return nil, nil
	}
	parsed, err := time.Parse(domain.DateLayout, *raw)
	if err != nil {
		return nil, apperror.WithCode(apperror.KindValidation, "validation.failed", "Las fechas deben tener el formato AAAA-MM-DD")
	}
	return &parsed, nil
}

// patchDate reads a date of a PATCH: absent means unchanged, null or empty clears the date.
func patchDate(o optionalDate) (bool, *time.Time, error) {
	if !o.set {
		return false, nil, nil
	}
	if o.value == nil || *o.value == "" {
		return true, nil, nil
	}
	parsed, err := parseDate(o.value)
	return true, parsed, err
}

// parseMoveTarget reads move_unresolved_to: absent or null keeps the issues where they are, "new" creates the
// next version, and a UUID names an unreleased version of the project.
func parseMoveTarget(raw json.RawMessage) (port.MoveTarget, error) {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 || string(trimmed) == "null" {
		return port.MoveTarget{Mode: usecase.MoveNone}, nil
	}
	var text string
	if err := json.Unmarshal(trimmed, &text); err != nil {
		return port.MoveTarget{}, domain.ErrVersionMoveTarget
	}
	if text == usecase.MoveNew {
		return port.MoveTarget{Mode: usecase.MoveNew}, nil
	}
	id, err := uuid.Parse(text)
	if err != nil {
		return port.MoveTarget{}, domain.ErrVersionMoveTarget
	}
	return port.MoveTarget{Mode: usecase.MoveVersion, VersionID: id}, nil
}
