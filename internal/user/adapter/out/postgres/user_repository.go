package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"

	"github.com/jhonsferg/corelog/internal/platform/apperror"
	"github.com/jhonsferg/corelog/internal/platform/database"
	"github.com/jhonsferg/corelog/internal/user/application/port"
	"github.com/jhonsferg/corelog/internal/user/domain"
)

// team_id se deriva de team_members (fuente de verdad, ADR-002). La columna users.team_id se eliminó en la migracion 000019.
// job_title and department are nullable in the schema; COALESCE maps NULL to an empty string for the domain.
const selectColumns = `id, name, email, password_hash, role, display_name, COALESCE(job_title, '') AS job_title, COALESCE(department, '') AS department, avatar_key, timezone, (SELECT tm.team_id FROM team_members tm WHERE tm.user_id = users.id ORDER BY tm.created_at LIMIT 1) AS team_id, created_at, updated_at, last_login_at, email_verified_at, deactivated_at`

type UserRepository struct {
	db *sqlx.DB
}

func NewUserRepository(db *sqlx.DB) *UserRepository {
	return &UserRepository{db: db}
}

var _ port.Repository = (*UserRepository)(nil)

func (r *UserRepository) Save(ctx context.Context, u *domain.User) error {
	row := fromDomain(u)

	const query = `
		INSERT INTO users (name, email, password_hash, role, display_name, job_title, department, avatar_key, timezone, last_login_at, email_verified_at, created_at, updated_at)
		VALUES (:name, :email, :password_hash, :role, :display_name, :job_title, :department, :avatar_key, :timezone, :last_login_at, :email_verified_at, :created_at, :updated_at)
		RETURNING id`

	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return apperror.Wrap(apperror.KindInternal, "failed to save user", err)
	}
	defer func() { _ = tx.Rollback() }()

	id, err := database.InsertReturningID(ctx, tx, query, row)
	if err != nil {
		return apperror.Wrap(apperror.KindInternal, "failed to save user", err)
	}
	u.ID = id
	if u.TeamID != nil {
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO team_members (team_id, user_id, role, created_at)
			VALUES ($1, $2, 'member', $3)
			ON CONFLICT (team_id, user_id) DO NOTHING`,
			u.TeamID, u.ID, u.CreatedAt,
		); err != nil {
			return apperror.Wrap(apperror.KindInternal, "failed to sync team membership", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return apperror.Wrap(apperror.KindInternal, "failed to save user", err)
	}
	return nil
}

func (r *UserRepository) FindByID(ctx context.Context, id uuid.UUID) (*domain.User, error) {
	var row userRow

	query := fmt.Sprintf(`SELECT %s FROM users WHERE id = $1`, selectColumns)

	if err := r.db.GetContext(ctx, &row, query, id); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, domain.ErrUserNotFound
		}
		return nil, apperror.Wrap(apperror.KindInternal, "failed to find user", err)
	}

	return row.toDomain(), nil
}

func (r *UserRepository) FindByEmail(ctx context.Context, email string) (*domain.User, error) {
	var row userRow

	query := fmt.Sprintf(`SELECT %s FROM users WHERE email = $1`, selectColumns)

	if err := r.db.GetContext(ctx, &row, query, email); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, domain.ErrUserNotFound
		}
		return nil, apperror.Wrap(apperror.KindInternal, "failed to find user", err)
	}

	return row.toDomain(), nil
}

// Update writes the profile and security columns. last_login_at is owned by RecordLogin
// and is deliberately not written here, so a stale in-memory user cannot roll it back.
func (r *UserRepository) Update(ctx context.Context, u *domain.User) error {
	row := fromDomain(u)

	const query = `
		UPDATE users
		SET name = :name, email = :email, password_hash = :password_hash,
		    display_name = :display_name, job_title = :job_title, department = :department,
		    avatar_key = :avatar_key, timezone = :timezone, email_verified_at = :email_verified_at,
		    updated_at = :updated_at, deactivated_at = :deactivated_at
		WHERE id = :id`

	result, err := sqlx.NamedExecContext(ctx, database.Executor(ctx, r.db), query, row)
	if err != nil {
		return apperror.Wrap(apperror.KindInternal, "failed to update user", err)
	}

	affected, err := result.RowsAffected()
	if err != nil {
		return apperror.Wrap(apperror.KindInternal, "failed to update user", err)
	}
	if affected == 0 {
		return domain.ErrUserNotFound
	}

	ex := database.Executor(ctx, r.db)
	if _, err := ex.ExecContext(ctx, `DELETE FROM team_members WHERE user_id = $1`, u.ID); err != nil {
		return apperror.Wrap(apperror.KindInternal, "failed to sync team membership", err)
	}
	if u.TeamID != nil {
		if _, err := ex.ExecContext(ctx, `
			INSERT INTO team_members (team_id, user_id, role, created_at)
			VALUES ($1, $2, 'member', $3)
			ON CONFLICT (team_id, user_id) DO NOTHING`,
			u.TeamID,
			u.ID,
			u.UpdatedAt,
		); err != nil {
			return apperror.Wrap(apperror.KindInternal, "failed to sync team membership", err)
		}
	}

	return nil
}

// RecordLogin stores last_login_at for the user.
func (r *UserRepository) RecordLogin(ctx context.Context, userID uuid.UUID, at time.Time) error {
	result, err := r.db.ExecContext(ctx, `UPDATE users SET last_login_at = $2 WHERE id = $1`, userID, at)
	if err != nil {
		return apperror.Wrap(apperror.KindInternal, "failed to record login", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return apperror.Wrap(apperror.KindInternal, "failed to record login", err)
	}
	if affected == 0 {
		return domain.ErrUserNotFound
	}
	return nil
}

func (r *UserRepository) Count(ctx context.Context) (int, error) {
	var count int
	if err := r.db.GetContext(ctx, &count, `SELECT COUNT(1) FROM users`); err != nil {
		return 0, apperror.Wrap(apperror.KindInternal, "failed to count users", err)
	}
	return count, nil
}

func (r *UserRepository) Search(ctx context.Context, filter port.SearchFilter) ([]*domain.User, error) {
	query := fmt.Sprintf(`SELECT %s FROM users WHERE deactivated_at IS NULL`, selectColumns)
	args := []any{}

	if filter.Query != "" {
		args = append(args, "%"+filter.Query+"%")
		query += fmt.Sprintf(" AND (name ILIKE $%d OR email ILIKE $%d)", len(args), len(args))
	}
	if filter.TeamID != nil {
		args = append(args, *filter.TeamID)
		query += fmt.Sprintf(" AND EXISTS (SELECT 1 FROM team_members tm WHERE tm.user_id = users.id AND tm.team_id = $%d)", len(args))
	}

	query += " ORDER BY name ASC LIMIT 20"

	return r.selectUsers(ctx, query, args, "failed to search users")
}

// FindOrganizationRole returns the role of the user in the organization. found is false when
// the user is not a member; the use case maps that case to 404.
func (r *UserRepository) FindOrganizationRole(ctx context.Context, orgID, userID uuid.UUID) (string, bool, error) {
	var role string
	err := r.db.GetContext(ctx, &role,
		`SELECT role FROM organization_members WHERE organization_id = $1 AND user_id = $2`,
		orgID, userID,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return "", false, nil
	}
	if err != nil {
		return "", false, apperror.Wrap(apperror.KindInternal, "failed to load organization membership", err)
	}
	return role, true, nil
}

// SearchOrganizationMembers returns live members of the organization. Guest actors only see
// members that are not guests or that share at least one project of this organization with them.
func (r *UserRepository) SearchOrganizationMembers(ctx context.Context, filter port.OrganizationSearchFilter) ([]*domain.User, error) {
	query := fmt.Sprintf(`SELECT %s FROM users
		WHERE deactivated_at IS NULL
		  AND users.id IN (SELECT om.user_id FROM organization_members om WHERE om.organization_id = $1)`, selectColumns)
	args := []any{filter.OrganizationID}

	if filter.ActorIsGuest {
		args = append(args, filter.ActorID)
		query += fmt.Sprintf(` AND (
		    users.id IN (SELECT om.user_id FROM organization_members om WHERE om.organization_id = $1 AND om.role <> 'guest')
		    OR users.id IN (
		        SELECT pb.user_id FROM project_members pb
		        JOIN project_members pa ON pa.project_id = pb.project_id AND pa.user_id = $%d
		        JOIN projects p ON p.id = pa.project_id AND p.organization_id = $1
		    )
		)`, len(args))
	}
	if filter.Query != "" {
		args = append(args, "%"+filter.Query+"%")
		n := len(args)
		query += fmt.Sprintf(` AND (users.name ILIKE $%d OR users.email ILIKE $%d OR users.display_name ILIKE $%d)`, n, n, n)
	}

	query += " ORDER BY users.name ASC LIMIT 20"

	return r.selectUsers(ctx, query, args, "failed to search organization members")
}

func (r *UserRepository) selectUsers(ctx context.Context, query string, args []any, failure string) ([]*domain.User, error) {
	var rows []userRow
	if err := r.db.SelectContext(ctx, &rows, query, args...); err != nil {
		return nil, apperror.Wrap(apperror.KindInternal, failure, err)
	}

	users := make([]*domain.User, 0, len(rows))
	for _, row := range rows {
		users = append(users, row.toDomain())
	}
	return users, nil
}
