package http

import (
	"net/http"

	"github.com/jhonsferg/corelog/internal/platform/httpresponse"
	appmw "github.com/jhonsferg/corelog/internal/platform/middleware"
	"github.com/jhonsferg/corelog/internal/user/application/port"
	"github.com/jhonsferg/corelog/internal/user/domain"
)

// consentRefRequest is one accepted legal document version sent at registration.
type consentRefRequest struct {
	Slug    string `json:"slug"`
	Version int    `json:"version"`
}

type pendingConsentResponse struct {
	Slug    string `json:"slug"`
	Title   string `json:"title"`
	Version int    `json:"version"`
}

// meResponse is the current user plus the legal documents still to accept.
type meResponse struct {
	userResponse
	PendingConsents []pendingConsentResponse `json:"pending_consents"`
}

// WithConsents enables the pending consents in the login and current user responses.
func (h *Handler) WithConsents(gate port.ConsentGate) *Handler {
	h.consents = gate
	return h
}

func toConsentRefs(in []consentRefRequest) []port.ConsentRef {
	out := make([]port.ConsentRef, 0, len(in))
	for _, c := range in {
		out = append(out, port.ConsentRef{Slug: c.Slug, Version: c.Version})
	}
	return out
}

func consentMeta(r *http.Request, source string) port.ConsentMeta {
	return port.ConsentMeta{Source: source, IP: appmw.ClientIP(r), UserAgent: r.UserAgent()}
}

// pendingConsents lists the required legal documents the user has not accepted; it is always non-nil.
func (h *Handler) pendingConsents(r *http.Request, user *domain.User) ([]pendingConsentResponse, error) {
	out := []pendingConsentResponse{}
	if h.consents == nil {
		return out, nil
	}
	pending, err := h.consents.Pending(r.Context(), user.ID)
	if err != nil {
		return nil, err
	}
	for _, p := range pending {
		out = append(out, pendingConsentResponse{Slug: p.Slug, Title: p.Title, Version: p.Version})
	}
	return out, nil
}

// writeSession answers a successful login with the token, the user and the pending consents.
func (h *Handler) writeSession(w http.ResponseWriter, r *http.Request, token string, user *domain.User) {
	pending, err := h.pendingConsents(r, user)
	if err != nil {
		httpresponse.Error(w, err)
		return
	}
	httpresponse.JSON(w, http.StatusOK, authResponse{Token: token, User: toPrivateUserResponse(user), PendingConsents: pending})
}

// writeMe answers the current user with the pending consents.
func (h *Handler) writeMe(w http.ResponseWriter, r *http.Request, user *domain.User) {
	pending, err := h.pendingConsents(r, user)
	if err != nil {
		httpresponse.Error(w, err)
		return
	}
	httpresponse.JSON(w, http.StatusOK, meResponse{userResponse: toPrivateUserResponse(user), PendingConsents: pending})
}
