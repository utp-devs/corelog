package http

import (
	"encoding/json"
	"net/http"

	"github.com/google/uuid"

	"github.com/jhonsferg/corelog/internal/issue/application/port"
	"github.com/jhonsferg/corelog/internal/issue/domain"
	"github.com/jhonsferg/corelog/internal/platform/apperror"
	"github.com/jhonsferg/corelog/internal/platform/httpresponse"
)

// Convert changes the type of an issue. The body takes issue_type_id and an optional parent_id (null removes the
// parent). The answer is the issue with the changes it received, like PATCH.
func (h *Handler) Convert(w http.ResponseWriter, r *http.Request) {
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
	fields, ok := decodeFields(w, r, "issue_type_id", "parent_id")
	if !ok {
		return
	}
	in, err := parseConvertBody(fields)
	if err != nil {
		httpresponse.Error(w, err)
		return
	}
	result, err := h.ops.ConvertIssue(r.Context(), id, actorID, globalRole, in)
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

// parseConvertBody reads the convert request. issue_type_id is required; parent_id may be null.
func parseConvertBody(fields map[string]json.RawMessage) (port.ConvertInput, error) {
	var in port.ConvertInput
	raw, ok := fields["issue_type_id"]
	if !ok {
		return in, domain.ErrInvalidIssueType
	}
	if err := json.Unmarshal(raw, &in.IssueTypeID); err != nil || in.IssueTypeID == uuid.Nil {
		return in, domain.ErrInvalidIssueType
	}
	if raw, ok := fields["parent_id"]; ok {
		parent, err := parseNullableUUID(raw)
		if err != nil {
			return in, apperror.WithCode(apperror.KindValidation, "issue.parent_invalid", "parent_id debe ser un UUID o null")
		}
		in.ParentID = parent
	}
	return in, nil
}
