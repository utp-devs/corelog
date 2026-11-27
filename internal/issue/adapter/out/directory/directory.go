package directory

import (
	"context"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"

	"github.com/jhonsferg/corelog/internal/issue/application/port"
	"github.com/jhonsferg/corelog/internal/platform/apperror"
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

// SharesTeam indica si los dos usuarios comparten al menos un equipo, segun la tabla team_members.
func (d *UserDirectory) SharesTeam(ctx context.Context, userA, userB uuid.UUID) (bool, error) {
	var shared bool
	query := d.db.Rebind(`SELECT EXISTS (
		SELECT 1 FROM team_members a
		JOIN team_members b ON b.team_id = a.team_id
		WHERE a.user_id = ? AND b.user_id = ?
	)`)
	if err := d.db.GetContext(ctx, &shared, query, userA, userB); err != nil {
		return false, apperror.Wrap(apperror.KindInternal, "failed to check shared teams", err)
	}
	return shared, nil
}
