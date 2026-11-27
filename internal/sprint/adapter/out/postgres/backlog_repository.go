package postgres

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"

	"github.com/jhonsferg/corelog/internal/sprint/application/port"
	"github.com/jhonsferg/corelog/internal/sprint/domain"
)

// maxBacklogRows acota la lectura del backlog; si se supera se marca como truncada.
const maxBacklogRows = 3000

type backlogRow struct {
	ID                      uuid.UUID  `db:"id"`
	Key                     string     `db:"key"`
	Summary                 string     `db:"summary"`
	IssueType               string     `db:"issue_type"`
	IssueTypeName           string     `db:"issue_type_name"`
	IssueTypeIcon           string     `db:"issue_type_icon"`
	IssueTypeColor          string     `db:"issue_type_color"`
	Status                  string     `db:"status"`
	StatusName              string     `db:"status_name"`
	StatusCategory          string     `db:"status_category"`
	Priority                string     `db:"priority"`
	PriorityName            string     `db:"priority_name"`
	PriorityIcon            string     `db:"priority_icon"`
	PriorityColor           string     `db:"priority_color"`
	AssigneeID              *uuid.UUID `db:"assignee_id"`
	AssigneeName            string     `db:"assignee_name"`
	StoryPoints             *float64   `db:"story_points"`
	OriginalEstimateSeconds *int64     `db:"original_estimate_seconds"`
	Rank                    string     `db:"rank"`
	EpicID                  *uuid.UUID `db:"epic_id"`
	EpicKey                 string     `db:"epic_key"`
	EpicSummary             string     `db:"epic_summary"`
	Flagged                 bool       `db:"flagged"`
	SprintID                *uuid.UUID `db:"sprint_id"`
}

// Backlog lee el backlog de un proyecto con un numero fijo de sentencias: sprints abiertos, incidencias de
// todas las secciones (con tipo, estado, prioridad y responsable ya resueltos), etiquetas en lote y epicas.
func (r *SprintRepository) Backlog(ctx context.Context, projectID uuid.UUID, filter port.BacklogFilter) (*port.BacklogData, error) {
	var sprintRows []sprintRow
	if err := sqlx.SelectContext(ctx, r.ext(ctx), &sprintRows, `
		SELECT `+sprintColumns+` FROM sprints
		WHERE project_id = $1 AND status IN ('active', 'planned')
		  AND EXISTS (SELECT 1 FROM projects p WHERE p.id = sprints.project_id AND p.deleted_at IS NULL)
		ORDER BY CASE status WHEN 'active' THEN 0 ELSE 1 END, created_at, id`, projectID); err != nil {
		return nil, internal("failed to read backlog sprints", err)
	}
	data := &port.BacklogData{Sprints: make([]*domain.Sprint, 0, len(sprintRows))}
	sprintIDs := make([]uuid.UUID, 0, len(sprintRows))
	for _, row := range sprintRows {
		data.Sprints = append(data.Sprints, row.toDomain())
		sprintIDs = append(sprintIDs, row.ID)
	}

	query := `
		SELECT i.id, p.key || '-' || i.key_number AS key, i.summary, i.issue_type,
		       COALESCE(it.name, '') AS issue_type_name, COALESCE(it.icon, '') AS issue_type_icon, COALESCE(it.color, '') AS issue_type_color,
		       i.status, COALESCE(st.name, '') AS status_name, corelog_issue_category(i.status_id, i.status) AS status_category,
		       i.priority, COALESCE(pr.name, '') AS priority_name, COALESCE(pr.icon, '') AS priority_icon, COALESCE(pr.color, '') AS priority_color,
		       i.assignee_id, COALESCE(u.display_name, u.name, '') AS assignee_name,
		       i.story_points, i.original_estimate_seconds, i.rank,
		       ep.id AS epic_id, COALESCE(p.key || '-' || ep.key_number, '') AS epic_key, COALESCE(ep.summary, '') AS epic_summary,
		       i.flagged, i.sprint_id
		FROM issues i
		JOIN projects p ON p.id = i.project_id AND p.deleted_at IS NULL
		LEFT JOIN issue_types it ON it.id = i.issue_type_id
		LEFT JOIN statuses st ON st.id = i.status_id
		LEFT JOIN priorities pr ON pr.id = i.priority_id
		LEFT JOIN users u ON u.id = i.assignee_id
		LEFT JOIN issues ep ON ep.id = i.parent_id AND ep.issue_type = 'epic' AND ep.deleted_at IS NULL
		WHERE i.project_id = $1 AND i.deleted_at IS NULL AND ` + notEpic + `
		  AND (i.sprint_id = ANY($2::uuid[]) OR (i.sprint_id IS NULL AND ` + backlogOnlyPending + `))`
	args := []any{projectID, idStrings(sprintIDs)}
	if filter.EpicID != nil {
		args = append(args, *filter.EpicID)
		query += fmt.Sprintf(` AND i.parent_id = $%d`, len(args))
	}
	if q := strings.TrimSpace(filter.Query); q != "" {
		args = append(args, q)
		query += fmt.Sprintf(` AND (strpos(lower(i.summary), lower($%d)) > 0 OR strpos(lower(p.key || '-' || i.key_number), lower($%d)) > 0)`, len(args), len(args))
	}
	query += fmt.Sprintf(` ORDER BY i.rank, i.id LIMIT %d`, maxBacklogRows+1)
	var rows []backlogRow
	if err := sqlx.SelectContext(ctx, r.ext(ctx), &rows, query, args...); err != nil {
		return nil, internal("failed to read backlog issues", err)
	}
	if len(rows) > maxBacklogRows {
		rows = rows[:maxBacklogRows]
		data.Truncated = true
	}
	data.Issues = make([]port.BacklogIssue, 0, len(rows))
	index := make(map[uuid.UUID]int, len(rows))
	ids := make([]uuid.UUID, 0, len(rows))
	for _, row := range rows {
		index[row.ID] = len(data.Issues)
		ids = append(ids, row.ID)
		data.Issues = append(data.Issues, port.BacklogIssue{
			ID: row.ID, Key: row.Key, Summary: row.Summary, IssueType: row.IssueType, IssueTypeName: row.IssueTypeName,
			IssueTypeIcon: row.IssueTypeIcon, IssueTypeColor: row.IssueTypeColor, Status: row.Status, StatusName: row.StatusName,
			StatusCategory: row.StatusCategory, Priority: row.Priority, PriorityName: row.PriorityName, PriorityIcon: row.PriorityIcon,
			PriorityColor: row.PriorityColor, AssigneeID: row.AssigneeID, AssigneeName: row.AssigneeName, StoryPoints: row.StoryPoints,
			OriginalEstimateSeconds: row.OriginalEstimateSeconds, Rank: row.Rank, EpicID: row.EpicID, EpicKey: row.EpicKey,
			EpicSummary: row.EpicSummary, Flagged: row.Flagged, SprintID: row.SprintID, Labels: []port.BacklogLabel{},
		})
	}

	if len(ids) > 0 {
		var labels []struct {
			IssueID uuid.UUID `db:"issue_id"`
			ID      uuid.UUID `db:"id"`
			Name    string    `db:"name"`
			Color   string    `db:"color"`
		}
		if err := sqlx.SelectContext(ctx, r.ext(ctx), &labels, `
			SELECT il.issue_id, l.id, l.name, l.color
			FROM issue_labels il JOIN labels l ON l.id = il.label_id
			WHERE il.issue_id = ANY($1::uuid[])
			ORDER BY l.name`, idStrings(ids)); err != nil {
			return nil, internal("failed to read backlog labels", err)
		}
		for _, l := range labels {
			i := index[l.IssueID]
			data.Issues[i].Labels = append(data.Issues[i].Labels, port.BacklogLabel{ID: l.ID, Name: l.Name, Color: l.Color})
		}
	}

	var epics []struct {
		ID         uuid.UUID `db:"id"`
		Key        string    `db:"key"`
		Summary    string    `db:"summary"`
		Status     string    `db:"status"`
		Total      int       `db:"total"`
		Done       int       `db:"done"`
		Points     float64   `db:"points"`
		DonePoints float64   `db:"done_points"`
	}
	if err := sqlx.SelectContext(ctx, r.ext(ctx), &epics, `
		SELECT e.id, p.key || '-' || e.key_number AS key, e.summary, corelog_issue_category(e.status_id, e.status) AS status,
		       COUNT(c.id) AS total,
		       COUNT(c.id) FILTER (WHERE corelog_issue_category(c.status_id, c.status) = 'done') AS done,
		       COALESCE(SUM(c.story_points), 0) AS points,
		       COALESCE(SUM(c.story_points) FILTER (WHERE corelog_issue_category(c.status_id, c.status) = 'done'), 0) AS done_points
		FROM issues e
		JOIN projects p ON p.id = e.project_id AND p.deleted_at IS NULL
		LEFT JOIN issues c ON c.parent_id = e.id AND c.deleted_at IS NULL
		WHERE e.project_id = $1 AND e.issue_type = 'epic' AND e.deleted_at IS NULL
		GROUP BY e.id, p.key, e.key_number, e.summary, e.status_id, e.status, e.rank
		ORDER BY e.rank, e.id`, projectID); err != nil {
		return nil, internal("failed to read backlog epics", err)
	}
	data.Epics = make([]port.BacklogEpic, 0, len(epics))
	for _, e := range epics {
		data.Epics = append(data.Epics, port.BacklogEpic{ID: e.ID, Key: e.Key, Summary: e.Summary, Status: e.Status,
			Total: e.Total, Done: e.Done, Points: e.Points, DonePoints: e.DonePoints})
	}
	return data, nil
}
