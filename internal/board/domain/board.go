package domain

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

type Type string

const (
	TypeKanban Type = "kanban"
	TypeScrum  Type = "scrum"
)

type IssueStatus string

const (
	StatusOpen       IssueStatus = "open"
	StatusInProgress IssueStatus = "in_progress"
	StatusResolved   IssueStatus = "resolved"
	StatusClosed     IssueStatus = "closed"
)

// Board is a view of the issues of a project. Its columns map to sets of workflow statuses.
type Board struct {
	ID        uuid.UUID
	ProjectID uuid.UUID
	Name      string
	Type      Type
	Config    json.RawMessage
	IsDefault bool
	Columns   []Column
	CreatedAt time.Time
	UpdatedAt time.Time

	SwimlaneMode     SwimlaneMode
	Estimation       Estimation
	CardFields       []string
	ShowUnmappedLane bool
	// DoneRetentionDays hides the cards of done statuses not updated for that many days; nil keeps them all.
	DoneRetentionDays *int
	QuickFilters      []QuickFilter
	Swimlanes         []Swimlane
}

// Column is a board column. WIPLimit is the maximum number of cards (max_issues in the API). The legacy Status is
// kept for boards created before columns were bound to workflow statuses.
type Column struct {
	ID        uuid.UUID
	BoardID   uuid.UUID
	Name      string
	Status    IssueStatus
	Position  int
	WIPLimit  *int
	MinIssues *int
	Color     string
	StatusIDs []uuid.UUID
	CreatedAt time.Time
	UpdatedAt time.Time
	Cards     []IssueCard
}

type IssueCard struct {
	ID          uuid.UUID
	Type        string
	Summary     string
	Status      IssueStatus
	Priority    string
	AssigneeID  *uuid.UUID
	Assignee    *string
	StoryPoints *float64
	Rank        string
}

func NewDefaultBoard(projectID uuid.UUID, boardType Type) (*Board, error) {
	if projectID == uuid.Nil || (boardType != TypeKanban && boardType != TypeScrum) {
		return nil, ErrInvalidBoard
	}
	now := time.Now().UTC()
	board := &Board{
		ProjectID: projectID, Name: "Default", Type: boardType,
		Config: json.RawMessage(`{}`), IsDefault: true, CreatedAt: now, UpdatedAt: now,
	}
	definitions := []struct {
		name   string
		status IssueStatus
	}{
		{name: "Open", status: StatusOpen},
		{name: "In progress", status: StatusInProgress},
		{name: "Resolved", status: StatusResolved},
		{name: "Closed", status: StatusClosed},
	}
	board.Columns = make([]Column, 0, len(definitions))
	for position, definition := range definitions {
		board.Columns = append(board.Columns, Column{
			BoardID: board.ID, Name: definition.name,
			Status: definition.status, Position: position, CreatedAt: now, UpdatedAt: now,
		})
	}
	return board, nil
}
