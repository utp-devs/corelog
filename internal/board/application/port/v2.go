package port

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/jhonsferg/corelog/internal/board/domain"
)

// WorkflowStatus is a status of the workflow of a project.
type WorkflowStatus struct {
	ID       uuid.UUID
	Name     string
	Category string
}

// BoardDetail is a board with its settings, columns, quick filters, swimlanes and the statuses of the workflow
// that its columns map. Statuses no column maps show in the implicit unmapped lane.
type BoardDetail struct {
	Board            *domain.Board
	WorkflowStatuses []WorkflowStatus
	CanManage        bool
	CanMoveCards     bool
}

// BoardSummary is the list entry of a board.
type BoardSummary struct {
	ID           uuid.UUID
	ProjectID    uuid.UUID
	Name         string
	Type         domain.Type
	IsDefault    bool
	ColumnCount  int
	SwimlaneMode domain.SwimlaneMode
}

// CreateBoardInput creates a board. Columns empty: one column per status category of the workflow.
type CreateBoardInput struct {
	Settings domain.Settings
	Type     domain.Type
	Columns  []domain.ColumnSpec
}

// UpdateBoardInput holds the settings to change; nil fields keep their value.
type UpdateBoardInput struct {
	Name              *string
	SwimlaneMode      *domain.SwimlaneMode
	Estimation        *domain.Estimation
	CardFields        []string
	CardFieldsSet     bool
	ShowUnmappedLane  *bool
	DoneRetentionDays *int
	DoneRetentionSet  bool
}

// QueryInput is a quick filter or a query swimlane to store.
type QueryInput struct {
	Name  string
	Query string
}

// SprintSelector narrows the cards of the view. Mode is "active", "all", "none" (cards without sprint) or "id".
type SprintSelector struct {
	Mode string
	ID   uuid.UUID
}

// ViewQuery are the parameters of the board view.
type ViewQuery struct {
	// QuickFilterIDs are the ids of the quick filters to apply (all must match).
	QuickFilterIDs []uuid.UUID
	Sprint         SprintSelector
	// Swimlane, when set, keeps only the cards of that lane id.
	Swimlane string
}

// ViewResult is the assembled board view.
type ViewResult struct {
	Detail *BoardDetail
	View   domain.View
	Sprint *SprintInfo
}

// SprintInfo describes the sprint the view is filtered by.
type SprintInfo struct {
	ID   uuid.UUID
	Name string
}

// MoveInput moves a card to a column.
type MoveInput struct {
	IssueID       uuid.UUID
	ColumnID      uuid.UUID
	BeforeIssueID uuid.UUID
	AfterIssueID  uuid.UUID
	// Swimlane is the lane the card was dropped on ("" when the board has no swimlanes).
	Swimlane string
	// TransitionID picks the transition when several lead into the column or when the client already asked
	// the user for the input the transition needs.
	TransitionID string
	Input        TransitionInput
}

// WIPFlag is the advisory state of the target column after a move.
type WIPFlag struct {
	ColumnID uuid.UUID
	Count    int
	Min      *int
	Max      *int
	State    domain.WIPState
	// Exceeded is true when the column is over its maximum.
	Exceeded bool
}

// MoveResult is the outcome of a card move.
type MoveResult struct {
	IssueID      uuid.UUID
	ColumnID     uuid.UUID
	Status       WorkflowStatus
	TransitionID string
	Transitioned bool
	Rank         string
	WIP          WIPFlag
}

// BoardsService is the in-port of the board v2 use cases.
type BoardsService interface {
	List(ctx context.Context, projectID, actorID uuid.UUID, globalRole string) ([]BoardSummary, error)
	Create(ctx context.Context, projectID, actorID uuid.UUID, globalRole string, in CreateBoardInput) (*BoardDetail, error)
	Get(ctx context.Context, boardID, actorID uuid.UUID, globalRole string) (*BoardDetail, error)
	Update(ctx context.Context, boardID, actorID uuid.UUID, globalRole string, in UpdateBoardInput) (*BoardDetail, error)
	Delete(ctx context.Context, boardID, actorID uuid.UUID, globalRole string) error
	ReplaceColumns(ctx context.Context, boardID, actorID uuid.UUID, globalRole string, columns []domain.ColumnSpec) (*BoardDetail, error)
	ReplaceQuickFilters(ctx context.Context, boardID, actorID uuid.UUID, globalRole string, filters []QueryInput) (*BoardDetail, error)
	ReplaceSwimlanes(ctx context.Context, boardID, actorID uuid.UUID, globalRole string, lanes []QueryInput) (*BoardDetail, error)
	View(ctx context.Context, boardID, actorID uuid.UUID, globalRole string, query ViewQuery) (*ViewResult, error)
	MoveCard(ctx context.Context, boardID, actorID uuid.UUID, globalRole string, in MoveInput) (*MoveResult, error)
}

// CardQuery selects the cards of a board view.
type CardQuery struct {
	ProjectID         uuid.UUID
	Sprint            SprintSelector
	DoneRetentionDays *int
	Now               time.Time
	Limit             int
}

// CardPlace is what a move needs to know about a card.
type CardPlace struct {
	StatusID   uuid.UUID
	Rank       string
	AssigneeID *uuid.UUID
	EpicID     *uuid.UUID
}

// BoardRepository is the persistence of boards v2.
type BoardRepository interface {
	ListByProject(ctx context.Context, projectID uuid.UUID) ([]domain.Board, error)
	// FindByID returns the board with columns (and their statuses), quick filters and swimlanes, or ErrBoardNotFound.
	FindByID(ctx context.Context, id uuid.UUID) (*domain.Board, error)
	Create(ctx context.Context, board *domain.Board) error
	UpdateSettings(ctx context.Context, board *domain.Board) error
	Delete(ctx context.Context, id uuid.UUID) error
	ReplaceColumns(ctx context.Context, boardID uuid.UUID, columns []domain.ColumnSpec, now time.Time) error
	ReplaceQuickFilters(ctx context.Context, boardID uuid.UUID, filters []domain.QuickFilter) error
	ReplaceSwimlanes(ctx context.Context, boardID uuid.UUID, lanes []domain.Swimlane) error
	WorkflowStatuses(ctx context.Context, projectID uuid.UUID) ([]WorkflowStatus, error)
	// Cards returns the live cards of the project in rank order, and the sprint filtered by when it is one.
	Cards(ctx context.Context, query CardQuery) ([]domain.Card, *SprintInfo, error)
	// CardPlace returns the card of a project or domain.ErrCardNotFound.
	CardPlace(ctx context.Context, projectID, issueID uuid.UUID) (*CardPlace, error)
	// RankNeighbors returns the rank of the anchor and the ranks of its neighbors in the project order,
	// ignoring the moved issue.
	RankNeighbors(ctx context.Context, projectID, anchorID, movedID uuid.UUID) (anchor, lower, upper string, err error)
	SetRank(ctx context.Context, projectID, issueID uuid.UUID, rank string, now time.Time) error
	// CountInStatuses counts the live cards of the project in the statuses.
	CountInStatuses(ctx context.Context, projectID uuid.UUID, statusIDs []uuid.UUID) (int, error)
}

// QueryFilter compiles the text of a quick filter or swimlane query. The minimal evaluator of the board
// implements it until the search phase provides a full query language.
type QueryFilter interface {
	Compile(text string) (domain.Matcher, error)
}

// TransitionOption is a transition the caller may run on a card.
type TransitionOption struct {
	ID                 string
	Name               string
	ToStatusID         uuid.UUID
	RequiredFields     []string
	RequiresComment    bool
	RequiresResolution bool
	RequiresAssignee   bool
}

// NeedsInput reports whether the transition asks the user for data.
func (o TransitionOption) NeedsInput() bool {
	return len(o.RequiredFields) > 0 || o.RequiresComment || o.RequiresResolution || o.RequiresAssignee
}

// TransitionInput is the data supplied for a transition.
type TransitionInput struct {
	Fields       map[string]string
	Comment      string
	ResolutionID uuid.UUID
	AssigneeID   uuid.UUID
}

// Empty reports whether no data was supplied.
func (i TransitionInput) Empty() bool {
	return len(i.Fields) == 0 && i.Comment == "" && i.ResolutionID == uuid.Nil && i.AssigneeID == uuid.Nil
}

// CardTransitions runs workflow transitions on cards. The composition root adapts the workflow engine of the
// issue module to it.
type CardTransitions interface {
	Options(ctx context.Context, actorID uuid.UUID, globalRole string, issueID uuid.UUID) ([]TransitionOption, error)
	Run(ctx context.Context, actorID uuid.UUID, globalRole string, issueID uuid.UUID, transitionID string, expectedStatusID uuid.UUID, input TransitionInput) error
}

// CardEditor applies the effects a drop on a swimlane has on a card.
type CardEditor interface {
	SetAssignee(ctx context.Context, actorID uuid.UUID, globalRole string, issueID uuid.UUID, assigneeID *uuid.UUID) error
	SetEpic(ctx context.Context, actorID uuid.UUID, globalRole string, issueID, epicID uuid.UUID) error
}
