package note

import (
	httpadapter "github.com/jhonsferg/corelog/internal/note/adapter/in/http"
	"github.com/jhonsferg/corelog/internal/note/application/port"
	"github.com/jhonsferg/corelog/internal/note/application/usecase"
	"github.com/jhonsferg/corelog/internal/platform/clock"
)

// Module wires the notes hexagon. Search is the provider the global search consumes.
type Module struct {
	Service port.Service
	Search  port.SearchProvider
	Handler *httpadapter.Handler
}

// NewModule builds the notes module. links decides whether a user may link an issue or a project.
func NewModule(repo port.Repository, links port.LinkAccess, jwtSecret string) *Module {
	service := usecase.NewService(repo, links, clock.RealClock{})
	return &Module{
		Service: service,
		Search:  service,
		Handler: httpadapter.NewHandler(service, jwtSecret),
	}
}
