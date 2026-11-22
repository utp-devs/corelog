package postgres

import (
	"time"

	"github.com/google/uuid"

	"github.com/jhonsferg/corelog/internal/ticket/domain"
)

type ticketRow struct {
	ID          uuid.UUID  `db:"id"`
	ProjectID   uuid.UUID  `db:"project_id"`
	Title       string     `db:"title"`
	Description string     `db:"description"`
	Status      string     `db:"status"`
	Priority    string     `db:"priority"`
	IssueType   string     `db:"issue_type"`
	ParentID    *uuid.UUID `db:"parent_id"`
	DueDate     *time.Time `db:"due_date"`
	StoryPoints *float64   `db:"story_points"`
	Rank        string     `db:"rank"`
	ClosedAt    *time.Time `db:"closed_at"`
	RequesterID uuid.UUID  `db:"requester_id"`
	AssigneeID  *uuid.UUID `db:"assignee_id"`
	CreatedAt   time.Time  `db:"created_at"`
	UpdatedAt   time.Time  `db:"updated_at"`
}

func (r ticketRow) toDomain() *domain.Ticket {
	return &domain.Ticket{
		ID:          r.ID,
		ProjectID:   r.ProjectID,
		Title:       r.Title,
		Description: r.Description,
		Status:      domain.Status(r.Status),
		Priority:    domain.Priority(r.Priority),
		IssueType:   domain.IssueType(r.IssueType),
		ParentID:    r.ParentID,
		DueDate:     r.DueDate,
		StoryPoints: r.StoryPoints,
		Rank:        r.Rank,
		ClosedAt:    r.ClosedAt,
		RequesterID: r.RequesterID,
		AssigneeID:  r.AssigneeID,
		CreatedAt:   r.CreatedAt,
		UpdatedAt:   r.UpdatedAt,
	}
}

func fromDomain(t *domain.Ticket) ticketRow {
	return ticketRow{
		ID:          t.ID,
		ProjectID:   t.ProjectID,
		Title:       t.Title,
		Description: t.Description,
		Status:      string(t.Status),
		Priority:    string(t.Priority),
		IssueType:   string(t.IssueType),
		ParentID:    t.ParentID,
		DueDate:     t.DueDate,
		StoryPoints: t.StoryPoints,
		Rank:        t.Rank,
		ClosedAt:    t.ClosedAt,
		RequesterID: t.RequesterID,
		AssigneeID:  t.AssigneeID,
		CreatedAt:   t.CreatedAt,
		UpdatedAt:   t.UpdatedAt,
	}
}
