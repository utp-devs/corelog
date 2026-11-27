package release

import (
	"github.com/jhonsferg/corelog/internal/platform/clock"
	httpadapter "github.com/jhonsferg/corelog/internal/release/adapter/in/http"
	"github.com/jhonsferg/corelog/internal/release/application/port"
	"github.com/jhonsferg/corelog/internal/release/application/usecase"
)

// Module groups the versions and release notes of a project.
type Module struct {
	Service port.Service
	Handler *httpadapter.Handler
}

// Dependencies are the collaborators the module needs. Events and Tx may be nil in tests.
type Dependencies struct {
	Repository port.Repository
	Authorizer port.Authorizer
	Projects   port.ProjectKeys
	Tx         port.TxRunner
	Events     port.EventPublisher
	Clock      clock.Clock
	JWTSecret  string
}

// NewModule wires the version service and its HTTP handler.
func NewModule(deps Dependencies) *Module {
	service := usecase.NewVersionService(deps.Repository, deps.Authorizer, deps.Tx, deps.Events, deps.Clock)
	return &Module{Service: service, Handler: httpadapter.NewHandler(service, deps.Projects, deps.JWTSecret)}
}
