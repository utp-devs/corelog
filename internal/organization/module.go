package organization

import (
	httpadapter "github.com/jhonsferg/corelog/internal/organization/adapter/in/http"
	"github.com/jhonsferg/corelog/internal/organization/application/port"
	"github.com/jhonsferg/corelog/internal/organization/application/usecase"
	"github.com/jhonsferg/corelog/internal/platform/clock"
	"github.com/jhonsferg/corelog/internal/platform/database"
	"github.com/jhonsferg/corelog/internal/platform/events"
)

// Module holds the wired organization use cases and their HTTP handler.
type Module struct {
	Service port.Service
	Handler *httpadapter.Handler
}

// NewModule connects the repository, the transaction manager, the event publisher and the clock, and
// builds the HTTP handler. emails resolves the account email needed to accept invitations; publicURL is
// the origin used in accept links (empty means the request origin).
func NewModule(repo port.Repository, tx database.TxManager, pub events.Publisher, clk clock.Clock,
	jwtSecret string, emails httpadapter.UserEmailLookup, publicURL string) *Module {
	service := usecase.NewOrganizationService(repo, tx, pub, clk)
	return &Module{
		Service: service,
		Handler: httpadapter.NewHandler(service, emails, clk, jwtSecret, publicURL),
	}
}
