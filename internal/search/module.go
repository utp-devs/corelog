package search

import (
	"github.com/jhonsferg/corelog/internal/platform/clock"
	httpadapter "github.com/jhonsferg/corelog/internal/search/adapter/in/http"
	"github.com/jhonsferg/corelog/internal/search/application/port"
	"github.com/jhonsferg/corelog/internal/search/application/usecase"
)

type Module struct {
	Service port.Service
	Handler *httpadapter.Handler
	// Issues runs CoreQL queries; nil until WithIssueSearch is called.
	Issues port.IssueSearchService
	// Filters manages saved filters; nil until WithFilters is called.
	Filters port.FilterService
	// Global is the global search v2; nil until WithGlobal is called.
	Global port.GlobalService

	jql *usecase.JQLService
}

// NewModule conecta el indice de busqueda (SQL sobre proyectos e incidencias).
func NewModule(index port.Index, jwtSecret string) *Module {
	service := usecase.NewSearchService(index)
	return &Module{Service: service, Handler: httpadapter.NewHandler(service, jwtSecret)}
}

// IssueSearchDeps are the out-ports of the CoreQL issue search. Custom, Viewers and Suggest may be nil.
type IssueSearchDeps struct {
	Searcher port.IssueSearcher
	Custom   port.CustomFieldSource
	Viewers  port.ViewerDirectory
	Suggest  port.SuggestSource
	Clock    clock.Clock
}

// WithIssueSearch enables POST|GET /issues/search, POST /search/validate and GET /search/suggest.
func (m *Module) WithIssueSearch(deps IssueSearchDeps) *Module {
	m.jql = usecase.NewJQLService(deps.Searcher, deps.Custom, deps.Viewers, deps.Clock).WithSuggestions(deps.Suggest)
	m.Issues = m.jql
	m.Handler.WithIssueSearch(m.jql)
	return m
}

// WithFilters enables the saved filter endpoints. It needs WithIssueSearch first, which validates and runs the
// stored queries.
func (m *Module) WithFilters(repo port.FilterRepository, clk clock.Clock) *Module {
	if m.jql == nil {
		return m
	}
	m.Filters = usecase.NewFilterService(repo, m.jql, clk)
	m.Handler.WithFilters(m.Filters)
	return m
}

// WithGlobal enables the global search v2. notes may be nil.
func (m *Module) WithGlobal(index port.GlobalIndex, notes port.NoteSource) *Module {
	m.Global = usecase.NewGlobalService(index, notes)
	m.Handler.WithGlobal(m.Global)
	return m
}
