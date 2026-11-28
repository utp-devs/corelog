// Package http exposes the webhook management endpoints of an organization.
package http

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/jhonsferg/corelog/internal/platform/apperror"
	"github.com/jhonsferg/corelog/internal/platform/httpresponse"
	appmw "github.com/jhonsferg/corelog/internal/platform/middleware"
	"github.com/jhonsferg/corelog/internal/webhook/application/port"
	"github.com/jhonsferg/corelog/internal/webhook/domain"
)

// Handler serves /organizations/{org}/webhooks.
type Handler struct {
	service   port.Service
	jwtSecret string
}

// NewHandler returns the handler.
func NewHandler(service port.Service, jwtSecret string) *Handler {
	return &Handler{service: service, jwtSecret: jwtSecret}
}

// RegisterRoutes mounts the routes with their full paths.
func (h *Handler) RegisterRoutes(r chi.Router) {
	r.Route("/organizations/{org}/webhooks", func(r chi.Router) {
		r.Use(appmw.Authenticate(h.jwtSecret))
		r.Get("/", h.List)
		r.Post("/", h.Create)
		r.Get("/{id}", h.Get)
		r.Patch("/{id}", h.Update)
		r.Delete("/{id}", h.Delete)
		r.Post("/{id}/test", h.Test)
		r.Post("/{id}/rotate-secret", h.Rotate)
		r.Get("/{id}/deliveries", h.Deliveries)
		r.Post("/{id}/deliveries/{delivery}/redeliver", h.Redeliver)
	})
}

type webhookDTO struct {
	ID          uuid.UUID  `json:"id"`
	OrgID       uuid.UUID  `json:"organization_id"`
	ProjectID   *uuid.UUID `json:"project_id"`
	Name        string     `json:"name"`
	URL         string     `json:"url"`
	Events      []string   `json:"events"`
	FilterQuery *string    `json:"filter_query"`
	Enabled     bool       `json:"enabled"`
	CreatedBy   *uuid.UUID `json:"created_by"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
	// Secret is the signing secret, present only in the creation response.
	Secret string `json:"secret,omitempty"`
}

func toDTO(w domain.Webhook, secret string) webhookDTO {
	evs := w.Events
	if evs == nil {
		evs = []string{}
	}
	return webhookDTO{ID: w.ID, OrgID: w.OrgID, ProjectID: w.ProjectID, Name: w.Name, URL: w.URL, Events: evs,
		FilterQuery: w.FilterQuery, Enabled: w.Enabled, CreatedBy: w.CreatedBy, CreatedAt: w.CreatedAt,
		UpdatedAt: w.UpdatedAt, Secret: secret}
}

type deliveryDTO struct {
	ID              uuid.UUID       `json:"id"`
	WebhookID       uuid.UUID       `json:"webhook_id"`
	EventID         *int64          `json:"event_id"`
	Event           string          `json:"event"`
	Payload         json.RawMessage `json:"payload"`
	Status          string          `json:"status"`
	Attempts        int             `json:"attempts"`
	NextAttemptAt   *time.Time      `json:"next_attempt_at"`
	ResponseStatus  *int            `json:"response_status"`
	ResponseExcerpt string          `json:"response_excerpt"`
	DurationMS      int             `json:"duration_ms"`
	CreatedAt       time.Time       `json:"created_at"`
}

func toDeliveryDTO(d domain.Delivery) deliveryDTO {
	payload := json.RawMessage(d.Payload)
	if len(payload) == 0 {
		payload = json.RawMessage("{}")
	}
	return deliveryDTO{ID: d.ID, WebhookID: d.WebhookID, EventID: d.EventID, Event: d.EventType, Payload: payload,
		Status: string(d.Status), Attempts: d.Attempts, NextAttemptAt: d.NextAttemptAt, ResponseStatus: d.ResponseStatus,
		ResponseExcerpt: d.ResponseExcerpt, DurationMS: d.DurationMS, CreatedAt: d.CreatedAt}
}

type createRequest struct {
	Name        string     `json:"name"`
	URL         string     `json:"url"`
	Events      []string   `json:"events"`
	FilterQuery *string    `json:"filter_query"`
	ProjectID   *uuid.UUID `json:"project_id"`
	Enabled     *bool      `json:"enabled"`
}

type updateRequest struct {
	Name        *string  `json:"name"`
	URL         *string  `json:"url"`
	Events      []string `json:"events"`
	FilterQuery *string  `json:"filter_query"`
	Enabled     *bool    `json:"enabled"`
}

// scope extracts the caller and the organization of the path, or writes the error.
func scope(w http.ResponseWriter, r *http.Request) (actor, org uuid.UUID, ok bool) {
	actor, has := appmw.UserIDFromContext(r.Context())
	if !has {
		httpresponse.Error(w, apperror.New(apperror.KindUnauthorized, "No autenticado"))
		return uuid.Nil, uuid.Nil, false
	}
	org, err := uuid.Parse(chi.URLParam(r, "org"))
	if err != nil {
		httpresponse.Error(w, apperror.WithCode(apperror.KindNotFound, "organization.not_found", "Organizacion no encontrada"))
		return uuid.Nil, uuid.Nil, false
	}
	return actor, org, true
}

func idParam(w http.ResponseWriter, r *http.Request, name string, notFound error) (uuid.UUID, bool) {
	id, err := uuid.Parse(chi.URLParam(r, name))
	if err != nil {
		httpresponse.Error(w, notFound)
		return uuid.Nil, false
	}
	return id, true
}

// List returns the webhooks of the organization; ?project_id= narrows to one project.
func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	actor, org, ok := scope(w, r)
	if !ok {
		return
	}
	var project *uuid.UUID
	if raw := r.URL.Query().Get("project_id"); raw != "" {
		id, err := uuid.Parse(raw)
		if err != nil {
			httpresponse.Error(w, domain.ErrProjectNotInOrg)
			return
		}
		project = &id
	}
	list, err := h.service.List(r.Context(), actor, org, project)
	if err != nil {
		httpresponse.Error(w, err)
		return
	}
	out := make([]webhookDTO, len(list))
	for i, wh := range list {
		out[i] = toDTO(wh, "")
	}
	httpresponse.JSON(w, http.StatusOK, out)
}

// Create stores a webhook and returns its secret once.
func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	actor, org, ok := scope(w, r)
	if !ok {
		return
	}
	var req createRequest
	if err := decodeStrict(r, &req); err != nil {
		httpresponse.Error(w, err)
		return
	}
	wh, secret, err := h.service.Create(r.Context(), actor, org, port.CreateInput{
		Name: req.Name, URL: req.URL, Events: req.Events, FilterQuery: req.FilterQuery, ProjectID: req.ProjectID, Enabled: req.Enabled,
	})
	if err != nil {
		httpresponse.Error(w, err)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	httpresponse.JSON(w, http.StatusCreated, toDTO(wh, secret))
}

// Get returns one webhook.
func (h *Handler) Get(w http.ResponseWriter, r *http.Request) {
	actor, org, ok := scope(w, r)
	if !ok {
		return
	}
	id, ok := idParam(w, r, "id", domain.ErrNotFound)
	if !ok {
		return
	}
	wh, err := h.service.Get(r.Context(), actor, org, id)
	if err != nil {
		httpresponse.Error(w, err)
		return
	}
	httpresponse.JSON(w, http.StatusOK, toDTO(wh, ""))
}

// Update changes a webhook. An empty filter_query removes the filter.
func (h *Handler) Update(w http.ResponseWriter, r *http.Request) {
	actor, org, ok := scope(w, r)
	if !ok {
		return
	}
	id, ok := idParam(w, r, "id", domain.ErrNotFound)
	if !ok {
		return
	}
	var req updateRequest
	if err := decodeStrict(r, &req); err != nil {
		httpresponse.Error(w, err)
		return
	}
	wh, err := h.service.Update(r.Context(), actor, org, id, port.UpdateInput{
		Name: req.Name, URL: req.URL, Events: req.Events, FilterQuery: req.FilterQuery, Enabled: req.Enabled,
	})
	if err != nil {
		httpresponse.Error(w, err)
		return
	}
	httpresponse.JSON(w, http.StatusOK, toDTO(wh, ""))
}

// Delete removes a webhook.
func (h *Handler) Delete(w http.ResponseWriter, r *http.Request) {
	actor, org, ok := scope(w, r)
	if !ok {
		return
	}
	id, ok := idParam(w, r, "id", domain.ErrNotFound)
	if !ok {
		return
	}
	if err := h.service.Delete(r.Context(), actor, org, id); err != nil {
		httpresponse.Error(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// Test sends a ping and returns the recorded delivery.
func (h *Handler) Test(w http.ResponseWriter, r *http.Request) {
	actor, org, ok := scope(w, r)
	if !ok {
		return
	}
	id, ok := idParam(w, r, "id", domain.ErrNotFound)
	if !ok {
		return
	}
	d, err := h.service.Test(r.Context(), actor, org, id)
	if err != nil {
		httpresponse.Error(w, err)
		return
	}
	httpresponse.JSON(w, http.StatusOK, toDeliveryDTO(d))
}

// Rotate replaces the secret and returns the new one once.
func (h *Handler) Rotate(w http.ResponseWriter, r *http.Request) {
	actor, org, ok := scope(w, r)
	if !ok {
		return
	}
	id, ok := idParam(w, r, "id", domain.ErrNotFound)
	if !ok {
		return
	}
	secret, err := h.service.RotateSecret(r.Context(), actor, org, id)
	if err != nil {
		httpresponse.Error(w, err)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	httpresponse.JSON(w, http.StatusOK, map[string]string{"secret": secret})
}

// Deliveries lists the latest deliveries of a webhook (?limit=, default 50, max 100).
func (h *Handler) Deliveries(w http.ResponseWriter, r *http.Request) {
	actor, org, ok := scope(w, r)
	if !ok {
		return
	}
	id, ok := idParam(w, r, "id", domain.ErrNotFound)
	if !ok {
		return
	}
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	list, err := h.service.Deliveries(r.Context(), actor, org, id, limit)
	if err != nil {
		httpresponse.Error(w, err)
		return
	}
	out := make([]deliveryDTO, len(list))
	for i, d := range list {
		out[i] = toDeliveryDTO(d)
	}
	httpresponse.JSON(w, http.StatusOK, out)
}

// Redeliver queues a copy of a delivery.
func (h *Handler) Redeliver(w http.ResponseWriter, r *http.Request) {
	actor, org, ok := scope(w, r)
	if !ok {
		return
	}
	id, ok := idParam(w, r, "id", domain.ErrNotFound)
	if !ok {
		return
	}
	did, ok := idParam(w, r, "delivery", domain.ErrDeliveryNotFound)
	if !ok {
		return
	}
	d, err := h.service.Redeliver(r.Context(), actor, org, id, did)
	if err != nil {
		httpresponse.Error(w, err)
		return
	}
	httpresponse.JSON(w, http.StatusAccepted, toDeliveryDTO(d))
}

func decodeStrict(r *http.Request, into any) error {
	raw, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
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
