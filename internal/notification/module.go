package notification

import (
	httpadapter "github.com/jhonsferg/corelog/internal/notification/adapter/in/http"
	"github.com/jhonsferg/corelog/internal/notification/application/port"
	"github.com/jhonsferg/corelog/internal/notification/application/usecase"
)

type Module struct {
	Service port.Service
	Handler *httpadapter.Handler
}

func NewModule(repo port.Repository, jwtSecret string) *Module {
	service := usecase.NewNotificationService(repo)
	return &Module{Service: service, Handler: httpadapter.NewHandler(service, jwtSecret)}
}
