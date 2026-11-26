package postgres

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jmoiron/sqlx"

	"github.com/jhonsferg/corelog/internal/platform/apperror"
	"github.com/jhonsferg/corelog/internal/platform/database"
	"github.com/jhonsferg/corelog/internal/team/application/port"
	"github.com/jhonsferg/corelog/internal/team/domain"
)

const uniqueViolation = "23505"

type TeamRepository struct {
	db *sqlx.DB
}

func NewTeamRepository(db *sqlx.DB) *TeamRepository {
	return &TeamRepository{db: db}
}

var _ port.Repository = (*TeamRepository)(nil)

func (r *TeamRepository) exec(ctx context.Context) sqlx.ExtContext {
	return database.Executor(ctx, r.db)
}

func nullableString(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func nullableUUID(id uuid.UUID) *uuid.UUID {
	if id == uuid.Nil {
		return nil
	}
	return &id
}

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == uniqueViolation
}

func (r *TeamRepository) Save(ctx context.Context, t *domain.Team) error {
	const query = `
		INSERT INTO teams (organization_id, name, description, avatar_color, created_by, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id`

	var id uuid.UUID
	err := sqlx.GetContext(ctx, r.exec(ctx), &id, query,
		t.OrganizationID, t.Name, t.Description, nullableString(t.AvatarColor),
		nullableUUID(t.CreatedBy), t.CreatedAt, t.UpdatedAt)
	if err != nil {
		if isUniqueViolation(err) {
			return domain.ErrTeamNameTaken
		}
		return apperror.Wrap(apperror.KindInternal, "failed to save team", err)
	}
	t.ID = id
	return nil
}

func (r *TeamRepository) FindByID(ctx context.Context, id, viewerID uuid.UUID) (*domain.Team, error) {
	var row teamRow
	query := `SELECT ` + teamViewColumns("$2") + ` FROM teams t WHERE t.id = $1`

	if err := sqlx.GetContext(ctx, r.exec(ctx), &row, query, id, viewerID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, domain.ErrTeamNotFound
		}
		return nil, apperror.Wrap(apperror.KindInternal, "failed to find team", err)
	}
	return row.toDomain(), nil
}

func (r *TeamRepository) FindByName(ctx context.Context, name string) (*domain.Team, error) {
	var row teamRow
	const query = `SELECT ` + teamColumns + ` FROM teams WHERE name = $1`

	if err := sqlx.GetContext(ctx, r.exec(ctx), &row, query, name); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, domain.ErrTeamNotFound
		}
		return nil, apperror.Wrap(apperror.KindInternal, "failed to find team", err)
	}
	return row.toDomain(), nil
}

func (r *TeamRepository) FindByOrganization(ctx context.Context, organizationID, viewerID uuid.UUID) ([]*domain.Team, error) {
	var rows []teamRow
	query := `SELECT ` + teamViewColumns("$2") + ` FROM teams t WHERE t.organization_id = $1 ORDER BY t.name ASC`

	if err := sqlx.SelectContext(ctx, r.exec(ctx), &rows, query, organizationID, viewerID); err != nil {
		return nil, apperror.Wrap(apperror.KindInternal, "failed to list teams", err)
	}
	teams := make([]*domain.Team, 0, len(rows))
	for _, row := range rows {
		teams = append(teams, row.toDomain())
	}
	return teams, nil
}

// FindByOrganizationAndMember returns the teams of the organization that userID belongs to, by name.
func (r *TeamRepository) FindByOrganizationAndMember(ctx context.Context, organizationID, memberID, viewerID uuid.UUID) ([]*domain.Team, error) {
	var rows []teamRow
	query := `SELECT ` + teamViewColumns("$3") + ` FROM teams t
		WHERE t.organization_id = $1
		  AND EXISTS (SELECT 1 FROM team_members tm WHERE tm.team_id = t.id AND tm.user_id = $2)
		ORDER BY t.name ASC`

	if err := sqlx.SelectContext(ctx, r.exec(ctx), &rows, query, organizationID, memberID, viewerID); err != nil {
		return nil, apperror.Wrap(apperror.KindInternal, "failed to list teams of member", err)
	}
	teams := make([]*domain.Team, 0, len(rows))
	for _, row := range rows {
		teams = append(teams, row.toDomain())
	}
	return teams, nil
}

func (r *TeamRepository) Update(ctx context.Context, t *domain.Team) error {
	const query = `
		UPDATE teams
		SET name = $3, description = $4, avatar_color = $5, updated_at = $6
		WHERE id = $1 AND organization_id = $2`

	result, err := r.exec(ctx).ExecContext(ctx, query,
		t.ID, t.OrganizationID, t.Name, t.Description, nullableString(t.AvatarColor), t.UpdatedAt)
	if err != nil {
		if isUniqueViolation(err) {
			return domain.ErrTeamNameTaken
		}
		return apperror.Wrap(apperror.KindInternal, "failed to update team", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return apperror.Wrap(apperror.KindInternal, "failed to update team", err)
	}
	if affected == 0 {
		return domain.ErrTeamNotFound
	}
	return nil
}

func (r *TeamRepository) Delete(ctx context.Context, organizationID, id uuid.UUID) error {
	const query = `DELETE FROM teams WHERE id = $1 AND organization_id = $2`

	result, err := r.exec(ctx).ExecContext(ctx, query, id, organizationID)
	if err != nil {
		return apperror.Wrap(apperror.KindInternal, "failed to delete team", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return apperror.Wrap(apperror.KindInternal, "failed to delete team", err)
	}
	if affected == 0 {
		return domain.ErrTeamNotFound
	}
	return nil
}

// FindMember resolves the membership only when the team belongs to organizationID.
func (r *TeamRepository) FindMember(ctx context.Context, organizationID, teamID, userID uuid.UUID) (*domain.Member, error) {
	var row memberRow
	const query = `
		SELECT tm.team_id, tm.user_id, tm.role, tm.created_at
		FROM team_members tm
		JOIN teams t ON t.id = tm.team_id
		WHERE tm.team_id = $1 AND tm.user_id = $2 AND t.organization_id = $3`

	if err := sqlx.GetContext(ctx, r.exec(ctx), &row, query, teamID, userID, organizationID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, domain.ErrMemberNotFound
		}
		return nil, apperror.Wrap(apperror.KindInternal, "failed to find team member", err)
	}
	return row.toDomain(), nil
}

func (r *TeamRepository) ListMembers(ctx context.Context, organizationID, teamID uuid.UUID) ([]*domain.Member, error) {
	var rows []memberRow
	const query = `
		SELECT tm.team_id, tm.user_id, tm.role, tm.created_at
		FROM team_members tm
		JOIN teams t ON t.id = tm.team_id
		WHERE tm.team_id = $1 AND t.organization_id = $2
		ORDER BY tm.created_at ASC, tm.user_id ASC`

	if err := sqlx.SelectContext(ctx, r.exec(ctx), &rows, query, teamID, organizationID); err != nil {
		return nil, apperror.Wrap(apperror.KindInternal, "failed to list team members", err)
	}
	members := make([]*domain.Member, 0, len(rows))
	for _, row := range rows {
		members = append(members, row.toDomain())
	}
	return members, nil
}

// AddMember inserts the membership only when the team belongs to organizationID.
func (r *TeamRepository) AddMember(ctx context.Context, organizationID uuid.UUID, m *domain.Member) error {
	const query = `
		INSERT INTO team_members (team_id, user_id, role)
		SELECT t.id, $2, $3 FROM teams t
		WHERE t.id = $1 AND t.organization_id = $4
		RETURNING created_at`

	var createdAt time.Time
	err := sqlx.GetContext(ctx, r.exec(ctx), &createdAt, query, m.TeamID, m.UserID, m.Role, organizationID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return domain.ErrTeamNotFound
		}
		if isUniqueViolation(err) {
			return domain.ErrMemberExists
		}
		return apperror.Wrap(apperror.KindInternal, "failed to add team member", err)
	}
	m.CreatedAt = createdAt
	return nil
}

func (r *TeamRepository) UpdateMemberRole(ctx context.Context, organizationID, teamID, userID uuid.UUID, role string) error {
	const query = `
		UPDATE team_members tm SET role = $3
		FROM teams t
		WHERE t.id = tm.team_id AND tm.team_id = $1 AND tm.user_id = $2 AND t.organization_id = $4`

	result, err := r.exec(ctx).ExecContext(ctx, query, teamID, userID, role, organizationID)
	if err != nil {
		return apperror.Wrap(apperror.KindInternal, "failed to update team member role", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return apperror.Wrap(apperror.KindInternal, "failed to update team member role", err)
	}
	if affected == 0 {
		return domain.ErrMemberNotFound
	}
	return nil
}

func (r *TeamRepository) RemoveMember(ctx context.Context, organizationID, teamID, userID uuid.UUID) error {
	const query = `
		DELETE FROM team_members tm
		USING teams t
		WHERE t.id = tm.team_id AND tm.team_id = $1 AND tm.user_id = $2 AND t.organization_id = $3`

	result, err := r.exec(ctx).ExecContext(ctx, query, teamID, userID, organizationID)
	if err != nil {
		return apperror.Wrap(apperror.KindInternal, "failed to remove team member", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return apperror.Wrap(apperror.KindInternal, "failed to remove team member", err)
	}
	if affected == 0 {
		return domain.ErrMemberNotFound
	}
	return nil
}
