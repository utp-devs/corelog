// Package audit is the audit log hexagon. It consumes domain events and stores
// immutable entries. HTTP endpoints are not part of this module yet.
package audit

import (
	"github.com/jhonsferg/corelog/internal/audit/application/port"
	"github.com/jhonsferg/corelog/internal/audit/application/usecase"
	"github.com/jhonsferg/corelog/internal/platform/clock"
	"github.com/jhonsferg/corelog/internal/platform/events"
)

// Module exposes the audit consumer for the events dispatcher and the
// retention job for the jobs scheduler.
type Module struct {
	Consumer     events.Consumer
	RetentionJob usecase.RetentionJob
}

// NewModule wires the audit use cases. retentionDays <= 0 selects the default (365).
func NewModule(repo port.Repository, retentionDays int, clk clock.Clock) *Module {
	return &Module{
		Consumer:     usecase.NewConsumer(repo, clk),
		RetentionJob: usecase.NewRetentionJob(repo, retentionDays, clk),
	}
}
