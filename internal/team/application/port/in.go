package port

import (
	"context"

	"github.com/google/uuid"

	"github.com/jhonsferg/corelog/internal/team/domain"
)

// CreateInput is the request to create a team inside an organization.
type CreateInput struct {
	OrganizationID uuid.UUID
	Name           string
	Description    string
	AvatarColor    string
}

// Service is the team use-case surface. actorID is the authenticated user; every
// operation checks organization membership and team roles for that actor.
type Service interface {
	Create(ctx context.Context, actorID uuid.UUID, in CreateInput) (*domain.Team, error)
	Get(ctx context.Context, actorID, id uuid.UUID) (*domain.Team, error)
	List(ctx context.Context, actorID, organizationID uuid.UUID) ([]*domain.Team, error)
	Update(ctx context.Context, actorID, id uuid.UUID, patch domain.TeamPatch) (*domain.Team, error)
	Delete(ctx context.Context, actorID, id uuid.UUID) error
	ListMembers(ctx context.Context, actorID, teamID uuid.UUID) ([]*domain.Member, error)
	AddMember(ctx context.Context, actorID, teamID, userID uuid.UUID, role string) (*domain.Member, error)
	UpdateMemberRole(ctx context.Context, actorID, teamID, userID uuid.UUID, role string) (*domain.Member, error)
	RemoveMember(ctx context.Context, actorID, teamID, userID uuid.UUID) error
}
