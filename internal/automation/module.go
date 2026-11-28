// Package automation is the rule engine hexagon: rules with triggers, conditions and actions, executed from the
// domain events outbox as a system identity with loop prevention, chain depth and rate limits.
package automation

import (
	httpadapter "github.com/jhonsferg/corelog/internal/automation/adapter/in/http"
	"github.com/jhonsferg/corelog/internal/automation/application/usecase"
)

// Module bundles the management service, the engine (events consumer and scheduled runner) and the handler.
type Module struct {
	Service *usecase.Service
	Engine  *usecase.Engine
	Handler *httpadapter.Handler
}

// NewModule wires the use cases.
func NewModule(deps usecase.Deps, jwtSecret string) *Module {
	service := usecase.NewService(deps)
	return &Module{Service: service, Engine: usecase.NewEngine(deps), Handler: httpadapter.NewHandler(service, jwtSecret)}
}
