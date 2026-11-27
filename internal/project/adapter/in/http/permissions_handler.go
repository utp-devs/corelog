package http

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/google/uuid"

	authusecase "github.com/jhonsferg/corelog/internal/authorization/application/usecase"
	authdomain "github.com/jhonsferg/corelog/internal/authorization/domain"
	"github.com/jhonsferg/corelog/internal/platform/apperror"
	"github.com/jhonsferg/corelog/internal/platform/httpresponse"
)

// permissionService is the permission scheme use case as the HTTP adapter sees it.
type permissionService interface {
	Get(ctx context.Context, actorID uuid.UUID, globalRole string, projectID uuid.UUID) (authusecase.SchemeView, error)
	Replace(ctx context.Context, actorID uuid.UUID, globalRole string, projectID uuid.UUID, grants []authdomain.Grant) (authusecase.SchemeView, error)
	Reset(ctx context.Context, actorID uuid.UUID, globalRole string, projectID uuid.UUID) (authusecase.SchemeView, error)
	Effective(ctx context.Context, actorID uuid.UUID, globalRole string, projectID uuid.UUID) (authusecase.EffectiveView, error)
}

// WithPermissions connects the permission scheme use case that serves the /permissions routes.
func (h *Handler) WithPermissions(service *authusecase.PermissionService) *Handler {
	h.permissions = service
	return h
}

// GetPermissions returns the full permission scheme of the project.
func (h *Handler) GetPermissions(w http.ResponseWriter, r *http.Request) {
	actorID, globalRole, ok := actorFromRequest(r)
	if !ok {
		httpresponse.Error(w, apperror.New(apperror.KindUnauthorized, "no autorizado"))
		return
	}
	projectID, ok := h.projectIDFromPath(w, r, actorID, globalRole)
	if !ok {
		return
	}
	view, err := h.permissions.Get(r.Context(), actorID, globalRole, projectID)
	if err != nil {
		httpresponse.Error(w, err)
		return
	}
	httpresponse.JSON(w, http.StatusOK, toPermissionScheme(view))
}

// ReplacePermissions stores a complete permission scheme for the project.
func (h *Handler) ReplacePermissions(w http.ResponseWriter, r *http.Request) {
	actorID, globalRole, ok := actorFromRequest(r)
	if !ok {
		httpresponse.Error(w, apperror.New(apperror.KindUnauthorized, "no autorizado"))
		return
	}
	projectID, ok := h.projectIDFromPath(w, r, actorID, globalRole)
	if !ok {
		return
	}
	var req replacePermissionsRequest
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&req); err != nil {
		httpresponse.Error(w, apperror.New(apperror.KindValidation, "invalid request body"))
		return
	}
	grants := make([]authdomain.Grant, 0, len(req.Grants))
	for _, g := range req.Grants {
		holders := make([]authdomain.Holder, 0, len(g.Holders))
		for _, holder := range g.Holders {
			holders = append(holders, authdomain.Holder(holder))
		}
		grants = append(grants, authdomain.Grant{Permission: authdomain.Permission(g.Permission), Holders: holders})
	}
	view, err := h.permissions.Replace(r.Context(), actorID, globalRole, projectID, grants)
	if err != nil {
		httpresponse.Error(w, err)
		return
	}
	httpresponse.JSON(w, http.StatusOK, toPermissionScheme(view))
}

// ResetPermissions removes the custom scheme of the project. The default scheme applies afterwards.
func (h *Handler) ResetPermissions(w http.ResponseWriter, r *http.Request) {
	actorID, globalRole, ok := actorFromRequest(r)
	if !ok {
		httpresponse.Error(w, apperror.New(apperror.KindUnauthorized, "no autorizado"))
		return
	}
	projectID, ok := h.projectIDFromPath(w, r, actorID, globalRole)
	if !ok {
		return
	}
	view, err := h.permissions.Reset(r.Context(), actorID, globalRole, projectID)
	if err != nil {
		httpresponse.Error(w, err)
		return
	}
	httpresponse.JSON(w, http.StatusOK, toPermissionScheme(view))
}

// MyPermissions returns the role of the caller in the project and the permissions it grants.
func (h *Handler) MyPermissions(w http.ResponseWriter, r *http.Request) {
	actorID, globalRole, ok := actorFromRequest(r)
	if !ok {
		httpresponse.Error(w, apperror.New(apperror.KindUnauthorized, "no autorizado"))
		return
	}
	projectID, ok := h.projectIDFromPath(w, r, actorID, globalRole)
	if !ok {
		return
	}
	view, err := h.permissions.Effective(r.Context(), actorID, globalRole, projectID)
	if err != nil {
		httpresponse.Error(w, err)
		return
	}
	permissions := make([]string, 0, len(view.Permissions))
	for _, p := range view.Permissions {
		permissions = append(permissions, string(p))
	}
	httpresponse.JSON(w, http.StatusOK, effectivePermissionsDTO{Role: view.Role, Permissions: permissions})
}
