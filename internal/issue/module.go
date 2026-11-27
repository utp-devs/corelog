package issue

import (
	httpadapter "github.com/jhonsferg/corelog/internal/issue/adapter/in/http"
	"github.com/jhonsferg/corelog/internal/issue/application/port"
	"github.com/jhonsferg/corelog/internal/issue/application/usecase"
	"github.com/jhonsferg/corelog/internal/platform/database"
)

type Module struct {
	Service port.Service
	Handler *httpadapter.Handler
	service *usecase.IssueService
}

func NewModule(repo port.Repository, directory port.UserDirectory, collaboration port.CollaborationRepository, issueLinks port.IssueLinkRepository, workflows port.WorkflowRepository, watchers port.IssueWatcherRepository, savedFilters port.SavedFilterRepository, jwtSecret string, authorizer port.ProjectAuthorizer, notifier port.Notifier, projectRoles ...port.ProjectRoleResolver) *Module {
	dependencies := []any{collaboration, issueLinks, workflows, watchers, savedFilters, repo}
	if authorizer != nil {
		dependencies = append(dependencies, authorizer)
	}
	for _, resolver := range projectRoles {
		dependencies = append(dependencies, resolver)
	}
	service := usecase.NewIssueService(repo, directory, dependencies...).WithNotifier(notifier)
	handler := httpadapter.NewHandler(service, jwtSecret)

	return &Module{
		Service: service,
		Handler: handler,
		service: service,
	}
}

// WithTxManager fija el gestor de transacciones que agrupa asignar y desasignar con su historial.
// Sin esta llamada las escrituras se ejecutan sin transaccion.
func (m *Module) WithTxManager(tx database.TxManager) *Module {
	m.service.WithTxManager(tx)
	return m
}
