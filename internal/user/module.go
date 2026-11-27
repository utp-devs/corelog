package user

import (
	"time"

	"github.com/jhonsferg/corelog/internal/platform/clock"
	"github.com/jhonsferg/corelog/internal/platform/database"
	"github.com/jhonsferg/corelog/internal/platform/events"
	httpadapter "github.com/jhonsferg/corelog/internal/user/adapter/in/http"
	"github.com/jhonsferg/corelog/internal/user/application/port"
	"github.com/jhonsferg/corelog/internal/user/application/usecase"
)

type Module struct {
	Service port.Service
	Handler *httpadapter.Handler
}

// NewModule wires the user use cases. tx and detacher make account deactivation atomic and remove the
// closed account from organization, team and project memberships.
func NewModule(repo port.Repository, sessions port.SessionRepository, tx database.TxManager,
	detacher port.MembershipDetacher, jwtSecret string, jwtExpiration time.Duration) *Module {
	service := usecase.NewUserService(repo, sessions).WithAccountClosure(tx, detacher)
	handler := httpadapter.NewHandler(service, jwtSecret, jwtExpiration)

	return &Module{
		Service: service,
		Handler: handler,
	}
}

// NewTwoFactorService wires the TOTP second factor. The cipher encrypts the stored secrets; the publisher
// records enable, disable and recovery code events inside the transaction of each change.
func NewTwoFactorService(users port.Repository, repo port.TwoFactorRepository, cipher port.SecretCipher,
	tx database.TxManager, publisher events.Publisher, clk clock.Clock) port.TwoFactorService {
	return usecase.NewTwoFactorService(users, repo, cipher, tx, publisher, clk)
}
