// Package importexport is the CSV import and project export hexagon.
package importexport

import (
	httpadapter "github.com/jhonsferg/corelog/internal/importexport/adapter/in/http"
	"github.com/jhonsferg/corelog/internal/importexport/application/usecase"
)

// Module bundles the service (which also runs the queued imports) and the handler.
type Module struct {
	Service *usecase.Service
	Handler *httpadapter.Handler
}

// NewModule wires the use cases.
func NewModule(deps usecase.Deps, jwtSecret string) *Module {
	service := usecase.NewService(deps)
	return &Module{Service: service, Handler: httpadapter.NewHandler(service, jwtSecret)}
}
