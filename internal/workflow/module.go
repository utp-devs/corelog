// Package workflow wires the workflow module: the pure engine in domain, the use cases in application and the
// PostgreSQL adapters.
package workflow

import (
	"github.com/jhonsferg/corelog/internal/platform/clock"
	"github.com/jhonsferg/corelog/internal/workflow/application/port"
	"github.com/jhonsferg/corelog/internal/workflow/application/usecase"
)

// Module exposes the workflow service.
type Module struct {
	Service *usecase.Service
}

// NewModule builds the module over a repository. A nil clock means the system clock.
func NewModule(repo port.Repository, c clock.Clock) *Module {
	return &Module{Service: usecase.NewService(repo, c)}
}
