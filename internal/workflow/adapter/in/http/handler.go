// Package http exposes the workflow administration API: the workflow of a project, its validation, replacement
// and reset, the explicit move of issues, the template catalog and the organization status catalog.
package http

import (
	"context"
	"encoding/json"
	"io"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/jhonsferg/corelog/internal/platform/apperror"
	"github.com/jhonsferg/corelog/internal/platform/httpresponse"
	appmw "github.com/jhonsferg/corelog/internal/platform/middleware"
	"github.com/jhonsferg/corelog/internal/workflow/application/port"
	"github.com/jhonsferg/corelog/internal/workflow/domain"
)

// maxBody bounds the JSON body of the administration endpoints.
const maxBody = 256 << 10

// ProjectResolver resolves a project key for the actor, as the project module does for its own routes.
type ProjectResolver interface {
	ResolveProjectKey(ctx context.Context, actorID uuid.UUID, globalRole string, orgID uuid.UUID, key string) (uuid.UUID, error)
}

// Handler serves the workflow administration endpoints.
type Handler struct {
	admin     port.Admin
	projects  ProjectResolver
	jwtSecret string
}

// NewHandler builds the handler over the administration use cases.
func NewHandler(admin port.Admin, projects ProjectResolver, jwtSecret string) *Handler {
	return &Handler{admin: admin, projects: projects, jwtSecret: jwtSecret}
}

// RegisterRoutes mounts the endpoints. Every route requires an authenticated user; permissions are decided by
// the use cases.
func (h *Handler) RegisterRoutes(r chi.Router) {
	auth := appmw.Authenticate(h.jwtSecret)
	r.Route("/workflow-templates", func(r chi.Router) {
		r.Use(auth)
		r.Get("/", h.ListTemplates)
	})
	r.Route("/projects/{id}/workflow", func(r chi.Router) {
		r.Use(auth)
		r.Get("/", h.Get)
		r.Put("/", h.Replace)
		r.Post("/validate", h.Validate)
		r.Post("/reset", h.Reset)
		r.Post("/migrate-issues", h.MigrateIssues)
	})
	r.Route("/organizations/{org}/statuses", func(r chi.Router) {
		r.Use(auth)
		r.Get("/", h.ListStatuses)
		r.Post("/", h.CreateStatus)
		r.Put("/reorder", h.ReorderStatuses)
		r.Patch("/{id}", h.UpdateStatus)
		r.Delete("/{id}", h.DeleteStatus)
	})
}

// actor returns the authenticated user and the platform role.
func actor(r *http.Request) (uuid.UUID, string, error) {
	id, ok := appmw.UserIDFromContext(r.Context())
	if !ok {
		return uuid.Nil, "", apperror.New(apperror.KindUnauthorized, "not authenticated")
	}
	role, _ := appmw.RoleFromContext(r.Context())
	return id, role, nil
}

// projectID resolves the {id} segment: a UUID as is, or a project key looked up among the projects the actor can
// see. organization_id in the query narrows a key lookup.
func (h *Handler) projectID(r *http.Request, actorID uuid.UUID, globalRole string) (uuid.UUID, error) {
	segment := chi.URLParam(r, "id")
	if id, err := uuid.Parse(segment); err == nil {
		return id, nil
	}
	orgID := uuid.Nil
	if value := r.URL.Query().Get("organization_id"); value != "" {
		parsed, err := uuid.Parse(value)
		if err != nil {
			return uuid.Nil, apperror.WithCode(apperror.KindValidation, "validation.failed", "organization_id debe ser un UUID válido")
		}
		orgID = parsed
	}
	return h.projects.ResolveProjectKey(r.Context(), actorID, globalRole, orgID, segment)
}

// pathUUID parses a UUID path parameter.
func pathUUID(r *http.Request, name string) (uuid.UUID, error) {
	id, err := uuid.Parse(chi.URLParam(r, name))
	if err != nil {
		return uuid.Nil, apperror.WithCode(apperror.KindValidation, "validation.failed", "el identificador no es un UUID válido")
	}
	return id, nil
}

// decode reads a strict JSON body: unknown fields and malformed JSON are refused.
func decode(r *http.Request, dst any) error {
	d := json.NewDecoder(io.LimitReader(r.Body, maxBody))
	d.DisallowUnknownFields()
	if err := d.Decode(dst); err != nil {
		return malformedBody()
	}
	return nil
}

// Get returns the stored workflow of the project.
func (h *Handler) Get(w http.ResponseWriter, r *http.Request) {
	actorID, role, err := actor(r)
	if err != nil {
		writeError(w, err)
		return
	}
	projectID, err := h.projectID(r, actorID, role)
	if err != nil {
		writeError(w, err)
		return
	}
	view, err := h.admin.GetWorkflow(r.Context(), actorID, role, projectID)
	if err != nil {
		writeError(w, err)
		return
	}
	out, err := toWorkflowResponse(view)
	if err != nil {
		writeError(w, err)
		return
	}
	httpresponse.JSON(w, http.StatusOK, out)
}

// Validate is a dry run of a definition. It answers 200 with the problems, even when the definition is refused.
func (h *Handler) Validate(w http.ResponseWriter, r *http.Request) {
	actorID, role, err := actor(r)
	if err != nil {
		writeError(w, err)
		return
	}
	projectID, err := h.projectID(r, actorID, role)
	if err != nil {
		writeError(w, err)
		return
	}
	var body definitionBody
	if err := decode(r, &body); err != nil {
		writeError(w, err)
		return
	}
	def, issues := body.toDefinition()
	if len(issues) > 0 {
		httpresponse.JSON(w, http.StatusOK, toValidationResponse(port.ValidationReport{Errors: issues}))
		return
	}
	report, err := h.admin.ValidateWorkflow(r.Context(), actorID, role, projectID, def)
	if err != nil {
		writeError(w, err)
		return
	}
	httpresponse.JSON(w, http.StatusOK, toValidationResponse(report))
}

// Replace stores a new workflow for the project.
func (h *Handler) Replace(w http.ResponseWriter, r *http.Request) {
	actorID, role, err := actor(r)
	if err != nil {
		writeError(w, err)
		return
	}
	projectID, err := h.projectID(r, actorID, role)
	if err != nil {
		writeError(w, err)
		return
	}
	var body definitionBody
	if err := decode(r, &body); err != nil {
		writeError(w, err)
		return
	}
	def, issues := body.toDefinition()
	if len(issues) > 0 {
		writeError(w, invalidRequest(issues))
		return
	}
	result, err := h.admin.ReplaceWorkflow(r.Context(), actorID, role, projectID, def)
	if err != nil {
		writeError(w, err)
		return
	}
	out, err := toSaveResponse(result)
	if err != nil {
		writeError(w, err)
		return
	}
	httpresponse.JSON(w, http.StatusOK, out)
}

// Reset applies the default workflow of a template.
func (h *Handler) Reset(w http.ResponseWriter, r *http.Request) {
	actorID, role, err := actor(r)
	if err != nil {
		writeError(w, err)
		return
	}
	projectID, err := h.projectID(r, actorID, role)
	if err != nil {
		writeError(w, err)
		return
	}
	var body resetBody
	if err := decode(r, &body); err != nil {
		writeError(w, err)
		return
	}
	mapping, issues := parseMapping(body.StatusMapping, nil)
	if len(issues) > 0 {
		writeError(w, invalidRequest(issues))
		return
	}
	result, err := h.admin.ResetWorkflow(r.Context(), actorID, role, projectID, body.Template, body.ExpectedVersion, mapping)
	if err != nil {
		writeError(w, err)
		return
	}
	out, err := toSaveResponse(result)
	if err != nil {
		writeError(w, err)
		return
	}
	httpresponse.JSON(w, http.StatusOK, out)
}

// MigrateIssues moves issues between statuses of the project.
func (h *Handler) MigrateIssues(w http.ResponseWriter, r *http.Request) {
	actorID, role, err := actor(r)
	if err != nil {
		writeError(w, err)
		return
	}
	projectID, err := h.projectID(r, actorID, role)
	if err != nil {
		writeError(w, err)
		return
	}
	var body migrateBody
	if err := decode(r, &body); err != nil {
		writeError(w, err)
		return
	}
	mapping, issues := parseMapping(body.StatusMapping, nil)
	if len(issues) > 0 {
		writeError(w, invalidRequest(issues))
		return
	}
	result, err := h.admin.MigrateIssues(r.Context(), actorID, role, projectID, mapping)
	if err != nil {
		writeError(w, err)
		return
	}
	httpresponse.JSON(w, http.StatusOK, toMigrationResponse(result))
}

// ListTemplates returns the template workflows. Any authenticated user may read them.
func (h *Handler) ListTemplates(w http.ResponseWriter, r *http.Request) {
	if _, _, err := actor(r); err != nil {
		writeError(w, err)
		return
	}
	out := make([]templateResponse, 0, len(domain.TemplateNames()))
	for _, t := range h.admin.ListTemplates(r.Context()) {
		item, err := toTemplateResponse(t)
		if err != nil {
			writeError(w, err)
			return
		}
		out = append(out, item)
	}
	httpresponse.JSON(w, http.StatusOK, out)
}

// ListStatuses returns the status catalog of the organization.
func (h *Handler) ListStatuses(w http.ResponseWriter, r *http.Request) {
	actorID, _, err := actor(r)
	if err != nil {
		writeError(w, err)
		return
	}
	orgID, err := pathUUID(r, "org")
	if err != nil {
		writeError(w, err)
		return
	}
	list, err := h.admin.ListStatuses(r.Context(), actorID, orgID)
	if err != nil {
		writeError(w, err)
		return
	}
	out := make([]orgStatusResponse, 0, len(list))
	for _, s := range list {
		out = append(out, toOrgStatusResponse(s))
	}
	httpresponse.JSON(w, http.StatusOK, out)
}

// CreateStatus appends a status to the catalog.
func (h *Handler) CreateStatus(w http.ResponseWriter, r *http.Request) {
	actorID, _, err := actor(r)
	if err != nil {
		writeError(w, err)
		return
	}
	orgID, err := pathUUID(r, "org")
	if err != nil {
		writeError(w, err)
		return
	}
	var body createStatusBody
	if err := decode(r, &body); err != nil {
		writeError(w, err)
		return
	}
	created, err := h.admin.CreateStatus(r.Context(), actorID, orgID, body.Name, domain.Category(body.Category))
	if err != nil {
		writeError(w, err)
		return
	}
	httpresponse.JSON(w, http.StatusCreated, toOrgStatusResponse(created))
}

// UpdateStatus renames a status or changes its category.
func (h *Handler) UpdateStatus(w http.ResponseWriter, r *http.Request) {
	actorID, _, err := actor(r)
	if err != nil {
		writeError(w, err)
		return
	}
	orgID, err := pathUUID(r, "org")
	if err != nil {
		writeError(w, err)
		return
	}
	id, err := pathUUID(r, "id")
	if err != nil {
		writeError(w, err)
		return
	}
	var body statusWriteBody
	if err := decode(r, &body); err != nil {
		writeError(w, err)
		return
	}
	patch := port.StatusPatch{Name: body.Name}
	if body.Category != nil {
		category := domain.Category(*body.Category)
		patch.Category = &category
	}
	updated, err := h.admin.UpdateStatus(r.Context(), actorID, orgID, id, patch)
	if err != nil {
		writeError(w, err)
		return
	}
	httpresponse.JSON(w, http.StatusOK, toOrgStatusResponse(updated))
}

// DeleteStatus removes a status. reassign_to, when given, receives its issues and workflow positions.
func (h *Handler) DeleteStatus(w http.ResponseWriter, r *http.Request) {
	actorID, _, err := actor(r)
	if err != nil {
		writeError(w, err)
		return
	}
	orgID, err := pathUUID(r, "org")
	if err != nil {
		writeError(w, err)
		return
	}
	id, err := pathUUID(r, "id")
	if err != nil {
		writeError(w, err)
		return
	}
	var reassignTo uuid.UUID
	if value := r.URL.Query().Get("reassign_to"); value != "" {
		parsed, err := uuid.Parse(value)
		if err != nil {
			writeError(w, apperror.WithCode(apperror.KindValidation, "validation.failed", "reassign_to debe ser un UUID válido"))
			return
		}
		reassignTo = parsed
	}
	result, err := h.admin.DeleteStatus(r.Context(), actorID, orgID, id, reassignTo)
	if err != nil {
		writeError(w, err)
		return
	}
	httpresponse.JSON(w, http.StatusOK, deleteStatusResponse{ID: id.String(), ReassignedIssues: result.ReassignedIssues})
}

// ReorderStatuses stores the catalog order. status_ids names every status once.
func (h *Handler) ReorderStatuses(w http.ResponseWriter, r *http.Request) {
	actorID, _, err := actor(r)
	if err != nil {
		writeError(w, err)
		return
	}
	orgID, err := pathUUID(r, "org")
	if err != nil {
		writeError(w, err)
		return
	}
	var body reorderBody
	if err := decode(r, &body); err != nil {
		writeError(w, err)
		return
	}
	ids := make([]uuid.UUID, 0, len(body.StatusIDs))
	for _, text := range body.StatusIDs {
		id, err := uuid.Parse(text)
		if err != nil {
			writeError(w, apperror.WithCode(apperror.KindValidation, "validation.failed", "status_ids debe contener UUID válidos"))
			return
		}
		ids = append(ids, id)
	}
	list, err := h.admin.ReorderStatuses(r.Context(), actorID, orgID, ids)
	if err != nil {
		writeError(w, err)
		return
	}
	out := make([]orgStatusResponse, 0, len(list))
	for _, s := range list {
		out = append(out, toOrgStatusResponse(s))
	}
	httpresponse.JSON(w, http.StatusOK, out)
}
