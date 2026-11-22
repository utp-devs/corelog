package http

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/jhonsferg/corelog/internal/dashboard/application/port"
	"github.com/jhonsferg/corelog/internal/platform/apperror"
	"github.com/jhonsferg/corelog/internal/platform/httpresponse"
	appmw "github.com/jhonsferg/corelog/internal/platform/middleware"
)

type Handler struct {
	service   port.Service
	jwtSecret string
}

func NewHandler(service port.Service, jwtSecret string) *Handler {
	return &Handler{service: service, jwtSecret: jwtSecret}
}

func (h *Handler) RegisterRoutes(r chi.Router) {
	r.Route("/dashboard", func(r chi.Router) {
		r.Use(appmw.Authenticate(h.jwtSecret))
		r.Get("/me", h.Me)
	})
}

type sprintResponse struct {
	ID         string  `json:"id"`
	Name       string  `json:"name"`
	ProjectKey string  `json:"project_key"`
	EndDate    *string `json:"end_date"`
}

type issueResponse struct {
	ID         string  `json:"id"`
	Title      string  `json:"title"`
	Status     string  `json:"status"`
	Priority   string  `json:"priority"`
	ProjectKey string  `json:"project_key"`
	DueDate    *string `json:"due_date"`
}

type summaryResponse struct {
	AssignedOpen     int              `json:"assigned_open"`
	AssignedByStatus map[string]int   `json:"assigned_by_status"`
	Overdue          int              `json:"overdue"`
	ProjectCount     int              `json:"project_count"`
	ActiveSprints    []sprintResponse `json:"active_sprints"`
	RecentIssues     []issueResponse  `json:"recent_issues"`
}

// Me devuelve el resumen personal del usuario autenticado.
func (h *Handler) Me(w http.ResponseWriter, r *http.Request) {
	userID, ok := appmw.UserIDFromContext(r.Context())
	if !ok {
		httpresponse.Error(w, apperror.New(apperror.KindUnauthorized, "not authenticated"))
		return
	}
	summary, err := h.service.Me(r.Context(), userID)
	if err != nil {
		httpresponse.Error(w, err)
		return
	}
	response := summaryResponse{
		AssignedOpen: summary.AssignedOpen, AssignedByStatus: summary.AssignedByStatus,
		Overdue: summary.Overdue, ProjectCount: summary.ProjectCount,
		ActiveSprints: make([]sprintResponse, 0, len(summary.ActiveSprints)),
		RecentIssues:  make([]issueResponse, 0, len(summary.RecentIssues)),
	}
	for _, sprint := range summary.ActiveSprints {
		response.ActiveSprints = append(response.ActiveSprints, sprintResponse{ID: sprint.ID.String(), Name: sprint.Name, ProjectKey: sprint.ProjectKey, EndDate: formatDate(sprint.EndDate)})
	}
	for _, issue := range summary.RecentIssues {
		response.RecentIssues = append(response.RecentIssues, issueResponse{ID: issue.ID.String(), Title: issue.Title, Status: issue.Status, Priority: issue.Priority, ProjectKey: issue.ProjectKey, DueDate: formatDate(issue.DueDate)})
	}
	httpresponse.JSON(w, http.StatusOK, response)
}

func formatDate(value *time.Time) *string {
	if value == nil {
		return nil
	}
	formatted := value.Format("2006-01-02")
	return &formatted
}
