// Package report is the reporting hexagon: burndown, burnup, velocity, cumulative flow, control chart,
// created versus resolved, resolution time, time tracking, workload, version and sprint summary reports.
package report

import (
	"github.com/jhonsferg/corelog/internal/platform/clock"
	httpadapter "github.com/jhonsferg/corelog/internal/report/adapter/in/http"
	"github.com/jhonsferg/corelog/internal/report/application/port"
	"github.com/jhonsferg/corelog/internal/report/application/usecase"
)

// Module bundles the service and the HTTP handler.
type Module struct {
	Service port.Service
	Handler *httpadapter.Handler
}

// Dependencies are the collaborators of the module. Filter and Sprints may be nil.
type Dependencies struct {
	Reader     port.Reader
	Authorizer port.Authorizer
	Filter     port.IssueFilter
	Sprints    port.SprintSummarizer
	Clock      clock.Clock
	JWTSecret  string
}

// NewModule wires the use case and the handler.
func NewModule(d Dependencies) *Module {
	service := usecase.NewService(d.Reader, d.Authorizer, d.Filter, d.Sprints, d.Clock)
	return &Module{Service: service, Handler: httpadapter.NewHandler(service, d.JWTSecret)}
}
