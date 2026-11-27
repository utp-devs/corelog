package http

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/jhonsferg/corelog/internal/platform/apperror"
	"github.com/jhonsferg/corelog/internal/platform/httpresponse"
	appmw "github.com/jhonsferg/corelog/internal/platform/middleware"
	"github.com/jhonsferg/corelog/internal/sprint/application/port"
)

type backlogLabelResponse struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Color string `json:"color"`
}

type backlogIssueResponse struct {
	ID                      string                 `json:"id"`
	Key                     string                 `json:"key"`
	Summary                 string                 `json:"summary"`
	IssueType               string                 `json:"issue_type"`
	IssueTypeName           string                 `json:"issue_type_name"`
	IssueTypeIcon           string                 `json:"issue_type_icon"`
	IssueTypeColor          string                 `json:"issue_type_color"`
	Status                  string                 `json:"status"`
	StatusName              string                 `json:"status_name"`
	StatusCategory          string                 `json:"status_category"`
	Priority                string                 `json:"priority"`
	PriorityName            string                 `json:"priority_name"`
	PriorityIcon            string                 `json:"priority_icon"`
	PriorityColor           string                 `json:"priority_color"`
	AssigneeID              *string                `json:"assignee_id"`
	AssigneeName            string                 `json:"assignee_name"`
	StoryPoints             *float64               `json:"story_points"`
	OriginalEstimateSeconds *int64                 `json:"original_estimate_seconds"`
	Rank                    string                 `json:"rank"`
	EpicID                  *string                `json:"epic_id"`
	EpicKey                 string                 `json:"epic_key"`
	EpicSummary             string                 `json:"epic_summary"`
	Flagged                 bool                   `json:"flagged"`
	Labels                  []backlogLabelResponse `json:"labels"`
}

type categoryTotalsResponse struct {
	Issues                  int     `json:"issues"`
	Points                  float64 `json:"points"`
	OriginalEstimateSeconds int64   `json:"original_estimate_seconds"`
}

type totalsResponse struct {
	categoryTotalsResponse
	ByCategory map[string]categoryTotalsResponse `json:"by_category"`
}

type backlogSectionResponse struct {
	Kind   string                 `json:"kind"`
	Sprint *sprintResponse        `json:"sprint"`
	Issues []backlogIssueResponse `json:"issues"`
	Totals totalsResponse         `json:"totals"`
}

type backlogEpicResponse struct {
	ID         string  `json:"id"`
	Key        string  `json:"key"`
	Summary    string  `json:"summary"`
	Status     string  `json:"status"`
	Total      int     `json:"total"`
	Done       int     `json:"done"`
	Points     float64 `json:"points"`
	DonePoints float64 `json:"done_points"`
	Progress   float64 `json:"progress"`
}

type backlogResponse struct {
	ProjectID        string                   `json:"project_id"`
	Sections         []backlogSectionResponse `json:"sections"`
	Epics            []backlogEpicResponse    `json:"epics"`
	CanManageSprints bool                     `json:"can_manage_sprints"`
	CanMoveCards     bool                     `json:"can_move_cards"`
	Truncated        bool                     `json:"truncated"`
}

// Backlog responde GET /projects/{id}/backlog.
func (h *Handler) Backlog(w http.ResponseWriter, r *http.Request) {
	actorID, ok := appmw.UserIDFromContext(r.Context())
	if !ok {
		httpresponse.Error(w, apperror.New(apperror.KindUnauthorized, "No autenticado"))
		return
	}
	globalRole, _ := appmw.RoleFromContext(r.Context())
	projectID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpresponse.Error(w, apperror.WithCode(apperror.KindValidation, "validation.failed", "El identificador de proyecto no es valido"))
		return
	}
	filter := port.BacklogFilter{Version: r.URL.Query().Get("version"), Query: r.URL.Query().Get("q")}
	if raw := r.URL.Query().Get("epic"); raw != "" {
		epic, err := uuid.Parse(raw)
		if err != nil {
			httpresponse.Error(w, apperror.WithCode(apperror.KindValidation, "validation.failed", "El identificador de epica no es valido"))
			return
		}
		filter.EpicID = &epic
	}
	backlog, err := h.service.Backlog(r.Context(), actorID, globalRole, projectID, filter)
	if err != nil {
		httpresponse.Error(w, err)
		return
	}
	httpresponse.JSON(w, http.StatusOK, toBacklogResponse(backlog))
}

func toBacklogResponse(b *port.Backlog) backlogResponse {
	out := backlogResponse{
		ProjectID: b.ProjectID.String(), Sections: make([]backlogSectionResponse, 0, len(b.Sections)),
		Epics: make([]backlogEpicResponse, 0, len(b.Epics)), CanManageSprints: b.CanManageSprint,
		CanMoveCards: b.CanMoveCards, Truncated: b.Truncated,
	}
	for _, s := range b.Sections {
		section := backlogSectionResponse{Kind: s.Kind, Issues: make([]backlogIssueResponse, 0, len(s.Issues)), Totals: toTotals(s.Totals)}
		if s.Sprint != nil {
			sp := toSprintResponse(s.Sprint)
			section.Sprint = &sp
		}
		for _, i := range s.Issues {
			item := backlogIssueResponse{
				ID: i.ID.String(), Key: i.Key, Summary: i.Summary, IssueType: i.IssueType, IssueTypeName: i.IssueTypeName,
				IssueTypeIcon: i.IssueTypeIcon, IssueTypeColor: i.IssueTypeColor, Status: i.Status, StatusName: i.StatusName,
				StatusCategory: i.StatusCategory, Priority: i.Priority, PriorityName: i.PriorityName, PriorityIcon: i.PriorityIcon,
				PriorityColor: i.PriorityColor, AssigneeName: i.AssigneeName, StoryPoints: i.StoryPoints,
				OriginalEstimateSeconds: i.OriginalEstimateSeconds, Rank: i.Rank, EpicKey: i.EpicKey, EpicSummary: i.EpicSummary,
				Flagged: i.Flagged, Labels: make([]backlogLabelResponse, 0, len(i.Labels)),
			}
			if i.AssigneeID != nil {
				v := i.AssigneeID.String()
				item.AssigneeID = &v
			}
			if i.EpicID != nil {
				v := i.EpicID.String()
				item.EpicID = &v
			}
			for _, l := range i.Labels {
				item.Labels = append(item.Labels, backlogLabelResponse{ID: l.ID.String(), Name: l.Name, Color: l.Color})
			}
			section.Issues = append(section.Issues, item)
		}
		out.Sections = append(out.Sections, section)
	}
	for _, e := range b.Epics {
		progress := 0.0
		if e.Total > 0 {
			progress = float64(e.Done) / float64(e.Total)
		}
		out.Epics = append(out.Epics, backlogEpicResponse{ID: e.ID.String(), Key: e.Key, Summary: e.Summary, Status: e.Status,
			Total: e.Total, Done: e.Done, Points: e.Points, DonePoints: e.DonePoints, Progress: progress})
	}
	return out
}

func toTotals(t port.SectionTotals) totalsResponse {
	out := totalsResponse{
		categoryTotalsResponse: categoryTotalsResponse{Issues: t.Issues, Points: t.Points, OriginalEstimateSeconds: t.EstimateSeconds},
		ByCategory:             make(map[string]categoryTotalsResponse, len(t.ByCategory)),
	}
	for k, v := range t.ByCategory {
		out.ByCategory[k] = categoryTotalsResponse{Issues: v.Issues, Points: v.Points, OriginalEstimateSeconds: v.EstimateSeconds}
	}
	return out
}

type rankRequest struct {
	ProjectID *uuid.UUID  `json:"project_id"`
	IssueIDs  []uuid.UUID `json:"issue_ids"`
	Target    *struct {
		SprintID *uuid.UUID `json:"sprint_id"`
	} `json:"target"`
	BeforeIssueID *uuid.UUID `json:"before_issue_id"`
	AfterIssueID  *uuid.UUID `json:"after_issue_id"`
}

type rankResponse struct {
	ProjectID string   `json:"project_id"`
	IssueIDs  []string `json:"issue_ids"`
	SprintID  *string  `json:"sprint_id"`
}

// Rank responde PUT /backlog/rank: mueve varias incidencias a una seccion junto a un ancla.
func (h *Handler) Rank(w http.ResponseWriter, r *http.Request) {
	actorID, ok := appmw.UserIDFromContext(r.Context())
	if !ok {
		httpresponse.Error(w, apperror.New(apperror.KindUnauthorized, "No autenticado"))
		return
	}
	globalRole, _ := appmw.RoleFromContext(r.Context())
	var body rankRequest
	if err := decodeStrict(r, &body, false); err != nil {
		httpresponse.Error(w, err)
		return
	}
	if body.Target == nil {
		httpresponse.Error(w, apperror.WithCode(apperror.KindValidation, "validation.failed", "Indica el destino (target) del movimiento"))
		return
	}
	projectID := body.ProjectID
	if raw := r.URL.Query().Get("project_id"); raw != "" && projectID == nil {
		parsed, err := uuid.Parse(raw)
		if err != nil {
			httpresponse.Error(w, apperror.WithCode(apperror.KindValidation, "validation.failed", "El identificador de proyecto no es valido"))
			return
		}
		projectID = &parsed
	}
	project, err := h.service.Rank(r.Context(), actorID, globalRole, port.RankInput{
		ProjectID: projectID, IssueIDs: body.IssueIDs, TargetSprintID: body.Target.SprintID,
		BeforeIssueID: body.BeforeIssueID, AfterIssueID: body.AfterIssueID,
	})
	if err != nil {
		httpresponse.Error(w, err)
		return
	}
	response := rankResponse{ProjectID: project.String(), IssueIDs: make([]string, 0, len(body.IssueIDs))}
	for _, id := range body.IssueIDs {
		response.IssueIDs = append(response.IssueIDs, id.String())
	}
	if body.Target.SprintID != nil {
		v := body.Target.SprintID.String()
		response.SprintID = &v
	}
	httpresponse.JSON(w, http.StatusOK, response)
}
