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
	MemberCount    int        `db:"member_count"`
	MyRole         *string    `db:"my_role"`
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
	t.MemberCount = r.MemberCount
	if r.MyRole != nil {
		t.MyRole = *r.MyRole
	}
	return t
}

// teamColumns lists every selected column explicitly (no SELECT *).
const teamColumns = `id, organization_id, name, description, avatar_color, created_by, created_at, updated_at`

// teamViewColumns is teamColumns plus the read-side values: the member count of the team and the role of the
// viewer (bound to the placeholder viewerParam, for example "$2"). Both are scalar subqueries, so a list of
// teams costs one statement and no per-row round trip.
//
// SQL safety: viewerParam is only ever a positional placeholder literal ("$2" or "$3") written in the
// repository's own query constants. Caller input (ids, names, roles) is never concatenated; it is bound as
// an argument at execution time.
func teamViewColumns(viewerParam string) string {
	return `t.id, t.organization_id, t.name, t.description, t.avatar_color, t.created_by, t.created_at, t.updated_at,
	(SELECT COUNT(*) FROM team_members cm WHERE cm.team_id = t.id) AS member_count,
	(SELECT vm.role FROM team_members vm WHERE vm.team_id = t.id AND vm.user_id = ` + viewerParam + `) AS my_role`
}

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
