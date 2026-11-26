// Package postgres implements the audit repository on PostgreSQL. The
// audit_events table is append-only: the database rejects UPDATE and TRUNCATE,
// and DELETE only runs inside PurgeOlderThan.
package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"

	"github.com/jhonsferg/corelog/internal/audit/application/port"
	"github.com/jhonsferg/corelog/internal/audit/domain"
)

// Repository stores audit entries in the audit_events table.
type Repository struct {
	db *sqlx.DB
}

var _ port.Repository = (*Repository)(nil)

// NewRepository returns a repository backed by db.
func NewRepository(db *sqlx.DB) *Repository {
	return &Repository{db: db}
}

const insertSQL = `
INSERT INTO audit_events (
    organization_id, project_id, actor_id, action, entity_type, entity_id, summary,
    before, after, ip, user_agent, source_event_id, created_at
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8::jsonb, $9::jsonb, $10::inet, $11, $12, $13)
ON CONFLICT (source_event_id) DO NOTHING`

// Save inserts the entry. A duplicate source_event_id is skipped silently.
func (r *Repository) Save(ctx context.Context, e domain.Entry) error {
	before, err := jsonArg(e.Before)
	if err != nil {
		return fmt.Errorf("audit: encode before: %w", err)
	}
	after, err := jsonArg(e.After)
	if err != nil {
		return fmt.Errorf("audit: encode after: %w", err)
	}
	var ip, userAgent any
	if e.IP != "" {
		ip = e.IP
	}
	if e.UserAgent != "" {
		userAgent = e.UserAgent
	}
	var sourceID any
	if e.SourceEventID != nil {
		sourceID = *e.SourceEventID
	}
	if _, err := r.db.ExecContext(ctx, insertSQL,
		nullUUID(e.OrganizationID), nullUUID(e.ProjectID), nullUUID(e.ActorID),
		e.Action, e.EntityType, nullUUID(e.EntityID), e.Summary,
		before, after, ip, userAgent, sourceID, e.CreatedAt,
	); err != nil {
		return fmt.Errorf("audit: insert entry: %w", err)
	}
	return nil
}

type entryRow struct {
	ID             int64          `db:"id"`
	OrganizationID uuid.NullUUID  `db:"organization_id"`
	ProjectID      uuid.NullUUID  `db:"project_id"`
	ActorID        uuid.NullUUID  `db:"actor_id"`
	Action         string         `db:"action"`
	EntityType     string         `db:"entity_type"`
	EntityID       uuid.NullUUID  `db:"entity_id"`
	Summary        string         `db:"summary"`
	Before         []byte         `db:"before"`
	After          []byte         `db:"after"`
	IP             sql.NullString `db:"ip"`
	UserAgent      sql.NullString `db:"user_agent"`
	SourceEventID  sql.NullInt64  `db:"source_event_id"`
	CreatedAt      time.Time      `db:"created_at"`
}

const selectColumns = `
SELECT id, organization_id, project_id, actor_id, action, entity_type, entity_id, summary,
       before, after, host(ip) AS ip, user_agent, source_event_id, created_at
FROM audit_events`

// List returns one page of entries matching f, newest first.
func (r *Repository) List(ctx context.Context, f port.ListFilter) (port.ListPage, error) {
	limit := f.Limit
	if limit <= 0 {
		limit = port.DefaultLimit
	}
	if limit > port.MaxLimit {
		limit = port.MaxLimit
	}

	where := []string{"TRUE"}
	var args []any
	add := func(cond string, value any) {
		args = append(args, value)
		where = append(where, fmt.Sprintf(cond, len(args)))
	}
	if f.OrganizationID != nil {
		add("organization_id = $%d", *f.OrganizationID)
	}
	if f.ActorID != nil {
		add("actor_id = $%d", *f.ActorID)
	}
	if f.Action != "" {
		add("action = $%d", f.Action)
	}
	if f.EntityType != "" {
		add("entity_type = $%d", f.EntityType)
	}
	if f.EntityID != nil {
		add("entity_id = $%d", *f.EntityID)
	}
	if f.Since != nil {
		add("created_at >= $%d", *f.Since)
	}
	if f.Until != nil {
		add("created_at < $%d", *f.Until)
	}
	if f.Cursor > 0 {
		add("id < $%d", f.Cursor)
	}
	args = append(args, limit+1)

	query := selectColumns + "\nWHERE " + joinAnd(where) +
		fmt.Sprintf("\nORDER BY id DESC\nLIMIT $%d", len(args))

	var rows []entryRow
	if err := r.db.SelectContext(ctx, &rows, query, args...); err != nil {
		return port.ListPage{}, fmt.Errorf("audit: list entries: %w", err)
	}

	page := port.ListPage{}
	if len(rows) > limit {
		rows = rows[:limit]
		page.NextCursor = rows[len(rows)-1].ID
	}
	page.Entries = make([]domain.Entry, 0, len(rows))
	for _, row := range rows {
		entry, err := row.toDomain()
		if err != nil {
			return port.ListPage{}, err
		}
		page.Entries = append(page.Entries, entry)
	}
	return page, nil
}

// PurgeOlderThan deletes entries created before cutoff. The retention setting
// is local to this transaction, so no other statement can delete rows.
func (r *Repository) PurgeOlderThan(ctx context.Context, cutoff time.Time) (int64, error) {
	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return 0, fmt.Errorf("audit: begin purge: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.ExecContext(ctx, `SELECT set_config('corelog.audit_retention', 'on', true)`); err != nil {
		return 0, fmt.Errorf("audit: enable retention: %w", err)
	}
	res, err := tx.ExecContext(ctx, `DELETE FROM audit_events WHERE created_at < $1`, cutoff)
	if err != nil {
		return 0, fmt.Errorf("audit: delete expired entries: %w", err)
	}
	deleted, err := res.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("audit: count deleted entries: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("audit: commit purge: %w", err)
	}
	return deleted, nil
}

func (row entryRow) toDomain() (domain.Entry, error) {
	entry := domain.Entry{
		ID:             row.ID,
		Action:         row.Action,
		EntityType:     row.EntityType,
		Summary:        row.Summary,
		IP:             row.IP.String,
		UserAgent:      row.UserAgent.String,
		CreatedAt:      row.CreatedAt,
		OrganizationID: nullUUIDPtr(row.OrganizationID),
		ProjectID:      nullUUIDPtr(row.ProjectID),
		ActorID:        nullUUIDPtr(row.ActorID),
		EntityID:       nullUUIDPtr(row.EntityID),
	}
	if row.SourceEventID.Valid {
		id := row.SourceEventID.Int64
		entry.SourceEventID = &id
	}
	var err error
	if entry.Before, err = decodeObject(row.Before); err != nil {
		return domain.Entry{}, fmt.Errorf("audit: decode before of entry %d: %w", row.ID, err)
	}
	if entry.After, err = decodeObject(row.After); err != nil {
		return domain.Entry{}, fmt.Errorf("audit: decode after of entry %d: %w", row.ID, err)
	}
	return entry, nil
}

func decodeObject(raw []byte) (map[string]any, error) {
	if len(raw) == 0 {
		return nil, nil
	}
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		return nil, err
	}
	return m, nil
}

func jsonArg(m map[string]any) (any, error) {
	if len(m) == 0 {
		return nil, nil
	}
	raw, err := json.Marshal(m)
	if err != nil {
		return nil, err
	}
	return string(raw), nil
}

func nullUUID(id *uuid.UUID) any {
	if id == nil {
		return nil
	}
	return *id
}

func nullUUIDPtr(n uuid.NullUUID) *uuid.UUID {
	if !n.Valid {
		return nil
	}
	id := n.UUID
	return &id
}

func joinAnd(parts []string) string {
	out := parts[0]
	for _, p := range parts[1:] {
		out += " AND " + p
	}
	return out
}
