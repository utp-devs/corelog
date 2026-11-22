package ticket

import (
	httpadapter "github.com/jhonsferg/corelog/internal/ticket/adapter/in/http"
	"github.com/jhonsferg/corelog/internal/ticket/application/port"
	"github.com/jhonsferg/corelog/internal/ticket/application/usecase"
)

type Module struct {
	Service port.Service
	Handler *httpadapter.Handler
}

func NewModule(repo port.Repository, directory port.UserDirectory, collaboration port.CollaborationRepository, issueLinks port.IssueLinkRepository, workflows port.WorkflowRepository, watchers port.IssueWatcherRepository, savedFilters port.SavedFilterRepository, jwtSecret string, projectRoles ...port.ProjectRoleResolver) *Module {
	dependencies := []any{collaboration, issueLinks, workflows, watchers, savedFilters, repo}
	for _, resolver := range projectRoles {
		dependencies = append(dependencies, resolver)
	}
	service := usecase.NewTicketService(repo, directory, dependencies...)
	handler := httpadapter.NewHandler(service, jwtSecret)

	return &Module{
		Service: service,
		Handler: handler,
	}
}
