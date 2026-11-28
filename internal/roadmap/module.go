// Package roadmap is the timeline read model of a project: epics, versions and sprints as bars.
package roadmap

import (
	httpadapter "github.com/jhonsferg/corelog/internal/roadmap/adapter/in/http"
	"github.com/jhonsferg/corelog/internal/roadmap/application/port"
	"github.com/jhonsferg/corelog/internal/roadmap/application/usecase"
)

// Module bundles the roadmap service and its HTTP handler.
type Module struct {
	Service port.Service
	Handler *httpadapter.Handler
}

// NewModule wires the repository and the project authorizer (the root of composition provides both).
func NewModule(repo port.Repository, authorizer port.Authorizer, jwtSecret string) *Module {
	service := usecase.NewService(repo, authorizer)
	return &Module{Service: service, Handler: httpadapter.NewHandler(service, jwtSecret)}
}
