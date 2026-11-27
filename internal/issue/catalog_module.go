package issue

import (
	httpadapter "github.com/jhonsferg/corelog/internal/issue/adapter/in/http"
	"github.com/jhonsferg/corelog/internal/issue/application/port"
	"github.com/jhonsferg/corelog/internal/issue/application/usecase"
	"github.com/jhonsferg/corelog/internal/platform/clock"
	"github.com/jhonsferg/corelog/internal/platform/database"
)

// CatalogModule holds the catalog use cases and their HTTP handler.
type CatalogModule struct {
	Service port.CatalogService
	Handler *httpadapter.CatalogHandler
}

// NewCatalogModule wires the catalog service. store is the catalog repository, roles checks organization
// roles, authorizer checks project actions.
func NewCatalogModule(store port.CatalogStore, roles port.OrgRoleChecker, authorizer port.ProjectAuthorizer,
	tx database.TxManager, clk clock.Clock, jwtSecret string) *CatalogModule {
	service := usecase.NewCatalogService(store, roles, authorizer, tx, clk)
	return &CatalogModule{
		Service: service,
		Handler: httpadapter.NewCatalogHandler(service, jwtSecret),
	}
}

// WithCatalog makes the issue use cases resolve types, priorities and resolutions through the catalogs.
// Without it issues keep the legacy enum values only.
func (m *Module) WithCatalog(catalog port.IssueCatalog) *Module {
	m.service.WithCatalog(catalog)
	return m
}
