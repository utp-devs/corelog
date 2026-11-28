// Package http exposes the automation rule endpoints.
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

	"github.com/jhonsferg/corelog/internal/automation/application/port"
	"github.com/jhonsferg/corelog/internal/automation/domain"
	"github.com/jhonsferg/corelog/internal/platform/apperror"
	"github.com/jhonsferg/corelog/internal/platform/httpresponse"
	appmw "github.com/jhonsferg/corelog/internal/platform/middleware"
)

// Handler serves the automation routes.
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
	r.Route("/projects/{project}/automation-rules", func(r chi.Router) {
		r.Use(appmw.Authenticate(h.jwtSecret))
		r.Get("/", h.List)
		r.Post("/", h.Create)
	})
	r.Route("/automation-rules", func(r chi.Router) {
		r.Use(appmw.Authenticate(h.jwtSecret))
		r.Post("/validate", h.Validate)
		r.Get("/{id}", h.Get)
		r.Patch("/{id}", h.Patch)
		r.Delete("/{id}", h.Delete)
		r.Post("/{id}/enable", h.Enable)
		r.Post("/{id}/disable", h.Disable)
		r.Get("/{id}/runs", h.Runs)
	})
}

type ruleDTO struct {
	ID         uuid.UUID          `json:"id"`
	OrgID      uuid.UUID          `json:"organization_id"`
	ProjectID  *uuid.UUID         `json:"project_id"`
	Name       string             `json:"name"`
	Enabled    bool               `json:"enabled"`
	Trigger    domain.Trigger     `json:"trigger"`
	Conditions []domain.Condition `json:"conditions"`
	Actions    []domain.Action    `json:"actions"`
	CreatedBy  *uuid.UUID         `json:"created_by"`
	Version    int                `json:"version"`
	RunCount   int64              `json:"run_count"`
	LastRunAt  *time.Time         `json:"last_run_at"`
	CreatedAt  time.Time          `json:"created_at"`
	UpdatedAt  time.Time          `json:"updated_at"`
}

func toDTO(r domain.Rule) ruleDTO {
	conds := r.Conditions
	if conds == nil {
		conds = []domain.Condition{}
	}
	return ruleDTO{ID: r.ID, OrgID: r.OrgID, ProjectID: r.ProjectID, Name: r.Name, Enabled: r.Enabled, Trigger: r.Trigger,
		Conditions: conds, Actions: r.Actions, CreatedBy: r.CreatedBy, Version: r.Version, RunCount: r.RunCount,
		LastRunAt: r.LastRunAt, CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt}
}

type runDTO struct {
	ID        uuid.UUID  `json:"id"`
	RuleID    uuid.UUID  `json:"rule_id"`
	EventID   *int64     `json:"event_id"`
	IssueID   *uuid.UUID `json:"issue_id"`
	Status    string     `json:"status"`
	Detail    string     `json:"detail"`
	CreatedAt time.Time  `json:"created_at"`
}

type createRequest struct {
	Name       string             `json:"name"`
	Enabled    *bool              `json:"enabled"`
	Trigger    domain.Trigger     `json:"trigger"`
	Conditions []domain.Condition `json:"conditions"`
	Actions    []domain.Action    `json:"actions"`
}

type patchRequest struct {
	Name       *string             `json:"name"`
	Enabled    *bool               `json:"enabled"`
	Trigger    *domain.Trigger     `json:"trigger"`
	Conditions *[]domain.Condition `json:"conditions"`
	Actions    *[]domain.Action    `json:"actions"`
	Version    *int                `json:"version"`
}

type validateRequest struct {
	ProjectID  uuid.UUID          `json:"project_id"`
	IssueID    *uuid.UUID         `json:"issue_id"`
	Name       string             `json:"name"`
	Trigger    domain.Trigger     `json:"trigger"`
	Conditions []domain.Condition `json:"conditions"`
	Actions    []domain.Action    `json:"actions"`
}

func caller(w http.ResponseWriter, r *http.Request) (uuid.UUID, string, bool) {
	id, ok := appmw.UserIDFromContext(r.Context())
	if !ok {
		httpresponse.Error(w, apperror.New(apperror.KindUnauthorized, "No autenticado"))
		return uuid.Nil, "", false
	}
	role, _ := appmw.RoleFromContext(r.Context())
	return id, role, true
}

func pathID(w http.ResponseWriter, r *http.Request, name string, notFound error) (uuid.UUID, bool) {
	id, err := uuid.Parse(chi.URLParam(r, name))
	if err != nil {
		httpresponse.Error(w, notFound)
		return uuid.Nil, false
	}
	return id, true
}

var errProjectNotFound = apperror.WithCode(apperror.KindNotFound, "project.not_found", "Proyecto no encontrado")

// List returns the rules of a project.
func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	actor, role, ok := caller(w, r)
	if !ok {
		return
	}
	project, ok := pathID(w, r, "project", errProjectNotFound)
	if !ok {
		return
	}
	list, err := h.service.List(r.Context(), actor, role, project)
	if err != nil {
		httpresponse.Error(w, err)
		return
	}
	out := make([]ruleDTO, len(list))
	for i, rule := range list {
		out[i] = toDTO(rule)
	}
	httpresponse.JSON(w, http.StatusOK, out)
}

// Create stores a rule.
func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	actor, role, ok := caller(w, r)
	if !ok {
		return
	}
	project, ok := pathID(w, r, "project", errProjectNotFound)
	if !ok {
		return
	}
	var req createRequest
	if err := decodeStrict(r, &req); err != nil {
		httpresponse.Error(w, err)
		return
	}
	rule, err := h.service.Create(r.Context(), actor, role, project, port.RuleInput{
		Name: req.Name, Enabled: req.Enabled, Trigger: req.Trigger, Conditions: req.Conditions, Actions: req.Actions,
	})
	if err != nil {
		httpresponse.Error(w, err)
		return
	}
	httpresponse.JSON(w, http.StatusCreated, toDTO(rule))
}

// Get returns a rule.
func (h *Handler) Get(w http.ResponseWriter, r *http.Request) {
	actor, role, ok := caller(w, r)
	if !ok {
		return
	}
	id, ok := pathID(w, r, "id", domain.ErrNotFound)
	if !ok {
		return
	}
	rule, err := h.service.Get(r.Context(), actor, role, id)
	if err != nil {
		httpresponse.Error(w, err)
		return
	}
	httpresponse.JSON(w, http.StatusOK, toDTO(rule))
}

// Patch changes a rule; "version" makes stale edits fail with 409.
func (h *Handler) Patch(w http.ResponseWriter, r *http.Request) {
	actor, role, ok := caller(w, r)
	if !ok {
		return
	}
	id, ok := pathID(w, r, "id", domain.ErrNotFound)
	if !ok {
		return
	}
	var req patchRequest
	if err := decodeStrict(r, &req); err != nil {
		httpresponse.Error(w, err)
		return
	}
	rule, err := h.service.Patch(r.Context(), actor, role, id, port.PatchInput{
		Name: req.Name, Enabled: req.Enabled, Trigger: req.Trigger, Conditions: req.Conditions, Actions: req.Actions, Version: req.Version,
	})
	if err != nil {
		httpresponse.Error(w, err)
		return
	}
	httpresponse.JSON(w, http.StatusOK, toDTO(rule))
}

// Delete removes a rule.
func (h *Handler) Delete(w http.ResponseWriter, r *http.Request) {
	actor, role, ok := caller(w, r)
	if !ok {
		return
	}
	id, ok := pathID(w, r, "id", domain.ErrNotFound)
	if !ok {
		return
	}
	if err := h.service.Delete(r.Context(), actor, role, id); err != nil {
		httpresponse.Error(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// Enable turns a rule on.
func (h *Handler) Enable(w http.ResponseWriter, r *http.Request) { h.toggle(w, r, true) }

// Disable turns a rule off.
func (h *Handler) Disable(w http.ResponseWriter, r *http.Request) { h.toggle(w, r, false) }

func (h *Handler) toggle(w http.ResponseWriter, r *http.Request, enabled bool) {
	actor, role, ok := caller(w, r)
	if !ok {
		return
	}
	id, ok := pathID(w, r, "id", domain.ErrNotFound)
	if !ok {
		return
	}
	rule, err := h.service.SetEnabled(r.Context(), actor, role, id, enabled)
	if err != nil {
		httpresponse.Error(w, err)
		return
	}
	httpresponse.JSON(w, http.StatusOK, toDTO(rule))
}

// Runs lists the latest runs (?status=success|skipped|failed, ?limit=).
func (h *Handler) Runs(w http.ResponseWriter, r *http.Request) {
	actor, role, ok := caller(w, r)
	if !ok {
		return
	}
	id, ok := pathID(w, r, "id", domain.ErrNotFound)
	if !ok {
		return
	}
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	runs, err := h.service.Runs(r.Context(), actor, role, id, r.URL.Query().Get("status"), limit)
	if err != nil {
		httpresponse.Error(w, err)
		return
	}
	out := make([]runDTO, len(runs))
	for i, run := range runs {
		out[i] = runDTO{ID: run.ID, RuleID: run.RuleID, EventID: run.EventID, IssueID: run.IssueID, Status: run.Status,
			Detail: run.Detail, CreatedAt: run.CreatedAt}
	}
	httpresponse.JSON(w, http.StatusOK, out)
}

// Validate is the dry run of a definition.
func (h *Handler) Validate(w http.ResponseWriter, r *http.Request) {
	actor, role, ok := caller(w, r)
	if !ok {
		return
	}
	var req validateRequest
	if err := decodeStrict(r, &req); err != nil {
		httpresponse.Error(w, err)
		return
	}
	res, err := h.service.Validate(r.Context(), actor, role, req.ProjectID, port.RuleInput{
		Name: req.Name, Trigger: req.Trigger, Conditions: req.Conditions, Actions: req.Actions,
	}, req.IssueID)
	if err != nil {
		httpresponse.Error(w, err)
		return
	}
	httpresponse.JSON(w, http.StatusOK, map[string]any{"valid": res.Valid, "errors": res.Errors, "planned_actions": res.Planned})
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
