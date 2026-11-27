package http

import (
	"net/http"

	"github.com/jhonsferg/corelog/internal/platform/apperror"
	"github.com/jhonsferg/corelog/internal/platform/httpresponse"
	appmw "github.com/jhonsferg/corelog/internal/platform/middleware"
)

// VoteIssue records the caller's vote on the issue in the path. Repeating the call changes nothing.
func (h *Handler) VoteIssue(w http.ResponseWriter, r *http.Request) {
	h.changeVote(w, r, true)
}

// UnvoteIssue removes the caller's vote from the issue in the path. Repeating the call changes nothing.
func (h *Handler) UnvoteIssue(w http.ResponseWriter, r *http.Request) {
	h.changeVote(w, r, false)
}

func (h *Handler) changeVote(w http.ResponseWriter, r *http.Request, cast bool) {
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
	if cast {
		err = h.service.VoteIssue(r.Context(), issueID, actorID, globalRole)
	} else {
		err = h.service.UnvoteIssue(r.Context(), issueID, actorID, globalRole)
	}
	if err != nil {
		httpresponse.Error(w, err)
		return
	}
	httpresponse.JSON(w, http.StatusNoContent, nil)
}
