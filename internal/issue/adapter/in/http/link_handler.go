package http

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/jhonsferg/corelog/internal/issue/application/port"
	"github.com/jhonsferg/corelog/internal/issue/domain"
	"github.com/jhonsferg/corelog/internal/platform/apperror"
	"github.com/jhonsferg/corelog/internal/platform/httpresponse"
	appmw "github.com/jhonsferg/corelog/internal/platform/middleware"
	"github.com/jhonsferg/corelog/internal/platform/validation"
)

type createIssueLinkRequest struct {
	Target string `json:"target" validate:"required,max=255"`
	Kind   string `json:"kind" validate:"required,max=30"`
}

// linkIssueResponse is the issue at the far end of a link.
type linkIssueResponse struct {
	ID       string `json:"id"`
	Key      string `json:"key"`
	Summary  string `json:"summary"`
	Status   string `json:"status"`
	Type     string `json:"type"`
	Priority string `json:"priority"`
}

// linkResponse is one link seen from the viewed issue. Direction is "outward" when the viewed issue is the source.
type linkResponse struct {
	ID        string            `json:"id"`
	Kind      string            `json:"kind"`
	Direction string            `json:"direction"`
	Label     string            `json:"label"`
	Issue     linkIssueResponse `json:"issue"`
}

func toLinkResponse(link port.LinkedIssue) linkResponse {
	direction := "inward"
	if link.Outward {
		direction = "outward"
	}
	return linkResponse{
		ID: link.LinkID.String(), Kind: string(link.Kind), Direction: direction,
		Label: link.Kind.Label(link.Outward),
		Issue: linkIssueResponse{
			ID: link.Other.ID.String(), Key: link.Other.Key, Summary: link.Other.Summary,
			Status: string(link.Other.Status), Type: link.Other.TypeName, Priority: link.Other.PriorityName,
		},
	}
}

// ListIssueLinks lists the links of an issue, grouped by kind.
func (h *Handler) ListIssueLinks(w http.ResponseWriter, r *http.Request) {
	actorID, ok := appmw.UserIDFromContext(r.Context())
	if !ok {
		httpresponse.Error(w, apperror.New(apperror.KindUnauthorized, "no autenticado"))
		return
	}
	globalRole, _ := appmw.RoleFromContext(r.Context())
	issueID, err := h.issueRef(r, "id")
	if err != nil {
		httpresponse.Error(w, err)
		return
	}
	links, err := h.service.ListIssueLinks(r.Context(), issueID, actorID, globalRole)
	if err != nil {
		httpresponse.Error(w, err)
		return
	}
	out := make([]linkResponse, 0, len(links))
	for _, link := range links {
		out = append(out, toLinkResponse(link))
	}
	httpresponse.JSON(w, http.StatusOK, out)
}

// CreateIssueLink links the issue in the path to the target given as a UUID or a key.
func (h *Handler) CreateIssueLink(w http.ResponseWriter, r *http.Request) {
	actorID, ok := appmw.UserIDFromContext(r.Context())
	if !ok {
		httpresponse.Error(w, apperror.New(apperror.KindUnauthorized, "no autenticado"))
		return
	}
	sourceID, err := h.issueRef(r, "id")
	if err != nil {
		httpresponse.Error(w, err)
		return
	}
	var req createIssueLinkRequest
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxIssueBody))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&req); err != nil {
		httpresponse.Error(w, apperror.New(apperror.KindValidation, "cuerpo de la peticion no valido"))
		return
	}
	if err := validation.Struct(req); err != nil {
		httpresponse.Error(w, validation.Failed(req, err))
		return
	}
	kind, err := domain.ParseIssueLinkKind(req.Kind)
	if err != nil {
		httpresponse.Error(w, err)
		return
	}
	globalRole, _ := appmw.RoleFromContext(r.Context())
	link, err := h.service.CreateIssueLink(r.Context(), sourceID, actorID, globalRole, req.Target, kind)
	if err != nil {
		httpresponse.Error(w, err)
		return
	}
	httpresponse.JSON(w, http.StatusCreated, toLinkResponse(link))
}

// DeleteIssueLink removes a link of the issue in the path.
func (h *Handler) DeleteIssueLink(w http.ResponseWriter, r *http.Request) {
	actorID, ok := appmw.UserIDFromContext(r.Context())
	if !ok {
		httpresponse.Error(w, apperror.New(apperror.KindUnauthorized, "no autenticado"))
		return
	}
	issueID, err := h.issueRef(r, "id")
	if err != nil {
		httpresponse.Error(w, err)
		return
	}
	linkID, err := uuid.Parse(chi.URLParam(r, "link_id"))
	if err != nil {
		httpresponse.Error(w, domain.ErrIssueLinkNotFound)
		return
	}
	globalRole, _ := appmw.RoleFromContext(r.Context())
	if err := h.service.DeleteIssueLink(r.Context(), issueID, linkID, actorID, globalRole); err != nil {
		httpresponse.Error(w, err)
		return
	}
	httpresponse.JSON(w, http.StatusNoContent, nil)
}
