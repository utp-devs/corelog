package http

import (
	"net/http"

	"github.com/google/uuid"

	"github.com/jhonsferg/corelog/internal/issue/domain"
	"github.com/jhonsferg/corelog/internal/platform/apperror"
	"github.com/jhonsferg/corelog/internal/platform/httpresponse"
	appmw "github.com/jhonsferg/corelog/internal/platform/middleware"
)

type childProgressResponse struct {
	Total   int `json:"total"`
	Done    int `json:"done"`
	Percent int `json:"percent"`
}

type childIssueResponse struct {
	ID           string  `json:"id"`
	Key          string  `json:"key"`
	Summary      string  `json:"summary"`
	Type         string  `json:"type"`
	Status       string  `json:"status"`
	AssigneeID   *string `json:"assignee_id"`
	AssigneeName *string `json:"assignee_name"`
}

type childrenResponse struct {
	Progress childProgressResponse `json:"progress"`
	Children []childIssueResponse  `json:"children"`
}

// ListChildren returns the direct children of an issue with the progress of the done ones.
func (h *Handler) ListChildren(w http.ResponseWriter, r *http.Request) {
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
	view, err := h.service.ListChildren(r.Context(), issueID, actorID, globalRole)
	if err != nil {
		httpresponse.Error(w, err)
		return
	}
	out := childrenResponse{
		Progress: childProgressResponse{Total: view.Progress.Total, Done: view.Progress.Done, Percent: view.Progress.Percent},
		Children: make([]childIssueResponse, 0, len(view.Children)),
	}
	for _, child := range view.Children {
		out.Children = append(out.Children, toChildIssueResponse(child, view.AssigneeNames))
	}
	httpresponse.JSON(w, http.StatusOK, out)
}

func toChildIssueResponse(issue *domain.Issue, names map[uuid.UUID]string) childIssueResponse {
	typeName := string(issue.IssueType)
	if issue.IssueTypeRef != nil {
		typeName = issue.IssueTypeRef.Name
	}
	out := childIssueResponse{
		ID: issue.ID.String(), Key: issue.Key(), Summary: issue.Summary,
		Type: typeName, Status: string(issue.Status),
	}
	if issue.AssigneeID != nil {
		id := issue.AssigneeID.String()
		out.AssigneeID = &id
		if name, ok := names[*issue.AssigneeID]; ok {
			out.AssigneeName = &name
		}
	}
	return out
}
