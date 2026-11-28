package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jmoiron/sqlx"

	"github.com/jhonsferg/corelog/internal/board/application/port"
	"github.com/jhonsferg/corelog/internal/board/domain"
	"github.com/jhonsferg/corelog/internal/platform/apperror"
)

var _ port.BoardRepository = (*BoardRepository)(nil)

// labelSeparator joins the label names of a card in one aggregated column.
const labelSeparator = "\x1f"

type boardRow struct {
	ID                uuid.UUID       `db:"id"`
	ProjectID         uuid.UUID       `db:"project_id"`
	Name              string          `db:"name"`
	Type              string          `db:"type"`
	Config            json.RawMessage `db:"config"`
	IsDefault         bool            `db:"is_default"`
	SwimlaneMode      string          `db:"swimlane_mode"`
	Estimation        string          `db:"estimation"`
	CardFields        json.RawMessage `db:"card_fields"`
	ShowUnmappedLane  bool            `db:"show_unmapped_lane"`
	DoneRetentionDays *int            `db:"done_retention_days"`
	CreatedAt         time.Time       `db:"created_at"`
	UpdatedAt         time.Time       `db:"updated_at"`
}

const boardColumnsSQL = `b.id, b.project_id, b.name, b.type, b.config, b.is_default, b.swimlane_mode, b.estimation,
	b.card_fields, b.show_unmapped_lane, b.done_retention_days, b.created_at, b.updated_at`

func (r boardRow) toDomain() domain.Board {
	var fields []string
	if err := json.Unmarshal(r.CardFields, &fields); err != nil || fields == nil {
		fields = []string{}
	}
	return domain.Board{
		ID: r.ID, ProjectID: r.ProjectID, Name: r.Name, Type: domain.Type(r.Type), Config: r.Config, IsDefault: r.IsDefault,
		SwimlaneMode: domain.SwimlaneMode(r.SwimlaneMode), Estimation: domain.Estimation(r.Estimation), CardFields: fields,
		ShowUnmappedLane: r.ShowUnmappedLane, DoneRetentionDays: r.DoneRetentionDays, CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt,
	}
}

type columnRow struct {
	ID        uuid.UUID `db:"id"`
	BoardID   uuid.UUID `db:"board_id"`
	Name      string    `db:"name"`
	Status    string    `db:"status"`
	Position  int       `db:"position"`
	WIPLimit  *int      `db:"wip_limit"`
	MinIssues *int      `db:"min_issues"`
	Color     string    `db:"color"`
	StatusIDs string    `db:"status_ids"`
	CreatedAt time.Time `db:"created_at"`
	UpdatedAt time.Time `db:"updated_at"`
}

const columnsSQL = `SELECT c.id, c.board_id, c.name, COALESCE(c.status, '') AS status, c.position, c.wip_limit, c.min_issues, c.color,
	COALESCE(string_agg(bcs.status_id::text, ',' ORDER BY bcs.status_id), '') AS status_ids, c.created_at, c.updated_at
	FROM board_columns c LEFT JOIN board_column_statuses bcs ON bcs.column_id = c.id`

func (c columnRow) toDomain() domain.Column {
	column := domain.Column{
		ID: c.ID, BoardID: c.BoardID, Name: c.Name, Status: domain.IssueStatus(c.Status), Position: c.Position,
		WIPLimit: c.WIPLimit, MinIssues: c.MinIssues, Color: c.Color, StatusIDs: []uuid.UUID{}, CreatedAt: c.CreatedAt, UpdatedAt: c.UpdatedAt,
	}
	if c.StatusIDs != "" {
		for _, part := range strings.Split(c.StatusIDs, ",") {
			if id, err := uuid.Parse(part); err == nil {
				column.StatusIDs = append(column.StatusIDs, id)
			}
		}
	}
	return column
}

type queryRow struct {
	ID       uuid.UUID `db:"id"`
	BoardID  uuid.UUID `db:"board_id"`
	Name     string    `db:"name"`
	Query    string    `db:"query_text"`
	Position int       `db:"position"`
}

func internalErr(message string, err error) error {
	return apperror.Wrap(apperror.KindInternal, message, err)
}

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}

// ListByProject returns the boards of a live project with their columns, oldest first (the default first).
func (r *BoardRepository) ListByProject(ctx context.Context, projectID uuid.UUID) ([]domain.Board, error) {
	var rows []boardRow
	if err := sqlx.SelectContext(ctx, r.q, &rows, `SELECT `+boardColumnsSQL+`
		FROM boards b JOIN projects p ON p.id = b.project_id AND p.deleted_at IS NULL
		WHERE b.project_id = $1 AND b.deleted_at IS NULL ORDER BY b.is_default DESC, b.created_at, b.id`, projectID); err != nil {
		return nil, internalErr("failed to list boards", err)
	}
	boards := make([]domain.Board, len(rows))
	index := map[uuid.UUID]int{}
	for i, row := range rows {
		boards[i] = row.toDomain()
		boards[i].Columns = []domain.Column{}
		index[row.ID] = i
	}
	if len(rows) == 0 {
		return boards, nil
	}
	var columns []columnRow
	if err := sqlx.SelectContext(ctx, r.q, &columns, columnsSQL+`
		JOIN boards b ON b.id = c.board_id WHERE b.project_id = $1 AND b.deleted_at IS NULL
		GROUP BY c.id ORDER BY c.board_id, c.position`, projectID); err != nil {
		return nil, internalErr("failed to list board columns", err)
	}
	for _, c := range columns {
		if i, ok := index[c.BoardID]; ok {
			boards[i].Columns = append(boards[i].Columns, c.toDomain())
		}
	}
	return boards, nil
}

// FindByID returns a board with its columns and their statuses, quick filters and swimlanes (four statements).
func (r *BoardRepository) FindByID(ctx context.Context, id uuid.UUID) (*domain.Board, error) {
	var row boardRow
	err := sqlx.GetContext(ctx, r.q, &row, `SELECT `+boardColumnsSQL+`
		FROM boards b JOIN projects p ON p.id = b.project_id AND p.deleted_at IS NULL WHERE b.id = $1 AND b.deleted_at IS NULL`, id)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, domain.ErrBoardNotFound
	}
	if err != nil {
		return nil, internalErr("failed to read board", err)
	}
	board := row.toDomain()
	var columns []columnRow
	if err := sqlx.SelectContext(ctx, r.q, &columns, columnsSQL+` WHERE c.board_id = $1 GROUP BY c.id ORDER BY c.position`, id); err != nil {
		return nil, internalErr("failed to read board columns", err)
	}
	board.Columns = make([]domain.Column, 0, len(columns))
	for _, c := range columns {
		board.Columns = append(board.Columns, c.toDomain())
	}
	var filters []queryRow
	if err := sqlx.SelectContext(ctx, r.q, &filters, `SELECT id, board_id, name, query_text, position FROM board_quick_filters WHERE board_id = $1 ORDER BY position`, id); err != nil {
		return nil, internalErr("failed to read quick filters", err)
	}
	board.QuickFilters = make([]domain.QuickFilter, 0, len(filters))
	for _, f := range filters {
		board.QuickFilters = append(board.QuickFilters, domain.QuickFilter{ID: f.ID, BoardID: f.BoardID, Name: f.Name, Query: f.Query, Position: f.Position})
	}
	var lanes []queryRow
	if err := sqlx.SelectContext(ctx, r.q, &lanes, `SELECT id, board_id, name, query_text, position FROM board_swimlanes WHERE board_id = $1 ORDER BY position`, id); err != nil {
		return nil, internalErr("failed to read swimlanes", err)
	}
	board.Swimlanes = make([]domain.Swimlane, 0, len(lanes))
	for _, l := range lanes {
		board.Swimlanes = append(board.Swimlanes, domain.Swimlane{ID: l.ID, BoardID: l.BoardID, Name: l.Name, Query: l.Query, Position: l.Position})
	}
	return &board, nil
}

// Create inserts the board with its columns and status bindings in one transaction and sets the generated ids.
func (r *BoardRepository) Create(ctx context.Context, board *domain.Board) error {
	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return internalErr("failed to create board", err)
	}
	defer func() { _ = tx.Rollback() }()
	cardFields, _ := json.Marshal(board.CardFields)
	err = tx.QueryRowxContext(ctx, `
		INSERT INTO boards (project_id, name, type, is_default, swimlane_mode, estimation, card_fields, show_unmapped_lane, done_retention_days, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7::jsonb, $8, $9, $10, $10) RETURNING id`,
		board.ProjectID, board.Name, string(board.Type), board.IsDefault, string(board.SwimlaneMode), string(board.Estimation),
		string(cardFields), board.ShowUnmappedLane, board.DoneRetentionDays, board.CreatedAt).Scan(&board.ID)
	if err != nil {
		if isUniqueViolation(err) {
			return domain.ErrBoardNameTaken
		}
		return internalErr("failed to create board", err)
	}
	for i := range board.Columns {
		column := &board.Columns[i]
		column.BoardID = board.ID
		if err := tx.QueryRowxContext(ctx, `
			INSERT INTO board_columns (board_id, name, position, wip_limit, min_issues, color, created_at, updated_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $7) RETURNING id`,
			board.ID, column.Name, i, column.WIPLimit, column.MinIssues, column.Color, board.CreatedAt).Scan(&column.ID); err != nil {
			return internalErr("failed to create board column", err)
		}
		if err := bindStatuses(ctx, tx, column.ID, column.StatusIDs); err != nil {
			return err
		}
	}
	if err := tx.Commit(); err != nil {
		return internalErr("failed to create board", err)
	}
	return nil
}

func bindStatuses(ctx context.Context, tx sqlx.ExtContext, columnID uuid.UUID, statusIDs []uuid.UUID) error {
	if len(statusIDs) == 0 {
		return nil
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO board_column_statuses (column_id, status_id) SELECT $1, unnest($2::uuid[])`, columnID, uuidStrings(statusIDs)); err != nil {
		return internalErr("failed to map column statuses", err)
	}
	return nil
}

func uuidStrings(ids []uuid.UUID) []string {
	out := make([]string, len(ids))
	for i, id := range ids {
		out[i] = id.String()
	}
	return out
}

// UpdateSettings writes the editable settings of the board.
func (r *BoardRepository) UpdateSettings(ctx context.Context, board *domain.Board) error {
	cardFields, _ := json.Marshal(board.CardFields)
	result, err := r.db.ExecContext(ctx, `
		UPDATE boards SET name = $2, swimlane_mode = $3, estimation = $4, card_fields = $5::jsonb, show_unmapped_lane = $6,
		       done_retention_days = $7, updated_at = $8
		WHERE id = $1 AND deleted_at IS NULL`, board.ID, board.Name, string(board.SwimlaneMode), string(board.Estimation), string(cardFields),
		board.ShowUnmappedLane, board.DoneRetentionDays, board.UpdatedAt)
	if err != nil {
		if isUniqueViolation(err) {
			return domain.ErrBoardNameTaken
		}
		return internalErr("failed to update board", err)
	}
	if n, _ := result.RowsAffected(); n == 0 {
		return domain.ErrBoardNotFound
	}
	return nil
}

// Delete logically deletes a board; its columns, bindings, quick filters and swimlanes stay stored and are hidden
// with it. A deleted board is never the default.
func (r *BoardRepository) Delete(ctx context.Context, id uuid.UUID) error {
	result, err := r.db.ExecContext(ctx, `UPDATE boards SET deleted_at = now(), is_default = FALSE WHERE id = $1 AND deleted_at IS NULL`, id)
	if err != nil {
		return internalErr("failed to delete board", err)
	}
	if n, _ := result.RowsAffected(); n == 0 {
		return domain.ErrBoardNotFound
	}
	return nil
}

// ReplaceColumns keeps the columns whose id is given (updating them in place), creates the others and removes
// the rest, all in one transaction. The position follows the order of specs.
func (r *BoardRepository) ReplaceColumns(ctx context.Context, boardID uuid.UUID, specs []domain.ColumnSpec, now time.Time) error {
	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return internalErr("failed to replace board columns", err)
	}
	defer func() { _ = tx.Rollback() }()
	var locked uuid.UUID
	if err := tx.GetContext(ctx, &locked, `SELECT id FROM boards WHERE id = $1 AND deleted_at IS NULL FOR UPDATE`, boardID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return domain.ErrBoardNotFound
		}
		return internalErr("failed to replace board columns", err)
	}
	var keep []uuid.UUID
	for _, spec := range specs {
		if spec.ID != uuid.Nil {
			keep = append(keep, spec.ID)
		}
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM board_columns WHERE board_id = $1 AND NOT (id = ANY($2::uuid[]))`, boardID, uuidStrings(keep)); err != nil {
		return internalErr("failed to replace board columns", err)
	}
	for i, spec := range specs {
		id := spec.ID
		if id == uuid.Nil {
			err = tx.QueryRowxContext(ctx, `
				INSERT INTO board_columns (board_id, name, position, wip_limit, min_issues, color, created_at, updated_at)
				VALUES ($1, $2, $3, $4, $5, $6, $7, $7) RETURNING id`,
				boardID, spec.Name, i, spec.MaxIssues, spec.MinIssues, spec.Color, now).Scan(&id)
		} else {
			var result sql.Result
			result, err = tx.ExecContext(ctx, `
				UPDATE board_columns SET name = $3, position = $4, wip_limit = $5, min_issues = $6, color = $7, status = NULL, updated_at = $8
				WHERE id = $1 AND board_id = $2`, id, boardID, spec.Name, i, spec.MaxIssues, spec.MinIssues, spec.Color, now)
			if err == nil {
				if n, _ := result.RowsAffected(); n == 0 {
					return domain.ErrColumnNotFound
				}
				_, err = tx.ExecContext(ctx, `DELETE FROM board_column_statuses WHERE column_id = $1`, id)
			}
		}
		if err != nil {
			return internalErr("failed to replace board columns", err)
		}
		if err := bindStatuses(ctx, tx, id, spec.StatusIDs); err != nil {
			return err
		}
	}
	if _, err := tx.ExecContext(ctx, `UPDATE boards SET updated_at = $2 WHERE id = $1`, boardID, now); err != nil {
		return internalErr("failed to replace board columns", err)
	}
	if err := tx.Commit(); err != nil {
		return internalErr("failed to replace board columns", err)
	}
	return nil
}

// ReplaceQuickFilters replaces the quick filters of the board.
func (r *BoardRepository) ReplaceQuickFilters(ctx context.Context, boardID uuid.UUID, filters []domain.QuickFilter) error {
	return r.replaceQueries(ctx, "board_quick_filters", boardID, len(filters), func(i int) (string, string) { return filters[i].Name, filters[i].Query })
}

// ReplaceSwimlanes replaces the query swimlanes of the board.
func (r *BoardRepository) ReplaceSwimlanes(ctx context.Context, boardID uuid.UUID, lanes []domain.Swimlane) error {
	return r.replaceQueries(ctx, "board_swimlanes", boardID, len(lanes), func(i int) (string, string) { return lanes[i].Name, lanes[i].Query })
}

// replaceQueries rewrites the rows of one of the two query tables (table is a constant of this package).
func (r *BoardRepository) replaceQueries(ctx context.Context, table string, boardID uuid.UUID, n int, at func(int) (string, string)) error {
	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return internalErr("failed to replace board queries", err)
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.ExecContext(ctx, `DELETE FROM `+table+` WHERE board_id = $1`, boardID); err != nil {
		return internalErr("failed to replace board queries", err)
	}
	for i := 0; i < n; i++ {
		name, query := at(i)
		if _, err := tx.ExecContext(ctx, `INSERT INTO `+table+` (board_id, name, query_text, position) VALUES ($1, $2, $3, $4)`, boardID, name, query, i); err != nil {
			return internalErr("failed to replace board queries", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return internalErr("failed to replace board queries", err)
	}
	return nil
}

// WorkflowStatuses returns the statuses of the workflow of the project in workflow order.
func (r *BoardRepository) WorkflowStatuses(ctx context.Context, projectID uuid.UUID) ([]port.WorkflowStatus, error) {
	var rows []struct {
		ID       uuid.UUID `db:"id"`
		Name     string    `db:"name"`
		Category string    `db:"category"`
	}
	if err := sqlx.SelectContext(ctx, r.q, &rows, `
		SELECT (s.value ->> 'id')::uuid AS id, s.value ->> 'name' AS name, s.value ->> 'category' AS category
		FROM workflows w CROSS JOIN LATERAL jsonb_array_elements(w.definition -> 'statuses') WITH ORDINALITY AS s(value, ord)
		WHERE w.project_id = $1 ORDER BY s.ord`, projectID); err != nil {
		return nil, internalErr("failed to read workflow statuses", err)
	}
	out := make([]port.WorkflowStatus, len(rows))
	for i, row := range rows {
		out[i] = port.WorkflowStatus{ID: row.ID, Name: row.Name, Category: row.Category}
	}
	return out, nil
}

type cardRow struct {
	ID               uuid.UUID  `db:"id"`
	Key              string     `db:"key"`
	Summary          string     `db:"summary"`
	TypeName         string     `db:"type_name"`
	TypeKey          string     `db:"type_key"`
	TypeIcon         string     `db:"type_icon"`
	TypeColor        string     `db:"type_color"`
	PriorityName     string     `db:"priority_name"`
	PriorityKey      string     `db:"priority_key"`
	PriorityIcon     string     `db:"priority_icon"`
	PriorityColor    string     `db:"priority_color"`
	StatusID         uuid.UUID  `db:"status_id"`
	StatusName       string     `db:"status_name"`
	StatusCategory   string     `db:"status_category"`
	AssigneeID       *uuid.UUID `db:"assignee_id"`
	AssigneeName     string     `db:"assignee_name"`
	Labels           string     `db:"labels"`
	StoryPoints      *float64   `db:"story_points"`
	RemainingSeconds *int64     `db:"remaining_estimate_seconds"`
	Flagged          bool       `db:"flagged"`
	Rank             string     `db:"rank"`
	SprintID         *uuid.UUID `db:"sprint_id"`
	EpicID           *uuid.UUID `db:"epic_id"`
	EpicKey          *string    `db:"epic_key"`
	EpicSummary      *string    `db:"epic_summary"`
	ParentID         *uuid.UUID `db:"parent_id"`
	ParentKey        *string    `db:"parent_key"`
	ParentSummary    *string    `db:"parent_summary"`
	ParentType       *string    `db:"parent_type"`
	SubtaskTotal     int        `db:"subtask_total"`
	SubtaskDone      int        `db:"subtask_done"`
}

// cardsSQL selects the live cards of a project in one statement: type, priority, status, assignee, labels,
// epic, parent and subtask counters are joined or aggregated in the same query. Epics are not cards.
const cardsSQL = `
SELECT i.id, p.key || '-' || i.key_number AS key, i.summary,
       COALESCE(it.name, '') AS type_name, COALESCE(it.legacy_key, '') AS type_key, COALESCE(it.icon, '') AS type_icon, COALESCE(it.color, '') AS type_color,
       COALESCE(pr.name, '') AS priority_name, COALESCE(pr.legacy_key, '') AS priority_key, COALESCE(pr.icon, '') AS priority_icon, COALESCE(pr.color, '') AS priority_color,
       i.status_id, st.name AS status_name, st.category AS status_category,
       i.assignee_id, COALESCE(NULLIF(u.display_name, ''), u.name, '') AS assignee_name,
       COALESCE((SELECT string_agg(l.name, E'\x1f' ORDER BY l.name) FROM issue_labels il JOIN labels l ON l.id = il.label_id AND l.deleted_at IS NULL WHERE il.issue_id = i.id), '') AS labels,
       i.story_points, i.remaining_estimate_seconds, i.flagged, i.rank, i.sprint_id,
       CASE WHEN pt.hierarchy_level = 1 THEN par.id WHEN gt.hierarchy_level = 1 THEN gp.id END AS epic_id,
       CASE WHEN pt.hierarchy_level = 1 THEN p.key || '-' || par.key_number WHEN gt.hierarchy_level = 1 THEN p.key || '-' || gp.key_number END AS epic_key,
       CASE WHEN pt.hierarchy_level = 1 THEN par.summary WHEN gt.hierarchy_level = 1 THEN gp.summary END AS epic_summary,
       par.id AS parent_id, p.key || '-' || par.key_number AS parent_key, par.summary AS parent_summary,
       CASE WHEN pt.hierarchy_level = 1 THEN 'epic' ELSE COALESCE(pt.legacy_key, pt.name) END AS parent_type,
       (SELECT count(*) FROM issues c WHERE c.parent_id = i.id AND c.deleted_at IS NULL) AS subtask_total,
       (SELECT count(*) FROM issues c JOIN statuses cs ON cs.id = c.status_id WHERE c.parent_id = i.id AND c.deleted_at IS NULL AND cs.category = 'done') AS subtask_done
FROM issues i
JOIN projects p ON p.id = i.project_id AND p.deleted_at IS NULL
JOIN statuses st ON st.id = i.status_id
LEFT JOIN issue_types it ON it.id = i.issue_type_id
LEFT JOIN priorities pr ON pr.id = i.priority_id
LEFT JOIN users u ON u.id = i.assignee_id
LEFT JOIN issues par ON par.id = i.parent_id AND par.deleted_at IS NULL
LEFT JOIN issue_types pt ON pt.id = par.issue_type_id
LEFT JOIN issues gp ON gp.id = par.parent_id AND gp.deleted_at IS NULL
LEFT JOIN issue_types gt ON gt.id = gp.issue_type_id
WHERE i.project_id = $1 AND i.deleted_at IS NULL AND COALESCE(it.hierarchy_level, 0) <> 1
  AND ($2::text = 'all'
       OR ($2::text = 'active' AND i.sprint_id IN (SELECT s.id FROM sprints s WHERE s.project_id = $1 AND s.status = 'active' AND s.deleted_at IS NULL))
       OR ($2::text = 'none' AND i.sprint_id IS NULL)
       OR ($2::text = 'id' AND i.sprint_id = $3::uuid))
  AND ($4::int IS NULL OR st.category <> 'done' OR i.updated_at >= $5::timestamptz - make_interval(days => $4::int))
ORDER BY i.rank COLLATE "C", i.created_at, i.id
LIMIT $6`

// Cards returns the cards of the project in rank order with the sprint the query filters by, if any. It runs one
// statement for the cards and, for the active or a chosen sprint, one for the sprint: the count does not grow
// with the number of cards.
func (r *BoardRepository) Cards(ctx context.Context, q port.CardQuery) ([]domain.Card, *port.SprintInfo, error) {
	mode := q.Sprint.Mode
	if mode == "" {
		mode = "all"
	}
	var rows []cardRow
	sprintID := uuid.Nil
	if mode == "id" {
		sprintID = q.Sprint.ID
	}
	if err := sqlx.SelectContext(ctx, r.q, &rows, cardsSQL, q.ProjectID, mode, sprintID, q.DoneRetentionDays, q.Now, q.Limit); err != nil {
		return nil, nil, internalErr("failed to read board cards", err)
	}
	cards := make([]domain.Card, len(rows))
	for i, row := range rows {
		cards[i] = row.toDomain()
	}
	var info *port.SprintInfo
	if mode == "active" || mode == "id" {
		var s struct {
			ID   uuid.UUID `db:"id"`
			Name string    `db:"name"`
		}
		err := sqlx.GetContext(ctx, r.q, &s, `SELECT id, name FROM sprints WHERE project_id = $1 AND deleted_at IS NULL AND (($2::text = 'active' AND status = 'active') OR ($2::text = 'id' AND id = $3::uuid)) LIMIT 1`,
			q.ProjectID, mode, sprintID)
		switch {
		case err == nil:
			info = &port.SprintInfo{ID: s.ID, Name: s.Name}
		case !errors.Is(err, sql.ErrNoRows):
			return nil, nil, internalErr("failed to read board sprint", err)
		}
	}
	return cards, info, nil
}

func (c cardRow) toDomain() domain.Card {
	card := domain.Card{
		ID: c.ID, Key: c.Key, Summary: c.Summary,
		Type: c.TypeName, TypeKey: c.TypeKey, TypeIcon: c.TypeIcon, TypeColor: c.TypeColor,
		Priority: c.PriorityName, PriorityKey: c.PriorityKey, PriorityIcon: c.PriorityIcon, PriorityColor: c.PriorityColor,
		StatusID: c.StatusID, StatusName: c.StatusName, StatusCategory: c.StatusCategory,
		AssigneeID: c.AssigneeID, AssigneeName: c.AssigneeName, Labels: []string{},
		StoryPoints: c.StoryPoints, RemainingSeconds: c.RemainingSeconds, Flagged: c.Flagged, Rank: c.Rank, SprintID: c.SprintID,
		SubtaskTotal: c.SubtaskTotal, SubtaskDone: c.SubtaskDone,
	}
	if c.Labels != "" {
		card.Labels = strings.Split(c.Labels, labelSeparator)
	}
	if c.EpicID != nil && c.EpicKey != nil && c.EpicSummary != nil {
		card.Epic = &domain.CardRef{ID: *c.EpicID, Key: *c.EpicKey, Summary: *c.EpicSummary, Type: "epic"}
	}
	if c.ParentID != nil && c.ParentKey != nil && c.ParentSummary != nil {
		kind := ""
		if c.ParentType != nil {
			kind = *c.ParentType
		}
		card.Parent = &domain.CardRef{ID: *c.ParentID, Key: *c.ParentKey, Summary: *c.ParentSummary, Type: kind}
	}
	return card
}

// CardPlace returns the status, rank, assignee and epic of a live issue of the project.
func (r *BoardRepository) CardPlace(ctx context.Context, projectID, issueID uuid.UUID) (*port.CardPlace, error) {
	var row struct {
		StatusID   uuid.UUID  `db:"status_id"`
		Rank       string     `db:"rank"`
		AssigneeID *uuid.UUID `db:"assignee_id"`
		EpicID     *uuid.UUID `db:"epic_id"`
	}
	err := sqlx.GetContext(ctx, r.q, &row, `
		SELECT i.status_id, i.rank, i.assignee_id,
		       CASE WHEN pt.hierarchy_level = 1 THEN par.id WHEN gt.hierarchy_level = 1 THEN gp.id END AS epic_id
		FROM issues i
		LEFT JOIN issues par ON par.id = i.parent_id AND par.deleted_at IS NULL
		LEFT JOIN issue_types pt ON pt.id = par.issue_type_id
		LEFT JOIN issues gp ON gp.id = par.parent_id AND gp.deleted_at IS NULL
		LEFT JOIN issue_types gt ON gt.id = gp.issue_type_id
		WHERE i.id = $1 AND i.project_id = $2 AND i.deleted_at IS NULL`, issueID, projectID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, domain.ErrCardNotFound
	}
	if err != nil {
		return nil, internalErr("failed to read card", err)
	}
	return &port.CardPlace{StatusID: row.StatusID, Rank: row.Rank, AssigneeID: row.AssigneeID, EpicID: row.EpicID}, nil
}

// RankNeighbors returns the rank of the anchor and the closest ranks around it in the project, skipping the moved issue.
// Ranks compare bytewise (collation C) like the rank package does.
func (r *BoardRepository) RankNeighbors(ctx context.Context, projectID, anchorID, movedID uuid.UUID) (string, string, string, error) {
	var row struct {
		Anchor string  `db:"anchor"`
		Lower  *string `db:"lower_rank"`
		Upper  *string `db:"upper_rank"`
	}
	err := sqlx.GetContext(ctx, r.q, &row, `
		SELECT a.rank AS anchor,
		       (SELECT max(x.rank COLLATE "C") FROM issues x WHERE x.project_id = a.project_id AND x.deleted_at IS NULL AND x.id <> $3 AND x.rank COLLATE "C" < a.rank COLLATE "C") AS lower_rank,
		       (SELECT min(x.rank COLLATE "C") FROM issues x WHERE x.project_id = a.project_id AND x.deleted_at IS NULL AND x.id <> $3 AND x.rank COLLATE "C" > a.rank COLLATE "C") AS upper_rank
		FROM issues a WHERE a.id = $2 AND a.project_id = $1 AND a.deleted_at IS NULL`, projectID, anchorID, movedID)
	if errors.Is(err, sql.ErrNoRows) {
		return "", "", "", domain.ErrCardNotFound
	}
	if err != nil {
		return "", "", "", internalErr("failed to read card neighbors", err)
	}
	lower, upper := "", ""
	if row.Lower != nil {
		lower = *row.Lower
	}
	if row.Upper != nil {
		upper = *row.Upper
	}
	return row.Anchor, lower, upper, nil
}

// SetRank stores the rank of a live issue of the project.
func (r *BoardRepository) SetRank(ctx context.Context, projectID, issueID uuid.UUID, rank string, now time.Time) error {
	result, err := r.db.ExecContext(ctx, `UPDATE issues SET rank = $3, updated_at = $4 WHERE id = $1 AND project_id = $2 AND deleted_at IS NULL`, issueID, projectID, rank, now)
	if err != nil {
		return internalErr("failed to rank card", err)
	}
	if n, _ := result.RowsAffected(); n == 0 {
		return domain.ErrCardNotFound
	}
	return nil
}

// CountInStatuses counts the live issues of the project in the given statuses.
func (r *BoardRepository) CountInStatuses(ctx context.Context, projectID uuid.UUID, statusIDs []uuid.UUID) (int, error) {
	var n int
	if err := sqlx.GetContext(ctx, r.q, &n, `SELECT count(*) FROM issues WHERE project_id = $1 AND deleted_at IS NULL AND status_id = ANY($2::uuid[])`,
		projectID, uuidStrings(statusIDs)); err != nil {
		return 0, internalErr("failed to count column cards", err)
	}
	return n, nil
}
