package http

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"time"

	"github.com/google/uuid"

	"github.com/jhonsferg/corelog/internal/issue/application/port"
	"github.com/jhonsferg/corelog/internal/issue/domain"
	"github.com/jhonsferg/corelog/internal/platform/apperror"
	"github.com/jhonsferg/corelog/internal/platform/httpresponse"
	appmw "github.com/jhonsferg/corelog/internal/platform/middleware"
	"github.com/jhonsferg/corelog/internal/platform/validation"
)

// maxIssueBody bounds the body of issue writes: a 100000-character description in UTF-8 plus the other fields.
const maxIssueBody = 1 << 20

// patchableFields are the JSON fields PATCH /issues/{id} accepts. Anything else is rejected.
var patchableFields = map[string]bool{
	"summary": true, "description": true, "issue_type": true, "issue_type_id": true, "priority": true,
	"priority_id": true, "parent_id": true, "assignee_id": true, "story_points": true,
	"original_estimate_seconds": true, "remaining_estimate_seconds": true, "start_date": true,
	"due_date": true, "flagged": true, "custom": true, "expected_updated_at": true,
}

// providedSystemFields maps the fields of a create body to the layout keys of the required-field check.
func providedSystemFields(sent map[string]json.RawMessage) map[string]bool {
	has := func(keys ...string) bool {
		for _, key := range keys {
			if raw, ok := sent[key]; ok && string(bytes.TrimSpace(raw)) != "null" {
				return true
			}
		}
		return false
	}
	return map[string]bool{
		"summary":      has("summary"),
		"description":  has("description"),
		"priority":     has("priority", "priority_id"),
		"assignee":     has("assignee_id"),
		"parent":       has("parent_id"),
		"due_date":     has("due_date"),
		"story_points": has("story_points"),
	}
}

func decodeField[T any](sent map[string]json.RawMessage, key string, problems *[]apperror.FieldError) port.Field[T] {
	raw, ok := sent[key]
	if !ok {
		return port.Field[T]{}
	}
	if string(bytes.TrimSpace(raw)) == "null" {
		return port.Null[T]()
	}
	var value T
	if err := json.Unmarshal(raw, &value); err != nil {
		*problems = append(*problems, apperror.FieldError{Field: key, Message: "el valor no tiene el tipo esperado"})
		return port.Field[T]{}
	}
	return port.Some(value)
}

func decodeDate(sent map[string]json.RawMessage, key string, problems *[]apperror.FieldError) port.Field[time.Time] {
	field := decodeField[apiDate](sent, key, problems)
	if !field.Set {
		return port.Field[time.Time]{}
	}
	return port.Field[time.Time]{Set: true, Value: field.Value.value()}
}

// parsePatch turns a PATCH body into a PatchInput. Unknown fields and mistyped values are validation errors.
func parsePatch(raw []byte) (port.PatchInput, error) {
	var sent map[string]json.RawMessage
	if err := json.Unmarshal(raw, &sent); err != nil || sent == nil {
		return port.PatchInput{}, apperror.New(apperror.KindValidation, "invalid request body")
	}
	var problems []apperror.FieldError
	for key := range sent {
		if !patchableFields[key] {
			problems = append(problems, apperror.FieldError{Field: key, Message: "campo desconocido"})
		}
	}
	in := port.PatchInput{
		Summary:                  decodeField[string](sent, "summary", &problems),
		Description:              decodeField[string](sent, "description", &problems),
		IssueTypeID:              decodeField[uuid.UUID](sent, "issue_type_id", &problems),
		IssueType:                decodeField[domain.IssueType](sent, "issue_type", &problems),
		PriorityID:               decodeField[uuid.UUID](sent, "priority_id", &problems),
		Priority:                 decodeField[domain.Priority](sent, "priority", &problems),
		ParentID:                 decodeField[uuid.UUID](sent, "parent_id", &problems),
		AssigneeID:               decodeField[uuid.UUID](sent, "assignee_id", &problems),
		StoryPoints:              decodeField[float64](sent, "story_points", &problems),
		OriginalEstimateSeconds:  decodeField[int64](sent, "original_estimate_seconds", &problems),
		RemainingEstimateSeconds: decodeField[int64](sent, "remaining_estimate_seconds", &problems),
		StartDate:                decodeDate(sent, "start_date", &problems),
		DueDate:                  decodeDate(sent, "due_date", &problems),
		Flagged:                  decodeField[bool](sent, "flagged", &problems),
	}
	if expected := decodeField[time.Time](sent, "expected_updated_at", &problems); expected.Set && expected.Value != nil {
		in.ExpectedUpdatedAt = expected.Value
	}
	if raw, ok := sent["custom"]; ok && string(bytes.TrimSpace(raw)) != "null" {
		if err := json.Unmarshal(raw, &in.Custom); err != nil {
			problems = append(problems, apperror.FieldError{Field: "custom", Message: "debe ser un objeto"})
		}
	}
	if len(problems) > 0 {
		return port.PatchInput{}, apperror.WithCode(apperror.KindValidation, validation.CodeFailed, "invalid request body").WithDetails(problems...)
	}
	return in, nil
}

type changeResponse struct {
	Field string  `json:"field"`
	From  *string `json:"from"`
	To    *string `json:"to"`
}

type patchResponse struct {
	issueResponse
	BatchID string           `json:"batch_id,omitempty"`
	Changes []changeResponse `json:"changes"`
}

// Patch updates only the fields present in the body. An explicit null clears nullable fields.
func (h *Handler) Patch(w http.ResponseWriter, r *http.Request) {
	actorID, ok := appmw.UserIDFromContext(r.Context())
	if !ok {
		httpresponse.Error(w, apperror.New(apperror.KindUnauthorized, "not authenticated"))
		return
	}
	globalRole, _ := appmw.RoleFromContext(r.Context())
	id, err := h.issueRef(r, "id")
	if err != nil {
		httpresponse.Error(w, err)
		return
	}
	raw, err := io.ReadAll(http.MaxBytesReader(w, r.Body, maxIssueBody))
	if err != nil {
		httpresponse.Error(w, apperror.New(apperror.KindValidation, "invalid request body"))
		return
	}
	input, err := parsePatch(raw)
	if err != nil {
		httpresponse.Error(w, err)
		return
	}
	input.HideForbidden = true
	result, err := h.service.Patch(r.Context(), id, actorID, globalRole, input)
	if err != nil {
		httpresponse.Error(w, err)
		return
	}
	resp, err := h.buildIssueResponse(r, result.Issue)
	if err != nil {
		httpresponse.Error(w, err)
		return
	}
	out := patchResponse{issueResponse: resp, Changes: make([]changeResponse, 0, len(result.Changes))}
	if len(result.Changes) > 0 {
		out.BatchID = result.BatchID.String()
	}
	for _, change := range result.Changes {
		out.Changes = append(out.Changes, changeResponse{Field: change.Field, From: change.From, To: change.To})
	}
	httpresponse.JSON(w, http.StatusOK, out)
}

// Delete soft-deletes the issue.
func (h *Handler) Delete(w http.ResponseWriter, r *http.Request) {
	actorID, ok := appmw.UserIDFromContext(r.Context())
	if !ok {
		httpresponse.Error(w, apperror.New(apperror.KindUnauthorized, "not authenticated"))
		return
	}
	globalRole, _ := appmw.RoleFromContext(r.Context())
	id, err := h.issueRef(r, "id")
	if err != nil {
		httpresponse.Error(w, err)
		return
	}
	if err := h.service.Delete(r.Context(), id, actorID, globalRole); err != nil {
		httpresponse.Error(w, err)
		return
	}
	httpresponse.JSON(w, http.StatusNoContent, nil)
}

// Restore brings back a soft-deleted issue. The path takes the issue id: a deleted issue has no resolvable key.
func (h *Handler) Restore(w http.ResponseWriter, r *http.Request) {
	actorID, ok := appmw.UserIDFromContext(r.Context())
	if !ok {
		httpresponse.Error(w, apperror.New(apperror.KindUnauthorized, "not authenticated"))
		return
	}
	globalRole, _ := appmw.RoleFromContext(r.Context())
	id, err := h.issueRef(r, "id")
	if err != nil {
		httpresponse.Error(w, err)
		return
	}
	issue, err := h.service.Restore(r.Context(), id, actorID, globalRole)
	if err != nil {
		httpresponse.Error(w, err)
		return
	}
	h.writeIssue(w, r, http.StatusOK, issue)
}

type transitionResponse struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	ToStatus string `json:"to_status"`
}

type countsResponse struct {
	Comments    int `json:"comments"`
	Attachments int `json:"attachments"`
	Children    int `json:"children"`
	Links       int `json:"links"`
	Watchers    int `json:"watchers"`
	Votes       int `json:"votes"`
}

type breadcrumbProject struct {
	ID   string `json:"id"`
	Key  string `json:"key"`
	Name string `json:"name"`
}

type breadcrumbParent struct {
	ID      string `json:"id"`
	Key     string `json:"key"`
	Summary string `json:"summary"`
	Type    string `json:"type"`
}

type breadcrumbResponse struct {
	Project breadcrumbProject `json:"project"`
	Parent  *breadcrumbParent `json:"parent"`
}

// issueViewResponse is the issue plus the context a detail screen needs.
type issueViewResponse struct {
	issueResponse
	Permissions          []string             `json:"permissions"`
	AvailableTransitions []transitionResponse `json:"available_transitions"`
	Counts               countsResponse       `json:"counts"`
	Breadcrumb           breadcrumbResponse   `json:"breadcrumb"`
}

func toIssueViewResponse(resp issueResponse, view *port.IssueView) issueViewResponse {
	out := issueViewResponse{
		issueResponse:        resp,
		Permissions:          view.Permissions,
		AvailableTransitions: make([]transitionResponse, 0, len(view.Transitions)),
		Counts: countsResponse{
			Comments: view.Counts.Comments, Attachments: view.Counts.Attachments, Children: view.Counts.Children,
			Links: view.Counts.Links, Watchers: view.Counts.Watchers, Votes: view.Counts.Votes,
		},
		Breadcrumb: breadcrumbResponse{Project: breadcrumbProject{
			ID: view.Breadcrumb.ProjectID.String(), Key: view.Breadcrumb.ProjectKey, Name: view.Breadcrumb.ProjectName,
		}},
	}
	if out.Permissions == nil {
		out.Permissions = []string{}
	}
	for _, t := range view.Transitions {
		out.AvailableTransitions = append(out.AvailableTransitions, transitionResponse{ID: t.ID, Name: t.Name, ToStatus: string(t.To)})
	}
	if p := view.Breadcrumb.Parent; p != nil {
		out.Breadcrumb.Parent = &breadcrumbParent{ID: p.ID.String(), Key: p.Key, Summary: p.Summary, Type: p.Type}
	}
	return out
}
