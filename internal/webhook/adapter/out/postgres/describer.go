package postgres

import (
	"context"
	"database/sql"
	"errors"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"

	"github.com/jhonsferg/corelog/internal/platform/events"
	"github.com/jhonsferg/corelog/internal/webhook/application/port"
)

// Describer is the PostgreSQL adapter of port.EventDescriber. It reads projects, issues and users to enrich
// the payload of an event.
type Describer struct{ db *sqlx.DB }

var _ port.EventDescriber = (*Describer)(nil)

// NewDescriber returns a describer bound to db.
func NewDescriber(db *sqlx.DB) *Describer { return &Describer{db: db} }

// Describe resolves the organization, project, issue and actor of an event.
func (d *Describer) Describe(ctx context.Context, e events.Event) (port.EventInfo, bool, error) {
	info := port.EventInfo{}
	if e.ProjectID != nil {
		var p struct {
			ID    uuid.UUID `db:"id"`
			Key   string    `db:"key"`
			Name  string    `db:"name"`
			OrgID uuid.UUID `db:"organization_id"`
		}
		err := sqlx.GetContext(ctx, d.db, &p, `SELECT id, key, name, organization_id FROM projects WHERE id = $1`, *e.ProjectID)
		if errors.Is(err, sql.ErrNoRows) {
			return info, false, nil
		}
		if err != nil {
			return info, false, err
		}
		info.OrgID = p.OrgID
		info.Project = &port.ProjectInfo{ID: p.ID, Key: p.Key, Name: p.Name}
	} else if e.OrgID != nil {
		info.OrgID = *e.OrgID
	} else {
		return info, false, nil
	}
	if id, ok := issueID(e); ok {
		var i struct {
			ID         uuid.UUID  `db:"id"`
			Key        string     `db:"key"`
			Summary    string     `db:"summary"`
			Status     string     `db:"status"`
			Priority   string     `db:"priority"`
			IssueType  string     `db:"issue_type"`
			AssigneeID *uuid.UUID `db:"assignee_id"`
			ReporterID uuid.UUID  `db:"reporter_id"`
		}
		err := sqlx.GetContext(ctx, d.db, &i, `
			SELECT i.id, p.key || '-' || i.key_number AS key, i.summary,
			       COALESCE(s.name, i.status) AS status, COALESCE(pr.name, i.priority) AS priority,
			       COALESCE(t.name, i.issue_type) AS issue_type, i.assignee_id, i.reporter_id
			FROM issues i JOIN projects p ON p.id = i.project_id
			LEFT JOIN statuses s ON s.id = i.status_id
			LEFT JOIN priorities pr ON pr.id = i.priority_id
			LEFT JOIN issue_types t ON t.id = i.issue_type_id
			WHERE i.id = $1`, id)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return info, false, err
		}
		if err == nil {
			info.Issue = &port.IssueInfo{ID: i.ID, Key: i.Key, Summary: i.Summary, Status: i.Status, Priority: i.Priority,
				IssueType: i.IssueType, AssigneeID: i.AssigneeID, ReporterID: i.ReporterID}
		}
	}
	if e.ActorID != nil {
		err := sqlx.GetContext(ctx, d.db, &info.ActorName, `SELECT display_name FROM users WHERE id = $1`, *e.ActorID)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return info, false, err
		}
	}
	return info, true, nil
}

// issueID extracts the issue of an issue or comment event.
func issueID(e events.Event) (uuid.UUID, bool) {
	if e.EntityType == "issue" && e.EntityID != nil {
		return *e.EntityID, true
	}
	if raw, ok := e.Payload["issue_id"].(string); ok {
		if id, err := uuid.Parse(raw); err == nil {
			return id, true
		}
	}
	return uuid.Nil, false
}
