package ticket

import (
	httpadapter "github.com/jhonsferg/corelog/internal/ticket/adapter/in/http"
	"github.com/jhonsferg/corelog/internal/ticket/application/port"
	"github.com/jhonsferg/corelog/internal/ticket/application/usecase"
)

type Module struct {
	Service port.Service
	Handler *httpadapter.Handler
}

func NewModule(repo port.Repository, directory port.UserDirectory, collaboration port.CollaborationRepository, issueLinks port.IssueLinkRepository, jwtSecret string) *Module {
	service := usecase.NewTicketService(repo, directory, collaboration, issueLinks)
	handler := httpadapter.NewHandler(service, jwtSecret)

	return &Module{
		Service: service,
		Handler: handler,
	}
}
