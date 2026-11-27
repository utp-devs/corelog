package postgres

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"

	"github.com/jhonsferg/corelog/internal/platform/apperror"
	"github.com/jhonsferg/corelog/internal/platform/database"
	"github.com/jhonsferg/corelog/internal/project/application/port"
	"github.com/jhonsferg/corelog/internal/project/domain"
)

type teamGrantRow struct {
	ProjectID uuid.UUID `db:"project_id"`
	TeamID    uuid.UUID `db:"team_id"`
	TeamName  string    `db:"team_name"`
	Role      string    `db:"role"`
	CreatedAt time.Time `db:"created_at"`
}

type teamUserRow struct {
	UserID   uuid.UUID `db:"user_id"`
	TeamID   uuid.UUID `db:"team_id"`
	TeamName string    `db:"team_name"`
	Role     string    `db:"role"`
}

// TeamGrantRepository stores team roles of projects in project_team_roles.
type TeamGrantRepository struct {
	db *sqlx.DB
}

func NewTeamGrantRepository(db *sqlx.DB) *TeamGrantRepository {
	return &TeamGrantRepository{db: db}
}

var _ port.TeamGrantRepository = (*TeamGrantRepository)(nil)

func (r *TeamGrantRepository) ListTeamGrants(ctx context.Context, projectID uuid.UUID) ([]port.TeamGrant, error) {
	var rows []teamGrantRow
	if err := sqlx.SelectContext(ctx, database.Executor(ctx, r.db), &rows, `
		SELECT ptr.project_id, ptr.team_id, t.name AS team_name, ptr.role, ptr.created_at
		FROM project_team_roles ptr
		JOIN teams t ON t.id = ptr.team_id
		WHERE ptr.project_id = $1
		ORDER BY t.name ASC, ptr.team_id ASC`, projectID); err != nil {
		return nil, apperror.Wrap(apperror.KindInternal, "failed to list team roles", err)
	}
	out := make([]port.TeamGrant, 0, len(rows))
	for _, row := range rows {
		out = append(out, port.TeamGrant{
			ProjectID: row.ProjectID,
			TeamID:    row.TeamID,
			TeamName:  row.TeamName,
			Role:      row.Role,
			CreatedAt: row.CreatedAt,
		})
	}
	return out, nil
}

// ListTeamUsers joins the granted teams with their members. Only live members of the project
// organization count, the same rule the access lookup applies.
func (r *TeamGrantRepository) ListTeamUsers(ctx context.Context, projectID uuid.UUID) ([]port.TeamUserGrant, error) {
	var rows []teamUserRow
	if err := sqlx.SelectContext(ctx, database.Executor(ctx, r.db), &rows, `
		SELECT tm.user_id, ptr.team_id, t.name AS team_name, ptr.role
		FROM project_team_roles ptr
		JOIN projects p ON p.id = ptr.project_id
		JOIN teams t ON t.id = ptr.team_id
		JOIN team_members tm ON tm.team_id = ptr.team_id
		JOIN organization_members om ON om.organization_id = p.organization_id AND om.user_id = tm.user_id
		WHERE ptr.project_id = $1
		ORDER BY tm.user_id ASC, t.name ASC`, projectID); err != nil {
		return nil, apperror.Wrap(apperror.KindInternal, "failed to list team members", err)
	}
	out := make([]port.TeamUserGrant, 0, len(rows))
	for _, row := range rows {
		out = append(out, port.TeamUserGrant{UserID: row.UserID, TeamID: row.TeamID, TeamName: row.TeamName, Role: row.Role})
	}
	return out, nil
}

func (r *TeamGrantRepository) TeamInOrganization(ctx context.Context, orgID, teamID uuid.UUID) (string, bool, error) {
	var name string
	err := sqlx.GetContext(ctx, database.Executor(ctx, r.db), &name,
		`SELECT name FROM teams WHERE id = $1 AND organization_id = $2`, teamID, orgID)
	if errors.Is(err, sql.ErrNoRows) {
		return "", false, nil
	}
	if err != nil {
		return "", false, apperror.Wrap(apperror.KindInternal, "failed to check team organization", err)
	}
	return name, true, nil
}

func (r *TeamGrantRepository) UpsertTeamGrant(ctx context.Context, grant port.TeamGrant) error {
	if _, err := database.Executor(ctx, r.db).ExecContext(ctx, `
		INSERT INTO project_team_roles (project_id, team_id, role, created_at)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (project_id, team_id) DO UPDATE SET role = EXCLUDED.role`,
		grant.ProjectID, grant.TeamID, grant.Role, grant.CreatedAt); err != nil {
		if isForeignKeyViolation(err) {
			return domain.ErrTeamNotInOrg
		}
		return apperror.Wrap(apperror.KindInternal, "failed to save team role", err)
	}
	return nil
}

func (r *TeamGrantRepository) RemoveTeamGrant(ctx context.Context, projectID, teamID uuid.UUID) error {
	result, err := database.Executor(ctx, r.db).ExecContext(ctx, `
		DELETE FROM project_team_roles WHERE project_id = $1 AND team_id = $2`, projectID, teamID)
	if err != nil {
		return apperror.Wrap(apperror.KindInternal, "failed to remove team role", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return apperror.Wrap(apperror.KindInternal, "failed to remove team role", err)
	}
	if affected == 0 {
		return domain.ErrTeamGrantNotFound
	}
	return nil
}
