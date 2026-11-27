// Package workflow wires the workflow module: the pure engine in domain, the use cases in application and the
// PostgreSQL adapters.
package workflow

import (
	"github.com/jhonsferg/corelog/internal/platform/clock"
	"github.com/jhonsferg/corelog/internal/platform/database"
	httpadapter "github.com/jhonsferg/corelog/internal/workflow/adapter/in/http"
	"github.com/jhonsferg/corelog/internal/workflow/application/port"
	"github.com/jhonsferg/corelog/internal/workflow/application/usecase"
)

// Module exposes the workflow service used by the issue engine.
type Module struct {
	Service *usecase.Service
}

// NewModule builds the module over a repository. A nil clock means the system clock.
func NewModule(repo port.Repository, c clock.Clock) *Module {
	return &Module{Service: usecase.NewService(repo, c)}
}

// ManagementModule is the administration side: the workflow editor API and the status catalog.
type ManagementModule struct {
	Management *usecase.Management
	Handler    *httpadapter.Handler
}

// Dependencies are the ports the administration side needs from the outside world. The composition root
// provides them; the workflow module imports none of the other modules.
type Dependencies struct {
	Authorizer  port.Authorizer
	OrgRoles    port.OrgRoles
	Transaction database.TxManager
	Projects    httpadapter.ProjectResolver
	JWTSecret   string
}

// NewManagementModule builds the administration use cases and their HTTP handler over a repository.
func NewManagementModule(repo port.ManagementRepository, c clock.Clock, deps Dependencies) *ManagementModule {
	management := usecase.NewManagement(repo, deps.Authorizer, deps.OrgRoles, deps.Transaction, c)
	return &ManagementModule{
		Management: management,
		Handler:    httpadapter.NewHandler(management, deps.Projects, deps.JWTSecret),
	}
}
