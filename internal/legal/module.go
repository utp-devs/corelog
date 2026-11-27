// Package legal is the hexagon of versioned legal documents and the auditable consent of each user.
package legal

import (
	httpadapter "github.com/jhonsferg/corelog/internal/legal/adapter/in/http"
	"github.com/jhonsferg/corelog/internal/legal/application/port"
	"github.com/jhonsferg/corelog/internal/legal/application/usecase"
	"github.com/jhonsferg/corelog/internal/platform/clock"
	"github.com/jhonsferg/corelog/internal/platform/database"
	"github.com/jhonsferg/corelog/internal/platform/events"
)

// Module bundles the service and the HTTP handler.
type Module struct {
	Service port.Service
	Handler *httpadapter.Handler
}

// NewModule wires the repository, the transaction manager, the event publisher (nil disables events) and the
// clock. enforcement is the LEGAL_ENFORCEMENT value ("warn" or "block").
func NewModule(repo port.Repository, tx database.TxManager, pub events.Publisher, clk clock.Clock, jwtSecret, enforcement string) *Module {
	service := usecase.NewService(repo, tx, pub, clk)
	return &Module{Service: service, Handler: httpadapter.NewHandler(service, jwtSecret, enforcement)}
}
