package postgres

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"

	"github.com/jhonsferg/corelog/internal/importexport/application/port"
	"github.com/jhonsferg/corelog/internal/platform/apperror"
	"github.com/jhonsferg/corelog/internal/platform/database"
)

// Size caps of the project backup.
const (
	dumpMaxIssues   = 20000
	dumpMaxComments = 100000
	dumpMaxWorklogs = 100000
	dumpMaxLinks    = 100000
)

// Reader is the PostgreSQL adapter of port.Reader. It reads issues and related rows of a project.
type Reader struct{ db *sqlx.DB }

var _ port.Reader = (*Reader)(nil)

// NewReader returns a reader bound to db.
func NewReader(db *sqlx.DB) *Reader { return &Reader{db: db} }

func (r *Reader) ext(ctx context.Context) sqlx.ExtContext { return database.Executor(ctx, r.db) }

// ProjectKey returns the key of a live project.
func (r *Reader) ProjectKey(ctx context.Context, projectID uuid.UUID) (string, error) {
	var key string
	err := sqlx.GetContext(ctx, r.ext(ctx), &key, `SELECT key FROM projects WHERE id = $1 AND deleted_at IS NULL`, projectID)
	if errors.Is(err, sql.ErrNoRows) {
		return "", apperror.WithCode(apperror.KindNotFound, "project.not_found", "Proyecto no encontrado")
	}
	if err != nil {
		return "", internal("no se pudo leer el proyecto", err)
	}
	return key, nil
}

// Rows returns the export rows of the given issues of the project in the order of ids.
func (r *Reader) Rows(ctx context.Context, projectID uuid.UUID, ids []uuid.UUID) ([]port.ExportRow, error) {
	if len(ids) == 0 {
		return []port.ExportRow{}, nil
	}
	query, args, err := sqlx.In(`
		SELECT i.id, p.key || '-' || i.key_number AS key, i.summary, i.description,
		       COALESCE(t.name, i.issue_type) AS type, COALESCE(pr.name, i.priority) AS priority,
		       COALESCE(s.name, i.status) AS status, COALESCE(a.email, '') AS assignee, COALESCE(rp.email, '') AS reporter,
		       i.created_at, i.updated_at, i.due_date, i.story_points,
		       COALESCE((SELECT json_agg(l.name ORDER BY l.name) FROM issue_labels il JOIN labels l ON l.id = il.label_id
		                 WHERE il.issue_id = i.id), '[]'::json) AS labels
		FROM issues i JOIN projects p ON p.id = i.project_id
		LEFT JOIN issue_types t ON t.id = i.issue_type_id
		LEFT JOIN priorities pr ON pr.id = i.priority_id
		LEFT JOIN statuses s ON s.id = i.status_id
		LEFT JOIN users a ON a.id = i.assignee_id
		LEFT JOIN users rp ON rp.id = i.reporter_id
		WHERE i.project_id = ? AND i.deleted_at IS NULL AND i.id IN (?)`, projectID, ids)
	if err != nil {
		return nil, internal("no se pudo preparar la exportacion", err)
	}
	var rows []struct {
		ID          uuid.UUID         `db:"id"`
		Key         string            `db:"key"`
		Summary     string            `db:"summary"`
		Description string            `db:"description"`
		Type        string            `db:"type"`
		Priority    string            `db:"priority"`
		Status      string            `db:"status"`
		Assignee    string            `db:"assignee"`
		Reporter    string            `db:"reporter"`
		CreatedAt   time.Time         `db:"created_at"`
		UpdatedAt   time.Time         `db:"updated_at"`
		DueDate     *time.Time        `db:"due_date"`
		Points      *float64          `db:"story_points"`
		Labels      database.TextList `db:"labels"`
	}
	if err := sqlx.SelectContext(ctx, r.ext(ctx), &rows, r.db.Rebind(query), args...); err != nil {
		return nil, internal("no se pudo leer las incidencias", err)
	}
	byID := make(map[uuid.UUID]port.ExportRow, len(rows))
	for _, row := range rows {
		byID[row.ID] = port.ExportRow{Key: row.Key, Summary: row.Summary, Description: row.Description, Type: row.Type,
			Priority: row.Priority, Status: row.Status, AssigneeEmail: row.Assignee, ReporterEmail: row.Reporter,
			Labels: row.Labels, Created: row.CreatedAt, Updated: row.UpdatedAt, DueDate: row.DueDate, StoryPoints: row.Points}
	}
	out := make([]port.ExportRow, 0, len(ids))
	for _, id := range ids {
		if row, ok := byID[id]; ok {
			out = append(out, row)
		}
	}
	return out, nil
}

// Dump reads the project, its issues, comments, worklogs and links with the size caps.
func (r *Reader) Dump(ctx context.Context, projectID uuid.UUID, now time.Time) (port.ProjectDump, error) {
	ex := r.ext(ctx)
	dump := port.ProjectDump{Version: "v1", ExportedAt: now, Issues: []map[string]any{}}
	var project struct {
		ID          uuid.UUID `db:"id"`
		Key         string    `db:"key"`
		Name        string    `db:"name"`
		Description string    `db:"description"`
		Type        string    `db:"type"`
		Visibility  string    `db:"visibility"`
		CreatedAt   time.Time `db:"created_at"`
	}
	err := sqlx.GetContext(ctx, ex, &project, `SELECT id, key, name, description, type, visibility, created_at
		FROM projects WHERE id = $1 AND deleted_at IS NULL`, projectID)
	if errors.Is(err, sql.ErrNoRows) {
		return dump, apperror.WithCode(apperror.KindNotFound, "project.not_found", "Proyecto no encontrado")
	}
	if err != nil {
		return dump, internal("no se pudo leer el proyecto", err)
	}
	dump.Project = map[string]any{"id": project.ID, "key": project.Key, "name": project.Name, "description": project.Description,
		"type": project.Type, "visibility": project.Visibility, "created_at": project.CreatedAt}

	var issues []struct {
		ID          uuid.UUID  `db:"id"`
		Key         string     `db:"key"`
		Summary     string     `db:"summary"`
		Description string     `db:"description"`
		Type        string     `db:"type"`
		Priority    string     `db:"priority"`
		Status      string     `db:"status"`
		Assignee    *string    `db:"assignee"`
		Reporter    string     `db:"reporter"`
		ParentID    *uuid.UUID `db:"parent_id"`
		DueDate     *time.Time `db:"due_date"`
		Points      *float64   `db:"story_points"`
		CreatedAt   time.Time  `db:"created_at"`
		UpdatedAt   time.Time  `db:"updated_at"`
	}
	if err := sqlx.SelectContext(ctx, ex, &issues, `
		SELECT i.id, p.key || '-' || i.key_number AS key, i.summary, i.description,
		       COALESCE(t.name, i.issue_type) AS type, COALESCE(pr.name, i.priority) AS priority,
		       COALESCE(s.name, i.status) AS status, a.email AS assignee, rp.email AS reporter, i.parent_id, i.due_date,
		       i.story_points, i.created_at, i.updated_at
		FROM issues i JOIN projects p ON p.id = i.project_id
		LEFT JOIN issue_types t ON t.id = i.issue_type_id LEFT JOIN priorities pr ON pr.id = i.priority_id
		LEFT JOIN statuses s ON s.id = i.status_id LEFT JOIN users a ON a.id = i.assignee_id
		JOIN users rp ON rp.id = i.reporter_id
		WHERE i.project_id = $1 AND i.deleted_at IS NULL ORDER BY i.key_number LIMIT $2`, projectID, dumpMaxIssues+1); err != nil {
		return dump, internal("no se pudieron leer las incidencias", err)
	}
	if len(issues) > dumpMaxIssues {
		issues, dump.Truncated = issues[:dumpMaxIssues], true
	}
	index := make(map[uuid.UUID]map[string]any, len(issues))
	for _, i := range issues {
		m := map[string]any{"id": i.ID, "key": i.Key, "summary": i.Summary, "description": i.Description, "type": i.Type,
			"priority": i.Priority, "status": i.Status, "assignee_email": i.Assignee, "reporter_email": i.Reporter,
			"parent_id": i.ParentID, "due_date": i.DueDate, "story_points": i.Points, "created_at": i.CreatedAt,
			"updated_at": i.UpdatedAt, "comments": []map[string]any{}, "worklogs": []map[string]any{}, "links": []map[string]any{}}
		index[i.ID] = m
		dump.Issues = append(dump.Issues, m)
	}

	var comments []struct {
		IssueID   uuid.UUID `db:"issue_id"`
		Author    string    `db:"author"`
		Body      string    `db:"body"`
		CreatedAt time.Time `db:"created_at"`
	}
	if err := sqlx.SelectContext(ctx, ex, &comments, `
		SELECT c.issue_id, u.email AS author, c.body, c.created_at FROM comments c
		JOIN issues i ON i.id = c.issue_id JOIN users u ON u.id = c.author_id
		WHERE i.project_id = $1 AND i.deleted_at IS NULL AND c.deleted_at IS NULL ORDER BY c.created_at LIMIT $2`,
		projectID, dumpMaxComments+1); err != nil {
		return dump, internal("no se pudieron leer los comentarios", err)
	}
	if len(comments) > dumpMaxComments {
		comments, dump.Truncated = comments[:dumpMaxComments], true
	}
	for _, c := range comments {
		if m, ok := index[c.IssueID]; ok {
			m["comments"] = append(m["comments"].([]map[string]any), map[string]any{"author_email": c.Author, "body": c.Body, "created_at": c.CreatedAt})
		}
	}

	var logs []struct {
		IssueID     uuid.UUID `db:"issue_id"`
		Author      string    `db:"author"`
		StartedAt   time.Time `db:"started_at"`
		Seconds     int       `db:"time_spent_seconds"`
		Description string    `db:"description"`
	}
	if err := sqlx.SelectContext(ctx, ex, &logs, `
		SELECT w.issue_id, u.email AS author, w.started_at, w.time_spent_seconds, w.description FROM worklogs w
		JOIN issues i ON i.id = w.issue_id JOIN users u ON u.id = w.author_id
		WHERE i.project_id = $1 AND i.deleted_at IS NULL AND w.deleted_at IS NULL ORDER BY w.started_at LIMIT $2`,
		projectID, dumpMaxWorklogs+1); err != nil {
		return dump, internal("no se pudieron leer los registros de trabajo", err)
	}
	if len(logs) > dumpMaxWorklogs {
		logs, dump.Truncated = logs[:dumpMaxWorklogs], true
	}
	for _, w := range logs {
		if m, ok := index[w.IssueID]; ok {
			m["worklogs"] = append(m["worklogs"].([]map[string]any), map[string]any{"author_email": w.Author,
				"started_at": w.StartedAt, "time_spent_seconds": w.Seconds, "description": w.Description})
		}
	}

	var links []struct {
		Source uuid.UUID `db:"source_issue_id"`
		Kind   string    `db:"kind"`
		Target string    `db:"target_key"`
	}
	if err := sqlx.SelectContext(ctx, ex, &links, `
		SELECT l.source_issue_id, l.kind, tp.key || '-' || ti.key_number AS target_key FROM issue_links l
		JOIN issues si ON si.id = l.source_issue_id JOIN issues ti ON ti.id = l.target_issue_id
		JOIN projects tp ON tp.id = ti.project_id
		WHERE si.project_id = $1 AND si.deleted_at IS NULL AND ti.deleted_at IS NULL ORDER BY l.created_at LIMIT $2`,
		projectID, dumpMaxLinks+1); err != nil {
		return dump, internal("no se pudieron leer los vinculos", err)
	}
	if len(links) > dumpMaxLinks {
		links, dump.Truncated = links[:dumpMaxLinks], true
	}
	for _, l := range links {
		if m, ok := index[l.Source]; ok {
			m["links"] = append(m["links"].([]map[string]any), map[string]any{"kind": l.Kind, "target": l.Target})
		}
	}
	return dump, nil
}
