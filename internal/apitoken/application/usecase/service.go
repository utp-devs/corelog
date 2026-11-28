// Package usecase implements the API token service.
package usecase

import (
	"context"
	"crypto/subtle"
	"time"

	"github.com/google/uuid"

	"github.com/jhonsferg/corelog/internal/apitoken/application/port"
	"github.com/jhonsferg/corelog/internal/apitoken/domain"
	"github.com/jhonsferg/corelog/internal/platform/apperror"
	"github.com/jhonsferg/corelog/internal/platform/clock"
	"github.com/jhonsferg/corelog/internal/platform/database"
	"github.com/jhonsferg/corelog/internal/platform/events"
)

// touchInterval throttles last_used_at writes.
const touchInterval = time.Minute

// Service implements port.Service.
type Service struct {
	repo    port.Repository
	tx      database.TxManager
	pub     events.Publisher
	clk     clock.Clock
	limiter port.Limiter
}

var _ port.Service = (*Service)(nil)

// NewService wires the service. pub and limiter may be nil.
func NewService(repo port.Repository, tx database.TxManager, pub events.Publisher, clk clock.Clock, limiter port.Limiter) *Service {
	return &Service{repo: repo, tx: tx, pub: pub, clk: clk, limiter: limiter}
}

// Create stores a new token and returns its plaintext once.
func (s *Service) Create(ctx context.Context, userID uuid.UUID, in port.CreateInput) (domain.Token, string, error) {
	name, err := domain.ValidateName(in.Name)
	if err != nil {
		return domain.Token{}, "", err
	}
	scopes, err := domain.ParseScopes(in.Scopes)
	if err != nil {
		return domain.Token{}, "", err
	}
	now := s.clk.Now()
	if err := domain.ValidateExpiry(in.ExpiresAt, now); err != nil {
		return domain.Token{}, "", err
	}
	tok := domain.Token{UserID: userID, Name: name, Scopes: scopes, ExpiresAt: in.ExpiresAt, CreatedAt: now}
	if tok.Has(domain.ScopeAdmin) && in.GlobalRole != "admin" {
		return domain.Token{}, "", domain.ErrAdminScope
	}
	plaintext, err := domain.Generate()
	if err != nil {
		return domain.Token{}, "", apperror.Wrap(apperror.KindInternal, "no se pudo generar el token", err)
	}
	tok.Prefix = domain.DisplayPrefix(plaintext)
	err = s.tx.InTx(ctx, func(ctx context.Context) error {
		active, err := s.repo.CountActive(ctx, userID, now)
		if err != nil {
			return err
		}
		if active >= domain.MaxActivePerUser {
			return domain.ErrTooMany
		}
		if err := s.repo.Insert(ctx, &tok, domain.Hash(plaintext)); err != nil {
			return err
		}
		return s.publish(ctx, "auth.token_created", userID, tok, now)
	})
	if err != nil {
		return domain.Token{}, "", err
	}
	return tok, plaintext, nil
}

// publish writes the audit event. The payload never carries the token itself.
func (s *Service) publish(ctx context.Context, kind string, userID uuid.UUID, t domain.Token, at time.Time) error {
	if s.pub == nil {
		return nil
	}
	scopes := make([]string, len(t.Scopes))
	for i, sc := range t.Scopes {
		scopes[i] = string(sc)
	}
	id := t.ID
	return s.pub.Publish(ctx, events.Event{
		ActorID: &userID, EntityID: &id, Type: kind, EntityType: "api_token", CreatedAt: at,
		Payload: map[string]any{"token_id": t.ID.String(), "name": t.Name, "prefix": t.Prefix, "scopes": scopes},
	})
}

// List returns the tokens of the user, newest first.
func (s *Service) List(ctx context.Context, userID uuid.UUID, includeRevoked bool) ([]domain.Token, error) {
	return s.repo.List(ctx, userID, includeRevoked)
}

// Revoke revokes a token of the user.
func (s *Service) Revoke(ctx context.Context, userID, id uuid.UUID) error {
	now := s.clk.Now()
	return s.tx.InTx(ctx, func(ctx context.Context) error {
		t, err := s.repo.Revoke(ctx, userID, id, now)
		if err != nil {
			return err
		}
		return s.publish(ctx, "auth.token_revoked", userID, t, now)
	})
}

// Authenticate resolves a bearer plaintext to an identity.
func (s *Service) Authenticate(ctx context.Context, plaintext string) (port.Identity, error) {
	if !domain.WellFormed(plaintext) {
		return port.Identity{}, domain.ErrInvalid
	}
	hash := domain.Hash(plaintext)
	tok, owner, storedHash, err := s.repo.FindByHash(ctx, hash)
	if err != nil {
		return port.Identity{}, domain.ErrInvalid
	}
	// The lookup is by hash already; the explicit constant-time compare keeps the check independent of how the
	// repository matched the row.
	if subtle.ConstantTimeCompare([]byte(hash), []byte(storedHash)) != 1 {
		return port.Identity{}, domain.ErrInvalid
	}
	now := s.clk.Now()
	if !tok.Active(now) || owner.Deactivated {
		return port.Identity{}, domain.ErrInvalid
	}
	if s.limiter != nil {
		if ok, _ := s.limiter.Allow("token:" + tok.ID.String()); !ok {
			return port.Identity{}, domain.ErrRateLimited
		}
	}
	_ = s.repo.Touch(ctx, tok.ID, now, now.Add(-touchInterval))
	role := "user"
	if tok.Has(domain.ScopeAdmin) && owner.Role == "admin" {
		role = "admin"
	}
	return port.Identity{UserID: tok.UserID, Role: role, Scopes: tok.Scopes, TokenID: tok.ID}, nil
}
