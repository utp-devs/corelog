package dashboard

import (
	httpadapter "github.com/jhonsferg/corelog/internal/dashboard/adapter/in/http"
	"github.com/jhonsferg/corelog/internal/dashboard/application/port"
	"github.com/jhonsferg/corelog/internal/dashboard/application/usecase"
)

type Module struct {
	Service port.Service
	Handler *httpadapter.Handler
}

// NewModule conecta el lector del resumen personal.
func NewModule(reader port.Reader, jwtSecret string) *Module {
	service := usecase.NewDashboardService(reader)
	return &Module{Service: service, Handler: httpadapter.NewHandler(service, jwtSecret)}
}
