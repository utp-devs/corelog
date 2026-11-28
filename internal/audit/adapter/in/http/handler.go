// Package http exposes the read-only audit log API. There are no write endpoints: entries are append-only.
package http

import (
	"bytes"
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/jhonsferg/corelog/internal/audit/application/port"
	"github.com/jhonsferg/corelog/internal/audit/domain"
	"github.com/jhonsferg/corelog/internal/platform/apperror"
	"github.com/jhonsferg/corelog/internal/platform/httpresponse"
	appmw "github.com/jhonsferg/corelog/internal/platform/middleware"
	"github.com/jhonsferg/corelog/internal/platform/pagination"
)

const codeFilterInvalid = "audit.filter_invalid"

// Handler serves GET /admin/audit, GET /admin/audit/export and the organization scoped equivalents.
type Handler struct {
	service   port.Service
	jwtSecret string
}

// NewHandler returns the audit handler.
func NewHandler(service port.Service, jwtSecret string) *Handler {
	return &Handler{service: service, jwtSecret: jwtSecret}
}

// RegisterRoutes registers the routes with their full paths.
func (h *Handler) RegisterRoutes(r chi.Router) {
	r.Group(func(r chi.Router) {
		r.Use(appmw.Authenticate(h.jwtSecret))
		r.Group(func(r chi.Router) {
			r.Use(appmw.RequireRole("admin"))
			r.Get("/admin/audit", h.ListPlatform)
			r.Get("/admin/audit/export", h.ExportPlatform)
		})
		r.Get("/organizations/{org}/audit", h.ListOrganization)
		r.Get("/organizations/{org}/audit/export", h.ExportOrganization)
	})
}

type entryResponse struct {
	ID             int64          `json:"id"`
	OrganizationID *uuid.UUID     `json:"organization_id"`
	ProjectID      *uuid.UUID     `json:"project_id"`
	ActorID        *uuid.UUID     `json:"actor_id"`
	ActorName      string         `json:"actor_name"`
	Action         string         `json:"action"`
	EntityType     string         `json:"entity_type"`
	EntityID       *uuid.UUID     `json:"entity_id"`
	Summary        string         `json:"summary"`
	Before         map[string]any `json:"before"`
	After          map[string]any `json:"after"`
	IP             string         `json:"ip"`
	UserAgent      string         `json:"user_agent"`
	RequestID      string         `json:"request_id"`
	CreatedAt      time.Time      `json:"created_at"`
}

func toResponse(e domain.Entry) entryResponse {
	return entryResponse{
		ID: e.ID, OrganizationID: e.OrganizationID, ProjectID: e.ProjectID, ActorID: e.ActorID,
		ActorName: e.ActorName, Action: e.Action, EntityType: e.EntityType, EntityID: e.EntityID,
		Summary: e.Summary, Before: e.Before, After: e.After, IP: e.IP, UserAgent: e.UserAgent,
		RequestID: e.RequestID, CreatedAt: e.CreatedAt,
	}
}

func (h *Handler) ListPlatform(w http.ResponseWriter, r *http.Request) {
	f, err := parseFilter(r)
	if err != nil {
		httpresponse.Error(w, err)
		return
	}
	page, err := h.service.ListPlatform(r.Context(), f)
	writePage(w, page, err)
}

func (h *Handler) ListOrganization(w http.ResponseWriter, r *http.Request) {
	actor, orgID, ok := caller(w, r)
	if !ok {
		return
	}
	f, err := parseFilter(r)
	if err != nil {
		httpresponse.Error(w, err)
		return
	}
	page, err := h.service.ListOrganization(r.Context(), actor, orgID, f)
	writePage(w, page, err)
}

func (h *Handler) ExportPlatform(w http.ResponseWriter, r *http.Request) {
	f, err := parseFilter(r)
	if err != nil {
		httpresponse.Error(w, err)
		return
	}
	var buf bytes.Buffer
	if err := h.service.ExportPlatform(r.Context(), f, &buf); err != nil {
		httpresponse.Error(w, err)
		return
	}
	writeCSV(w, buf.Bytes())
}

func (h *Handler) ExportOrganization(w http.ResponseWriter, r *http.Request) {
	actor, orgID, ok := caller(w, r)
	if !ok {
		return
	}
	f, err := parseFilter(r)
	if err != nil {
		httpresponse.Error(w, err)
		return
	}
	var buf bytes.Buffer
	if err := h.service.ExportOrganization(r.Context(), actor, orgID, f, &buf); err != nil {
		httpresponse.Error(w, err)
		return
	}
	writeCSV(w, buf.Bytes())
}

func caller(w http.ResponseWriter, r *http.Request) (actor, orgID uuid.UUID, ok bool) {
	actor, has := appmw.UserIDFromContext(r.Context())
	if !has {
		httpresponse.Error(w, apperror.New(apperror.KindUnauthorized, "not authenticated"))
		return uuid.Nil, uuid.Nil, false
	}
	orgID, err := uuid.Parse(chi.URLParam(r, "org"))
	if err != nil {
		httpresponse.Error(w, apperror.WithCode(apperror.KindNotFound, "organization.not_found", "organizacion no encontrada"))
		return uuid.Nil, uuid.Nil, false
	}
	return actor, orgID, true
}

func writeCSV(w http.ResponseWriter, body []byte) {
	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", `attachment; filename="auditoria.csv"`)
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(body)
}

type pageResponse struct {
	Data []entryResponse `json:"data"`
	Meta struct {
		NextCursor *string `json:"next_cursor"`
	} `json:"meta"`
}

func writePage(w http.ResponseWriter, page port.ListPage, err error) {
	if err != nil {
		httpresponse.Error(w, err)
		return
	}
	out := pageResponse{Data: make([]entryResponse, 0, len(page.Entries))}
	for _, e := range page.Entries {
		out.Data = append(out.Data, toResponse(e))
	}
	if page.NextCursor > 0 {
		if c, err := pagination.EncodeCursor(page.NextCursor); err == nil {
			out.Meta.NextCursor = &c
		}
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(out)
}

func invalid(field, msg string) error {
	return apperror.WithCode(apperror.KindValidation, codeFilterInvalid, msg).
		WithDetails(apperror.FieldError{Field: field, Message: msg})
}

func parseFilter(r *http.Request) (port.ListFilter, error) {
	q := r.URL.Query()
	var f port.ListFilter
	if v := q.Get("actor"); v != "" {
		id, err := uuid.Parse(v)
		if err != nil {
			return f, invalid("actor", "El actor debe ser un identificador válido")
		}
		f.ActorID = &id
	}
	f.EntityType = q.Get("entity_type")
	f.Action = q.Get("action")
	if len(f.EntityType) > 64 || len(f.Action) > 64 {
		return f, invalid("action", "El filtro es demasiado largo")
	}
	if v := q.Get("from"); v != "" {
		t, err := parseTime(v, false)
		if err != nil {
			return f, invalid("from", "La fecha inicial no es válida (use AAAA-MM-DD o RFC 3339)")
		}
		f.Since = &t
	}
	if v := q.Get("to"); v != "" {
		t, err := parseTime(v, true)
		if err != nil {
			return f, invalid("to", "La fecha final no es válida (use AAAA-MM-DD o RFC 3339)")
		}
		f.Until = &t
	}
	if f.Since != nil && f.Until != nil && !f.Until.After(*f.Since) {
		return f, invalid("to", "La fecha final debe ser posterior a la inicial")
	}
	if v := q.Get("limit"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 1 || n > port.MaxLimit {
			return f, invalid("limit", "El límite debe estar entre 1 y "+strconv.Itoa(port.MaxLimit))
		}
		f.Limit = n
	}
	var cursor int64
	if err := pagination.DecodeCursor(q.Get("cursor"), &cursor); err != nil || cursor < 0 {
		return f, invalid("cursor", "El cursor no es válido")
	}
	f.Cursor = cursor
	return f, nil
}

// parseTime accepts RFC 3339 or a date. A date-only upper bound covers that whole day (exclusive next day).
func parseTime(v string, upper bool) (time.Time, error) {
	if t, err := time.Parse(time.RFC3339, v); err == nil {
		return t.UTC(), nil
	}
	t, err := time.Parse("2006-01-02", v)
	if err != nil {
		return time.Time{}, err
	}
	if upper {
		t = t.AddDate(0, 0, 1)
	}
	return t.UTC(), nil
}
