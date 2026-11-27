package http

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/jhonsferg/corelog/internal/platform/apperror"
	"github.com/jhonsferg/corelog/internal/platform/httpresponse"
	"github.com/jhonsferg/corelog/internal/workflow/application/port"
	"github.com/jhonsferg/corelog/internal/workflow/domain"
)

// detailResponse is one problem with its JSON path, as the workflow editor shows it.
type detailResponse struct {
	Path    string `json:"path"`
	Code    string `json:"code"`
	Message string `json:"message"`
}

// mappingResponse names a removed status that still holds issues.
type mappingResponse struct {
	StatusID   string `json:"status_id"`
	Name       string `json:"name"`
	IssueCount int    `json:"issue_count"`
}

// failureBody is the error body of a refused definition. details carries one entry per path and
// mapping_required the statuses that need a target.
type failureBody struct {
	Message         string            `json:"message"`
	Code            string            `json:"code"`
	Details         []detailResponse  `json:"details,omitempty"`
	MappingRequired []mappingResponse `json:"mapping_required,omitempty"`
}

type failureEnvelope struct {
	Error failureBody `json:"error"`
}

// writeError answers an error. Refused definitions get the detailed body; everything else goes through the
// shared error mapping.
func writeError(w http.ResponseWriter, err error) {
	var failure *port.ValidationFailure
	if !errors.As(err, &failure) {
		httpresponse.Error(w, err)
		return
	}
	body := failureBody{Message: failure.Err.Message, Code: failure.Err.Code}
	for _, issue := range failure.Issues {
		body.Details = append(body.Details, detailResponse{Path: issue.Path, Code: issue.Code, Message: issue.Message})
	}
	for _, m := range failure.Mapping {
		body.MappingRequired = append(body.MappingRequired, mappingResponse{StatusID: m.StatusID.String(), Name: m.Name, IssueCount: m.IssueCount})
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusUnprocessableEntity)
	_ = json.NewEncoder(w).Encode(failureEnvelope{Error: body})
}

// invalidRequest builds the refusal of a request whose shape is wrong before any use case runs.
func invalidRequest(issues []domain.Issue) error {
	return &port.ValidationFailure{
		Err:    apperror.WithCode(apperror.KindValidation, "workflow.invalid", "el flujo de trabajo no es válido"),
		Issues: issues,
	}
}

// malformedBody is the answer to a body that is not JSON or has unknown fields.
func malformedBody() error {
	return apperror.WithCode(apperror.KindValidation, "validation.failed", "el cuerpo de la petición no es válido")
}
