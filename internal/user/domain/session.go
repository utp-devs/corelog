package domain

import (
	"time"

	"github.com/google/uuid"
)

// Session es una sesion de login. Su id viaja en el claim jti del token y permite revocarla.
type Session struct {
	ID         uuid.UUID
	UserID     uuid.UUID
	UserAgent  string
	CreatedAt  time.Time
	LastSeenAt time.Time
	ExpiresAt  time.Time
	RevokedAt  *time.Time
}

// IsActive indica si la sesion sigue valida en el instante dado.
func (s *Session) IsActive(now time.Time) bool {
	return s.RevokedAt == nil && now.Before(s.ExpiresAt)
}
