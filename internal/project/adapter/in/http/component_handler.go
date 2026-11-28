package http

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/jhonsferg/corelog/internal/platform/apperror"
	"github.com/jhonsferg/corelog/internal/platform/httpresponse"
	"github.com/jhonsferg/corelog/internal/platform/validation"
	"github.com/jhonsferg/corelog/internal/project/domain"
)

// componentRequest is the body of a new component. lead_id and default_assignee_policy are optional.
type componentRequest struct {
	Name                  string  `json:"name" validate:"required,min=1,max=80"`
	Description           string  `json:"description" validate:"max=5000"`
	LeadID                *string `json:"lead_id"`
	DefaultAssigneePolicy *string `json:"default_assignee_policy"`
}

// componentUpdateRequest is a partial update. An absent lead_id keeps the lead; null clears it.
type componentUpdateRequest struct {
	Name                  *string         `json:"name"`
	Description           *string         `json:"description"`
	LeadID                json.RawMessage `json:"lead_id"`
	DefaultAssigneePolicy *string         `json:"default_assignee_policy"`
}

type componentResponse struct {
	ID                    string    `json:"id"`
	ProjectID             string    `json:"project_id"`
	Name                  string    `json:"name"`
	Description           string    `json:"description"`
	LeadID                *string   `json:"lead_id"`
	LeadName              *string   `json:"lead_name"`
	DefaultAssigneePolicy string    `json:"default_assignee_policy"`
	IssuesCount           int       `json:"issues_count"`
	CreatedAt             time.Time `json:"created_at"`
	UpdatedAt             time.Time `json:"updated_at"`
}

func toComponentResponse(component domain.Component) componentResponse {
	response := componentResponse{
		ID: component.ID.String(), ProjectID: component.ProjectID.String(), Name: component.Name,
		Description: component.Description, DefaultAssigneePolicy: component.DefaultAssigneePolicy,
		IssuesCount: component.IssueCount, CreatedAt: component.CreatedAt, UpdatedAt: component.UpdatedAt,
	}
	if component.LeadID != nil {
		lead := component.LeadID.String()
		response.LeadID = &lead
		name := component.LeadName
		response.LeadName = &name
	}
	return response
}

func toComponentResponses(components []domain.Component) []componentResponse {
	out := make([]componentResponse, 0, len(components))
	for _, component := range components {
		out = append(out, toComponentResponse(component))
	}
	return out
}

func (h *Handler) ListComponents(w http.ResponseWriter, r *http.Request) {
	actorID, globalRole, ok := actorFromRequest(r)
	if !ok {
		httpresponse.Error(w, apperror.New(apperror.KindUnauthorized, "no autorizado"))
		return
	}
	projectID, ok := h.projectIDFromPath(w, r, actorID, globalRole)
	if !ok {
		return
	}
	components, err := h.service.ListComponents(r.Context(), actorID, globalRole, projectID)
	if err != nil {
		httpresponse.Error(w, err)
		return
	}
	httpresponse.JSON(w, http.StatusOK, toComponentResponses(components))
}

func (h *Handler) CreateComponent(w http.ResponseWriter, r *http.Request) {
	var req componentRequest
	if err := validation.DecodeStrict(r.Body, &req); err != nil {
		httpresponse.Error(w, apperror.New(apperror.KindValidation, "invalid request body"))
		return
	}
	if err := validation.Struct(req); err != nil {
		httpresponse.Error(w, validation.Failed(req, err))
		return
	}
	actorID, globalRole, ok := actorFromRequest(r)
	if !ok {
		httpresponse.Error(w, apperror.New(apperror.KindUnauthorized, "no autorizado"))
		return
	}
	projectID, ok := h.projectIDFromPath(w, r, actorID, globalRole)
	if !ok {
		return
	}
	lead, err := optionalUUIDString(req.LeadID)
	if err != nil {
		httpresponse.Error(w, err)
		return
	}
	in := domain.ComponentInput{Name: req.Name, Description: req.Description, LeadID: lead}
	if req.DefaultAssigneePolicy != nil {
		in.Policy = *req.DefaultAssigneePolicy
	}
	component, err := h.service.CreateComponent(r.Context(), actorID, globalRole, projectID, in)
	if err != nil {
		httpresponse.Error(w, err)
		return
	}
	httpresponse.JSON(w, http.StatusCreated, toComponentResponse(*component))
}

func (h *Handler) UpdateComponent(w http.ResponseWriter, r *http.Request) {
	componentID, err := uuid.Parse(chi.URLParam(r, "component_id"))
	if err != nil {
		httpresponse.Error(w, apperror.New(apperror.KindValidation, "invalid component id"))
		return
	}
	raw, err := readBody(r)
	if err != nil {
		httpresponse.Error(w, err)
		return
	}
	var req componentUpdateRequest
	if err := validation.DecodeStrict(bytes.NewReader(raw), &req); err != nil {
		httpresponse.Error(w, apperror.New(apperror.KindValidation, "invalid request body"))
		return
	}
	patch := domain.ComponentPatch{Name: req.Name, Description: req.Description, Policy: req.DefaultAssigneePolicy}
	if len(bytes.TrimSpace(req.LeadID)) > 0 {
		patch.LeadIDSet = true
		if patch.LeadID, err = optionalUUIDRaw(req.LeadID); err != nil {
			httpresponse.Error(w, err)
			return
		}
	}
	actorID, globalRole, ok := actorFromRequest(r)
	if !ok {
		httpresponse.Error(w, apperror.New(apperror.KindUnauthorized, "no autorizado"))
		return
	}
	projectID, ok := h.projectIDFromPath(w, r, actorID, globalRole)
	if !ok {
		return
	}
	component, err := h.service.UpdateComponent(r.Context(), actorID, globalRole, projectID, componentID, patch)
	if err != nil {
		httpresponse.Error(w, err)
		return
	}
	httpresponse.JSON(w, http.StatusOK, toComponentResponse(*component))
}

func (h *Handler) DeleteComponent(w http.ResponseWriter, r *http.Request) {
	componentID, err := uuid.Parse(chi.URLParam(r, "component_id"))
	if err != nil {
		httpresponse.Error(w, apperror.New(apperror.KindValidation, "invalid component id"))
		return
	}
	var moveTo *uuid.UUID
	if raw := r.URL.Query().Get("move_to"); raw != "" {
		parsed, err := uuid.Parse(raw)
		if err != nil {
			httpresponse.Error(w, domain.ErrComponentMoveTarget)
			return
		}
		moveTo = &parsed
	}
	actorID, globalRole, ok := actorFromRequest(r)
	if !ok {
		httpresponse.Error(w, apperror.New(apperror.KindUnauthorized, "no autorizado"))
		return
	}
	projectID, ok := h.projectIDFromPath(w, r, actorID, globalRole)
	if !ok {
		return
	}
	if err := h.service.DeleteComponent(r.Context(), actorID, globalRole, projectID, componentID, moveTo); err != nil {
		httpresponse.Error(w, err)
		return
	}
	httpresponse.JSON(w, http.StatusNoContent, nil)
}

// optionalUUIDString parses an optional id sent as a string; empty or absent means none.
func optionalUUIDString(raw *string) (*uuid.UUID, error) {
	if raw == nil || *raw == "" {
		return nil, nil
	}
	id, err := uuid.Parse(*raw)
	if err != nil {
		return nil, apperror.WithCode(apperror.KindValidation, "validation.failed", "El identificador del responsable no es valido")
	}
	return &id, nil
}

// optionalUUIDRaw parses a raw JSON value that is either null (none) or a UUID string.
func optionalUUIDRaw(raw json.RawMessage) (*uuid.UUID, error) {
	if string(bytes.TrimSpace(raw)) == "null" {
		return nil, nil
	}
	var text string
	if err := json.Unmarshal(raw, &text); err != nil {
		return nil, apperror.WithCode(apperror.KindValidation, "validation.failed", "El identificador del responsable no es valido")
	}
	return optionalUUIDString(&text)
}

// readBody reads a small JSON body.
func readBody(r *http.Request) ([]byte, error) {
	raw, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil {
		return nil, apperror.New(apperror.KindValidation, "invalid request body")
	}
	return raw, nil
}
