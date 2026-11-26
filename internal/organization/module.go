package organization

import (
	"github.com/jhonsferg/corelog/internal/organization/application/port"
	"github.com/jhonsferg/corelog/internal/organization/application/usecase"
	"github.com/jhonsferg/corelog/internal/platform/clock"
	"github.com/jhonsferg/corelog/internal/platform/database"
	"github.com/jhonsferg/corelog/internal/platform/events"
)

// Module holds the wired organization use cases. The HTTP handler is added by a later slice.
type Module struct {
	Service port.Service
}

// NewModule connects the repository, the transaction manager, the event publisher and the clock.
func NewModule(repo port.Repository, tx database.TxManager, pub events.Publisher, clk clock.Clock) *Module {
	return &Module{Service: usecase.NewOrganizationService(repo, tx, pub, clk)}
}
