package postgres

import (
	"context"
	"database/sql"
	"errors"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"

	"github.com/jhonsferg/corelog/internal/authorization/application/port"
	"github.com/jhonsferg/corelog/internal/authorization/domain"
	"github.com/jhonsferg/corelog/internal/platform/apperror"
)

// RoleLookup resuelve el rol de un usuario en un proyecto desde project_members.
type RoleLookup struct {
	db *sqlx.DB
}

func NewRoleLookup(db *sqlx.DB) *RoleLookup {
	return &RoleLookup{db: db}
}

var _ port.RoleLookup = (*RoleLookup)(nil)

// FindRole devuelve el rol del usuario en el proyecto, o nil si no es miembro.
func (r *RoleLookup) FindRole(ctx context.Context, projectID, userID uuid.UUID) (*domain.ProjectRole, error) {
	var role string
	err := r.db.GetContext(ctx, &role,
		`SELECT role FROM project_members WHERE project_id = $1 AND user_id = $2`,
		projectID, userID,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, apperror.Wrap(apperror.KindInternal, "failed to look up project role", err)
	}
	value := domain.ProjectRole(role)
	return &value, nil
}
