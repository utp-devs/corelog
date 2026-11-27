package user

import "github.com/jhonsferg/corelog/internal/user/application/port"

// WithConsents requires the current legal documents at registration and reports the pending ones at login
// and in the current user response.
func (m *Module) WithConsents(gate port.ConsentGate) *Module {
	m.users.WithConsents(gate)
	m.Handler.WithConsents(gate)
	return m
}
