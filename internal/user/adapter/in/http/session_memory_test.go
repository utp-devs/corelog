package http_test

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"

	"github.com/jhonsferg/corelog/internal/user/application/port"
	"github.com/jhonsferg/corelog/internal/user/domain"
)

// memorySessionRepo es un repositorio de sesiones en memoria para las pruebas HTTP.
type memorySessionRepo struct {
	sessions map[uuid.UUID]*domain.Session
}

func newMemorySessionRepo() *memorySessionRepo {
	return &memorySessionRepo{sessions: make(map[uuid.UUID]*domain.Session)}
}

func (r *memorySessionRepo) Create(_ context.Context, s *domain.Session) error {
	copied := *s
	r.sessions[s.ID] = &copied
	return nil
}

func (r *memorySessionRepo) ListActive(_ context.Context, userID uuid.UUID, now time.Time) ([]*domain.Session, error) {
	var out []*domain.Session
	for _, s := range r.sessions {
		if s.UserID == userID && s.IsActive(now) {
			copied := *s
			out = append(out, &copied)
		}
	}
	return out, nil
}

func (r *memorySessionRepo) Revoke(_ context.Context, userID, sessionID uuid.UUID, now time.Time) error {
	s, ok := r.sessions[sessionID]
	if !ok || s.UserID != userID || s.RevokedAt != nil {
		return domain.ErrSessionNotFound
	}
	s.RevokedAt = &now
	return nil
}

func (r *memorySessionRepo) RevokeExcept(_ context.Context, userID uuid.UUID, keepID *uuid.UUID, now time.Time) error {
	for id, s := range r.sessions {
		if s.UserID != userID || s.RevokedAt != nil || (keepID != nil && id == *keepID) {
			continue
		}
		s.RevokedAt = &now
	}
	return nil
}

func (r *memorySessionRepo) Touch(_ context.Context, _ uuid.UUID, _ time.Time) error { return nil }

func (r *memorySessionRepo) Validate(_ context.Context, sessionID, userID uuid.UUID) error {
	s, ok := r.sessions[sessionID]
	if !ok || s.UserID != userID || !s.IsActive(time.Now()) {
		return errors.New("session is no longer valid")
	}
	return nil
}

var _ port.SessionRepository = (*memorySessionRepo)(nil)
