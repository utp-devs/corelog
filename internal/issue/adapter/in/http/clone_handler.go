package http

import (
	"encoding/json"
	"net/http"

	"github.com/google/uuid"

	"github.com/jhonsferg/corelog/internal/issue/application/port"
	"github.com/jhonsferg/corelog/internal/platform/apperror"
	"github.com/jhonsferg/corelog/internal/platform/httpresponse"
)

// Clone copies an issue. The body takes an optional summary, the target project and the options to copy:
// attachments, links, subtasks and watchers. The answer is the new issue with status 201.
func (h *Handler) Clone(w http.ResponseWriter, r *http.Request) {
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
	fields, ok := decodeFields(w, r, "summary", "project_id", "include")
	if !ok {
		return
	}
	in, err := parseCloneBody(fields)
	if err != nil {
		httpresponse.Error(w, err)
		return
	}
	clone, err := h.ops.CloneIssue(r.Context(), id, actorID, globalRole, in)
	if err != nil {
		httpresponse.Error(w, err)
		return
	}
	h.writeIssue(w, r, http.StatusCreated, clone)
}

// parseCloneBody reads the clone request. Include must be a list of known options.
func parseCloneBody(fields map[string]json.RawMessage) (port.CloneInput, error) {
	var in port.CloneInput
	if raw, ok := fields["summary"]; ok && string(raw) != "null" {
		if err := json.Unmarshal(raw, &in.Summary); err != nil {
			return in, apperror.New(apperror.KindValidation, "invalid request body")
		}
	}
	if raw, ok := fields["project_id"]; ok && string(raw) != "null" {
		if err := json.Unmarshal(raw, &in.ProjectID); err != nil || in.ProjectID == uuid.Nil {
			return in, apperror.WithCode(apperror.KindValidation, "issue.project_invalid", "project_id debe ser un UUID valido")
		}
	}
	if raw, ok := fields["include"]; ok && string(raw) != "null" {
		if err := json.Unmarshal(raw, &in.Include); err != nil {
			return in, apperror.New(apperror.KindValidation, "invalid request body")
		}
	}
	return in, nil
}
