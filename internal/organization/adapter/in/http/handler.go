// Package http exposes the organization hexagon over HTTP: organizations, members, invitations.
package http

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/jhonsferg/corelog/internal/organization/application/port"
	"github.com/jhonsferg/corelog/internal/platform/apperror"
	"github.com/jhonsferg/corelog/internal/platform/clock"
	"github.com/jhonsferg/corelog/internal/platform/httpresponse"
	"github.com/jhonsferg/corelog/internal/platform/middleware"
	"github.com/jhonsferg/corelog/internal/platform/validation"
)

// CodeBodyInvalid is returned when a body is not valid JSON or has unknown fields.
const CodeBodyInvalid = "org.body_invalid"

// CodeIDInvalid is returned when a path identifier is not a UUID.
const CodeIDInvalid = "org.id_invalid"

// CodeStatusFilterInvalid is returned for an unknown status filter on the invitations list.
const CodeStatusFilterInvalid = "org.status_filter_invalid"

// UserEmailLookup resolves the email of an account. The JWT carries no email, so accepting an invitation
// needs it. The implementation lives in cmd/api; this package only declares what it needs.
type UserEmailLookup interface {
	EmailOf(ctx context.Context, userID uuid.UUID) (string, error)
}

// Handler serves the organization routes.
type Handler struct {
	service   port.Service
	emails    UserEmailLookup
	clock     clock.Clock
	jwtSecret string
	publicURL string
	// consents is nil when the legal module is not wired.
	consents ConsentGate
}

// NewHandler builds the handler. publicURL is the origin used in accept URLs (for example
// https://corelog.example.com); when empty, the origin of each request is used.
func NewHandler(service port.Service, emails UserEmailLookup, clk clock.Clock, jwtSecret, publicURL string) *Handler {
	return &Handler{
		service:   service,
		emails:    emails,
		clock:     clk,
		jwtSecret: jwtSecret,
		publicURL: strings.TrimRight(publicURL, "/"),
	}
}

func actorFrom(r *http.Request) (uuid.UUID, error) {
	id, ok := middleware.UserIDFromContext(r.Context())
	if !ok {
		return uuid.Nil, apperror.WithCode(apperror.KindUnauthorized, "auth.unauthorized", "autenticación requerida")
	}
	return id, nil
}

func pathUUID(r *http.Request, name string) (uuid.UUID, error) {
	id, err := uuid.Parse(chi.URLParam(r, name))
	if err != nil {
		return uuid.Nil, apperror.WithCode(apperror.KindValidation, CodeIDInvalid, "identificador no válido")
	}
	return id, nil
}

// decodeStrict decodes the body into dst rejecting unknown fields, then validates it.
func decodeStrict(r *http.Request, dst any) error {
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		return apperror.WithCode(apperror.KindValidation, CodeBodyInvalid, "cuerpo de la petición no válido")
	}
	if err := validation.Struct(dst); err != nil {
		return validation.Failed(dst, err)
	}
	return nil
}

// fail writes err through httpresponse; every handler error goes through here.
func fail(w http.ResponseWriter, err error) {
	httpresponse.Error(w, err)
}

// acceptURL builds ${PUBLIC_URL or request origin}/invite/<token>.
func (h *Handler) acceptURL(r *http.Request, token string) string {
	base := h.publicURL
	if base == "" {
		scheme := "http"
		if r.TLS != nil || strings.EqualFold(r.Header.Get("X-Forwarded-Proto"), "https") {
			scheme = "https"
		}
		base = scheme + "://" + r.Host
	}
	return base + "/invite/" + token
}

// chiURLParam reads a path parameter as a plain string (used for invitation tokens).
func chiURLParam(r *http.Request, name string) string {
	return chi.URLParam(r, name)
}
