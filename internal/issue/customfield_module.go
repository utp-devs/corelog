package issue

import (
	httpadapter "github.com/jhonsferg/corelog/internal/issue/adapter/in/http"
	"github.com/jhonsferg/corelog/internal/issue/application/port"
	"github.com/jhonsferg/corelog/internal/issue/application/usecase"
	"github.com/jhonsferg/corelog/internal/platform/clock"
	"github.com/jhonsferg/corelog/internal/platform/database"
)

// CustomFieldModule holds the custom field use cases and their HTTP handler.
type CustomFieldModule struct {
	Service port.CustomFieldService
	Handler *httpadapter.CustomFieldHandler
}

// WithCustomFields makes the issue use cases validate and return custom field values. Without it issues
// ignore the "custom" input.
func (m *Module) WithCustomFields(scope port.CustomFieldScopeReader, browse port.ProjectBrowseChecker) *Module {
	m.service.WithCustomFields(scope, browse)
	if required, ok := scope.(port.RequiredFieldReader); ok {
		m.service.WithRequiredFields(required)
	}
	return m
}

// NewCustomFieldModule wires the custom field service. store is the custom field repository, roles checks
// organization roles, authorizer checks project actions.
func NewCustomFieldModule(store port.CustomFieldStore, roles port.OrgRoleChecker, authorizer port.ProjectAuthorizer,
	tx database.TxManager, clk clock.Clock, jwtSecret string) *CustomFieldModule {
	service := usecase.NewCustomFieldService(store, roles, authorizer, tx, clk)
	return &CustomFieldModule{
		Service: service,
		Handler: httpadapter.NewCustomFieldHandler(service, jwtSecret),
	}
}
