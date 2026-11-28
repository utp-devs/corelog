package http

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/jhonsferg/corelog/internal/platform/apperror"
	"github.com/jhonsferg/corelog/internal/platform/httpresponse"
	"github.com/jhonsferg/corelog/internal/platform/validation"
)

// ListTeamGrants returns the team roles of the project. Requires manage_members.
func (h *Handler) ListTeamGrants(w http.ResponseWriter, r *http.Request) {
	actorID, globalRole, ok := actorFromRequest(r)
	if !ok {
		httpresponse.Error(w, apperror.New(apperror.KindUnauthorized, "unauthorized"))
		return
	}
	projectID, ok := h.projectIDFromPath(w, r, actorID, globalRole)
	if !ok {
		return
	}
	grants, err := h.service.ListTeamGrants(r.Context(), actorID, globalRole, projectID)
	if err != nil {
		httpresponse.Error(w, err)
		return
	}
	httpresponse.JSON(w, http.StatusOK, toTeamGrantResponses(grants))
}

// SetTeamGrant creates or replaces the role of one team on the project. The body is {team_id, role}.
func (h *Handler) SetTeamGrant(w http.ResponseWriter, r *http.Request) {
	var req teamGrantRequest
	if err := validation.DecodeStrict(r.Body, &req); err != nil {
		httpresponse.Error(w, apperror.New(apperror.KindValidation, "invalid request body"))
		return
	}
	if err := validation.Struct(req); err != nil {
		httpresponse.Error(w, validation.Failed(req, err))
		return
	}
	teamID, err := uuid.Parse(req.TeamID)
	if err != nil {
		httpresponse.Error(w, apperror.New(apperror.KindValidation, "invalid team id"))
		return
	}
	actorID, globalRole, ok := actorFromRequest(r)
	if !ok {
		httpresponse.Error(w, apperror.New(apperror.KindUnauthorized, "unauthorized"))
		return
	}
	projectID, ok := h.projectIDFromPath(w, r, actorID, globalRole)
	if !ok {
		return
	}
	grant, err := h.service.SetTeamGrant(r.Context(), actorID, globalRole, projectID, teamID, req.Role)
	if err != nil {
		httpresponse.Error(w, err)
		return
	}
	httpresponse.JSON(w, http.StatusOK, toTeamGrantResponse(*grant))
}

// RemoveTeamGrant deletes the team role of the project. Requires manage_members.
func (h *Handler) RemoveTeamGrant(w http.ResponseWriter, r *http.Request) {
	teamID, err := uuid.Parse(chi.URLParam(r, "team_id"))
	if err != nil {
		httpresponse.Error(w, apperror.New(apperror.KindValidation, "invalid team id"))
		return
	}
	actorID, globalRole, ok := actorFromRequest(r)
	if !ok {
		httpresponse.Error(w, apperror.New(apperror.KindUnauthorized, "unauthorized"))
		return
	}
	projectID, ok := h.projectIDFromPath(w, r, actorID, globalRole)
	if !ok {
		return
	}
	if err := h.service.RemoveTeamGrant(r.Context(), actorID, globalRole, projectID, teamID); err != nil {
		httpresponse.Error(w, err)
		return
	}
	httpresponse.JSON(w, http.StatusNoContent, nil)
}
