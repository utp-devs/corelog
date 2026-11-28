// Package webhook is the outgoing webhook hexagon: endpoint management, event capture from the domain events
// outbox, signed deliveries with retries and SSRF protection.
package webhook

import (
	"github.com/jhonsferg/corelog/internal/platform/crypto"
	httpadapter "github.com/jhonsferg/corelog/internal/webhook/adapter/in/http"
	"github.com/jhonsferg/corelog/internal/webhook/application/port"
	"github.com/jhonsferg/corelog/internal/webhook/application/usecase"
)

// Module bundles the service (which also provides the events consumer and the delivery worker) and the handler.
type Module struct {
	Service *usecase.Service
	Handler *httpadapter.Handler
}

// NewModule wires the use cases.
func NewModule(deps usecase.Deps, jwtSecret string) *Module {
	service := usecase.NewService(deps)
	return &Module{Service: service, Handler: httpadapter.NewHandler(service, jwtSecret)}
}

// CipherBox adapts a crypto.Cipher to port.SecretBox.
type CipherBox struct{ Cipher *crypto.Cipher }

var _ port.SecretBox = CipherBox{}

var secretAAD = []byte("corelog webhook secret")

// Seal encrypts a secret with AES-GCM.
func (b CipherBox) Seal(plain string) ([]byte, error) { return b.Cipher.Seal([]byte(plain), secretAAD) }

// Open decrypts a sealed secret.
func (b CipherBox) Open(sealed []byte) (string, error) {
	plain, err := b.Cipher.Open(sealed, secretAAD)
	return string(plain), err
}
