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

// UpdateProfileInput is a partial update: nil fields are left unchanged.
type UpdateProfileInput struct {
	Name        *string
	Email       *string
	DisplayName *string
	JobTitle    *string
	Department  *string
	Timezone    *string
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
	// Authenticate valida las credenciales y registra last_login_at en un inicio de sesion correcto.
	Authenticate(ctx context.Context, in AuthenticateInput) (*domain.User, error)
	GetByID(ctx context.Context, id uuid.UUID) (*domain.User, error)
	UpdateProfile(ctx context.Context, userID uuid.UUID, in UpdateProfileInput) (*domain.User, error)
	ChangePassword(ctx context.Context, userID uuid.UUID, in ChangePasswordInput) error
	SetTeam(ctx context.Context, userID uuid.UUID, teamID *uuid.UUID) (*domain.User, error)
	SearchTeammates(ctx context.Context, actorID uuid.UUID, query string) ([]*domain.User, error)
	// SearchInOrganization devuelve los miembros de la organizacion visibles para el actor.
	// Si el actor no es miembro devuelve domain.ErrNotOrganizationMember.
	SearchInOrganization(ctx context.Context, actorID, orgID uuid.UUID, query string) ([]*domain.User, error)
	Search(ctx context.Context, filter SearchFilter) ([]*domain.User, error)
}
