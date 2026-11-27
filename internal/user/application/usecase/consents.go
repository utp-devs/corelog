package usecase

import (
	"context"

	"github.com/jhonsferg/corelog/internal/user/application/port"
	"github.com/jhonsferg/corelog/internal/user/domain"
)

// WithConsents makes registration require the current legal documents. The consent rows are written in the
// same transaction as the account.
func (s *UserService) WithConsents(gate port.ConsentGate) *UserService {
	s.consents = gate
	return s
}

// validateConsents rejects a registration that does not cover every required legal document.
func (s *UserService) validateConsents(ctx context.Context, in port.RegisterInput) error {
	if s.consents == nil {
		return nil
	}
	return s.consents.Validate(ctx, in.Consents)
}

// saveWithConsents stores the new account and, when the legal gate is wired, its consents atomically.
func (s *UserService) saveWithConsents(ctx context.Context, user *domain.User, in port.RegisterInput) error {
	if s.consents == nil {
		return s.repo.Save(ctx, user)
	}
	run := func(ctx context.Context) error {
		if err := s.repo.Save(ctx, user); err != nil {
			return err
		}
		if len(in.Consents) == 0 {
			return nil
		}
		return s.consents.Record(ctx, user.ID, in.Consents, in.ConsentMeta)
	}
	if s.tx == nil {
		return run(ctx)
	}
	return s.tx.InTx(ctx, run)
}
