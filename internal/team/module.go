package team

import (
	"github.com/jhonsferg/corelog/internal/platform/database"
	httpadapter "github.com/jhonsferg/corelog/internal/team/adapter/in/http"
	"github.com/jhonsferg/corelog/internal/team/application/port"
	"github.com/jhonsferg/corelog/internal/team/application/usecase"
)

type Module struct {
	Service port.Service
	Handler *httpadapter.Handler
}

// NewModule wires the team module. orgs is the narrow organization directory
// (membership and team creation policy) and tx runs multi-write use cases atomically.
func NewModule(repo port.Repository, orgs port.OrgDirectory, tx database.TxManager, jwtSecret string) *Module {
	service := usecase.NewTeamService(repo, orgs, tx)
	handler := httpadapter.NewHandler(service, jwtSecret)

	return &Module{
		Service: service,
		Handler: handler,
	}
}
