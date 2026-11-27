package sprint

import (
	"github.com/jhonsferg/corelog/internal/platform/clock"
	"github.com/jhonsferg/corelog/internal/platform/database"
	httpadapter "github.com/jhonsferg/corelog/internal/sprint/adapter/in/http"
	"github.com/jhonsferg/corelog/internal/sprint/application/port"
	"github.com/jhonsferg/corelog/internal/sprint/application/usecase"
)

type Module struct {
	Service port.Service
	Handler *httpadapter.Handler
}

// NewModule conecta el repositorio, el autorizador de proyecto, el gestor de transacciones y el reloj.
func NewModule(repo port.Repository, authorizer port.Authorizer, tx database.TxManager, clk clock.Clock, jwtSecret string) *Module {
	service := usecase.NewSprintService(repo, authorizer, tx, clk)
	return &Module{Service: service, Handler: httpadapter.NewHandler(service, jwtSecret)}
}
