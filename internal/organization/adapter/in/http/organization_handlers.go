package http

import (
	"net/http"

	"github.com/google/uuid"

	"github.com/jhonsferg/corelog/internal/organization/application/port"
	"github.com/jhonsferg/corelog/internal/organization/domain"
	"github.com/jhonsferg/corelog/internal/platform/apperror"
	"github.com/jhonsferg/corelog/internal/platform/httpresponse"
)

// List returns the organizations of the caller with the caller's role in each.
func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	actor, err := actorFrom(r)
	if err != nil {
		fail(w, err)
		return
	}
	memberships, err := h.service.ListMine(r.Context(), actor)
	if err != nil {
		fail(w, err)
		return
	}
	out := make([]organizationResponse, 0, len(memberships))
	for _, m := range memberships {
		out = append(out, toOrganizationResponse(m.Organization, m.Role))
	}
	httpresponse.JSON(w, http.StatusOK, out)
}

// Create makes the caller the owner of a new organization.
func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	actor, err := actorFrom(r)
	if err != nil {
		fail(w, err)
		return
	}
	var req createOrganizationRequest
	if err := decodeStrict(r, &req); err != nil {
		fail(w, err)
		return
	}
	org, err := h.service.CreateOrganization(r.Context(), actor, port.CreateInput{
		Name:        req.Name,
		Slug:        req.Slug,
		Description: req.Description,
	})
	if err != nil {
		fail(w, err)
		return
	}
	httpresponse.JSON(w, http.StatusCreated, toOrganizationResponse(org, domain.RoleOwner))
}

// Get returns one organization to any of its members.
func (h *Handler) Get(w http.ResponseWriter, r *http.Request) {
	actor, orgID, ok := h.actorAndOrg(w, r)
	if !ok {
		return
	}
	org, err := h.service.GetOrganization(r.Context(), actor, orgID)
	if err != nil {
		fail(w, err)
		return
	}
	role, err := h.service.RequireRole(r.Context(), actor, orgID, domain.RoleGuest)
	if err != nil {
		fail(w, err)
		return
	}
	httpresponse.JSON(w, http.StatusOK, toOrganizationResponse(org, role))
}

// Update changes the name, description and/or settings. Owners and admins only.
func (h *Handler) Update(w http.ResponseWriter, r *http.Request) {
	actor, orgID, ok := h.actorAndOrg(w, r)
	if !ok {
		return
	}
	var req updateOrganizationRequest
	if err := decodeStrict(r, &req); err != nil {
		fail(w, err)
		return
	}
	org, err := h.service.UpdateOrganization(r.Context(), actor, orgID, port.UpdateInput{
		Name:        req.Name,
		Description: req.Description,
		Settings:    req.Settings.toDomain(),
	})
	if err != nil {
		fail(w, err)
		return
	}
	role, err := h.service.RequireRole(r.Context(), actor, orgID, domain.RoleGuest)
	if err != nil {
		fail(w, err)
		return
	}
	httpresponse.JSON(w, http.StatusOK, toOrganizationResponse(org, role))
}

// Delete soft-deletes the organization. Owners only, with the slug typed as confirmation.
func (h *Handler) Delete(w http.ResponseWriter, r *http.Request) {
	actor, orgID, ok := h.actorAndOrg(w, r)
	if !ok {
		return
	}
	var req deleteOrganizationRequest
	if err := decodeStrict(r, &req); err != nil {
		fail(w, err)
		return
	}
	if err := h.service.DeleteOrganization(r.Context(), actor, orgID, req.ConfirmSlug); err != nil {
		fail(w, err)
		return
	}
	httpresponse.JSON(w, http.StatusNoContent, nil)
}

// Leave removes the caller from the organization. The last owner cannot leave.
func (h *Handler) Leave(w http.ResponseWriter, r *http.Request) {
	actor, orgID, ok := h.actorAndOrg(w, r)
	if !ok {
		return
	}
	if err := h.service.LeaveOrganization(r.Context(), actor, orgID); err != nil {
		fail(w, err)
		return
	}
	httpresponse.JSON(w, http.StatusNoContent, nil)
}

// TransferOwnership makes another member the owner; the caller becomes admin. Owners only.
func (h *Handler) TransferOwnership(w http.ResponseWriter, r *http.Request) {
	actor, orgID, ok := h.actorAndOrg(w, r)
	if !ok {
		return
	}
	var req transferOwnershipRequest
	if err := decodeStrict(r, &req); err != nil {
		fail(w, err)
		return
	}
	newOwner, err := uuid.Parse(req.UserID)
	if err != nil {
		fail(w, apperror.WithCode(apperror.KindValidation, CodeIDInvalid, "identificador no válido"))
		return
	}
	if err := h.service.TransferOwnership(r.Context(), actor, orgID, newOwner); err != nil {
		fail(w, err)
		return
	}
	httpresponse.JSON(w, http.StatusNoContent, nil)
}

// ListMembers returns the members with their account data. Guests may not read the list.
func (h *Handler) ListMembers(w http.ResponseWriter, r *http.Request) {
	actor, orgID, ok := h.actorAndOrg(w, r)
	if !ok {
		return
	}
	views, err := h.service.ListMembers(r.Context(), actor, orgID)
	if err != nil {
		fail(w, err)
		return
	}
	httpresponse.JSON(w, http.StatusOK, toMemberResponses(views))
}

// ChangeMemberRole sets the role of a member. Owners and admins, within the role rules of the domain.
func (h *Handler) ChangeMemberRole(w http.ResponseWriter, r *http.Request) {
	actor, orgID, ok := h.actorAndOrg(w, r)
	if !ok {
		return
	}
	target, err := pathUUID(r, "user_id")
	if err != nil {
		fail(w, err)
		return
	}
	var req changeMemberRoleRequest
	if err := decodeStrict(r, &req); err != nil {
		fail(w, err)
		return
	}
	if err := h.service.ChangeMemberRole(r.Context(), actor, orgID, target, domain.Role(req.Role)); err != nil {
		fail(w, err)
		return
	}
	httpresponse.JSON(w, http.StatusNoContent, nil)
}

// RemoveMember removes a member. Owners and admins, within the role rules of the domain.
func (h *Handler) RemoveMember(w http.ResponseWriter, r *http.Request) {
	actor, orgID, ok := h.actorAndOrg(w, r)
	if !ok {
		return
	}
	target, err := pathUUID(r, "user_id")
	if err != nil {
		fail(w, err)
		return
	}
	if err := h.service.RemoveMember(r.Context(), actor, orgID, target); err != nil {
		fail(w, err)
		return
	}
	httpresponse.JSON(w, http.StatusNoContent, nil)
}

// actorAndOrg reads the authenticated user and the {org} path parameter. On failure it writes the error
// and returns ok false.
func (h *Handler) actorAndOrg(w http.ResponseWriter, r *http.Request) (uuid.UUID, uuid.UUID, bool) {
	actor, err := actorFrom(r)
	if err != nil {
		fail(w, err)
		return uuid.Nil, uuid.Nil, false
	}
	orgID, err := pathUUID(r, "org")
	if err != nil {
		fail(w, err)
		return uuid.Nil, uuid.Nil, false
	}
	return actor, orgID, true
}
