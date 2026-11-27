package http

import (
	"bytes"
	"encoding/json"
	"net/http"

	"github.com/google/uuid"

	"github.com/jhonsferg/corelog/internal/issue/application/port"
	"github.com/jhonsferg/corelog/internal/issue/domain"
	"github.com/jhonsferg/corelog/internal/platform/apperror"
	"github.com/jhonsferg/corelog/internal/platform/httpresponse"
)

// fieldMappingBody is the field_mapping of a move: source value id or key to target value.
type fieldMappingBody struct {
	IssueType map[string]string `json:"issue_type"`
	Priority  map[string]string `json:"priority"`
	Status    map[string]string `json:"status"`
}

// parseMoveBody turns the move fields of a request (direct body or bulk params) into a MoveInput.
func parseMoveBody(fields map[string]json.RawMessage) (port.MoveInput, error) {
	var in port.MoveInput
	raw, ok := fields["project_id"]
	if !ok {
		return in, domain.ErrBulkParams
	}
	if err := json.Unmarshal(raw, &in.ProjectID); err != nil || in.ProjectID == uuid.Nil {
		return in, apperror.WithCode(apperror.KindValidation, "issue.project_invalid", "project_id debe ser un UUID valido")
	}
	if raw, ok := fields["issue_type_id"]; ok && string(raw) != "null" {
		var id uuid.UUID
		if err := json.Unmarshal(raw, &id); err != nil || id == uuid.Nil {
			return in, apperror.WithCode(apperror.KindValidation, "issue.type_invalid", "issue_type_id debe ser un UUID valido")
		}
		in.IssueTypeID = &id
	}
	if raw, ok := fields["field_mapping"]; ok && string(raw) != "null" {
		mapping, err := parseFieldMapping(raw)
		if err != nil {
			return in, err
		}
		in.Mapping = mapping
	}
	return in, nil
}

// parseFieldMapping reads the mapping of a move. Source and target values must be ids, and statuses must exist.
func parseFieldMapping(raw json.RawMessage) (domain.MoveMapping, error) {
	var body fieldMappingBody
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&body); err != nil {
		return domain.MoveMapping{}, domain.ErrMoveMappingField
	}
	mapping := domain.MoveMapping{
		IssueTypes: map[uuid.UUID]uuid.UUID{},
		Priorities: map[uuid.UUID]uuid.UUID{},
		Statuses:   map[domain.Status]domain.Status{},
	}
	var err error
	if mapping.IssueTypes, err = uuidMap(body.IssueType); err != nil {
		return mapping, err
	}
	if mapping.Priorities, err = uuidMap(body.Priority); err != nil {
		return mapping, err
	}
	for from, to := range body.Status {
		if !domain.Status(from).IsValid() || !domain.Status(to).IsValid() {
			return mapping, domain.ErrMoveMappingInvalid
		}
		mapping.Statuses[domain.Status(from)] = domain.Status(to)
	}
	return mapping, nil
}

// uuidMap converts a map of id strings into ids. Any other text is an invalid mapping.
func uuidMap(values map[string]string) (map[uuid.UUID]uuid.UUID, error) {
	out := make(map[uuid.UUID]uuid.UUID, len(values))
	for from, to := range values {
		source, err := uuid.Parse(from)
		if err != nil {
			return nil, domain.ErrMoveMappingInvalid
		}
		target, err := uuid.Parse(to)
		if err != nil {
			return nil, domain.ErrMoveMappingInvalid
		}
		out[source] = target
	}
	return out, nil
}

type movedIssueResponse struct {
	ID      string `json:"id"`
	FromKey string `json:"from_key"`
	ToKey   string `json:"to_key"`
}

type moveResponse struct {
	issueResponse
	Warnings []string             `json:"warnings"`
	Moved    []movedIssueResponse `json:"moved"`
}

// Move moves the issue and its children to another project. The answer is the moved issue with the warnings
// (custom fields and labels that were dropped) and the keys that changed.
func (h *Handler) Move(w http.ResponseWriter, r *http.Request) {
	actorID, globalRole, ok := h.actorOf(w, r)
	if !ok {
		return
	}
	if h.ops == nil {
		h.operationsUnavailable(w)
		return
	}
	id, err := h.issueRef(r, "id")
	if err != nil {
		httpresponse.Error(w, err)
		return
	}
	fields, ok := decodeFields(w, r, "project_id", "issue_type_id", "field_mapping")
	if !ok {
		return
	}
	in, err := parseMoveBody(fields)
	if err != nil {
		httpresponse.Error(w, err)
		return
	}
	result, err := h.ops.MoveIssue(r.Context(), id, actorID, globalRole, in)
	if err != nil {
		httpresponse.Error(w, err)
		return
	}
	resp, err := h.buildIssueResponse(r, result.Issue)
	if err != nil {
		httpresponse.Error(w, err)
		return
	}
	out := moveResponse{issueResponse: resp, Warnings: result.Warnings, Moved: make([]movedIssueResponse, 0, len(result.Moved))}
	if out.Warnings == nil {
		out.Warnings = []string{}
	}
	for _, moved := range result.Moved {
		out.Moved = append(out.Moved, movedIssueResponse{ID: moved.ID.String(), FromKey: moved.FromKey, ToKey: moved.ToKey})
	}
	httpresponse.JSON(w, http.StatusOK, out)
}
