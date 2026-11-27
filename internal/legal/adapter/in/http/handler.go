// Package http exposes the legal documents and consents over HTTP.
package http

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/jhonsferg/corelog/internal/legal/application/port"
	"github.com/jhonsferg/corelog/internal/legal/domain"
	"github.com/jhonsferg/corelog/internal/platform/apperror"
	"github.com/jhonsferg/corelog/internal/platform/authtoken"
	"github.com/jhonsferg/corelog/internal/platform/httpresponse"
	appmw "github.com/jhonsferg/corelog/internal/platform/middleware"
)

// Enforcement modes of the consent requirement.
const (
	ModeWarn  = "warn"
	ModeBlock = "block"
)

const publicCacheControl = "public, max-age=300"

// Handler serves the legal endpoints.
type Handler struct {
	service   port.Service
	jwtSecret string
	mode      string
}

// NewHandler builds the handler. mode is the LEGAL_ENFORCEMENT value; anything but "block" means "warn".
func NewHandler(service port.Service, jwtSecret, mode string) *Handler {
	if mode != ModeBlock {
		mode = ModeWarn
	}
	return &Handler{service: service, jwtSecret: jwtSecret, mode: mode}
}

// RegisterRoutes mounts the public, authenticated and platform admin routes.
func (h *Handler) RegisterRoutes(r chi.Router) {
	r.Get("/legal/documents", h.ListDocuments)
	r.Get("/legal/documents/{slug}", h.GetDocument)

	r.Route("/me/consents", func(r chi.Router) {
		r.Use(appmw.Authenticate(h.jwtSecret))
		r.Get("/", h.MyConsents)
		r.Post("/", h.RecordConsents)
		r.Delete("/{slug}", h.RevokeConsent)
	})

	r.Route("/admin/legal", func(r chi.Router) {
		r.Use(appmw.Authenticate(h.jwtSecret))
		r.Use(appmw.RequireRole("admin"))
		r.Get("/documents", h.AdminListDocuments)
		r.Post("/documents", h.PublishDocument)
		r.Get("/consents", h.AuditConsents)
	})
}

func (h *Handler) ListDocuments(w http.ResponseWriter, r *http.Request) {
	docs, err := h.service.ListCurrent(r.Context())
	if err != nil {
		httpresponse.Error(w, err)
		return
	}
	var sb strings.Builder
	out := make([]documentSummary, 0, len(docs))
	for _, d := range docs {
		out = append(out, toSummary(d))
		sb.WriteString(string(d.Slug) + ":" + strconv.Itoa(d.Version) + ";")
	}
	sum := sha256.Sum256([]byte(sb.String()))
	if notModified(w, r, `"legal-`+hex.EncodeToString(sum[:8])+`"`) {
		return
	}
	httpresponse.JSON(w, http.StatusOK, out)
}

func (h *Handler) GetDocument(w http.ResponseWriter, r *http.Request) {
	version := 0
	if raw := r.URL.Query().Get("version"); raw != "" {
		v, err := strconv.Atoi(raw)
		if err != nil || v < 1 {
			httpresponse.Error(w, domain.ErrVersionInvalid)
			return
		}
		version = v
	}
	doc, err := h.service.GetDocument(r.Context(), domain.Slug(chi.URLParam(r, "slug")), version)
	if err != nil {
		httpresponse.Error(w, err)
		return
	}
	if notModified(w, r, `"`+string(doc.Slug)+`-v`+strconv.Itoa(doc.Version)+`"`) {
		return
	}
	httpresponse.JSON(w, http.StatusOK, toDetail(*doc))
}

// notModified sets the cache headers and answers 304 when the client already has the representation.
func notModified(w http.ResponseWriter, r *http.Request, etag string) bool {
	w.Header().Set("ETag", etag)
	w.Header().Set("Cache-Control", publicCacheControl)
	for _, candidate := range strings.Split(r.Header.Get("If-None-Match"), ",") {
		candidate = strings.TrimPrefix(strings.TrimSpace(candidate), "W/")
		if candidate == etag || candidate == "*" {
			w.WriteHeader(http.StatusNotModified)
			return true
		}
	}
	return false
}

func currentUser(w http.ResponseWriter, r *http.Request) (uuid.UUID, bool) {
	id, ok := appmw.UserIDFromContext(r.Context())
	if !ok {
		httpresponse.Error(w, apperror.New(apperror.KindUnauthorized, "not authenticated"))
	}
	return id, ok
}

func decodeStrict(w http.ResponseWriter, r *http.Request, dst any) bool {
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		httpresponse.Error(w, apperror.WithCode(apperror.KindValidation, "validation.failed", "El cuerpo de la peticion no es valido"))
		return false
	}
	return true
}

func (h *Handler) MyConsents(w http.ResponseWriter, r *http.Request) {
	userID, ok := currentUser(w, r)
	if !ok {
		return
	}
	st, err := h.service.Status(r.Context(), userID)
	if err != nil {
		httpresponse.Error(w, err)
		return
	}
	httpresponse.JSON(w, http.StatusOK, toStatus(st))
}

func (h *Handler) RecordConsents(w http.ResponseWriter, r *http.Request) {
	userID, ok := currentUser(w, r)
	if !ok {
		return
	}
	var req recordRequest
	if !decodeStrict(w, r, &req) {
		return
	}
	refs := make([]domain.Ref, 0, len(req.Accepted))
	for _, a := range req.Accepted {
		refs = append(refs, domain.Ref{Slug: domain.Slug(a.Slug), Version: a.Version})
	}
	st, err := h.service.Record(r.Context(), userID, port.RecordInput{
		Accepted: refs, Source: domain.Source(req.Source), IP: appmw.ClientIP(r), UserAgent: r.UserAgent(),
	})
	if err != nil {
		httpresponse.Error(w, err)
		return
	}
	httpresponse.JSON(w, http.StatusOK, toStatus(st))
}

func (h *Handler) RevokeConsent(w http.ResponseWriter, r *http.Request) {
	userID, ok := currentUser(w, r)
	if !ok {
		return
	}
	if err := h.service.Revoke(r.Context(), userID, domain.Slug(chi.URLParam(r, "slug"))); err != nil {
		httpresponse.Error(w, err)
		return
	}
	httpresponse.JSON(w, http.StatusNoContent, nil)
}

func (h *Handler) AdminListDocuments(w http.ResponseWriter, r *http.Request) {
	docs, err := h.service.AdminList(r.Context())
	if err != nil {
		httpresponse.Error(w, err)
		return
	}
	out := make([]documentDetail, 0, len(docs))
	for _, d := range docs {
		out = append(out, toDetail(d))
	}
	httpresponse.JSON(w, http.StatusOK, out)
}

func (h *Handler) PublishDocument(w http.ResponseWriter, r *http.Request) {
	actor, ok := currentUser(w, r)
	if !ok {
		return
	}
	var req publishRequest
	if !decodeStrict(w, r, &req) {
		return
	}
	doc, err := h.service.Publish(r.Context(), actor, port.PublishInput{
		Slug: domain.Slug(req.Slug), Title: req.Title, Summary: req.Summary, BodyMarkdown: req.BodyMarkdown, Required: req.Required,
	})
	if err != nil {
		httpresponse.Error(w, err)
		return
	}
	httpresponse.JSON(w, http.StatusCreated, toDetail(*doc))
}

func (h *Handler) AuditConsents(w http.ResponseWriter, r *http.Request) {
	var userID *uuid.UUID
	if raw := r.URL.Query().Get("user"); raw != "" {
		id, err := uuid.Parse(raw)
		if err != nil {
			httpresponse.Error(w, apperror.WithCode(apperror.KindValidation, "validation.failed", "user no es un UUID valido"))
			return
		}
		userID = &id
	}
	limit := 0
	if raw := r.URL.Query().Get("limit"); raw != "" {
		v, err := strconv.Atoi(raw)
		if err != nil || v < 1 {
			httpresponse.Error(w, apperror.WithCode(apperror.KindValidation, "validation.failed", "limit no es valido"))
			return
		}
		limit = v
	}
	rows, err := h.service.Audit(r.Context(), userID, limit)
	if err != nil {
		httpresponse.Error(w, err)
		return
	}
	out := make([]consentResponse, 0, len(rows))
	for _, c := range rows {
		out = append(out, toConsent(c))
	}
	httpresponse.JSON(w, http.StatusOK, out)
}

// Enforce is the router-level middleware of the block mode: while the caller has pending required
// consents, every authenticated call except the exempt ones answers 403 legal.consent_pending. In warn mode
// it does nothing.
func (h *Handler) Enforce(next http.Handler) http.Handler {
	if h.mode != ModeBlock {
		return next
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token, ok := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer ")
		if !ok || token == "" || exemptFromEnforcement(r) {
			next.ServeHTTP(w, r)
			return
		}
		claims, err := authtoken.Parse(h.jwtSecret, token)
		if err != nil {
			next.ServeHTTP(w, r) // the module answers 401
			return
		}
		pending, err := h.service.Pending(r.Context(), claims.UserID)
		if err != nil {
			httpresponse.Error(w, err)
			return
		}
		if len(pending) > 0 {
			slugs := make([]domain.Slug, 0, len(pending))
			for _, p := range pending {
				slugs = append(slugs, p.Slug)
			}
			httpresponse.Error(w, domain.ErrConsentPending(slugs))
			return
		}
		next.ServeHTTP(w, r)
	})
}

func exemptFromEnforcement(r *http.Request) bool {
	p := strings.TrimSuffix(r.URL.Path, "/")
	switch {
	case strings.HasPrefix(p, "/api/v1/legal/"), strings.HasPrefix(p, "/api/v1/auth/"):
		return true
	case p == "/api/v1/me/consents", strings.HasPrefix(p, "/api/v1/me/consents/"):
		return true
	case p == "/api/v1/users/me" && r.Method == http.MethodGet:
		return true
	}
	return false
}
