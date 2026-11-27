package board

import (
	httpadapter "github.com/jhonsferg/corelog/internal/board/adapter/in/http"
	"github.com/jhonsferg/corelog/internal/board/application/port"
	"github.com/jhonsferg/corelog/internal/board/application/usecase"
	"github.com/jhonsferg/corelog/internal/platform/clock"
)

type Module struct {
	Service port.Service
	Boards  port.BoardsService
	Handler *httpadapter.Handler
}

// Deps are the collaborators of the board module besides its own repositories.
type Deps struct {
	Repository  port.Repository
	Boards      port.BoardRepository
	Projects    port.ProjectAuthorizer
	Authorizer  port.Authorizer
	Mover       port.IssueMover
	Transitions port.CardTransitions
	Editor      port.CardEditor
	Clock       clock.Clock
	JWTSecret   string
}

func NewModule(d Deps) *Module {
	service := usecase.NewBoardService(d.Repository, d.Projects, d.Authorizer, d.Mover)
	boards := usecase.NewBoardsService(d.Boards, d.Projects, d.Authorizer, d.Transitions, d.Editor, usecase.MinimalQueryFilter{}, d.Clock)
	return &Module{Service: service, Boards: boards, Handler: httpadapter.NewHandler(service, d.JWTSecret).WithBoards(boards)}
}
