package postgres

import (
	"time"

	"github.com/google/uuid"

	"github.com/jhonsferg/corelog/internal/issue/domain"
)

// issueColumns is the column list every issue read selects. The project key comes from the project row.
const issueColumns = `id, project_id, summary, description, status, priority, issue_type, parent_id,
		       due_date, story_points, rank, closed_at, reporter_id, assignee_id, key_number,
		       (SELECT p.key FROM projects p WHERE p.id = issues.project_id) AS project_key, created_at, updated_at`

type issueRow struct {
	ID          uuid.UUID  `db:"id"`
	ProjectID   uuid.UUID  `db:"project_id"`
	Summary     string     `db:"summary"`
	Description string     `db:"description"`
	Status      string     `db:"status"`
	Priority    string     `db:"priority"`
	IssueType   string     `db:"issue_type"`
	ParentID    *uuid.UUID `db:"parent_id"`
	DueDate     *time.Time `db:"due_date"`
	StoryPoints *float64   `db:"story_points"`
	Rank        string     `db:"rank"`
	ClosedAt    *time.Time `db:"closed_at"`
	ReporterID  uuid.UUID  `db:"reporter_id"`
	KeyNumber   int        `db:"key_number"`
	ProjectKey  string     `db:"project_key"`
	AssigneeID  *uuid.UUID `db:"assignee_id"`
	CreatedAt   time.Time  `db:"created_at"`
	UpdatedAt   time.Time  `db:"updated_at"`
}

func (r issueRow) toDomain() *domain.Issue {
	return &domain.Issue{
		ID:          r.ID,
		ProjectID:   r.ProjectID,
		Summary:     r.Summary,
		Description: r.Description,
		Status:      domain.Status(r.Status),
		Priority:    domain.Priority(r.Priority),
		IssueType:   domain.IssueType(r.IssueType),
		ParentID:    r.ParentID,
		DueDate:     r.DueDate,
		StoryPoints: r.StoryPoints,
		Rank:        r.Rank,
		ClosedAt:    r.ClosedAt,
		ReporterID:  r.ReporterID,
		KeyNumber:   r.KeyNumber,
		ProjectKey:  r.ProjectKey,
		AssigneeID:  r.AssigneeID,
		CreatedAt:   r.CreatedAt,
		UpdatedAt:   r.UpdatedAt,
	}
}

func fromDomain(t *domain.Issue) issueRow {
	return issueRow{
		ID:          t.ID,
		ProjectID:   t.ProjectID,
		Summary:     t.Summary,
		Description: t.Description,
		Status:      string(t.Status),
		Priority:    string(t.Priority),
		IssueType:   string(t.IssueType),
		ParentID:    t.ParentID,
		DueDate:     t.DueDate,
		StoryPoints: t.StoryPoints,
		Rank:        t.Rank,
		ClosedAt:    t.ClosedAt,
		ReporterID:  t.ReporterID,
		KeyNumber:   t.KeyNumber,
		ProjectKey:  t.ProjectKey,
		AssigneeID:  t.AssigneeID,
		CreatedAt:   t.CreatedAt,
		UpdatedAt:   t.UpdatedAt,
	}
}
