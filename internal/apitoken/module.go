// Package apitoken is the personal API token hexagon: creation (plaintext shown once), listing, revocation and
// bearer authentication with scopes.
package apitoken

import (
	"context"

	httpadapter "github.com/jhonsferg/corelog/internal/apitoken/adapter/in/http"
	"github.com/jhonsferg/corelog/internal/apitoken/application/port"
	"github.com/jhonsferg/corelog/internal/apitoken/application/usecase"
	"github.com/jhonsferg/corelog/internal/apitoken/domain"
	"github.com/jhonsferg/corelog/internal/platform/clock"
	"github.com/jhonsferg/corelog/internal/platform/database"
	"github.com/jhonsferg/corelog/internal/platform/events"
	appmw "github.com/jhonsferg/corelog/internal/platform/middleware"
)

// Module bundles the service and the HTTP handler.
type Module struct {
	Service port.Service
	Handler *httpadapter.Handler
}

// NewModule wires the token use cases. pub and limiter may be nil.
func NewModule(repo port.Repository, tx database.TxManager, pub events.Publisher, clk clock.Clock, limiter port.Limiter, jwtSecret string) *Module {
	service := usecase.NewService(repo, tx, pub, clk, limiter)
	return &Module{Service: service, Handler: httpadapter.NewHandler(service, jwtSecret)}
}

// RegisterAuthentication installs bearer token authentication in the middleware for the whole process.
func (m *Module) RegisterAuthentication() {
	appmw.SetAPITokenAuthenticator(func(ctx context.Context, plaintext string) (appmw.APITokenIdentity, error) {
		id, err := m.Service.Authenticate(ctx, plaintext)
		if err != nil {
			return appmw.APITokenIdentity{}, err
		}
		scopes := id.Scopes
		return appmw.APITokenIdentity{
			UserID: id.UserID, Role: id.Role, TokenID: id.TokenID,
			AllowsMethod: func(method string) bool { return domain.AllowsMethod(scopes, method) },
		}, nil
	}, domain.SessionOnlyPath)
}
