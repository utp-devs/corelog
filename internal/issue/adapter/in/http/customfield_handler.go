package http

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/jhonsferg/corelog/internal/issue/application/port"
	"github.com/jhonsferg/corelog/internal/issue/domain"
	"github.com/jhonsferg/corelog/internal/platform/apperror"
	"github.com/jhonsferg/corelog/internal/platform/httpresponse"
	appmw "github.com/jhonsferg/corelog/internal/platform/middleware"
)

// CustomFieldHandler serves custom field definitions, their contexts and the project field layouts.
type CustomFieldHandler struct {
	service   port.CustomFieldService
	jwtSecret string
}

func NewCustomFieldHandler(service port.CustomFieldService, jwtSecret string) *CustomFieldHandler {
	return &CustomFieldHandler{service: service, jwtSecret: jwtSecret}
}

// RegisterCustomFieldRoutes mounts the definition routes of an organization and the layout routes of a project.
func (h *CustomFieldHandler) RegisterCustomFieldRoutes(r chi.Router) {
	auth := appmw.Authenticate(h.jwtSecret)
	base := "/organizations/{org}/custom-fields"
	r.With(auth).Get(base, h.list)
	r.With(auth).Post(base, h.create)
	r.With(auth).Patch(base+"/{id}", h.update)
	r.With(auth).Delete(base+"/{id}", h.delete)
	r.With(auth).Put(base+"/{id}/contexts", h.replaceContexts)
	r.With(auth).Get("/projects/{id}/fields", h.layout)
	r.With(auth).Put("/projects/{id}/fields", h.replaceLayout)
}

type customFieldOptionRequest struct {
	ID       string `json:"id" validate:"omitempty,max=64"`
	Label    string `json:"label" validate:"required,max=60"`
	Color    string `json:"color" validate:"max=40"`
	Archived bool   `json:"archived"`
}

type customFieldSettingsRequest struct {
	Min *float64 `json:"min"`
	Max *float64 `json:"max"`
}

type customFieldCreateRequest struct {
	Name        string                      `json:"name" validate:"required,max=60"`
	Description string                      `json:"description" validate:"max=500"`
	Type        string                      `json:"type" validate:"required,oneof=text paragraph number date datetime select multiselect checkbox user url labels"`
	Options     []customFieldOptionRequest  `json:"options" validate:"max=100,dive"`
	Settings    *customFieldSettingsRequest `json:"settings"`
}

type customFieldPatchRequest struct {
	Name        *string                     `json:"name" validate:"omitempty,max=60"`
	Description *string                     `json:"description" validate:"omitempty,max=500"`
	Options     *[]customFieldOptionRequest `json:"options" validate:"omitempty,max=100,dive"`
	Settings    *customFieldSettingsRequest `json:"settings"`
}

type customFieldContextRequest struct {
	ProjectID    string          `json:"project_id" validate:"omitempty,uuid"`
	IssueTypeID  string          `json:"issue_type_id" validate:"omitempty,uuid"`
	Required     bool            `json:"required"`
	DefaultValue json.RawMessage `json:"default_value"`
}

type customFieldContextsRequest struct {
	Contexts []customFieldContextRequest `json:"contexts" validate:"required,dive"`
}

type layoutItemRequest struct {
	Kind     string `json:"kind" validate:"required,oneof=system custom"`
	Key      string `json:"key" validate:"omitempty,max=40"`
	FieldID  string `json:"field_id" validate:"omitempty,uuid"`
	Required bool   `json:"required"`
}

type layoutRequest struct {
	Items []layoutItemRequest `json:"items" validate:"required,dive"`
}

type customFieldOptionResponse struct {
	ID       string `json:"id"`
	Label    string `json:"label"`
	Color    string `json:"color,omitempty"`
	Position int    `json:"position"`
	Archived bool   `json:"archived"`
}

type customFieldSettingsResponse struct {
	Min *float64 `json:"min,omitempty"`
	Max *float64 `json:"max,omitempty"`
}

type customFieldContextResponse struct {
	ProjectID    *string `json:"project_id"`
	IssueTypeID  *string `json:"issue_type_id"`
	Required     bool    `json:"required"`
	DefaultValue any     `json:"default_value"`
}

type customFieldResponse struct {
	ID          string                       `json:"id"`
	Name        string                       `json:"name"`
	Description string                       `json:"description"`
	Type        string                       `json:"type"`
	Options     []customFieldOptionResponse  `json:"options"`
	Settings    *customFieldSettingsResponse `json:"settings,omitempty"`
	Position    int                          `json:"position"`
	Contexts    []customFieldContextResponse `json:"contexts"`
	CreatedAt   time.Time                    `json:"created_at"`
	UpdatedAt   time.Time                    `json:"updated_at"`
}

type layoutEntryResponse struct {
	Kind     string                      `json:"kind"`
	Key      string                      `json:"key,omitempty"`
	FieldID  string                      `json:"field_id,omitempty"`
	Name     string                      `json:"name,omitempty"`
	Type     string                      `json:"type,omitempty"`
	Options  []customFieldOptionResponse `json:"options,omitempty"`
	Required bool                        `json:"required"`
}

type layoutResponse struct {
	IssueTypeID string                `json:"issue_type_id"`
	Configured  bool                  `json:"configured"`
	Items       []layoutEntryResponse `json:"items"`
}

func toCustomFieldResponse(field *domain.CustomField) customFieldResponse {
	resp := customFieldResponse{
		ID:          field.ID.String(),
		Name:        field.Name,
		Description: field.Description,
		Type:        string(field.Type),
		Options:     toOptionResponses(field.Options),
		Position:    field.Position,
		Contexts:    make([]customFieldContextResponse, 0, len(field.Contexts)),
		CreatedAt:   field.CreatedAt,
		UpdatedAt:   field.UpdatedAt,
	}
	if field.NumberMin != nil || field.NumberMax != nil {
		resp.Settings = &customFieldSettingsResponse{Min: field.NumberMin, Max: field.NumberMax}
	}
	for _, ctx := range field.Contexts {
		resp.Contexts = append(resp.Contexts, customFieldContextResponse{
			ProjectID:    optionalUUIDString(ctx.ProjectID),
			IssueTypeID:  optionalUUIDString(ctx.IssueTypeID),
			Required:     ctx.Required,
			DefaultValue: ctx.DefaultValue,
		})
	}
	return resp
}

func toOptionResponses(options []domain.FieldOption) []customFieldOptionResponse {
	out := make([]customFieldOptionResponse, 0, len(options))
	for _, option := range options {
		out = append(out, customFieldOptionResponse{ID: option.ID, Label: option.Label, Color: option.Color, Position: option.Position, Archived: option.Archived})
	}
	return out
}

func optionalUUIDString(id *uuid.UUID) *string {
	if id == nil {
		return nil
	}
	value := id.String()
	return &value
}

func toOptionInputs(in []customFieldOptionRequest) []domain.OptionInput {
	out := make([]domain.OptionInput, 0, len(in))
	for _, option := range in {
		out = append(out, domain.OptionInput{ID: option.ID, Label: option.Label, Color: option.Color, Archived: option.Archived})
	}
	return out
}

func (h *CustomFieldHandler) list(w http.ResponseWriter, r *http.Request) {
	actorID, orgID, ok := actorAndOrg(w, r)
	if !ok {
		return
	}
	fields, err := h.service.ListCustomFields(r.Context(), actorID, orgID)
	if err != nil {
		httpresponse.Error(w, err)
		return
	}
	out := make([]customFieldResponse, 0, len(fields))
	for _, field := range fields {
		out = append(out, toCustomFieldResponse(field))
	}
	httpresponse.JSON(w, http.StatusOK, out)
}

func (h *CustomFieldHandler) create(w http.ResponseWriter, r *http.Request) {
	actorID, orgID, ok := actorAndOrg(w, r)
	if !ok {
		return
	}
	var req customFieldCreateRequest
	if !decodeStrict(w, r, &req) {
		return
	}
	in := port.CustomFieldInput{
		Name:        req.Name,
		Description: req.Description,
		Type:        domain.FieldType(req.Type),
		Options:     toOptionInputs(req.Options),
	}
	if req.Settings != nil {
		in.NumberMin, in.NumberMax = req.Settings.Min, req.Settings.Max
	}
	field, err := h.service.CreateCustomField(r.Context(), actorID, orgID, in)
	if err != nil {
		httpresponse.Error(w, err)
		return
	}
	httpresponse.JSON(w, http.StatusCreated, toCustomFieldResponse(field))
}

func (h *CustomFieldHandler) update(w http.ResponseWriter, r *http.Request) {
	actorID, orgID, ok := actorAndOrg(w, r)
	if !ok {
		return
	}
	fieldID, ok := parseURLUUID(w, r, "id", "invalid custom field id")
	if !ok {
		return
	}
	var req customFieldPatchRequest
	if !decodeStrict(w, r, &req) {
		return
	}
	patch := port.CustomFieldPatch{Name: req.Name, Description: req.Description}
	if req.Options != nil {
		options := toOptionInputs(*req.Options)
		patch.Options = &options
	}
	if req.Settings != nil {
		patch.Settings = &port.CustomFieldSettings{Min: req.Settings.Min, Max: req.Settings.Max}
	}
	field, err := h.service.UpdateCustomField(r.Context(), actorID, orgID, fieldID, patch)
	if err != nil {
		httpresponse.Error(w, err)
		return
	}
	httpresponse.JSON(w, http.StatusOK, toCustomFieldResponse(field))
}

func (h *CustomFieldHandler) delete(w http.ResponseWriter, r *http.Request) {
	actorID, orgID, ok := actorAndOrg(w, r)
	if !ok {
		return
	}
	fieldID, ok := parseURLUUID(w, r, "id", "invalid custom field id")
	if !ok {
		return
	}
	if err := h.service.DeleteCustomField(r.Context(), actorID, orgID, fieldID); err != nil {
		httpresponse.Error(w, err)
		return
	}
	httpresponse.JSON(w, http.StatusNoContent, nil)
}

func (h *CustomFieldHandler) replaceContexts(w http.ResponseWriter, r *http.Request) {
	actorID, orgID, ok := actorAndOrg(w, r)
	if !ok {
		return
	}
	fieldID, ok := parseURLUUID(w, r, "id", "invalid custom field id")
	if !ok {
		return
	}
	var req customFieldContextsRequest
	if !decodeStrict(w, r, &req) {
		return
	}
	inputs := make([]port.ContextInput, 0, len(req.Contexts))
	for _, ctx := range req.Contexts {
		in := port.ContextInput{Required: ctx.Required, Default: ctx.DefaultValue}
		if ctx.ProjectID != "" {
			id, err := uuid.Parse(ctx.ProjectID)
			if err != nil {
				httpresponse.Error(w, apperror.New(apperror.KindValidation, "invalid project id"))
				return
			}
			in.ProjectID = &id
		}
		if ctx.IssueTypeID != "" {
			id, err := uuid.Parse(ctx.IssueTypeID)
			if err != nil {
				httpresponse.Error(w, apperror.New(apperror.KindValidation, "invalid issue type id"))
				return
			}
			in.IssueTypeID = &id
		}
		inputs = append(inputs, in)
	}
	field, err := h.service.ReplaceCustomFieldContexts(r.Context(), actorID, orgID, fieldID, inputs)
	if err != nil {
		httpresponse.Error(w, err)
		return
	}
	httpresponse.JSON(w, http.StatusOK, toCustomFieldResponse(field))
}

func (h *CustomFieldHandler) layout(w http.ResponseWriter, r *http.Request) {
	actorID, globalRole, projectID, ok := actorAndProject(w, r)
	if !ok {
		return
	}
	issueTypeID, ok := parseIssueTypeQuery(w, r)
	if !ok {
		return
	}
	view, err := h.service.ProjectLayout(r.Context(), actorID, globalRole, projectID, issueTypeID)
	if err != nil {
		httpresponse.Error(w, err)
		return
	}
	httpresponse.JSON(w, http.StatusOK, toLayoutResponse(view))
}

func (h *CustomFieldHandler) replaceLayout(w http.ResponseWriter, r *http.Request) {
	actorID, globalRole, projectID, ok := actorAndProject(w, r)
	if !ok {
		return
	}
	issueTypeID, ok := parseIssueTypeQuery(w, r)
	if !ok {
		return
	}
	var req layoutRequest
	if !decodeStrict(w, r, &req) {
		return
	}
	items := make([]domain.LayoutItem, 0, len(req.Items))
	for _, entry := range req.Items {
		item := domain.LayoutItem{Kind: domain.LayoutKind(entry.Kind), Key: entry.Key, Required: entry.Required}
		if entry.FieldID != "" {
			id, err := uuid.Parse(entry.FieldID)
			if err != nil {
				httpresponse.Error(w, apperror.New(apperror.KindValidation, "invalid custom field id"))
				return
			}
			item.FieldID = id
		}
		items = append(items, item)
	}
	view, err := h.service.ReplaceProjectLayout(r.Context(), actorID, globalRole, projectID, issueTypeID, items)
	if err != nil {
		httpresponse.Error(w, err)
		return
	}
	httpresponse.JSON(w, http.StatusOK, toLayoutResponse(view))
}

func parseIssueTypeQuery(w http.ResponseWriter, r *http.Request) (uuid.UUID, bool) {
	raw := r.URL.Query().Get("issue_type_id")
	if raw == "" {
		httpresponse.Error(w, apperror.New(apperror.KindValidation, "issue_type_id is required"))
		return uuid.Nil, false
	}
	id, err := uuid.Parse(raw)
	if err != nil {
		httpresponse.Error(w, apperror.New(apperror.KindValidation, "invalid issue_type_id"))
		return uuid.Nil, false
	}
	return id, true
}

func toLayoutResponse(view port.LayoutView) layoutResponse {
	resp := layoutResponse{IssueTypeID: view.IssueTypeID.String(), Configured: view.Configured, Items: make([]layoutEntryResponse, 0, len(view.Items))}
	for _, entry := range view.Items {
		out := layoutEntryResponse{Kind: string(entry.Item.Kind), Required: entry.Item.Required}
		if entry.Item.Kind == domain.LayoutSystem {
			out.Key = entry.Item.Key
		} else {
			out.FieldID = entry.Item.FieldID.String()
			if entry.Field != nil {
				out.Name = entry.Field.Name
				out.Type = string(entry.Field.Type)
				out.Options = toOptionResponses(entry.Field.Options)
			}
		}
		resp.Items = append(resp.Items, out)
	}
	return resp
}

// issueCustomFieldResponse describes a custom field that applies to an issue, so clients can render its value.
type issueCustomFieldResponse struct {
	FieldID  string                      `json:"field_id"`
	Name     string                      `json:"name"`
	Type     string                      `json:"type"`
	Options  []customFieldOptionResponse `json:"options"`
	Required bool                        `json:"required"`
}

func toIssueCustomFields(scope []domain.ScopedField) []issueCustomFieldResponse {
	out := make([]issueCustomFieldResponse, 0, len(scope))
	for _, entry := range scope {
		out = append(out, issueCustomFieldResponse{
			FieldID:  entry.Field.ID.String(),
			Name:     entry.Field.Name,
			Type:     string(entry.Field.Type),
			Options:  toOptionResponses(entry.Field.Options),
			Required: entry.Required,
		})
	}
	return out
}
