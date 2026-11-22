package domain

import (
	"strings"
	"time"

	"github.com/google/uuid"
)

// DefaultProjectID es el proyecto al que se asignan los tickets creados sin project_id.
// Coincide con el proyecto CORE de las migraciones.
var DefaultProjectID = uuid.MustParse("00000000-0000-4000-8000-000000000001")

type Ticket struct {
	ID          uuid.UUID
	ProjectID   uuid.UUID
	Title       string
	Description string
	Status      Status
	Priority    Priority
	IssueType   IssueType
	ParentID    *uuid.UUID
	DueDate     *time.Time
	StoryPoints *float64
	Rank        string
	ClosedAt    *time.Time
	RequesterID uuid.UUID
	AssigneeID  *uuid.UUID
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

type IssueType string

const (
	IssueTypeTask    IssueType = "task"
	IssueTypeBug     IssueType = "bug"
	IssueTypeStory   IssueType = "story"
	IssueTypeEpic    IssueType = "epic"
	IssueTypeSubtask IssueType = "subtask"
)

func (t IssueType) IsValid() bool {
	switch t {
	case IssueTypeTask, IssueTypeBug, IssueTypeStory, IssueTypeEpic, IssueTypeSubtask:
		return true
	default:
		return false
	}
}

func NewTicket(title, description string, priority Priority, requesterID uuid.UUID) (*Ticket, error) {
	if title == "" {
		return nil, ErrInvalidTitle
	}
	if description == "" {
		return nil, ErrInvalidDescription
	}
	if !priority.IsValid() {
		return nil, ErrInvalidPriority
	}
	if requesterID == uuid.Nil {
		return nil, ErrInvalidRequester
	}

	now := time.Now().UTC()
	return &Ticket{
		Title:       title,
		Description: description,
		Status:      StatusOpen,
		Priority:    priority,
		IssueType:   IssueTypeTask,
		Rank:        "m",
		RequesterID: requesterID,
		CreatedAt:   now,
		UpdatedAt:   now,
	}, nil
}

func (t *Ticket) ChangeStatus(newStatus Status) error {
	if !newStatus.IsValid() {
		return ErrInvalidStatus
	}
	if !t.Status.CanTransitionTo(newStatus) {
		return ErrIllegalTransition
	}
	t.Status = newStatus
	if newStatus == StatusClosed {
		now := time.Now().UTC()
		t.ClosedAt = &now
	} else if newStatus != StatusClosed {
		t.ClosedAt = nil
	}
	t.UpdatedAt = time.Now().UTC()
	return nil
}

func (t *Ticket) UpdateMetadata(issueType IssueType, parentID *uuid.UUID, dueDate *time.Time, storyPoints *float64) error {
	if !issueType.IsValid() {
		return ErrInvalidIssueType
	}
	if storyPoints != nil && (*storyPoints < 0 || *storyPoints > 999) {
		return ErrInvalidStoryPoints
	}
	t.IssueType = issueType
	t.ParentID = parentID
	t.DueDate = dueDate
	t.StoryPoints = storyPoints
	t.UpdatedAt = time.Now().UTC()
	return nil
}

func (t *Ticket) UpdateDetails(title, description string, priority Priority) error {
	if strings.TrimSpace(title) == "" {
		return ErrInvalidTitle
	}
	if strings.TrimSpace(description) == "" {
		return ErrInvalidDescription
	}
	if !priority.IsValid() {
		return ErrInvalidPriority
	}
	t.Title = strings.TrimSpace(title)
	t.Description = strings.TrimSpace(description)
	t.Priority = priority
	t.UpdatedAt = time.Now().UTC()
	return nil
}

func (t *Ticket) AssignTo(assigneeID uuid.UUID) error {
	t.AssigneeID = &assigneeID
	if t.Status == StatusOpen {
		if err := t.ChangeStatus(StatusInProgress); err != nil {
			return err
		}
	}
	t.UpdatedAt = time.Now().UTC()
	return nil
}

func (t *Ticket) IsClosed() bool {
	return t.Status == StatusClosed
}

func (t *Ticket) UpdateRank(rank string) error {
	rank = strings.TrimSpace(rank)
	if rank == "" || len(rank) > 64 {
		return ErrInvalidRank
	}
	t.Rank = rank
	t.UpdatedAt = time.Now().UTC()
	return nil
}
