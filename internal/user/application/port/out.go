package port

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/jhonsferg/corelog/internal/user/domain"
)

// OrganizationSearchFilter scopes a member search to one organization. ActorIsGuest
// restricts the result to members that share at least one project with the actor.
type OrganizationSearchFilter struct {
	OrganizationID uuid.UUID
	ActorID        uuid.UUID
	ActorIsGuest   bool
	Query          string
}

type Repository interface {
	Save(ctx context.Context, u *domain.User) error
	FindByID(ctx context.Context, id uuid.UUID) (*domain.User, error)
	FindByEmail(ctx context.Context, email string) (*domain.User, error)
	Update(ctx context.Context, u *domain.User) error
	// RecordLogin guarda last_login_at para el usuario.
	RecordLogin(ctx context.Context, userID uuid.UUID, at time.Time) error
	Search(ctx context.Context, filter SearchFilter) ([]*domain.User, error)
	// FindOrganizationRole devuelve el rol del usuario en la organizacion; found=false si no es miembro.
	FindOrganizationRole(ctx context.Context, orgID, userID uuid.UUID) (role string, found bool, err error)
	SearchOrganizationMembers(ctx context.Context, filter OrganizationSearchFilter) ([]*domain.User, error)
	Count(ctx context.Context) (int, error)
}

// SessionRepository persiste las sesiones de login. Todas las operaciones de escritura se limitan al usuario dueño.
type SessionRepository interface {
	Create(ctx context.Context, session *domain.Session) error
	ListActive(ctx context.Context, userID uuid.UUID, now time.Time) ([]*domain.Session, error)
	Revoke(ctx context.Context, userID, sessionID uuid.UUID, now time.Time) error
	RevokeExcept(ctx context.Context, userID uuid.UUID, keepID *uuid.UUID, now time.Time) error
	Touch(ctx context.Context, sessionID uuid.UUID, now time.Time) error
	// Validate confirma que la sesion sigue activa y la cuenta no esta desactivada.
	Validate(ctx context.Context, sessionID, userID uuid.UUID) error
}

// MembershipDetacher is the narrow out-port that account deactivation needs from the organization, team
// and project tables. It is implemented by the user postgres adapter, so the user hexagon never imports
// another hexagon. Both methods run inside the transaction opened by the use case.
type MembershipDetacher interface {
	// SoleOwnedOrganizations returns how many live organizations the user owns while being their only
	// owner. The owned organizations are locked until the transaction ends, so two owners of the same
	// organization cannot both leave it without an owner.
	SoleOwnedOrganizations(ctx context.Context, userID uuid.UUID) (int, error)
	// DetachMemberships removes the user from organization_members, team_members and project_members.
	DetachMemberships(ctx context.Context, userID uuid.UUID) error
}
