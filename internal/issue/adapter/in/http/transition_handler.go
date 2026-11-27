package http

import (
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

// WithTransitions fixes the service behind the transition endpoints.
func (h *Handler) WithTransitions(service port.TransitionService) *Handler {
	h.transitions = service
	return h
}

func nonNilStrings(in []string) []string {
	if in == nil {
		return []string{}
	}
	return in
}

// ListTransitions returns the transitions the caller may run on the issue, with their required fields.
func (h *Handler) ListTransitions(w http.ResponseWriter, r *http.Request) {
	actorID, ok := appmw.UserIDFromContext(r.Context())
	if !ok {
		httpresponse.Error(w, apperror.New(apperror.KindUnauthorized, "not authenticated"))
		return
	}
	id, err := h.issueRef(r, "id")
	if err != nil {
		httpresponse.Error(w, err)
		return
	}
	if h.transitions == nil {
		httpresponse.Error(w, domain.ErrTransitionUnsupported)
		return
	}
	globalRole, _ := appmw.RoleFromContext(r.Context())
	options, err := h.transitions.ListTransitions(r.Context(), id, actorID, globalRole)
	if err != nil {
		httpresponse.Error(w, err)
		return
	}
	out := make([]transitionResponse, 0, len(options))
	for _, option := range options {
		out = append(out, transitionResponse{
			ID: option.ID, Name: option.Name, ToStatus: string(domain.LegacyStatusOfCategory(option.ToCategory)),
			To:             transitionStatusRef{ID: option.ToStatusID.String(), Name: option.ToStatusName, Category: option.ToCategory},
			RequiredFields: nonNilStrings(option.RequiredFields), RequiresComment: option.RequiresComment,
			RequiresResolution: option.RequiresResolution, RequiresAssignee: option.RequiresAssignee, NeedsScreen: option.NeedsScreen(),
		})
	}
	httpresponse.JSON(w, http.StatusOK, out)
}

// runTransitionRequest is the body of POST /issues/{id}/transitions.
type runTransitionRequest struct {
	TransitionID     string            `json:"transition_id"`
	Fields           map[string]string `json:"fields"`
	Comment          string            `json:"comment"`
	ResolutionID     *uuid.UUID        `json:"resolution_id"`
	AssigneeID       *uuid.UUID        `json:"assignee_id"`
	ExpectedStatusID *uuid.UUID        `json:"expected_status_id"`
}

// maxTransitionBody bounds the request body of a transition.
const maxTransitionBody = 64 << 10

// transitionResultResponse is the issue after a transition with what the transition did.
type transitionResultResponse struct {
	issueResponse
	Transition appliedTransitionResponse `json:"transition"`
}

type appliedTransitionResponse struct {
	ID           string              `json:"id"`
	Name         string              `json:"name"`
	FromStatusID string              `json:"from_status_id"`
	To           transitionStatusRef `json:"to"`
}

// RunTransition runs a workflow transition on the issue in one transaction.
func (h *Handler) RunTransition(w http.ResponseWriter, r *http.Request) {
	actorID, ok := appmw.UserIDFromContext(r.Context())
	if !ok {
		httpresponse.Error(w, apperror.New(apperror.KindUnauthorized, "not authenticated"))
		return
	}
	id, err := h.issueRef(r, "id")
	if err != nil {
		httpresponse.Error(w, err)
		return
	}
	if h.transitions == nil {
		httpresponse.Error(w, domain.ErrTransitionUnsupported)
		return
	}
	var body runTransitionRequest
	decoder := json.NewDecoder(io.LimitReader(r.Body, maxTransitionBody))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&body); err != nil {
		httpresponse.Error(w, apperror.WithCode(apperror.KindValidation, "validation.failed", "el cuerpo de la petición no es válido"))
		return
	}
	if body.TransitionID == "" || len(body.TransitionID) > 64 {
		httpresponse.Error(w, apperror.WithCode(apperror.KindValidation, "validation.failed", "la petición no es válida").
			WithDetails(apperror.FieldError{Field: "transition_id", Message: "es obligatorio"}))
		return
	}
	req := port.TransitionRequest{
		IssueID: id, ActorID: actorID, TransitionID: body.TransitionID,
		Input: port.TransitionInput{Fields: body.Fields, Comment: body.Comment},
	}
	req.GlobalRole, _ = appmw.RoleFromContext(r.Context())
	if body.ResolutionID != nil {
		req.Input.ResolutionID = *body.ResolutionID
	}
	if body.AssigneeID != nil {
		req.Input.AssigneeID = *body.AssigneeID
	}
	if body.ExpectedStatusID != nil {
		req.ExpectedStatusID = *body.ExpectedStatusID
	}
	result, err := h.transitions.TransitionIssue(r.Context(), req)
	if err != nil {
		httpresponse.Error(w, err)
		return
	}
	resp, err := h.buildIssueResponse(r, result.Issue)
	if err != nil {
		httpresponse.Error(w, err)
		return
	}
	httpresponse.JSON(w, http.StatusOK, transitionResultResponse{
		issueResponse: resp,
		Transition: appliedTransitionResponse{
			ID: result.Plan.TransitionID, Name: result.Plan.TransitionName, FromStatusID: result.Plan.FromStatusID.String(),
			To: transitionStatusRef{ID: result.Plan.ToStatusID.String(), Name: result.Plan.ToStatusName, Category: result.Plan.ToCategory},
		},
	})
}
