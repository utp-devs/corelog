package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jmoiron/sqlx"

	"github.com/jhonsferg/corelog/internal/platform/apperror"
	"github.com/jhonsferg/corelog/internal/search/application/port"
	"github.com/jhonsferg/corelog/internal/search/domain"
)

// DefaultStatementTimeout bounds the execution of a user query.
const DefaultStatementTimeout = 5 * time.Second

// IssueSearchRepository runs CoreQL queries over issues. It implements the searcher, the custom field source
// and the viewer directory of the search use cases.
type IssueSearchRepository struct {
	db       *sqlx.DB
	versions VersionResolver
	timeout  time.Duration
}

var (
	_ port.IssueSearcher     = (*IssueSearchRepository)(nil)
	_ port.CustomFieldSource = (*IssueSearchRepository)(nil)
	_ port.ViewerDirectory   = (*IssueSearchRepository)(nil)
	_ port.SuggestSource     = (*IssueSearchRepository)(nil)
)

// NewIssueSearchRepository builds the repository with the default timeout and no version support.
func NewIssueSearchRepository(db *sqlx.DB) *IssueSearchRepository {
	return &IssueSearchRepository{db: db, timeout: DefaultStatementTimeout}
}

// WithVersions enables the version fields through the given resolver.
func (r *IssueSearchRepository) WithVersions(v VersionResolver) *IssueSearchRepository {
	r.versions = v
	return r
}

// WithTimeout sets the statement timeout of user queries.
func (r *IssueSearchRepository) WithTimeout(d time.Duration) *IssueSearchRepository {
	r.timeout = d
	return r
}

// readTx runs fn in a read-only transaction with a statement timeout. A cancelled statement is reported as the
// stable query timeout error.
func (r *IssueSearchRepository) readTx(ctx context.Context, fn func(tx *sqlx.Tx) error) error {
	tx, err := r.db.BeginTxx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return apperror.Wrap(apperror.KindInternal, "failed to start the search", err)
	}
	defer func() { _ = tx.Rollback() }()
	ms := r.timeout.Milliseconds()
	if ms < 1 {
		ms = 1
	}
	if _, err := tx.ExecContext(ctx, `SELECT set_config('statement_timeout', $1, true)`, strconv.FormatInt(ms, 10)); err != nil {
		return mapQueryError(err)
	}
	if err := fn(tx); err != nil {
		return mapQueryError(err)
	}
	return nil
}

// mapQueryError maps a cancelled or timed out statement to the stable timeout error.
func mapQueryError(err error) error {
	var appErr *apperror.Error
	if errors.As(err, &appErr) {
		return err
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "57014" {
		return domain.ErrQueryTimeout
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return domain.ErrQueryTimeout
	}
	if errors.Is(err, context.Canceled) {
		return err
	}
	return apperror.Wrap(apperror.KindInternal, "failed to run the search", err)
}

// SearchIssues builds and runs the query.
func (r *IssueSearchRepository) SearchIssues(ctx context.Context, q port.IssueQuery) (port.IssuePage, error) {
	built, err := BuildIssueQuery(q, r.versions)
	if err != nil {
		return port.IssuePage{}, err
	}
	if q.DryRun {
		return port.IssuePage{Items: []port.IssueRecord{}}, nil
	}
	page := port.IssuePage{Items: []port.IssueRecord{}}
	err = r.readTx(ctx, func(tx *sqlx.Tx) error {
		var ids []uuid.UUID
		if err := tx.SelectContext(ctx, &ids, built.SelectSQL, built.SelectArgs...); err != nil {
			return err
		}
		if q.WithTotal {
			if err := tx.GetContext(ctx, &page.Total, built.CountSQL, built.CountArgs...); err != nil {
				return err
			}
			page.HasTotal = true
		}
		items, err := loadRecords(ctx, tx, ids)
		if err != nil {
			return err
		}
		page.Items = items
		return nil
	})
	if err != nil {
		return port.IssuePage{}, err
	}
	return page, nil
}

type recordRow struct {
	ID                       uuid.UUID       `db:"id"`
	ProjectID                uuid.UUID       `db:"project_id"`
	ProjectKey               string          `db:"project_key"`
	KeyNumber                int             `db:"key_number"`
	Summary                  string          `db:"summary"`
	Description              string          `db:"description"`
	LegacyStatus             string          `db:"status"`
	ParentID                 *uuid.UUID      `db:"parent_id"`
	DueDate                  *time.Time      `db:"due_date"`
	StartDate                *time.Time      `db:"start_date"`
	StoryPoints              *float64        `db:"story_points"`
	Rank                     string          `db:"rank"`
	ClosedAt                 *time.Time      `db:"closed_at"`
	ReporterID               uuid.UUID       `db:"reporter_id"`
	AssigneeID               *uuid.UUID      `db:"assignee_id"`
	OriginalEstimateSeconds  *int64          `db:"original_estimate_seconds"`
	RemainingEstimateSeconds *int64          `db:"remaining_estimate_seconds"`
	Flagged                  bool            `db:"flagged"`
	VoteCount                int             `db:"vote_count"`
	ResolvedAt               *time.Time      `db:"resolved_at"`
	Custom                   json.RawMessage `db:"custom"`
	CreatedAt                time.Time       `db:"created_at"`
	UpdatedAt                time.Time       `db:"updated_at"`
	StatusID                 *uuid.UUID      `db:"status_id"`
	StatusName               *string         `db:"status_name"`
	StatusCategory           *string         `db:"status_category"`
	IssueTypeID              *uuid.UUID      `db:"issue_type_id"`
	IssueTypeName            *string         `db:"issue_type_name"`
	IssueTypeIcon            *string         `db:"issue_type_icon"`
	IssueTypeColor           *string         `db:"issue_type_color"`
	IssueTypeLevel           *int            `db:"issue_type_level"`
	PriorityID               *uuid.UUID      `db:"priority_id"`
	PriorityName             *string         `db:"priority_name"`
	PriorityIcon             *string         `db:"priority_icon"`
	PriorityColor            *string         `db:"priority_color"`
	ResolutionID             *uuid.UUID      `db:"resolution_id"`
	ResolutionName           *string         `db:"resolution_name"`
	ReporterName             *string         `db:"reporter_name"`
	AssigneeName             *string         `db:"assignee_name"`
}

const recordSQL = `
SELECT i.id, i.project_id, p.key AS project_key, i.key_number, i.summary, i.description, i.status, i.parent_id,
       i.due_date, i.start_date, i.story_points, i.rank, i.closed_at, i.reporter_id, i.assignee_id,
       i.original_estimate_seconds, i.remaining_estimate_seconds, i.flagged, i.vote_count, i.resolved_at,
       i.custom, i.created_at, i.updated_at,
       i.status_id, st.name AS status_name, st.category AS status_category,
       i.issue_type_id, it.name AS issue_type_name, it.icon AS issue_type_icon, it.color AS issue_type_color,
       it.hierarchy_level AS issue_type_level,
       i.priority_id, pr.name AS priority_name, pr.icon AS priority_icon, pr.color AS priority_color,
       i.resolution_id, rs.name AS resolution_name,
       ru.display_name AS reporter_name, au.display_name AS assignee_name
FROM issues i
JOIN projects p ON p.id = i.project_id
LEFT JOIN statuses st ON st.id = i.status_id
LEFT JOIN issue_types it ON it.id = i.issue_type_id
LEFT JOIN priorities pr ON pr.id = i.priority_id
LEFT JOIN resolutions rs ON rs.id = i.resolution_id
LEFT JOIN users ru ON ru.id = i.reporter_id
LEFT JOIN users au ON au.id = i.assignee_id
WHERE i.id = ANY($1::uuid[])`

func uuidStrings(ids []uuid.UUID) []string {
	out := make([]string, len(ids))
	for i, id := range ids {
		out[i] = id.String()
	}
	return out
}

// loadRecords loads the issues of a page, in the order of ids, with two statements.
func loadRecords(ctx context.Context, tx *sqlx.Tx, ids []uuid.UUID) ([]port.IssueRecord, error) {
	if len(ids) == 0 {
		return []port.IssueRecord{}, nil
	}
	var rows []recordRow
	if err := tx.SelectContext(ctx, &rows, recordSQL, uuidStrings(ids)); err != nil {
		return nil, err
	}
	var labelRows []struct {
		IssueID uuid.UUID `db:"issue_id"`
		Name    string    `db:"name"`
	}
	if err := tx.SelectContext(ctx, &labelRows, `
		SELECT il.issue_id, l.name FROM issue_labels il JOIN labels l ON l.id = il.label_id AND l.deleted_at IS NULL
		WHERE il.issue_id = ANY($1::uuid[]) ORDER BY l.name`, uuidStrings(ids)); err != nil {
		return nil, err
	}
	labels := map[uuid.UUID][]string{}
	for _, l := range labelRows {
		labels[l.IssueID] = append(labels[l.IssueID], l.Name)
	}
	byID := make(map[uuid.UUID]recordRow, len(rows))
	for _, row := range rows {
		byID[row.ID] = row
	}
	out := make([]port.IssueRecord, 0, len(ids))
	for _, id := range ids {
		row, ok := byID[id]
		if !ok {
			continue
		}
		out = append(out, row.toRecord(labels[id]))
	}
	return out, nil
}

func ref(id *uuid.UUID, name, icon, color, category *string, level *int) *port.IssueRef {
	if id == nil || name == nil {
		return nil
	}
	r := &port.IssueRef{ID: *id, Name: *name}
	if icon != nil {
		r.Icon = *icon
	}
	if color != nil {
		r.Color = *color
	}
	if category != nil {
		r.Category = *category
	}
	if level != nil {
		r.HierarchyLevel = *level
	}
	return r
}

func dateString(t *time.Time) *string {
	if t == nil {
		return nil
	}
	s := t.UTC().Format("2006-01-02")
	return &s
}

func (row recordRow) toRecord(labels []string) port.IssueRecord {
	custom := map[string]any{}
	if len(row.Custom) > 0 {
		_ = json.Unmarshal(row.Custom, &custom)
	}
	if labels == nil {
		labels = []string{}
	}
	rec := port.IssueRecord{
		ID: row.ID, Key: row.ProjectKey + "-" + strconv.Itoa(row.KeyNumber), ProjectID: row.ProjectID, ProjectKey: row.ProjectKey,
		Summary: row.Summary, Description: row.Description, LegacyStatus: row.LegacyStatus,
		Status:       ref(row.StatusID, row.StatusName, nil, nil, row.StatusCategory, nil),
		IssueType:    ref(row.IssueTypeID, row.IssueTypeName, row.IssueTypeIcon, row.IssueTypeColor, nil, row.IssueTypeLevel),
		Priority:     ref(row.PriorityID, row.PriorityName, row.PriorityIcon, row.PriorityColor, nil, nil),
		Resolution:   ref(row.ResolutionID, row.ResolutionName, nil, nil, nil, nil),
		ResolvedAt:   row.ResolvedAt,
		ParentID:     row.ParentID,
		StartDate:    dateString(row.StartDate),
		DueDate:      dateString(row.DueDate),
		StoryPoints:  row.StoryPoints,
		Flagged:      row.Flagged,
		VoteCount:    row.VoteCount,
		Rank:         row.Rank,
		ClosedAt:     row.ClosedAt,
		ReporterID:   row.ReporterID,
		AssigneeID:   row.AssigneeID,
		AssigneeName: row.AssigneeName,
		Labels:       labels,
		Custom:       custom,
		CreatedAt:    row.CreatedAt,
		UpdatedAt:    row.UpdatedAt,

		OriginalEstimateSeconds:  row.OriginalEstimateSeconds,
		RemainingEstimateSeconds: row.RemainingEstimateSeconds,
	}
	if row.ReporterName != nil {
		rec.ReporterName = *row.ReporterName
	}
	return rec
}

// CustomFields lists the custom fields of the organizations the viewer belongs to (every organization for a
// platform administrator), one entry per name and type.
func (r *IssueSearchRepository) CustomFields(ctx context.Context, viewer port.Viewer) ([]port.CustomFieldDef, error) {
	var rows []struct {
		Name string `db:"name"`
		Kind string `db:"field_type"`
		IDs  string `db:"ids"`
	}
	err := r.db.SelectContext(ctx, &rows, `
		SELECT min(f.name) AS name, f.field_type, string_agg(f.id::text, ',' ORDER BY f.id) AS ids
		FROM custom_fields f
		WHERE f.deleted_at IS NULL AND ($2::boolean OR f.organization_id IN (
		    SELECT om.organization_id FROM organization_members om
		    JOIN organizations o ON o.id = om.organization_id AND o.deleted_at IS NULL
		    WHERE om.user_id = $1::uuid))
		GROUP BY lower(f.name), f.field_type
		ORDER BY lower(min(f.name)), f.field_type`, viewer.UserID, viewer.PlatformAdmin)
	if err != nil {
		return nil, apperror.Wrap(apperror.KindInternal, "failed to list custom fields", err)
	}
	out := make([]port.CustomFieldDef, 0, len(rows))
	for _, row := range rows {
		def := port.CustomFieldDef{Name: row.Name, Kind: row.Kind}
		for _, s := range strings.Split(row.IDs, ",") {
			if id, err := uuid.Parse(s); err == nil {
				def.IDs = append(def.IDs, id)
			}
		}
		out = append(out, def)
	}
	return out, nil
}

// Timezone returns the time zone of the user, or UTC when none is stored.
func (r *IssueSearchRepository) Timezone(ctx context.Context, userID uuid.UUID) (string, error) {
	var tz string
	err := r.db.GetContext(ctx, &tz, `SELECT timezone FROM users WHERE id = $1`, userID)
	if errors.Is(err, sql.ErrNoRows) {
		return "UTC", nil
	}
	if err != nil {
		return "", apperror.Wrap(apperror.KindInternal, "failed to read the time zone", err)
	}
	return tz, nil
}
