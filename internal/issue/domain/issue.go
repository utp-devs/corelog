package domain

import (
	"strings"
	"time"

	"github.com/google/uuid"
)

// DefaultProjectID es el proyecto al que se asignan los issues creados sin project_id.
// Coincide con el proyecto CORE de las migraciones.
var DefaultProjectID = uuid.MustParse("00000000-0000-4000-8000-000000000001")

type Issue struct {
	ID          uuid.UUID
	ProjectID   uuid.UUID
	Summary     string
	Description string
	Status      Status
	Priority    Priority
	IssueType   IssueType
	ParentID    *uuid.UUID
	DueDate     *time.Time
	StoryPoints *float64
	Rank        string
	ClosedAt    *time.Time
	ReporterID  uuid.UUID
	// KeyNumber is the per-project issue number; ProjectKey is read from the project and not stored on the issue.
	KeyNumber  int
	ProjectKey string
	AssigneeID *uuid.UUID
	CreatedAt  time.Time
	UpdatedAt  time.Time
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

func NewIssue(summary, description string, priority Priority, reporterID uuid.UUID) (*Issue, error) {
	if summary == "" {
		return nil, ErrInvalidSummary
	}
	if description == "" {
		return nil, ErrInvalidDescription
	}
	if !priority.IsValid() {
		return nil, ErrInvalidPriority
	}
	if reporterID == uuid.Nil {
		return nil, ErrInvalidReporter
	}

	now := time.Now().UTC()
	return &Issue{
		Summary:     summary,
		Description: description,
		Status:      StatusOpen,
		Priority:    priority,
		IssueType:   IssueTypeTask,
		Rank:        "m",
		ReporterID:  reporterID,
		CreatedAt:   now,
		UpdatedAt:   now,
	}, nil
}

func (t *Issue) ChangeStatus(newStatus Status) error {
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

func (t *Issue) UpdateMetadata(issueType IssueType, parentID *uuid.UUID, dueDate *time.Time, storyPoints *float64) error {
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

func (t *Issue) UpdateDetails(summary, description string, priority Priority) error {
	if strings.TrimSpace(summary) == "" {
		return ErrInvalidSummary
	}
	if strings.TrimSpace(description) == "" {
		return ErrInvalidDescription
	}
	if !priority.IsValid() {
		return ErrInvalidPriority
	}
	t.Summary = strings.TrimSpace(summary)
	t.Description = strings.TrimSpace(description)
	t.Priority = priority
	t.UpdatedAt = time.Now().UTC()
	return nil
}

func (t *Issue) AssignTo(assigneeID uuid.UUID) error {
	t.AssigneeID = &assigneeID
	if t.Status == StatusOpen {
		if err := t.ChangeStatus(StatusInProgress); err != nil {
			return err
		}
	}
	t.UpdatedAt = time.Now().UTC()
	return nil
}

// ClearAssignee quita la persona asignada. El estado no cambia: desasignar no reabre ni mueve la incidencia.
func (t *Issue) ClearAssignee() {
	t.AssigneeID = nil
	t.UpdatedAt = time.Now().UTC()
}

func (t *Issue) IsClosed() bool {
	return t.Status == StatusClosed
}

func (t *Issue) UpdateRank(rank string) error {
	rank = strings.TrimSpace(rank)
	if rank == "" || len(rank) > 64 {
		return ErrInvalidRank
	}
	t.Rank = rank
	t.UpdatedAt = time.Now().UTC()
	return nil
}
