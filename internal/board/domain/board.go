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
}

type Column struct {
	ID        uuid.UUID
	BoardID   uuid.UUID
	Name      string
	Status    IssueStatus
	Position  int
	WIPLimit  *int
	CreatedAt time.Time
	UpdatedAt time.Time
}

func NewDefaultBoard(projectID uuid.UUID, boardType Type) (*Board, error) {
	if projectID == uuid.Nil || (boardType != TypeKanban && boardType != TypeScrum) {
		return nil, ErrInvalidBoard
	}
	now := time.Now().UTC()
	board := &Board{
		ID: uuid.New(), ProjectID: projectID, Name: "Default", Type: boardType,
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
			ID: uuid.New(), BoardID: board.ID, Name: definition.name,
			Status: definition.status, Position: position, CreatedAt: now, UpdatedAt: now,
		})
	}
	return board, nil
}
