package directory

import (
	"context"
	"database/sql"
	"errors"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"

	"github.com/jhonsferg/corelog/internal/platform/apperror"
	"github.com/jhonsferg/corelog/internal/ticket/application/port"
)

type UserDirectory struct {
	db *sqlx.DB
}

func NewUserDirectory(db *sqlx.DB) *UserDirectory {
	return &UserDirectory{db: db}
}

var _ port.UserDirectory = (*UserDirectory)(nil)

// NamesOf devuelve el nombre de los usuarios indicados. Los ids que no existen no aparecen en el mapa.
func (d *UserDirectory) NamesOf(ctx context.Context, userIDs []uuid.UUID) (map[uuid.UUID]string, error) {
	names := make(map[uuid.UUID]string, len(userIDs))
	if len(userIDs) == 0 {
		return names, nil
	}
	query, args, err := sqlx.In(`SELECT id, name FROM users WHERE id IN (?)`, userIDs)
	if err != nil {
		return nil, apperror.Wrap(apperror.KindInternal, "failed to build user names query", err)
	}
	var rows []struct {
		ID   uuid.UUID `db:"id"`
		Name string    `db:"name"`
	}
	if err := d.db.SelectContext(ctx, &rows, d.db.Rebind(query), args...); err != nil {
		return nil, apperror.Wrap(apperror.KindInternal, "failed to load user names", err)
	}
	for _, row := range rows {
		names[row.ID] = row.Name
	}
	return names, nil
}

func (d *UserDirectory) TeamOf(ctx context.Context, userID uuid.UUID) (*uuid.UUID, error) {
	query := d.db.Rebind(`SELECT team_id FROM users WHERE id = ?`)

	var teamID sql.NullString
	if err := d.db.GetContext(ctx, &teamID, query, userID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, apperror.New(apperror.KindValidation, "user does not exist")
		}
		return nil, apperror.Wrap(apperror.KindInternal, "failed to look up user team", err)
	}

	if !teamID.Valid {
		return nil, nil
	}

	parsed, err := uuid.Parse(teamID.String)
	if err != nil {
		return nil, apperror.Wrap(apperror.KindInternal, "failed to decode team id", err)
	}

	return &parsed, nil
}
