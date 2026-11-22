package project

import (
	httpadapter "github.com/jhonsferg/corelog/internal/project/adapter/in/http"
	"github.com/jhonsferg/corelog/internal/project/application/port"
	"github.com/jhonsferg/corelog/internal/project/application/usecase"
)

type Module struct {
	Service port.Service
	Handler *httpadapter.Handler
}

func NewModule(repo port.Repository, memberRepo port.MemberRepository, catalogRepo port.CatalogRepository, jwtSecret string, authorizer port.ProjectAuthorizer) *Module {
	service := usecase.NewProjectService(repo, memberRepo, catalogRepo).WithAuthorizer(authorizer)
	handler := httpadapter.NewHandler(service, jwtSecret)
	return &Module{Service: service, Handler: handler}
}
