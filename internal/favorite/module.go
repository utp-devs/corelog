package favorite

import (
	httpadapter "github.com/jhonsferg/corelog/internal/favorite/adapter/in/http"
	"github.com/jhonsferg/corelog/internal/favorite/application/port"
	"github.com/jhonsferg/corelog/internal/favorite/application/usecase"
	"github.com/jhonsferg/corelog/internal/favorite/domain"
	"github.com/jhonsferg/corelog/internal/platform/clock"
)

// Module wires the favorites hexagon. RecentRecorder is the entry point other modules use to record views.
type Module struct {
	Service        port.Service
	RecentRecorder port.RecentRecorder
	Handler        *httpadapter.Handler
}

// NewModule registers the visibility rule of projects. Other entity types are rejected as unsupported
// until their rules are added to the registry here.
func NewModule(repo port.Repository, projectVisibility port.EntityVisibility, jwtSecret string) *Module {
	visibility := map[domain.EntityType]port.EntityVisibility{
		domain.EntityProject: projectVisibility,
	}
	service := usecase.NewService(repo, visibility, clock.RealClock{})
	return &Module{
		Service:        service,
		RecentRecorder: service,
		Handler:        httpadapter.NewHandler(service, jwtSecret),
	}
}
