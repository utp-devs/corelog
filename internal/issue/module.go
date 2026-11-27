package issue

import (
	httpadapter "github.com/jhonsferg/corelog/internal/issue/adapter/in/http"
	"github.com/jhonsferg/corelog/internal/issue/application/port"
	"github.com/jhonsferg/corelog/internal/issue/application/usecase"
	"github.com/jhonsferg/corelog/internal/platform/clock"
	"github.com/jhonsferg/corelog/internal/platform/database"
	"github.com/jhonsferg/corelog/internal/platform/events"
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

// WithPermissions makes field edits check individual permissions (edit_issue, edit_own_issue, assign_issue,
// delete_issue) and fills the permissions of the issue view.
func (m *Module) WithPermissions(p port.IssuePermissionChecker) *Module {
	m.service.WithPermissions(p)
	return m
}

// WithEvents makes issue changes publish domain events inside their transaction.
func (m *Module) WithEvents(p events.Publisher) *Module {
	m.service.WithEvents(p)
	return m
}

// WithClock fixes the time source of the use cases.
func (m *Module) WithClock(c clock.Clock) *Module {
	m.service.WithClock(c)
	return m
}

// WithTransitions replaces the legacy transition provider of the issue view.
func (m *Module) WithTransitions(p port.TransitionProvider) *Module {
	m.service.WithTransitions(p)
	return m
}
