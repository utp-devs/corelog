package http

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/jhonsferg/corelog/internal/board/application/port"
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
	r.Route("/projects/{id}/board", func(r chi.Router) {
		r.Use(appmw.Authenticate(h.jwtSecret))
		r.Get("/", h.Get)
	})
}

func (h *Handler) Get(w http.ResponseWriter, r *http.Request) {
	projectID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpresponse.Error(w, apperror.New(apperror.KindValidation, "invalid project id"))
		return
	}
	actorID, ok := appmw.UserIDFromContext(r.Context())
	if !ok {
		httpresponse.Error(w, apperror.New(apperror.KindUnauthorized, "not authenticated"))
		return
	}
	globalRole, _ := appmw.RoleFromContext(r.Context())
	board, err := h.service.Get(r.Context(), projectID, actorID, globalRole)
	if err != nil {
		httpresponse.Error(w, err)
		return
	}
	httpresponse.JSON(w, http.StatusOK, toBoardResponse(board))
}

func toBoardResponse(board *port.BoardView) boardResponse {
	response := boardResponse{
		ID: board.ID, ProjectID: board.ProjectID, Name: board.Name, Type: board.Type,
		Columns: make([]boardColumnResponse, 0, len(board.Columns)),
	}
	for _, column := range board.Columns {
		columnResponse := boardColumnResponse{
			ID: column.ID, Name: column.Name, Status: column.Status, Position: column.Position,
			WIPLimit: column.WIPLimit, Count: len(column.Cards),
			Cards: make([]boardCardResponse, 0, len(column.Cards)),
		}
		for _, card := range column.Cards {
			columnResponse.Cards = append(columnResponse.Cards, boardCardResponse{
				ID: card.ID, Type: card.Type, Summary: card.Summary, Status: card.Status,
				Priority: card.Priority, AssigneeID: card.AssigneeID, Assignee: card.Assignee,
				StoryPoints: card.StoryPoints, Rank: card.Rank,
			})
		}
		response.Columns = append(response.Columns, columnResponse)
	}
	return response
}
