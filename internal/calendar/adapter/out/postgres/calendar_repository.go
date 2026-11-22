package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"

	"github.com/jhonsferg/corelog/internal/calendar/application/port"
	"github.com/jhonsferg/corelog/internal/platform/apperror"
)

// CalendarRepository lee hitos de sprints e incidencias. Es solo lectura: no guarda estado propio.
type CalendarRepository struct {
	db *sqlx.DB
}

func NewCalendarRepository(db *sqlx.DB) *CalendarRepository {
	return &CalendarRepository{db: db}
}

var _ port.Repository = (*CalendarRepository)(nil)

type eventRow struct {
	Kind     string    `db:"kind"`
	SourceID uuid.UUID `db:"source_id"`
	Title    string    `db:"title"`
	Day      time.Time `db:"day"`
}

// Events combina inicio y fin de sprint con las fechas limite de incidencias. Las fechas se comparan
// como date para que la zona horaria del servidor no mueva un evento de dia.
func (r *CalendarRepository) Events(ctx context.Context, projectID uuid.UUID, from, to time.Time) ([]port.Event, error) {
	const query = `
		SELECT 'sprint_start' AS kind, id AS source_id, name AS title, start_date AS day
		FROM sprints
		WHERE project_id = $1 AND start_date BETWEEN $2::date AND $3::date
		UNION ALL
		SELECT 'sprint_end', id, name, end_date
		FROM sprints
		WHERE project_id = $1 AND end_date BETWEEN $2::date AND $3::date
		UNION ALL
		SELECT 'issue_due', id, title, (due_date)::date
		FROM issues
		WHERE project_id = $1 AND (due_date)::date BETWEEN $2::date AND $3::date
		ORDER BY day, kind, title`

	var rows []eventRow
	if err := r.db.SelectContext(ctx, &rows, query, projectID, from.Format("2006-01-02"), to.Format("2006-01-02")); err != nil {
		return nil, apperror.Wrap(apperror.KindInternal, "failed to read calendar events", err)
	}
	events := make([]port.Event, 0, len(rows))
	for _, row := range rows {
		events = append(events, port.Event{
			Kind: row.Kind, Date: row.Day, Title: row.Title, SourceID: row.SourceID,
			Link: linkFor(row.Kind, projectID, row.SourceID),
		})
	}
	return events, nil
}

// linkFor devuelve la pantalla donde se ve el hito.
func linkFor(kind string, projectID, sourceID uuid.UUID) string {
	if kind == "issue_due" {
		return fmt.Sprintf("/issues/%s", sourceID)
	}
	return fmt.Sprintf("/projects/%s/sprints", projectID)
}
