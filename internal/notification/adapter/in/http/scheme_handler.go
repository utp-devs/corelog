package http

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/jhonsferg/corelog/internal/notification/application/port"
	"github.com/jhonsferg/corelog/internal/notification/domain"
	"github.com/jhonsferg/corelog/internal/platform/apperror"
	"github.com/jhonsferg/corelog/internal/platform/httpresponse"
	appmw "github.com/jhonsferg/corelog/internal/platform/middleware"
	"github.com/jhonsferg/corelog/internal/platform/pagination"
)

// WithSchemes enables the notification scheme endpoints.
func (h *Handler) WithSchemes(service port.SchemeService) *Handler {
	h.schemes = service
	return h
}

// WithOutboxAdmin enables the platform administration of the email queue.
func (h *Handler) WithOutboxAdmin(service port.OutboxAdmin) *Handler {
	h.outbox = service
	return h
}

func (h *Handler) registerExtraRoutes(r chi.Router) {
	if h.schemes != nil {
		r.Group(func(r chi.Router) {
			r.Use(appmw.Authenticate(h.jwtSecret))
			r.Get("/projects/{project}/notification-scheme", h.GetProjectScheme)
			r.Put("/projects/{project}/notification-scheme", h.PutProjectScheme)
			r.Delete("/projects/{project}/notification-scheme", h.ResetProjectScheme)
			r.Get("/organizations/{org}/notification-scheme", h.GetOrgScheme)
			r.Put("/organizations/{org}/notification-scheme", h.PutOrgScheme)
			r.Delete("/organizations/{org}/notification-scheme", h.ResetOrgScheme)
		})
	}
	if h.outbox != nil {
		r.Group(func(r chi.Router) {
			r.Use(appmw.Authenticate(h.jwtSecret))
			r.Use(appmw.RequireRole("admin"))
			r.Get("/admin/email-outbox", h.ListOutbox)
			r.Post("/admin/email-outbox/{id}/retry", h.RetryOutbox)
		})
	}
}

type recipientDTO struct {
	Type string `json:"type"`
	ID   string `json:"id,omitempty"`
	Role string `json:"role,omitempty"`
}

type ruleDTO struct {
	Event      string         `json:"event"`
	Recipients []recipientDTO `json:"recipients"`
	Channels   []string       `json:"channels"`
	Overridden *bool          `json:"overridden,omitempty"`
}

type schemeRequest struct {
	Rules []ruleDTO `json:"rules"`
}

type schemeResponse struct {
	Scope          string     `json:"scope"`
	OrganizationID string     `json:"organization_id"`
	ProjectID      *string    `json:"project_id"`
	IsDefault      bool       `json:"is_default"`
	UpdatedAt      *time.Time `json:"updated_at"`
	Rules          []ruleDTO  `json:"rules"`
}

func toSchemeResponse(v port.SchemeView) schemeResponse {
	resp := schemeResponse{Scope: "organization", OrganizationID: v.OrganizationID.String(), IsDefault: len(v.Overridden) == 0, UpdatedAt: v.UpdatedAt}
	if v.ProjectID != nil {
		resp.Scope = "project"
		id := v.ProjectID.String()
		resp.ProjectID = &id
	}
	resp.Rules = make([]ruleDTO, 0, len(v.Scheme.Rules))
	for _, rule := range v.Scheme.Rules {
		over := v.Overridden[rule.Event]
		dto := ruleDTO{Event: string(rule.Event), Recipients: []recipientDTO{}, Channels: []string{}, Overridden: &over}
		for _, rec := range rule.Recipients {
			item := recipientDTO{Type: string(rec.Type), Role: rec.Role}
			if rec.ID != uuid.Nil {
				item.ID = rec.ID.String()
			}
			dto.Recipients = append(dto.Recipients, item)
		}
		for _, c := range rule.Channels {
			dto.Channels = append(dto.Channels, string(c))
		}
		resp.Rules = append(resp.Rules, dto)
	}
	return resp
}

func decodeScheme(r *http.Request) (domain.Scheme, error) {
	var req schemeRequest
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&req); err != nil || req.Rules == nil {
		return domain.Scheme{}, domain.ErrSchemeInvalid("El cuerpo debe incluir la lista rules")
	}
	scheme := domain.Scheme{}
	for _, rule := range req.Rules {
		item := domain.Rule{Event: domain.Event(rule.Event)}
		for _, rec := range rule.Recipients {
			recipient := domain.Recipient{Type: domain.RecipientType(rec.Type), Role: rec.Role}
			if rec.ID != "" {
				id, err := uuid.Parse(rec.ID)
				if err != nil {
					return domain.Scheme{}, domain.ErrSchemeInvalid("El id del destinatario no es valido")
				}
				recipient.ID = id
			}
			item.Recipients = append(item.Recipients, recipient)
		}
		for _, c := range rule.Channels {
			item.Channels = append(item.Channels, domain.Channel(c))
		}
		scheme.Rules = append(scheme.Rules, item)
	}
	return scheme, nil
}

func pathID(r *http.Request, name string) (uuid.UUID, error) {
	id, err := uuid.Parse(chi.URLParam(r, name))
	if err != nil {
		return uuid.Nil, apperror.WithCode(apperror.KindNotFound, "notification.scope_not_found", "Recurso no encontrado")
	}
	return id, nil
}

func (h *Handler) caller(w http.ResponseWriter, r *http.Request) (uuid.UUID, string, bool) {
	userID, ok := h.actor(w, r)
	if !ok {
		return uuid.Nil, "", false
	}
	role, _ := appmw.RoleFromContext(r.Context())
	return userID, role, true
}

func (h *Handler) GetProjectScheme(w http.ResponseWriter, r *http.Request) {
	h.projectScheme(w, r, func(r *http.Request, actor uuid.UUID, role string, project uuid.UUID) (port.SchemeView, error) {
		return h.schemes.GetProject(r.Context(), actor, role, project)
	})
}

func (h *Handler) PutProjectScheme(w http.ResponseWriter, r *http.Request) {
	scheme, err := decodeScheme(r)
	if err != nil {
		httpresponse.Error(w, err)
		return
	}
	h.projectScheme(w, r, func(r *http.Request, actor uuid.UUID, role string, project uuid.UUID) (port.SchemeView, error) {
		return h.schemes.PutProject(r.Context(), actor, role, project, scheme)
	})
}

func (h *Handler) ResetProjectScheme(w http.ResponseWriter, r *http.Request) {
	h.projectScheme(w, r, func(r *http.Request, actor uuid.UUID, role string, project uuid.UUID) (port.SchemeView, error) {
		return h.schemes.ResetProject(r.Context(), actor, role, project)
	})
}

func (h *Handler) projectScheme(w http.ResponseWriter, r *http.Request, fn func(*http.Request, uuid.UUID, string, uuid.UUID) (port.SchemeView, error)) {
	actor, role, ok := h.caller(w, r)
	if !ok {
		return
	}
	project, err := pathID(r, "project")
	if err != nil {
		httpresponse.Error(w, err)
		return
	}
	view, err := fn(r, actor, role, project)
	if err != nil {
		httpresponse.Error(w, err)
		return
	}
	httpresponse.JSON(w, http.StatusOK, toSchemeResponse(view))
}

func (h *Handler) GetOrgScheme(w http.ResponseWriter, r *http.Request) {
	h.orgScheme(w, r, func(r *http.Request, actor, org uuid.UUID) (port.SchemeView, error) {
		return h.schemes.GetOrganization(r.Context(), actor, org)
	})
}

func (h *Handler) PutOrgScheme(w http.ResponseWriter, r *http.Request) {
	scheme, err := decodeScheme(r)
	if err != nil {
		httpresponse.Error(w, err)
		return
	}
	h.orgScheme(w, r, func(r *http.Request, actor, org uuid.UUID) (port.SchemeView, error) {
		return h.schemes.PutOrganization(r.Context(), actor, org, scheme)
	})
}

func (h *Handler) ResetOrgScheme(w http.ResponseWriter, r *http.Request) {
	h.orgScheme(w, r, func(r *http.Request, actor, org uuid.UUID) (port.SchemeView, error) {
		return h.schemes.ResetOrganization(r.Context(), actor, org)
	})
}

func (h *Handler) orgScheme(w http.ResponseWriter, r *http.Request, fn func(*http.Request, uuid.UUID, uuid.UUID) (port.SchemeView, error)) {
	actor, _, ok := h.caller(w, r)
	if !ok {
		return
	}
	org, err := pathID(r, "org")
	if err != nil {
		httpresponse.Error(w, err)
		return
	}
	view, err := fn(r, actor, org)
	if err != nil {
		httpresponse.Error(w, err)
		return
	}
	httpresponse.JSON(w, http.StatusOK, toSchemeResponse(view))
}

type outboxResponse struct {
	ID            string     `json:"id"`
	UserID        *string    `json:"user_id"`
	To            string     `json:"to"`
	Kind          string     `json:"kind"`
	Subject       string     `json:"subject"`
	Status        string     `json:"status"`
	Attempts      int        `json:"attempts"`
	NextAttemptAt time.Time  `json:"next_attempt_at"`
	LastError     string     `json:"last_error"`
	CreatedAt     time.Time  `json:"created_at"`
	SentAt        *time.Time `json:"sent_at"`
}

// maskEmail hides most of the local part: ana.lopez@example.com becomes a***@example.com.
func maskEmail(email string) string {
	local, domainPart, ok := strings.Cut(email, "@")
	if !ok || local == "" {
		return "***"
	}
	return local[:1] + "***@" + domainPart
}

func (h *Handler) ListOutbox(w http.ResponseWriter, r *http.Request) {
	params, err := pagination.ParseRequest(r)
	if err != nil {
		httpresponse.Error(w, err)
		return
	}
	entries, total, err := h.outbox.List(r.Context(), r.URL.Query().Get("status"), params.PageSize, params.Offset())
	if err != nil {
		httpresponse.Error(w, err)
		return
	}
	out := make([]outboxResponse, 0, len(entries))
	for _, e := range entries {
		item := outboxResponse{
			ID: e.ID.String(), To: maskEmail(e.To), Kind: e.Kind, Subject: e.Subject, Status: e.Status, Attempts: e.Attempts,
			NextAttemptAt: e.NextAttemptAt, LastError: e.LastError, CreatedAt: e.CreatedAt, SentAt: e.SentAt,
		}
		if e.UserID != nil {
			id := e.UserID.String()
			item.UserID = &id
		}
		out = append(out, item)
	}
	httpresponse.Paginated(w, http.StatusOK, out, httpresponse.PageMeta(pagination.NewMeta(params, total)))
}

func (h *Handler) RetryOutbox(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpresponse.Error(w, apperror.WithCode(apperror.KindNotFound, "notification.outbox_not_found", "Correo no encontrado"))
		return
	}
	if err := h.outbox.Retry(r.Context(), id); err != nil {
		httpresponse.Error(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
