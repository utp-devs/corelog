package sprint

import (
	httpadapter "github.com/jhonsferg/corelog/internal/sprint/adapter/in/http"
	"github.com/jhonsferg/corelog/internal/sprint/application/port"
	"github.com/jhonsferg/corelog/internal/sprint/application/usecase"
)

type Module struct {
	Service port.Service
	Handler *httpadapter.Handler
}

// NewModule conecta el repositorio y el autorizador de proyecto (el root lo provee).
func NewModule(repo port.Repository, authorizer port.Authorizer, jwtSecret string) *Module {
	service := usecase.NewSprintService(repo, authorizer)
	return &Module{Service: service, Handler: httpadapter.NewHandler(service, jwtSecret)}
}
