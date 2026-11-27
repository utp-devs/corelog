package project

import (
	"context"

	"github.com/jhonsferg/corelog/internal/platform/clock"

	httpadapter "github.com/jhonsferg/corelog/internal/project/adapter/in/http"
	"github.com/jhonsferg/corelog/internal/project/application/port"
	"github.com/jhonsferg/corelog/internal/project/application/usecase"
)

type Module struct {
	Service port.Service
	Handler *httpadapter.Handler
	service *usecase.ProjectService
}

func NewModule(repo port.Repository, memberRepo port.MemberRepository, catalogRepo port.CatalogRepository, orgs port.OrgDirectory, jwtSecret string, authorizer port.ProjectAuthorizer) *Module {
	service := usecase.NewProjectService(repo, memberRepo, orgs, catalogRepo).WithAuthorizer(authorizer)
	handler := httpadapter.NewHandler(service, service, jwtSecret)
	return &Module{Service: service, Handler: handler, service: service}
}

// WithClock fixes the time source of the component writes.
func (m *Module) WithClock(c clock.Clock) *Module {
	m.service.WithClock(c)
	return m
}

// WithEvents connects the publisher of project lifecycle events.
func (m *Module) WithEvents(events port.EventPublisher) *Module {
	m.service.WithEvents(events)
	return m
}

// WithTransactions connects the transaction runner used by multi-write operations.
func (m *Module) WithTransactions(tx txRunner) *Module {
	m.service.WithTransactions(tx)
	return m
}

// txRunner matches database.TxManager without importing the platform package here.
type txRunner interface {
	InTx(ctx context.Context, fn func(ctx context.Context) error) error
}

// WithTeamGrants connects the repository of team roles on projects.
func (m *Module) WithTeamGrants(teamGrants port.TeamGrantRepository) *Module {
	m.service.WithTeamGrants(teamGrants)
	return m
}

// WithRecents connects the recent-views recorder called after a project read.
func (m *Module) WithRecents(recents port.RecentRecorder) *Module {
	m.service.WithRecents(recents)
	return m
}
