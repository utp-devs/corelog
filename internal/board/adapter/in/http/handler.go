package http

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/jhonsferg/corelog/internal/board/application/port"
	"github.com/jhonsferg/corelog/internal/platform/apperror"
	"github.com/jhonsferg/corelog/internal/platform/httpresponse"
	appmw "github.com/jhonsferg/corelog/internal/platform/middleware"
	"github.com/jhonsferg/corelog/internal/platform/validation"
)

type Handler struct {
	service   port.Service
	boards    port.BoardsService
	jwtSecret string
}

func NewHandler(service port.Service, jwtSecret string) *Handler {
	return &Handler{service: service, jwtSecret: jwtSecret}
}

func (h *Handler) RegisterRoutes(r chi.Router) {
	if h.boards != nil {
		h.registerBoardRoutes(r)
	}
	r.Route("/projects/{id}/board", func(r chi.Router) {
		r.Use(appmw.Authenticate(h.jwtSecret))
		r.Get("/", h.Get)
		r.Post("/cards/{issue_id}/move", h.MoveCard)
		r.Patch("/columns/{column_id}", h.UpdateColumnWIP)
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
		CanManageBoard: board.CanManageBoard,
		Columns:        make([]boardColumnResponse, 0, len(board.Columns)),
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

type moveCardRequest struct {
	ColumnID uuid.UUID `json:"column_id"`
}

type updateColumnRequest struct {
	WIPLimit *int `json:"wip_limit"`
}

// MoveCard mueve una incidencia a otra columna del tablero y devuelve el tablero actualizado.
func (h *Handler) MoveCard(w http.ResponseWriter, r *http.Request) {
	projectID, issueID, actorID, globalRole, ok := h.boardContext(w, r, "issue_id")
	if !ok {
		return
	}
	var body moveCardRequest
	if err := validation.DecodeStrict(r.Body, &body); err != nil || body.ColumnID == uuid.Nil {
		httpresponse.Error(w, apperror.New(apperror.KindValidation, "invalid request body"))
		return
	}
	if err := h.service.MoveCard(r.Context(), projectID, actorID, globalRole, issueID, body.ColumnID); err != nil {
		httpresponse.Error(w, err)
		return
	}
	h.respondBoard(w, r, projectID, actorID, globalRole)
}

// UpdateColumnWIP fija o quita el limite WIP de una columna y devuelve el tablero actualizado.
func (h *Handler) UpdateColumnWIP(w http.ResponseWriter, r *http.Request) {
	projectID, columnID, actorID, globalRole, ok := h.boardContext(w, r, "column_id")
	if !ok {
		return
	}
	var body updateColumnRequest
	if err := validation.DecodeStrict(r.Body, &body); err != nil {
		httpresponse.Error(w, apperror.New(apperror.KindValidation, "invalid request body"))
		return
	}
	if err := h.service.UpdateColumnWIP(r.Context(), projectID, actorID, globalRole, columnID, body.WIPLimit); err != nil {
		httpresponse.Error(w, err)
		return
	}
	h.respondBoard(w, r, projectID, actorID, globalRole)
}

// boardContext extrae proyecto, identificador de la ruta (param), actor y rol global.
func (h *Handler) boardContext(w http.ResponseWriter, r *http.Request, param string) (uuid.UUID, uuid.UUID, uuid.UUID, string, bool) {
	projectID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpresponse.Error(w, apperror.New(apperror.KindValidation, "invalid project id"))
		return uuid.Nil, uuid.Nil, uuid.Nil, "", false
	}
	pathID, err := uuid.Parse(chi.URLParam(r, param))
	if err != nil {
		httpresponse.Error(w, apperror.New(apperror.KindValidation, "invalid id"))
		return uuid.Nil, uuid.Nil, uuid.Nil, "", false
	}
	actorID, ok := appmw.UserIDFromContext(r.Context())
	if !ok {
		httpresponse.Error(w, apperror.New(apperror.KindUnauthorized, "not authenticated"))
		return uuid.Nil, uuid.Nil, uuid.Nil, "", false
	}
	globalRole, _ := appmw.RoleFromContext(r.Context())
	return projectID, pathID, actorID, globalRole, true
}

func (h *Handler) respondBoard(w http.ResponseWriter, r *http.Request, projectID, actorID uuid.UUID, globalRole string) {
	board, err := h.service.Get(r.Context(), projectID, actorID, globalRole)
	if err != nil {
		httpresponse.Error(w, err)
		return
	}
	httpresponse.JSON(w, http.StatusOK, toBoardResponse(board))
}
