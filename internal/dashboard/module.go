package dashboard

import (
	httpadapter "github.com/jhonsferg/corelog/internal/dashboard/adapter/in/http"
	"github.com/jhonsferg/corelog/internal/dashboard/application/port"
	"github.com/jhonsferg/corelog/internal/dashboard/application/usecase"
	"github.com/jhonsferg/corelog/internal/platform/clock"
)

type Module struct {
	Service port.Service
	// Dashboards is nil until WithDashboards is called.
	Dashboards port.DashboardService
	Handler    *httpadapter.Handler
}

// NewModule conecta el lector del resumen personal.
func NewModule(reader port.Reader, jwtSecret string) *Module {
	service := usecase.NewDashboardService(reader)
	return &Module{Service: service, Handler: httpadapter.NewHandler(service, jwtSecret)}
}

// DashboardDeps are the collaborators of the dashboards. Issues and Upcoming may be nil.
type DashboardDeps struct {
	Repository port.Repository
	Data       port.DataSource
	Issues     port.IssueRunner
	Upcoming   port.UpcomingSource
	Clock      clock.Clock
}

// WithDashboards enables dashboards with gadgets, sharing and per-gadget data.
func (m *Module) WithDashboards(deps DashboardDeps) *Module {
	m.Dashboards = usecase.NewDashboardsService(usecase.DashboardsDeps{
		Repository: deps.Repository, Data: deps.Data, Issues: deps.Issues, Upcoming: deps.Upcoming, Clock: deps.Clock,
	})
	m.Handler.WithDashboards(m.Dashboards)
	return m
}
