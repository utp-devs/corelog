package port

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/jhonsferg/corelog/internal/user/domain"
)

type RegisterInput struct {
	Name     string
	Email    string
	Password string
	Role     domain.Role
}

type AuthenticateInput struct {
	Email    string
	Password string
}

type UpdateProfileInput struct {
	Name  string
	Email string
}

type ChangePasswordInput struct {
	CurrentPassword string
	NewPassword     string
}

type SearchFilter struct {
	Query  string
	TeamID *uuid.UUID
}

type SessionView struct {
	ID         string
	UserAgent  string
	CreatedAt  string
	LastSeenAt string
	Current    bool
}

type Service interface {
	// OpenSession registra una sesion nueva al iniciar sesion y devuelve su id.
	OpenSession(ctx context.Context, userID uuid.UUID, userAgent string, expiresAt time.Time) (uuid.UUID, error)
	ListSessions(ctx context.Context, userID, currentSessionID uuid.UUID) ([]SessionView, error)
	RevokeSession(ctx context.Context, userID, sessionID uuid.UUID) error
	RevokeOtherSessions(ctx context.Context, userID, currentSessionID uuid.UUID) error
	DeactivateAccount(ctx context.Context, userID uuid.UUID, password string) error
	Register(ctx context.Context, in RegisterInput) (*domain.User, error)
	Authenticate(ctx context.Context, in AuthenticateInput) (*domain.User, error)
	GetByID(ctx context.Context, id uuid.UUID) (*domain.User, error)
	UpdateProfile(ctx context.Context, userID uuid.UUID, in UpdateProfileInput) (*domain.User, error)
	ChangePassword(ctx context.Context, userID uuid.UUID, in ChangePasswordInput) error
	SetTeam(ctx context.Context, userID uuid.UUID, teamID *uuid.UUID) (*domain.User, error)
	SearchTeammates(ctx context.Context, actorID uuid.UUID, query string) ([]*domain.User, error)
	Search(ctx context.Context, filter SearchFilter) ([]*domain.User, error)
}
