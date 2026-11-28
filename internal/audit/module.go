// Package audit is the audit log hexagon. It consumes domain events, stores immutable entries and serves
// them through a read-only HTTP API.
package audit

import (
	httpadapter "github.com/jhonsferg/corelog/internal/audit/adapter/in/http"
	"github.com/jhonsferg/corelog/internal/audit/application/port"
	"github.com/jhonsferg/corelog/internal/audit/application/usecase"
	"github.com/jhonsferg/corelog/internal/platform/clock"
	"github.com/jhonsferg/corelog/internal/platform/events"
)

// Module exposes the audit consumer for the events dispatcher, the retention job for the jobs scheduler and
// the HTTP handler.
type Module struct {
	Consumer     events.Consumer
	RetentionJob usecase.RetentionJob
	Service      port.Service
	Handler      *httpadapter.Handler
}

// NewModule wires the audit use cases. retentionDays <= 0 selects the default (365). orgs decides who may read
// the audit log of an organization.
func NewModule(repo port.Repository, orgs port.OrgAccess, retentionDays int, jwtSecret string, clk clock.Clock) *Module {
	service := usecase.NewService(repo, orgs)
	return &Module{
		Consumer:     usecase.NewConsumer(repo, clk),
		RetentionJob: usecase.NewRetentionJob(repo, retentionDays, clk),
		Service:      service,
		Handler:      httpadapter.NewHandler(service, jwtSecret),
	}
}
