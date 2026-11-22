package postgres

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"

	"github.com/jhonsferg/corelog/internal/platform/apperror"
	"github.com/jhonsferg/corelog/internal/project/application/port"
)

type memberRow struct {
	ProjectID uuid.UUID `db:"project_id"`
	UserID    uuid.UUID `db:"user_id"`
	Role      string    `db:"role"`
	CreatedAt time.Time `db:"created_at"`
}

type MemberRepository struct {
	db *sqlx.DB
}

func NewMemberRepository(db *sqlx.DB) *MemberRepository {
	return &MemberRepository{db: db}
}

var _ port.MemberRepository = (*MemberRepository)(nil)

func (r *MemberRepository) FindRole(ctx context.Context, projectID, userID uuid.UUID) (*string, error) {
	var role string
	err := r.db.GetContext(ctx, &role, `
		SELECT role FROM project_members
		WHERE project_id = $1 AND user_id = $2`,
		projectID,
		userID,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, apperror.Wrap(apperror.KindInternal, "failed to find project member", err)
	}
	return &role, nil
}

func (r *MemberRepository) List(ctx context.Context, projectID uuid.UUID) ([]port.Member, error) {
	var rows []memberRow
	err := r.db.SelectContext(ctx, &rows, `
		SELECT project_id, user_id, role, created_at
		FROM project_members
		WHERE project_id = $1
		ORDER BY created_at ASC`,
		projectID,
	)
	if err != nil {
		return nil, apperror.Wrap(apperror.KindInternal, "failed to list project members", err)
	}

	members := make([]port.Member, 0, len(rows))
	for _, row := range rows {
		members = append(members, port.Member{
			ProjectID: row.ProjectID,
			UserID:    row.UserID,
			Role:      row.Role,
			CreatedAt: row.CreatedAt,
		})
	}
	return members, nil
}

func (r *MemberRepository) Add(ctx context.Context, member port.Member) error {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO project_members (project_id, user_id, role, created_at)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (project_id, user_id)
		DO UPDATE SET role = EXCLUDED.role`,
		member.ProjectID,
		member.UserID,
		member.Role,
		member.CreatedAt,
	)
	if err != nil {
		return apperror.Wrap(apperror.KindInternal, "failed to add project member", err)
	}
	return nil
}

func (r *MemberRepository) UpdateRole(ctx context.Context, projectID, userID uuid.UUID, role string) error {
	result, err := r.db.ExecContext(ctx, `
		UPDATE project_members
		SET role = $1
		WHERE project_id = $2 AND user_id = $3`,
		role,
		projectID,
		userID,
	)
	if err != nil {
		return apperror.Wrap(apperror.KindInternal, "failed to update project member", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return apperror.Wrap(apperror.KindInternal, "failed to update project member", err)
	}
	if affected == 0 {
		return apperror.New(apperror.KindNotFound, "project member not found")
	}
	return nil
}

func (r *MemberRepository) Remove(ctx context.Context, projectID, userID uuid.UUID) error {
	result, err := r.db.ExecContext(ctx, `
		DELETE FROM project_members
		WHERE project_id = $1 AND user_id = $2`,
		projectID,
		userID,
	)
	if err != nil {
		return apperror.Wrap(apperror.KindInternal, "failed to remove project member", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return apperror.Wrap(apperror.KindInternal, "failed to remove project member", err)
	}
	if affected == 0 {
		return apperror.New(apperror.KindNotFound, "project member not found")
	}
	return nil
}
