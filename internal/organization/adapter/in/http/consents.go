package http

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"github.com/google/uuid"

	"github.com/jhonsferg/corelog/internal/platform/apperror"
	"github.com/jhonsferg/corelog/internal/platform/httpresponse"
	appmw "github.com/jhonsferg/corelog/internal/platform/middleware"
)

// ConsentRef is one legal document version accepted together with an invitation.
type ConsentRef struct {
	Slug    string
	Version int
}

// ConsentGate is the narrow port to the legal module. Require records the consents sent with the invitation
// and fails with a validation error unless the user has accepted every required current document.
type ConsentGate interface {
	Require(ctx context.Context, userID uuid.UUID, refs []ConsentRef, source, ip, userAgent string) error
}

// WithConsentGate makes accepting an invitation require the current legal documents.
func (h *Handler) WithConsentGate(gate ConsentGate) *Handler {
	h.consents = gate
	return h
}

type acceptRequest struct {
	Consents []struct {
		Slug    string `json:"slug"`
		Version int    `json:"version"`
	} `json:"consents"`
}

// requireConsents reads the optional body of the accept call and enforces the legal consent. It writes the
// error response and returns false when the invitation must not be accepted.
func (h *Handler) requireConsents(w http.ResponseWriter, r *http.Request, actor uuid.UUID) bool {
	if h.consents == nil {
		return true
	}
	var req acceptRequest
	if r.Body != nil {
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil && !errors.Is(err, io.EOF) {
			httpresponse.Error(w, apperror.WithCode(apperror.KindValidation, "validation.failed", "El cuerpo de la peticion no es valido"))
			return false
		}
	}
	refs := make([]ConsentRef, 0, len(req.Consents))
	for _, c := range req.Consents {
		refs = append(refs, ConsentRef{Slug: c.Slug, Version: c.Version})
	}
	if err := h.consents.Require(r.Context(), actor, refs, "invite", appmw.ClientIP(r), r.UserAgent()); err != nil {
		httpresponse.Error(w, err)
		return false
	}
	return true
}
