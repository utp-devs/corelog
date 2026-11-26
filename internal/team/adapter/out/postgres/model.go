package postgres

import (
	"time"

	"github.com/google/uuid"

	"github.com/jhonsferg/corelog/internal/team/domain"
)

type teamRow struct {
	ID             uuid.UUID  `db:"id"`
	OrganizationID uuid.UUID  `db:"organization_id"`
	Name           string     `db:"name"`
	Description    string     `db:"description"`
	AvatarColor    *string    `db:"avatar_color"`
	CreatedBy      *uuid.UUID `db:"created_by"`
	CreatedAt      time.Time  `db:"created_at"`
	UpdatedAt      time.Time  `db:"updated_at"`
}

func (r teamRow) toDomain() *domain.Team {
	t := &domain.Team{
		ID:             r.ID,
		OrganizationID: r.OrganizationID,
		Name:           r.Name,
		Description:    r.Description,
		CreatedAt:      r.CreatedAt,
		UpdatedAt:      r.UpdatedAt,
	}
	if r.AvatarColor != nil {
		t.AvatarColor = *r.AvatarColor
	}
	if r.CreatedBy != nil {
		t.CreatedBy = *r.CreatedBy
	}
	return t
}

// teamColumns lists every selected column explicitly (no SELECT *).
const teamColumns = `id, organization_id, name, description, avatar_color, created_by, created_at, updated_at`

type memberRow struct {
	TeamID    uuid.UUID `db:"team_id"`
	UserID    uuid.UUID `db:"user_id"`
	Role      string    `db:"role"`
	CreatedAt time.Time `db:"created_at"`
}

func (r memberRow) toDomain() *domain.Member {
	return &domain.Member{
		TeamID:    r.TeamID,
		UserID:    r.UserID,
		Role:      r.Role,
		CreatedAt: r.CreatedAt,
	}
}
