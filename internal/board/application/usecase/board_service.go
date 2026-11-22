package usecase

import (
	"context"

	"github.com/google/uuid"
	"github.com/jhonsferg/corelog/internal/board/application/port"
)

type BoardService struct {
	repository port.Repository
	projects   port.ProjectAuthorizer
}

func NewBoardService(repository port.Repository, projects port.ProjectAuthorizer) *BoardService {
	return &BoardService{repository: repository, projects: projects}
}

var _ port.Service = (*BoardService)(nil)

func (s *BoardService) Get(ctx context.Context, projectID, actorID uuid.UUID, globalRole string) (*port.BoardView, error) {
	if _, err := s.projects.Get(ctx, actorID, globalRole, projectID); err != nil {
		return nil, err
	}
	board, err := s.repository.FindDefaultByProject(ctx, projectID)
	if err != nil {
		return nil, err
	}
	view := &port.BoardView{
		ID: board.ID, ProjectID: board.ProjectID, Name: board.Name, Type: string(board.Type),
		Columns: make([]port.ColumnView, 0, len(board.Columns)),
	}
	for _, column := range board.Columns {
		columnView := port.ColumnView{
			ID: column.ID, Name: column.Name, Status: string(column.Status), Position: column.Position,
			WIPLimit: column.WIPLimit, Cards: make([]port.CardView, 0, len(column.Cards)),
		}
		for _, card := range column.Cards {
			columnView.Cards = append(columnView.Cards, port.CardView{
				ID: card.ID, Type: card.Type, Summary: card.Summary, Status: string(card.Status),
				Priority: card.Priority, AssigneeID: card.AssigneeID, Assignee: card.Assignee,
				StoryPoints: card.StoryPoints, Rank: card.Rank,
			})
		}
		view.Columns = append(view.Columns, columnView)
	}
	return view, nil
}
