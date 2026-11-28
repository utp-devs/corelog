// Package http exposes the personal API token endpoints.
package http

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/jhonsferg/corelog/internal/apitoken/application/port"
	"github.com/jhonsferg/corelog/internal/apitoken/domain"
	"github.com/jhonsferg/corelog/internal/platform/apperror"
	"github.com/jhonsferg/corelog/internal/platform/httpresponse"
	appmw "github.com/jhonsferg/corelog/internal/platform/middleware"
)

// Handler serves /me/api-tokens. Every route needs a real session: a token cannot manage tokens.
type Handler struct {
	service   port.Service
	jwtSecret string
}

// NewHandler returns the handler.
func NewHandler(service port.Service, jwtSecret string) *Handler {
	return &Handler{service: service, jwtSecret: jwtSecret}
}

// RegisterRoutes mounts the routes on r.
func (h *Handler) RegisterRoutes(r chi.Router) {
	r.Route("/me/api-tokens", func(r chi.Router) {
		r.Use(appmw.Authenticate(h.jwtSecret))
		r.Use(requireSession)
		r.Get("/", h.List)
		r.Post("/", h.Create)
		r.Delete("/{id}", h.Revoke)
	})
}

func requireSession(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, viaToken := appmw.APITokenIDFromContext(r.Context()); viaToken {
			httpresponse.Error(w, domain.ErrSessionRequired)
			return
		}
		next.ServeHTTP(w, r)
	})
}

type tokenDTO struct {
	ID         uuid.UUID  `json:"id"`
	Name       string     `json:"name"`
	Prefix     string     `json:"prefix"`
	Scopes     []string   `json:"scopes"`
	ExpiresAt  *time.Time `json:"expires_at"`
	LastUsedAt *time.Time `json:"last_used_at"`
	RevokedAt  *time.Time `json:"revoked_at"`
	CreatedAt  time.Time  `json:"created_at"`
	// Token is the plaintext, present only in the creation response.
	Token string `json:"token,omitempty"`
}

func toDTO(t domain.Token, plaintext string) tokenDTO {
	scopes := make([]string, len(t.Scopes))
	for i, s := range t.Scopes {
		scopes[i] = string(s)
	}
	return tokenDTO{ID: t.ID, Name: t.Name, Prefix: t.Prefix, Scopes: scopes, ExpiresAt: t.ExpiresAt,
		LastUsedAt: t.LastUsedAt, RevokedAt: t.RevokedAt, CreatedAt: t.CreatedAt, Token: plaintext}
}

type createRequest struct {
	Name      string     `json:"name"`
	Scopes    []string   `json:"scopes"`
	ExpiresAt *time.Time `json:"expires_at"`
}

// List returns the tokens of the caller, without secrets.
func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	userID, ok := appmw.UserIDFromContext(r.Context())
	if !ok {
		httpresponse.Error(w, apperror.New(apperror.KindUnauthorized, "No autenticado"))
		return
	}
	tokens, err := h.service.List(r.Context(), userID)
	if err != nil {
		httpresponse.Error(w, err)
		return
	}
	out := make([]tokenDTO, len(tokens))
	for i, t := range tokens {
		out[i] = toDTO(t, "")
	}
	httpresponse.JSON(w, http.StatusOK, out)
}

// Create stores a token and returns its plaintext once.
func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	userID, ok := appmw.UserIDFromContext(r.Context())
	if !ok {
		httpresponse.Error(w, apperror.New(apperror.KindUnauthorized, "No autenticado"))
		return
	}
	var req createRequest
	if err := decodeStrict(r, &req); err != nil {
		httpresponse.Error(w, err)
		return
	}
	role, _ := appmw.RoleFromContext(r.Context())
	tok, plaintext, err := h.service.Create(r.Context(), userID, port.CreateInput{
		Name: req.Name, Scopes: req.Scopes, ExpiresAt: req.ExpiresAt, GlobalRole: role,
	})
	if err != nil {
		httpresponse.Error(w, err)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	httpresponse.JSON(w, http.StatusCreated, toDTO(tok, plaintext))
}

// Revoke revokes a token of the caller.
func (h *Handler) Revoke(w http.ResponseWriter, r *http.Request) {
	userID, ok := appmw.UserIDFromContext(r.Context())
	if !ok {
		httpresponse.Error(w, apperror.New(apperror.KindUnauthorized, "No autenticado"))
		return
	}
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpresponse.Error(w, domain.ErrNotFound)
		return
	}
	if err := h.service.Revoke(r.Context(), userID, id); err != nil {
		httpresponse.Error(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func decodeStrict(r *http.Request, into any) error {
	raw, err := io.ReadAll(io.LimitReader(r.Body, 1<<16))
	if err != nil {
		return apperror.WithCode(apperror.KindValidation, "validation.failed", "No se pudo leer el cuerpo de la peticion")
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(into); err != nil || dec.More() {
		return apperror.WithCode(apperror.KindValidation, "validation.failed", "El cuerpo de la peticion no es valido")
	}
	return nil
}
