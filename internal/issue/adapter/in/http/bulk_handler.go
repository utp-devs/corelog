package http

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"

	"github.com/google/uuid"

	"github.com/jhonsferg/corelog/internal/issue/application/port"
	"github.com/jhonsferg/corelog/internal/issue/domain"
	"github.com/jhonsferg/corelog/internal/platform/apperror"
	"github.com/jhonsferg/corelog/internal/platform/httpresponse"
	appmw "github.com/jhonsferg/corelog/internal/platform/middleware"
)

// WithOperations fixes the service of the bulk, clone, move, convert and export endpoints.
func (h *Handler) WithOperations(ops port.IssueOperations) *Handler {
	h.ops = ops
	return h
}

// operationsUnavailable answers when the operations service was not wired.
func (h *Handler) operationsUnavailable(w http.ResponseWriter) {
	httpresponse.Error(w, apperror.New(apperror.KindInternal, "issue operations are unavailable"))
}

// bulkRequest is the body of POST /issues/bulk. Params depends on the action and is decoded per action.
type bulkRequest struct {
	IssueIDs []string        `json:"issue_ids"`
	Action   string          `json:"action"`
	Params   json.RawMessage `json:"params"`
}

type bulkSucceeded struct {
	Issue issueResponse `json:"issue"`
}

type bulkFailed struct {
	Issue   string `json:"issue"`
	Code    string `json:"code"`
	Message string `json:"message"`
}

type bulkResponse struct {
	Succeeded []bulkSucceeded `json:"succeeded"`
	Failed    []bulkFailed    `json:"failed"`
}

// BulkChange applies one action to up to 200 issues. The answer is 200 with the issues that changed and the ones
// that failed, each with its own code; only a malformed request is rejected.
func (h *Handler) BulkChange(w http.ResponseWriter, r *http.Request) {
	actorID, globalRole, ok := h.actorOf(w, r)
	if !ok {
		return
	}
	if h.ops == nil {
		h.operationsUnavailable(w)
		return
	}
	var req bulkRequest
	if !decodeBody(w, r, &req) {
		return
	}
	action, err := domain.ParseBulkAction(req.Action)
	if err != nil {
		httpresponse.Error(w, err)
		return
	}
	in, err := parseBulkParams(action, req.Params)
	if err != nil {
		httpresponse.Error(w, err)
		return
	}
	in.Refs = req.IssueIDs
	in.Action = action

	result, err := h.ops.BulkChange(r.Context(), actorID, globalRole, in)
	if err != nil {
		httpresponse.Error(w, err)
		return
	}
	out := bulkResponse{Succeeded: make([]bulkSucceeded, 0, len(result.Succeeded)), Failed: make([]bulkFailed, 0, len(result.Failed))}
	for _, issue := range result.Succeeded {
		resp, err := h.buildIssueResponse(r, issue)
		if err != nil {
			httpresponse.Error(w, err)
			return
		}
		out.Succeeded = append(out.Succeeded, bulkSucceeded{Issue: resp})
	}
	for _, failure := range result.Failed {
		out.Failed = append(out.Failed, bulkFailed{Issue: failure.Ref, Code: failure.Code, Message: failure.Message})
	}
	httpresponse.JSON(w, http.StatusOK, out)
}

// decodeBody reads a JSON body into v, rejecting unknown fields. It writes the error and returns false on failure.
func decodeBody(w http.ResponseWriter, r *http.Request, v any) bool {
	raw, err := io.ReadAll(http.MaxBytesReader(w, r.Body, maxIssueBody))
	if err != nil {
		httpresponse.Error(w, apperror.New(apperror.KindValidation, "invalid request body"))
		return false
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		httpresponse.Error(w, apperror.New(apperror.KindValidation, "invalid request body"))
		return false
	}
	return true
}

// decodeFields reads a JSON object body and rejects members outside allowed. It writes the error and returns false.
func decodeFields(w http.ResponseWriter, r *http.Request, allowed ...string) (map[string]json.RawMessage, bool) {
	raw, err := io.ReadAll(http.MaxBytesReader(w, r.Body, maxIssueBody))
	if err != nil {
		httpresponse.Error(w, apperror.New(apperror.KindValidation, "invalid request body"))
		return nil, false
	}
	known := make(map[string]bool, len(allowed))
	for _, name := range allowed {
		known[name] = true
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil || fields == nil {
		httpresponse.Error(w, apperror.New(apperror.KindValidation, "invalid request body"))
		return nil, false
	}
	for name := range fields {
		if !known[name] {
			httpresponse.Error(w, apperror.New(apperror.KindValidation, "invalid request body"))
			return nil, false
		}
	}
	return fields, true
}

// actorOf returns the authenticated user and the global role, or writes 401.
func (h *Handler) actorOf(w http.ResponseWriter, r *http.Request) (uuid.UUID, string, bool) {
	actorID, ok := appmw.UserIDFromContext(r.Context())
	if !ok {
		httpresponse.Error(w, apperror.New(apperror.KindUnauthorized, "not authenticated"))
		return uuid.Nil, "", false
	}
	globalRole, _ := appmw.RoleFromContext(r.Context())
	return actorID, globalRole, true
}

// paramsObject decodes the params of an action into its fields. Unknown fields are a validation error.
func paramsObject(raw json.RawMessage, allowed ...string) (map[string]json.RawMessage, error) {
	if len(bytes.TrimSpace(raw)) == 0 || string(bytes.TrimSpace(raw)) == "null" {
		return map[string]json.RawMessage{}, nil
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil {
		return nil, domain.ErrBulkParams
	}
	known := make(map[string]bool, len(allowed))
	for _, name := range allowed {
		known[name] = true
	}
	for name := range fields {
		if !known[name] {
			return nil, domain.ErrBulkParams
		}
	}
	return fields, nil
}

// parseBulkParams turns the params of an action into its input. Each action accepts its own fields only.
func parseBulkParams(action domain.BulkAction, raw json.RawMessage) (port.BulkInput, error) {
	var in port.BulkInput
	switch action {
	case domain.BulkAssign:
		fields, err := paramsObject(raw, "assignee_id")
		if err != nil {
			return in, err
		}
		value, ok := fields["assignee_id"]
		if !ok {
			return in, domain.ErrBulkParams
		}
		in.Assignee, err = parseNullableUUID(value)
		return in, err
	case domain.BulkSetPriority:
		fields, err := paramsObject(raw, "priority_id")
		if err != nil {
			return in, err
		}
		var id uuid.UUID
		if err := decodeUUIDParam(fields, "priority_id", &id); err != nil {
			return in, err
		}
		in.PriorityID = id
		return in, nil
	case domain.BulkSetLabels:
		fields, err := paramsObject(raw, "add", "remove")
		if err != nil {
			return in, err
		}
		if in.LabelsAdd, err = decodeUUIDList(fields, "add"); err != nil {
			return in, err
		}
		if in.LabelsRemove, err = decodeUUIDList(fields, "remove"); err != nil {
			return in, err
		}
		return in, domain.ValidateLabelChange(in.LabelsAdd, in.LabelsRemove)
	case domain.BulkSetField:
		fields, err := paramsObject(raw, "fields")
		if err != nil {
			return in, err
		}
		body, ok := fields["fields"]
		if !ok {
			return in, domain.ErrBulkParams
		}
		if empty, err := isEmptyObject(body); err != nil || empty {
			return in, domain.ErrBulkParams
		}
		patch, err := parsePatch(body)
		if err != nil {
			return in, err
		}
		if patch.ExpectedUpdatedAt != nil {
			return in, domain.ErrBulkParams
		}
		in.Fields = patch
		return in, nil
	case domain.BulkTransition:
		fields, err := paramsObject(raw, "status")
		if err != nil {
			return in, err
		}
		var status string
		if err := decodeStringParam(fields, "status", &status); err != nil {
			return in, err
		}
		in.Status = domain.Status(status)
		return in, nil
	case domain.BulkMove:
		fields, err := paramsObject(raw, "project_id", "issue_type_id", "field_mapping")
		if err != nil {
			return in, err
		}
		move, err := parseMoveBody(fields)
		if err != nil {
			return in, err
		}
		in.Move = move
		return in, nil
	case domain.BulkDelete:
		_, err := paramsObject(raw)
		return in, err
	default:
		return in, domain.ErrBulkActionUnknown
	}
}

// isEmptyObject tells whether a raw JSON value is an object without members.
func isEmptyObject(raw json.RawMessage) (bool, error) {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil {
		return false, domain.ErrBulkParams
	}
	return len(fields) == 0, nil
}

// parseNullableUUID reads an optional id where null clears the value.
func parseNullableUUID(raw json.RawMessage) (port.Field[uuid.UUID], error) {
	if string(bytes.TrimSpace(raw)) == "null" {
		return port.Null[uuid.UUID](), nil
	}
	var id uuid.UUID
	if err := json.Unmarshal(raw, &id); err != nil {
		return port.Field[uuid.UUID]{}, domain.ErrBulkParams
	}
	return port.Some(id), nil
}

// decodeUUIDParam reads one required UUID member.
func decodeUUIDParam(fields map[string]json.RawMessage, name string, out *uuid.UUID) error {
	raw, ok := fields[name]
	if !ok {
		return domain.ErrBulkParams
	}
	if err := json.Unmarshal(raw, out); err != nil || *out == uuid.Nil {
		return domain.ErrBulkParams
	}
	return nil
}

// decodeStringParam reads one required string member.
func decodeStringParam(fields map[string]json.RawMessage, name string, out *string) error {
	raw, ok := fields[name]
	if !ok {
		return domain.ErrBulkParams
	}
	if err := json.Unmarshal(raw, out); err != nil || *out == "" {
		return domain.ErrBulkParams
	}
	return nil
}

// decodeUUIDList reads an optional list of UUIDs. A missing member is an empty list.
func decodeUUIDList(fields map[string]json.RawMessage, name string) ([]uuid.UUID, error) {
	raw, ok := fields[name]
	if !ok {
		return nil, nil
	}
	var ids []uuid.UUID
	if err := json.Unmarshal(raw, &ids); err != nil {
		return nil, domain.ErrBulkParams
	}
	return ids, nil
}
