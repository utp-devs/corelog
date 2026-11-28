package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jmoiron/sqlx"

	"github.com/jhonsferg/corelog/internal/calendar/application/port"
	"github.com/jhonsferg/corelog/internal/calendar/domain"
	"github.com/jhonsferg/corelog/internal/platform/apperror"
)

var _ port.EventRepository = (*CalendarRepository)(nil)

// derivedCap is the most derived dates and events one query returns.
const derivedCap = 2000

// visibleProjectSQL is the browse rule of a project row aliased as alias: the platform administrator sees every
// live project; anyone else needs a live organization membership and then either an owner or admin role, an
// explicit project role, a team grant, or the member role on a project that is not private.
func visibleProjectSQL(alias, viewer, admin string) string {
	return "(" + admin + " OR EXISTS (SELECT 1 FROM organization_members vom" +
		" JOIN organizations vo ON vo.id = vom.organization_id AND vo.deleted_at IS NULL" +
		" WHERE vom.organization_id = " + alias + ".organization_id AND vom.user_id = " + viewer +
		" AND (vom.role IN ('owner', 'admin')" +
		" OR EXISTS (SELECT 1 FROM project_members vpm WHERE vpm.project_id = " + alias + ".id AND vpm.user_id = " + viewer + ")" +
		" OR EXISTS (SELECT 1 FROM project_team_roles vptr JOIN team_members vtm ON vtm.team_id = vptr.team_id" +
		" WHERE vptr.project_id = " + alias + ".id AND vtm.user_id = " + viewer + ")" +
		" OR (vom.role = 'member' AND " + alias + ".visibility <> 'private'))))"
}

// eventVisibleSQL is the visibility rule of an event row aliased e for viewer $1 and platform flag $2: the
// owner and the guests, project events for who can browse the project, organization events for the members
// (the platform administrator sees every non-private event).
var eventVisibleSQL = `(e.owner_id = $1::uuid
	OR EXISTS (SELECT 1 FROM calendar_event_attendees ea WHERE ea.event_id = e.id AND ea.user_id = $1::uuid)
	OR ($2::boolean AND e.visibility <> 'private')
	OR (e.visibility = 'project' AND e.project_id IS NOT NULL AND EXISTS (
	      SELECT 1 FROM projects sp WHERE sp.id = e.project_id AND sp.deleted_at IS NULL AND ` + visibleProjectSQL("sp", "$1::uuid", "FALSE") + `))
	OR (e.visibility = 'organization' AND EXISTS (
	      SELECT 1 FROM organization_members om JOIN organizations oo ON oo.id = om.organization_id AND oo.deleted_at IS NULL
	      WHERE om.organization_id = e.organization_id AND om.user_id = $1::uuid)))`

const eventColumns = `e.id, e.owner_id, e.organization_id, e.project_id, e.title, e.description, e.location, e.starts_at, e.ends_at,
	e.all_day, e.timezone, e.recurrence_rule, e.recurrence_exceptions, e.visibility, e.color, e.created_at, e.updated_at`

type storedEventRow struct {
	ID             uuid.UUID       `db:"id"`
	OwnerID        uuid.UUID       `db:"owner_id"`
	OrganizationID uuid.UUID       `db:"organization_id"`
	ProjectID      *uuid.UUID      `db:"project_id"`
	Title          string          `db:"title"`
	Description    string          `db:"description"`
	Location       string          `db:"location"`
	StartsAt       time.Time       `db:"starts_at"`
	EndsAt         time.Time       `db:"ends_at"`
	AllDay         bool            `db:"all_day"`
	Timezone       string          `db:"timezone"`
	RecurrenceRule *string         `db:"recurrence_rule"`
	Exceptions     json.RawMessage `db:"recurrence_exceptions"`
	Visibility     string          `db:"visibility"`
	Color          string          `db:"color"`
	CreatedAt      time.Time       `db:"created_at"`
	UpdatedAt      time.Time       `db:"updated_at"`
}

func (r storedEventRow) event() domain.Event {
	return domain.Event{
		ID: r.ID, OwnerID: r.OwnerID, OrganizationID: r.OrganizationID, ProjectID: r.ProjectID, Title: r.Title,
		Description: r.Description, Location: r.Location, StartsAt: r.StartsAt.UTC(), EndsAt: r.EndsAt.UTC(), AllDay: r.AllDay,
		Timezone: r.Timezone, RecurrenceRule: r.RecurrenceRule, Exceptions: decodeExceptions(r.Exceptions),
		Visibility: domain.Visibility(r.Visibility), Color: r.Color, CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt,
	}
}

type overrideJSON struct {
	Title       *string    `json:"title,omitempty"`
	Description *string    `json:"description,omitempty"`
	Location    *string    `json:"location,omitempty"`
	Color       *string    `json:"color,omitempty"`
	StartsAt    *time.Time `json:"starts_at,omitempty"`
	EndsAt      *time.Time `json:"ends_at,omitempty"`
}

type exceptionJSON struct {
	OriginalStart time.Time     `json:"original_start"`
	Cancelled     bool          `json:"cancelled"`
	Override      *overrideJSON `json:"override,omitempty"`
}

func encodeExceptions(list []domain.Exception) []byte {
	out := make([]exceptionJSON, 0, len(list))
	for _, x := range list {
		item := exceptionJSON{OriginalStart: x.OriginalStart.UTC(), Cancelled: x.Cancelled}
		if o := x.Override; o != nil {
			item.Override = &overrideJSON{Title: o.Title, Description: o.Description, Location: o.Location, Color: o.Color, StartsAt: o.StartsAt, EndsAt: o.EndsAt}
		}
		out = append(out, item)
	}
	raw, _ := json.Marshal(out)
	return raw
}

func decodeExceptions(raw json.RawMessage) []domain.Exception {
	var items []exceptionJSON
	if err := json.Unmarshal(raw, &items); err != nil {
		return nil
	}
	out := make([]domain.Exception, 0, len(items))
	for _, it := range items {
		x := domain.Exception{OriginalStart: it.OriginalStart.UTC(), Cancelled: it.Cancelled}
		if o := it.Override; o != nil {
			x.Override = &domain.Override{Title: o.Title, Description: o.Description, Location: o.Location, Color: o.Color, StartsAt: o.StartsAt, EndsAt: o.EndsAt}
		}
		out = append(out, x)
	}
	return out
}

func internal(msg string, err error) error { return apperror.Wrap(apperror.KindInternal, msg, err) }

// OrganizationFor resolves the organization of a new event.
func (r *CalendarRepository) OrganizationFor(ctx context.Context, userID, requested uuid.UUID) (uuid.UUID, error) {
	var req any
	if requested != uuid.Nil {
		req = requested
	}
	var id uuid.UUID
	err := r.db.GetContext(ctx, &id, `
		SELECT om.organization_id FROM organization_members om
		JOIN organizations o ON o.id = om.organization_id AND o.deleted_at IS NULL
		WHERE om.user_id = $1::uuid AND ($2::uuid IS NULL OR om.organization_id = $2::uuid)
		ORDER BY om.joined_at, om.organization_id LIMIT 1`, userID, req)
	if errors.Is(err, sql.ErrNoRows) {
		return uuid.Nil, apperror.WithCode(apperror.KindValidation, domain.CodeInvalidEvent, "no perteneces a ninguna organización")
	}
	if err != nil {
		return uuid.Nil, internal("failed to resolve the event organization", err)
	}
	return id, nil
}

// ProjectOrganization returns the organization of a live project.
func (r *CalendarRepository) ProjectOrganization(ctx context.Context, projectID uuid.UUID) (uuid.UUID, error) {
	var id uuid.UUID
	err := r.db.GetContext(ctx, &id, `SELECT organization_id FROM projects WHERE id = $1 AND deleted_at IS NULL`, projectID)
	if errors.Is(err, sql.ErrNoRows) {
		return uuid.Nil, apperror.WithCode(apperror.KindNotFound, "project.not_found", "proyecto no encontrado")
	}
	if err != nil {
		return uuid.Nil, internal("failed to read the project organization", err)
	}
	return id, nil
}

// OrgRole returns the organization role of the user, or "".
func (r *CalendarRepository) OrgRole(ctx context.Context, userID, organizationID uuid.UUID) (string, error) {
	var role string
	err := r.db.GetContext(ctx, &role, `
		SELECT om.role FROM organization_members om
		JOIN organizations o ON o.id = om.organization_id AND o.deleted_at IS NULL
		WHERE om.organization_id = $1 AND om.user_id = $2`, organizationID, userID)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	if err != nil {
		return "", internal("failed to read the organization role", err)
	}
	return role, nil
}

// MembersAmong returns the ids that belong to the organization.
func (r *CalendarRepository) MembersAmong(ctx context.Context, organizationID uuid.UUID, userIDs []uuid.UUID) ([]uuid.UUID, error) {
	var out []uuid.UUID
	err := r.db.SelectContext(ctx, &out, `
		SELECT om.user_id FROM organization_members om
		WHERE om.organization_id = $1 AND om.user_id = ANY($2::uuid[])`, organizationID, uuidArray(userIDs))
	if err != nil {
		return nil, internal("failed to check the attendees", err)
	}
	return out, nil
}

func uuidArray(ids []uuid.UUID) string {
	s := "{"
	for i, id := range ids {
		if i > 0 {
			s += ","
		}
		s += id.String()
	}
	return s + "}"
}

// Create stores the event and its attendees.
func (r *CalendarRepository) Create(ctx context.Context, e *domain.Event) error {
	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return internal("failed to begin the event transaction", err)
	}
	defer func() { _ = tx.Rollback() }()
	err = tx.GetContext(ctx, &e.ID, `
		INSERT INTO calendar_events (owner_id, organization_id, project_id, title, description, location, starts_at, ends_at, all_day,
			timezone, recurrence_rule, recurrence_exceptions, visibility, color, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12::jsonb, $13, $14, $15, $16) RETURNING id`,
		e.OwnerID, e.OrganizationID, e.ProjectID, e.Title, e.Description, e.Location, e.StartsAt, e.EndsAt, e.AllDay,
		e.Timezone, e.RecurrenceRule, encodeExceptions(e.Exceptions), string(e.Visibility), e.Color, e.CreatedAt, e.UpdatedAt)
	if err != nil {
		return mapEventError(err, "failed to create the event")
	}
	if err := insertAttendees(ctx, tx, e.ID, e.Attendees); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return internal("failed to commit the event", err)
	}
	return nil
}

func mapEventError(err error, msg string) error {
	var pg *pgconn.PgError
	if errors.As(err, &pg) && pg.Code == "23503" {
		return apperror.WithCode(apperror.KindValidation, domain.CodeInvalidEvent, "evento no válido: una referencia no existe")
	}
	return internal(msg, err)
}

func insertAttendees(ctx context.Context, tx *sqlx.Tx, eventID uuid.UUID, list []domain.Attendee) error {
	if len(list) == 0 {
		return nil
	}
	ids := make([]uuid.UUID, len(list))
	responses := make([]string, len(list))
	for i, a := range list {
		ids[i] = a.UserID
		responses[i] = string(a.Response)
	}
	_, err := tx.ExecContext(ctx, `
		INSERT INTO calendar_event_attendees (event_id, user_id, response)
		SELECT $1, u.id, u.response FROM unnest($2::uuid[], $3::text[]) AS u(id, response)`, eventID, uuidArray(ids), textArray(responses))
	if err != nil {
		return mapEventError(err, "failed to store the attendees")
	}
	return nil
}

func textArray(items []string) string {
	s := "{"
	for i, it := range items {
		if i > 0 {
			s += ","
		}
		s += `"` + it + `"`
	}
	return s + "}"
}

// Update replaces the event fields and, when asked, the guest list.
func (r *CalendarRepository) Update(ctx context.Context, e *domain.Event, replaceAttendees bool) error {
	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return internal("failed to begin the event transaction", err)
	}
	defer func() { _ = tx.Rollback() }()
	res, err := tx.ExecContext(ctx, `
		UPDATE calendar_events SET project_id = $2, title = $3, description = $4, location = $5, starts_at = $6, ends_at = $7,
			all_day = $8, timezone = $9, recurrence_rule = $10, recurrence_exceptions = $11::jsonb, visibility = $12, color = $13,
			updated_at = $14
		WHERE id = $1 AND deleted_at IS NULL`,
		e.ID, e.ProjectID, e.Title, e.Description, e.Location, e.StartsAt, e.EndsAt, e.AllDay, e.Timezone, e.RecurrenceRule,
		encodeExceptions(e.Exceptions), string(e.Visibility), e.Color, e.UpdatedAt)
	if err != nil {
		return mapEventError(err, "failed to update the event")
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return domain.ErrEventNotFound
	}
	if replaceAttendees {
		if _, err := tx.ExecContext(ctx, `DELETE FROM calendar_event_attendees WHERE event_id = $1`, e.ID); err != nil {
			return internal("failed to replace the attendees", err)
		}
		if err := insertAttendees(ctx, tx, e.ID, e.Attendees); err != nil {
			return err
		}
	}
	if err := tx.Commit(); err != nil {
		return internal("failed to commit the event", err)
	}
	return nil
}

// Delete soft-deletes an event.
func (r *CalendarRepository) Delete(ctx context.Context, id uuid.UUID, at time.Time) error {
	res, err := r.db.ExecContext(ctx, `UPDATE calendar_events SET deleted_at = $2, updated_at = $2 WHERE id = $1 AND deleted_at IS NULL`, id, at)
	if err != nil {
		return internal("failed to delete the event", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return domain.ErrEventNotFound
	}
	return nil
}

// FindVisible returns one event the viewer can see.
func (r *CalendarRepository) FindVisible(ctx context.Context, viewer port.Viewer, id uuid.UUID) (*domain.Event, error) {
	var row storedEventRow
	err := r.db.GetContext(ctx, &row, `SELECT `+eventColumns+` FROM calendar_events e
		WHERE e.id = $3 AND e.deleted_at IS NULL AND `+eventVisibleSQL, viewer.UserID, viewer.PlatformAdmin(), id)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, domain.ErrEventNotFound
	}
	if err != nil {
		return nil, internal("failed to read the event", err)
	}
	e := row.event()
	attendees, err := r.attendeesOf(ctx, []uuid.UUID{e.ID})
	if err != nil {
		return nil, err
	}
	e.Attendees = attendees[e.ID]
	return &e, nil
}

// ListVisible returns the visible events that may have an occurrence in the window, with their attendees.
func (r *CalendarRepository) ListVisible(ctx context.Context, viewer port.Viewer, from, to time.Time, projects []uuid.UUID) ([]domain.Event, error) {
	var filter any
	if projects != nil {
		filter = uuidArray(projects)
	}
	var rows []storedEventRow
	err := r.db.SelectContext(ctx, &rows, `SELECT `+eventColumns+` FROM calendar_events e
		WHERE e.deleted_at IS NULL AND e.starts_at < $4
		  AND (e.recurrence_rule IS NOT NULL OR e.ends_at > $3 OR (e.ends_at = e.starts_at AND e.starts_at >= $3))
		  AND ($5::uuid[] IS NULL OR e.project_id IS NULL OR e.project_id = ANY($5::uuid[]))
		  AND `+eventVisibleSQL+`
		ORDER BY e.starts_at, e.id LIMIT `+fmt.Sprint(derivedCap),
		viewer.UserID, viewer.PlatformAdmin(), from, to, filter)
	if err != nil {
		return nil, internal("failed to list the events", err)
	}
	events := make([]domain.Event, len(rows))
	ids := make([]uuid.UUID, len(rows))
	for i, row := range rows {
		events[i] = row.event()
		ids[i] = events[i].ID
	}
	attendees, err := r.attendeesOf(ctx, ids)
	if err != nil {
		return nil, err
	}
	for i := range events {
		events[i].Attendees = attendees[events[i].ID]
	}
	return events, nil
}

type attendeeRow struct {
	EventID  uuid.UUID `db:"event_id"`
	UserID   uuid.UUID `db:"user_id"`
	Name     string    `db:"name"`
	Response string    `db:"response"`
}

func (r *CalendarRepository) attendeesOf(ctx context.Context, eventIDs []uuid.UUID) (map[uuid.UUID][]domain.Attendee, error) {
	out := map[uuid.UUID][]domain.Attendee{}
	if len(eventIDs) == 0 {
		return out, nil
	}
	var rows []attendeeRow
	err := r.db.SelectContext(ctx, &rows, `
		SELECT a.event_id, a.user_id, u.display_name AS name, a.response
		FROM calendar_event_attendees a JOIN users u ON u.id = a.user_id
		WHERE a.event_id = ANY($1::uuid[]) ORDER BY a.event_id, lower(u.display_name), a.user_id`, uuidArray(eventIDs))
	if err != nil {
		return nil, internal("failed to read the attendees", err)
	}
	for _, row := range rows {
		out[row.EventID] = append(out[row.EventID], domain.Attendee{UserID: row.UserID, Name: row.Name, Response: domain.Response(row.Response)})
	}
	return out, nil
}

// SetResponse stores the answer of a guest.
func (r *CalendarRepository) SetResponse(ctx context.Context, eventID, userID uuid.UUID, response domain.Response) (bool, error) {
	res, err := r.db.ExecContext(ctx, `UPDATE calendar_event_attendees SET response = $3 WHERE event_id = $1 AND user_id = $2`, eventID, userID, string(response))
	if err != nil {
		return false, internal("failed to store the response", err)
	}
	n, _ := res.RowsAffected()
	return n > 0, nil
}

type derivedRow struct {
	Kind       string    `db:"kind"`
	SourceID   uuid.UUID `db:"source_id"`
	Title      string    `db:"title"`
	Day        time.Time `db:"day"`
	ProjectID  uuid.UUID `db:"project_id"`
	ProjectKey string    `db:"project_key"`
}

// Derived lists issue due and start dates, sprint boundaries and version releases of the visible projects.
// Without an explicit project list it covers the projects the viewer is a member of, directly or by team.
func (r *CalendarRepository) Derived(ctx context.Context, q port.DerivedQuery) ([]port.Event, error) {
	var projects any
	if q.Projects != nil {
		projects = uuidArray(q.Projects)
	}
	query := `
		WITH vis AS (
			SELECT p.id, p.key FROM projects p
			WHERE p.deleted_at IS NULL AND ` + visibleProjectSQL("p", "$1::uuid", "$2::boolean") + `
			  AND (CASE WHEN $5::uuid[] IS NOT NULL THEN p.id = ANY($5::uuid[])
			       ELSE (EXISTS (SELECT 1 FROM project_members pm WHERE pm.project_id = p.id AND pm.user_id = $1::uuid)
			          OR EXISTS (SELECT 1 FROM project_team_roles ptr JOIN team_members tm ON tm.team_id = ptr.team_id
			                     WHERE ptr.project_id = p.id AND tm.user_id = $1::uuid)) END)
		)
		SELECT * FROM (
			SELECT 'issue_due' AS kind, i.id AS source_id, i.summary AS title, i.due_date AS day, vis.id AS project_id, vis.key AS project_key
			FROM issues i JOIN vis ON vis.id = i.project_id
			WHERE 'issue_due' = ANY($6::text[]) AND i.deleted_at IS NULL AND i.due_date BETWEEN $3::date AND $4::date
			UNION ALL
			SELECT 'issue_start', i.id, i.summary, i.start_date, vis.id, vis.key
			FROM issues i JOIN vis ON vis.id = i.project_id
			WHERE 'issue_start' = ANY($6::text[]) AND i.deleted_at IS NULL AND i.start_date BETWEEN $3::date AND $4::date
			UNION ALL
			SELECT 'sprint_start', s.id, s.name, s.start_date, vis.id, vis.key
			FROM sprints s JOIN vis ON vis.id = s.project_id
			WHERE 'sprint_start' = ANY($6::text[]) AND s.deleted_at IS NULL AND s.start_date BETWEEN $3::date AND $4::date
			UNION ALL
			SELECT 'sprint_end', s.id, s.name, s.end_date, vis.id, vis.key
			FROM sprints s JOIN vis ON vis.id = s.project_id
			WHERE 'sprint_end' = ANY($6::text[]) AND s.deleted_at IS NULL AND s.end_date BETWEEN $3::date AND $4::date
			UNION ALL
			SELECT 'version_release', v.id, v.name, v.release_date, vis.id, vis.key
			FROM versions v JOIN vis ON vis.id = v.project_id
			WHERE 'version_release' = ANY($6::text[]) AND v.deleted_at IS NULL AND v.archived_at IS NULL AND v.release_date BETWEEN $3::date AND $4::date
		) d ORDER BY day, kind, title LIMIT ` + fmt.Sprint(derivedCap)
	var rows []derivedRow
	err := r.db.SelectContext(ctx, &rows, query, q.Viewer.UserID, q.Viewer.PlatformAdmin(),
		q.From.UTC().Format("2006-01-02"), q.To.UTC().Add(-time.Nanosecond).Format("2006-01-02"), projects, textArray(q.Kinds))
	if err != nil {
		return nil, internal("failed to read the derived calendar dates", err)
	}
	out := make([]port.Event, 0, len(rows))
	for _, row := range rows {
		out = append(out, port.Event{
			Kind: row.Kind, Date: row.Day, Title: row.Title, SourceID: row.SourceID, ProjectID: row.ProjectID,
			ProjectKey: row.ProjectKey, Link: derivedLink(row.Kind, row.ProjectID, row.SourceID),
		})
	}
	return out, nil
}

func derivedLink(kind string, projectID, sourceID uuid.UUID) string {
	switch kind {
	case port.KindIssueDue, port.KindIssueStart:
		return fmt.Sprintf("/issues/%s", sourceID)
	case port.KindVersionRelease:
		return fmt.Sprintf("/projects/%s/versions", projectID)
	}
	return fmt.Sprintf("/projects/%s/sprints", projectID)
}

// FeedInfo returns the creation time of the active feed token.
func (r *CalendarRepository) FeedInfo(ctx context.Context, userID uuid.UUID) (*time.Time, error) {
	var at time.Time
	err := r.db.GetContext(ctx, &at, `SELECT created_at FROM calendar_feed_tokens WHERE user_id = $1 AND revoked_at IS NULL`, userID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, internal("failed to read the feed token", err)
	}
	return &at, nil
}

// ReplaceFeedToken stores the hash of a new token, replacing any previous one.
func (r *CalendarRepository) ReplaceFeedToken(ctx context.Context, userID uuid.UUID, tokenHash string, at time.Time) error {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO calendar_feed_tokens (user_id, token_hash, created_at, revoked_at) VALUES ($1, $2, $3, NULL)
		ON CONFLICT (user_id) DO UPDATE SET token_hash = EXCLUDED.token_hash, created_at = EXCLUDED.created_at, revoked_at = NULL`,
		userID, tokenHash, at)
	if err != nil {
		return internal("failed to store the feed token", err)
	}
	return nil
}

// RevokeFeedToken disables the feed token.
func (r *CalendarRepository) RevokeFeedToken(ctx context.Context, userID uuid.UUID, at time.Time) (bool, error) {
	res, err := r.db.ExecContext(ctx, `UPDATE calendar_feed_tokens SET revoked_at = $2 WHERE user_id = $1 AND revoked_at IS NULL`, userID, at)
	if err != nil {
		return false, internal("failed to revoke the feed token", err)
	}
	n, _ := res.RowsAffected()
	return n > 0, nil
}

// UserByFeedToken returns the owner of an active token hash.
func (r *CalendarRepository) UserByFeedToken(ctx context.Context, tokenHash string) (uuid.UUID, error) {
	var id uuid.UUID
	err := r.db.GetContext(ctx, &id, `
		SELECT t.user_id FROM calendar_feed_tokens t JOIN users u ON u.id = t.user_id
		WHERE t.token_hash = $1 AND t.revoked_at IS NULL`, tokenHash)
	if errors.Is(err, sql.ErrNoRows) {
		return uuid.Nil, domain.ErrFeedNotFound
	}
	if err != nil {
		return uuid.Nil, internal("failed to read the feed token", err)
	}
	return id, nil
}
