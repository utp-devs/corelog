package http

import (
	"net/http"
	"strings"

	"github.com/jhonsferg/corelog/internal/organization/application/port"
	"github.com/jhonsferg/corelog/internal/organization/domain"
	"github.com/jhonsferg/corelog/internal/platform/apperror"
	"github.com/jhonsferg/corelog/internal/platform/httpresponse"
)

// ListInvitations returns the invitations of the organization. status=pending (default) lists the ones
// still open at this instant; status=all includes accepted, revoked and expired ones.
func (h *Handler) ListInvitations(w http.ResponseWriter, r *http.Request) {
	actor, orgID, ok := h.actorAndOrg(w, r)
	if !ok {
		return
	}
	pendingOnly := true
	switch strings.TrimSpace(r.URL.Query().Get("status")) {
	case "", "pending":
	case "all":
		pendingOnly = false
	default:
		fail(w, apperror.WithCode(apperror.KindValidation, CodeStatusFilterInvalid, "el filtro de estado debe ser pending o all"))
		return
	}
	invs, err := h.service.ListInvitations(r.Context(), actor, orgID, pendingOnly)
	if err != nil {
		fail(w, err)
		return
	}
	now := h.clock.Now()
	out := make([]invitationResponse, 0, len(invs))
	for _, inv := range invs {
		out = append(out, toInvitationResponse(inv, now))
	}
	httpresponse.JSON(w, http.StatusOK, out)
}

// CreateInvitations invites several emails at once. The plain token and accept URL are returned only here.
func (h *Handler) CreateInvitations(w http.ResponseWriter, r *http.Request) {
	actor, orgID, ok := h.actorAndOrg(w, r)
	if !ok {
		return
	}
	var req createInvitationsRequest
	if err := decodeStrict(r, &req); err != nil {
		fail(w, err)
		return
	}
	in := make([]port.InviteInput, 0, len(req.Invitations))
	for _, item := range req.Invitations {
		in = append(in, port.InviteInput{Email: item.Email, Role: domain.Role(item.Role)})
	}
	results, err := h.service.CreateInvitations(r.Context(), actor, orgID, in)
	if err != nil {
		fail(w, err)
		return
	}
	now := h.clock.Now()
	resp := createInvitationsResponse{Invitations: make([]invitationCreatedResponse, 0, len(results))}
	for _, res := range results {
		resp.Invitations = append(resp.Invitations, toInvitationCreatedResponse(res, h.acceptURL(r, res.PlainToken), now))
	}
	httpresponse.JSON(w, http.StatusCreated, resp)
}

// ResendInvitation rotates the token of a pending invitation and restarts its expiry. The old link stops
// working at once; the new token is returned here.
func (h *Handler) ResendInvitation(w http.ResponseWriter, r *http.Request) {
	actor, orgID, ok := h.actorAndOrg(w, r)
	if !ok {
		return
	}
	invID, err := pathUUID(r, "id")
	if err != nil {
		fail(w, err)
		return
	}
	res, err := h.service.ResendInvitation(r.Context(), actor, orgID, invID)
	if err != nil {
		fail(w, err)
		return
	}
	httpresponse.JSON(w, http.StatusOK, toInvitationCreatedResponse(*res, h.acceptURL(r, res.PlainToken), h.clock.Now()))
}

// RevokeInvitation revokes a pending invitation.
func (h *Handler) RevokeInvitation(w http.ResponseWriter, r *http.Request) {
	actor, orgID, ok := h.actorAndOrg(w, r)
	if !ok {
		return
	}
	invID, err := pathUUID(r, "id")
	if err != nil {
		fail(w, err)
		return
	}
	if err := h.service.RevokeInvitation(r.Context(), actor, orgID, invID); err != nil {
		fail(w, err)
		return
	}
	httpresponse.JSON(w, http.StatusNoContent, nil)
}

// PreviewInvitation describes an invitation to whoever holds its token. Public and free of side effects.
func (h *Handler) PreviewInvitation(w http.ResponseWriter, r *http.Request) {
	token := chiURLParam(r, "token")
	preview, err := h.service.GetInvitationPreview(r.Context(), token)
	if err != nil {
		fail(w, err)
		return
	}
	httpresponse.JSON(w, http.StatusOK, invitationPreviewResponse{
		OrganizationName: preview.OrganizationName,
		Role:             string(preview.Role),
		Email:            preview.Email,
		Status:           string(preview.Status),
		ExpiresAt:        preview.ExpiresAt,
	})
}

// AcceptInvitation adds the caller to the organization. The caller's email must match the invitation.
func (h *Handler) AcceptInvitation(w http.ResponseWriter, r *http.Request) {
	actor, err := actorFrom(r)
	if err != nil {
		fail(w, err)
		return
	}
	email, err := h.emails.EmailOf(r.Context(), actor)
	if err != nil {
		fail(w, err)
		return
	}
	member, err := h.service.AcceptInvitation(r.Context(), actor, email, chiURLParam(r, "token"))
	if err != nil {
		fail(w, err)
		return
	}
	httpresponse.JSON(w, http.StatusCreated, membershipResponse{
		OrganizationID: member.OrganizationID.String(),
		UserID:         member.UserID.String(),
		Role:           string(member.Role),
		JoinedAt:       member.JoinedAt,
	})
}
