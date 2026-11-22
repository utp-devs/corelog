package calendar

import (
	httpadapter "github.com/jhonsferg/corelog/internal/calendar/adapter/in/http"
	"github.com/jhonsferg/corelog/internal/calendar/application/port"
	"github.com/jhonsferg/corelog/internal/calendar/application/usecase"
)

type Module struct {
	Service port.Service
	Handler *httpadapter.Handler
}

// NewModule conecta el lector de hitos y el autorizador de proyecto (el root lo provee).
func NewModule(repo port.Repository, authorizer port.Authorizer, jwtSecret string) *Module {
	service := usecase.NewCalendarService(repo, authorizer)
	return &Module{Service: service, Handler: httpadapter.NewHandler(service, jwtSecret)}
}
