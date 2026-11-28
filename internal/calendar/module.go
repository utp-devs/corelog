package calendar

import (
	httpadapter "github.com/jhonsferg/corelog/internal/calendar/adapter/in/http"
	"github.com/jhonsferg/corelog/internal/calendar/application/port"
	"github.com/jhonsferg/corelog/internal/calendar/application/usecase"
	"github.com/jhonsferg/corelog/internal/platform/clock"
)

type Module struct {
	Service port.Service
	// Events is nil until WithEvents is called.
	Events  port.EventService
	Handler *httpadapter.Handler
}

// NewModule conecta el lector de hitos y el autorizador de proyecto (el root lo provee).
func NewModule(repo port.Repository, authorizer port.Authorizer, jwtSecret string) *Module {
	service := usecase.NewCalendarService(repo, authorizer)
	return &Module{Service: service, Handler: httpadapter.NewHandler(service, jwtSecret)}
}

// WithEvents enables events with recurrence, the merged calendar and the ICS feed. publicURL is the base URL the
// feed address is built on.
func (m *Module) WithEvents(repo port.EventRepository, authorizer port.Authorizer, clk clock.Clock, publicURL string) *Module {
	m.Events = usecase.NewEventService(repo, authorizer, clk)
	m.Handler.WithEvents(m.Events, publicURL)
	return m
}
