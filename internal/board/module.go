package board

import (
	httpadapter "github.com/jhonsferg/corelog/internal/board/adapter/in/http"
	"github.com/jhonsferg/corelog/internal/board/application/port"
	"github.com/jhonsferg/corelog/internal/board/application/usecase"
)

type Module struct {
	Service port.Service
	Handler *httpadapter.Handler
}

func NewModule(repository port.Repository, projects port.ProjectAuthorizer, jwtSecret string) *Module {
	service := usecase.NewBoardService(repository, projects)
	return &Module{Service: service, Handler: httpadapter.NewHandler(service, jwtSecret)}
}
