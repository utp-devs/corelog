// Package worklog is the time tracking hexagon: worklogs, remaining estimate adjustments, issue estimates,
// the personal timesheet and the project time summary.
package worklog

import (
	"github.com/jhonsferg/corelog/internal/platform/clock"
	"github.com/jhonsferg/corelog/internal/platform/database"
	"github.com/jhonsferg/corelog/internal/platform/events"
	httpadapter "github.com/jhonsferg/corelog/internal/worklog/adapter/in/http"
	"github.com/jhonsferg/corelog/internal/worklog/application/port"
	"github.com/jhonsferg/corelog/internal/worklog/application/usecase"
)

// Module bundles the service and the HTTP handler.
type Module struct {
	Service port.Service
	Handler *httpadapter.Handler
}

// NewModule wires the repository, the project authorizer, the transaction manager, the event publisher
// (nil disables events) and the clock.
func NewModule(repo port.Repository, authorizer port.Authorizer, tx database.TxManager, pub events.Publisher, clk clock.Clock, jwtSecret string) *Module {
	service := usecase.NewService(repo, authorizer, tx, pub, clk)
	return &Module{Service: service, Handler: httpadapter.NewHandler(service, jwtSecret)}
}
