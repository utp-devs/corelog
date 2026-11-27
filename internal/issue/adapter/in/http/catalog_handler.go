package http

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/jhonsferg/corelog/internal/issue/application/port"
	"github.com/jhonsferg/corelog/internal/issue/domain"
	"github.com/jhonsferg/corelog/internal/platform/apperror"
	"github.com/jhonsferg/corelog/internal/platform/httpresponse"
	appmw "github.com/jhonsferg/corelog/internal/platform/middleware"
	"github.com/jhonsferg/corelog/internal/platform/validation"
)

// CatalogHandler serves the organization catalogs (issue types, priorities, resolutions) and the project
// issue-type settings.
type CatalogHandler struct {
	service   port.CatalogService
	jwtSecret string
}

func NewCatalogHandler(service port.CatalogService, jwtSecret string) *CatalogHandler {
	return &CatalogHandler{service: service, jwtSecret: jwtSecret}
}

// catalogRoutes maps the URL segment of each catalog to its kind.
var catalogRoutes = []struct {
	segment string
	kind    domain.CatalogKind
}{
	{"issue-types", domain.CatalogIssueType},
	{"priorities", domain.CatalogPriority},
	{"resolutions", domain.CatalogResolution},
}

// RegisterCatalogRoutes mounts the catalog and project issue-type routes. They are registered one by one, as
// the logo routes are, so they do not share a subrouter with the organization routes.
func (h *CatalogHandler) RegisterCatalogRoutes(r chi.Router) {
	auth := appmw.Authenticate(h.jwtSecret)
	for _, route := range catalogRoutes {
		kind := route.kind
		base := "/organizations/{org}/" + route.segment
		r.With(auth).Get(base, h.list(kind))
		r.With(auth).Post(base, h.create(kind))
		r.With(auth).Put(base+"/reorder", h.reorder(kind))
		r.With(auth).Patch(base+"/{id}", h.update(kind))
		r.With(auth).Delete(base+"/{id}", h.delete(kind))
	}
	r.With(auth).Get("/projects/{id}/issue-types", h.listProjectIssueTypes)
	r.With(auth).Put("/projects/{id}/issue-types", h.replaceProjectIssueTypes)
}

type catalogItemResponse struct {
	ID             string    `json:"id"`
	Name           string    `json:"name"`
	Description    string    `json:"description"`
	Icon           string    `json:"icon,omitempty"`
	Color          string    `json:"color,omitempty"`
	HierarchyLevel *int      `json:"hierarchy_level,omitempty"`
	Position       int       `json:"position"`
	IsDefault      bool      `json:"is_default"`
	UsageCount     int       `json:"usage_count"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

func toCatalogItemResponse(item *domain.CatalogItem) catalogItemResponse {
	resp := catalogItemResponse{
		ID:          item.ID.String(),
		Name:        item.Name,
		Description: item.Description,
		Icon:        item.Icon,
		Color:       item.Color,
		Position:    item.Position,
		IsDefault:   item.IsDefault,
		UsageCount:  item.Usage,
		CreatedAt:   item.CreatedAt,
		UpdatedAt:   item.UpdatedAt,
	}
	if item.Kind == domain.CatalogIssueType {
		level := item.HierarchyLevel
		resp.HierarchyLevel = &level
	}
	return resp
}

func toCatalogItemResponses(items []*domain.CatalogItem) []catalogItemResponse {
	out := make([]catalogItemResponse, 0, len(items))
	for _, item := range items {
		out = append(out, toCatalogItemResponse(item))
	}
	return out
}

type catalogCreateRequest struct {
	Name           string `json:"name" validate:"required,max=120"`
	Description    string `json:"description" validate:"max=500"`
	Icon           string `json:"icon" validate:"max=40"`
	Color          string `json:"color" validate:"max=40"`
	HierarchyLevel int    `json:"hierarchy_level" validate:"min=-1,max=1"`
	IsDefault      bool   `json:"is_default"`
}

type catalogPatchRequest struct {
	Name        *string `json:"name" validate:"omitempty,max=120"`
	Description *string `json:"description" validate:"omitempty,max=500"`
	Icon        *string `json:"icon" validate:"omitempty,max=40"`
	Color       *string `json:"color" validate:"omitempty,max=40"`
	IsDefault   *bool   `json:"is_default"`
}

type catalogReorderRequest struct {
	IDs []string `json:"ids" validate:"required,min=1,dive,uuid"`
}

type projectIssueTypesRequest struct {
	IssueTypeIDs []string `json:"issue_type_ids" validate:"required,min=1,dive,uuid"`
}

type projectIssueTypesResponse struct {
	Items      []catalogItemResponse `json:"items"`
	Configured bool                  `json:"configured"`
}

func (h *CatalogHandler) list(kind domain.CatalogKind) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		actorID, orgID, ok := actorAndOrg(w, r)
		if !ok {
			return
		}
		items, err := h.service.ListCatalog(r.Context(), actorID, orgID, kind)
		if err != nil {
			httpresponse.Error(w, err)
			return
		}
		httpresponse.JSON(w, http.StatusOK, toCatalogItemResponses(items))
	}
}

func (h *CatalogHandler) create(kind domain.CatalogKind) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		actorID, orgID, ok := actorAndOrg(w, r)
		if !ok {
			return
		}
		var req catalogCreateRequest
		if !decodeStrict(w, r, &req) {
			return
		}
		item, err := h.service.CreateCatalogItem(r.Context(), actorID, orgID, kind, port.CatalogInput{
			Name:           req.Name,
			Description:    req.Description,
			Icon:           req.Icon,
			Color:          req.Color,
			HierarchyLevel: req.HierarchyLevel,
			IsDefault:      req.IsDefault,
		})
		if err != nil {
			httpresponse.Error(w, err)
			return
		}
		httpresponse.JSON(w, http.StatusCreated, toCatalogItemResponse(item))
	}
}

func (h *CatalogHandler) update(kind domain.CatalogKind) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		actorID, orgID, ok := actorAndOrg(w, r)
		if !ok {
			return
		}
		id, ok := parseURLUUID(w, r, "id", "invalid catalog item id")
		if !ok {
			return
		}
		var req catalogPatchRequest
		if !decodeStrict(w, r, &req) {
			return
		}
		item, err := h.service.UpdateCatalogItem(r.Context(), actorID, orgID, kind, id, port.CatalogPatch{
			Name:        req.Name,
			Description: req.Description,
			Icon:        req.Icon,
			Color:       req.Color,
			IsDefault:   req.IsDefault,
		})
		if err != nil {
			httpresponse.Error(w, err)
			return
		}
		httpresponse.JSON(w, http.StatusOK, toCatalogItemResponse(item))
	}
}

func (h *CatalogHandler) delete(kind domain.CatalogKind) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		actorID, orgID, ok := actorAndOrg(w, r)
		if !ok {
			return
		}
		id, ok := parseURLUUID(w, r, "id", "invalid catalog item id")
		if !ok {
			return
		}
		var reassignTo *uuid.UUID
		if raw := r.URL.Query().Get("reassign_to"); raw != "" {
			value, err := uuid.Parse(raw)
			if err != nil {
				httpresponse.Error(w, apperror.New(apperror.KindValidation, "invalid reassign_to id"))
				return
			}
			reassignTo = &value
		}
		if err := h.service.DeleteCatalogItem(r.Context(), actorID, orgID, kind, id, reassignTo); err != nil {
			httpresponse.Error(w, err)
			return
		}
		httpresponse.JSON(w, http.StatusNoContent, nil)
	}
}

func (h *CatalogHandler) reorder(kind domain.CatalogKind) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		actorID, orgID, ok := actorAndOrg(w, r)
		if !ok {
			return
		}
		var req catalogReorderRequest
		if !decodeStrict(w, r, &req) {
			return
		}
		ids, ok := parseUUIDList(w, req.IDs)
		if !ok {
			return
		}
		items, err := h.service.ReorderCatalog(r.Context(), actorID, orgID, kind, ids)
		if err != nil {
			httpresponse.Error(w, err)
			return
		}
		httpresponse.JSON(w, http.StatusOK, toCatalogItemResponses(items))
	}
}

func (h *CatalogHandler) listProjectIssueTypes(w http.ResponseWriter, r *http.Request) {
	actorID, globalRole, projectID, ok := actorAndProject(w, r)
	if !ok {
		return
	}
	items, configured, err := h.service.ProjectIssueTypes(r.Context(), actorID, globalRole, projectID)
	if err != nil {
		httpresponse.Error(w, err)
		return
	}
	httpresponse.JSON(w, http.StatusOK, projectIssueTypesResponse{Items: toCatalogItemResponses(items), Configured: configured})
}

func (h *CatalogHandler) replaceProjectIssueTypes(w http.ResponseWriter, r *http.Request) {
	actorID, globalRole, projectID, ok := actorAndProject(w, r)
	if !ok {
		return
	}
	var req projectIssueTypesRequest
	if !decodeStrict(w, r, &req) {
		return
	}
	ids, ok := parseUUIDList(w, req.IssueTypeIDs)
	if !ok {
		return
	}
	items, err := h.service.ReplaceProjectIssueTypes(r.Context(), actorID, globalRole, projectID, ids)
	if err != nil {
		httpresponse.Error(w, err)
		return
	}
	httpresponse.JSON(w, http.StatusOK, projectIssueTypesResponse{Items: toCatalogItemResponses(items), Configured: true})
}

// actorAndOrg reads the authenticated user and the organization of the route. It writes the error and
// returns false when either is missing or malformed.
func actorAndOrg(w http.ResponseWriter, r *http.Request) (uuid.UUID, uuid.UUID, bool) {
	actorID, ok := appmw.UserIDFromContext(r.Context())
	if !ok {
		httpresponse.Error(w, apperror.New(apperror.KindUnauthorized, "not authenticated"))
		return uuid.Nil, uuid.Nil, false
	}
	orgID, ok := parseURLUUID(w, r, "org", "invalid organization id")
	if !ok {
		return uuid.Nil, uuid.Nil, false
	}
	return actorID, orgID, true
}

func actorAndProject(w http.ResponseWriter, r *http.Request) (uuid.UUID, string, uuid.UUID, bool) {
	actorID, ok := appmw.UserIDFromContext(r.Context())
	if !ok {
		httpresponse.Error(w, apperror.New(apperror.KindUnauthorized, "not authenticated"))
		return uuid.Nil, "", uuid.Nil, false
	}
	projectID, ok := parseURLUUID(w, r, "id", "invalid project id")
	if !ok {
		return uuid.Nil, "", uuid.Nil, false
	}
	globalRole, _ := appmw.RoleFromContext(r.Context())
	return actorID, globalRole, projectID, true
}

func parseURLUUID(w http.ResponseWriter, r *http.Request, name, message string) (uuid.UUID, bool) {
	id, err := uuid.Parse(chi.URLParam(r, name))
	if err != nil {
		httpresponse.Error(w, apperror.New(apperror.KindValidation, message))
		return uuid.Nil, false
	}
	return id, true
}

func parseUUIDList(w http.ResponseWriter, raw []string) ([]uuid.UUID, bool) {
	ids := make([]uuid.UUID, 0, len(raw))
	for _, value := range raw {
		id, err := uuid.Parse(value)
		if err != nil {
			httpresponse.Error(w, apperror.New(apperror.KindValidation, "invalid id in list"))
			return nil, false
		}
		ids = append(ids, id)
	}
	return ids, true
}

// decodeStrict decodes the JSON body rejecting unknown fields and validates the struct. It writes the error
// and returns false on failure.
func decodeStrict(w http.ResponseWriter, r *http.Request, target any) bool {
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		httpresponse.Error(w, apperror.New(apperror.KindValidation, "invalid request body"))
		return false
	}
	if err := validation.Struct(target); err != nil {
		httpresponse.Error(w, validation.Failed(target, err))
		return false
	}
	return true
}
