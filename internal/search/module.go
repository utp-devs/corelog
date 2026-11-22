package search

import (
	httpadapter "github.com/jhonsferg/corelog/internal/search/adapter/in/http"
	"github.com/jhonsferg/corelog/internal/search/application/port"
	"github.com/jhonsferg/corelog/internal/search/application/usecase"
)

type Module struct {
	Service port.Service
	Handler *httpadapter.Handler
}

// NewModule conecta el indice de busqueda (SQL sobre proyectos e incidencias).
func NewModule(index port.Index, jwtSecret string) *Module {
	service := usecase.NewSearchService(index)
	return &Module{Service: service, Handler: httpadapter.NewHandler(service, jwtSecret)}
}
