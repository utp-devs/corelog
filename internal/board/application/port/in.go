package port

import (
	"context"

	"github.com/google/uuid"
	projectdomain "github.com/jhonsferg/corelog/internal/project/domain"
)

type ProjectAuthorizer interface {
	Get(ctx context.Context, actorID uuid.UUID, globalRole string, id uuid.UUID) (*projectdomain.Project, error)
}

type Service interface {
	Get(ctx context.Context, projectID, actorID uuid.UUID, globalRole string) (*BoardView, error)
	MoveCard(ctx context.Context, projectID, actorID uuid.UUID, globalRole string, issueID, columnID uuid.UUID) error
	UpdateColumnWIP(ctx context.Context, projectID, actorID uuid.UUID, globalRole string, columnID uuid.UUID, limit *int) error
}

type BoardView struct {
	ID        uuid.UUID
	ProjectID uuid.UUID
	Name      string
	Type      string
	Columns   []ColumnView
}

type ColumnView struct {
	ID       uuid.UUID
	Name     string
	Status   string
	Position int
	WIPLimit *int
	Cards    []CardView
}

type CardView struct {
	ID          uuid.UUID
	Type        string
	Summary     string
	Status      string
	Priority    string
	AssigneeID  *uuid.UUID
	Assignee    *string
	StoryPoints *float64
	Rank        string
}
