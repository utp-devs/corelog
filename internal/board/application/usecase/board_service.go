package usecase

import (
	"context"

	"github.com/google/uuid"
	"github.com/jhonsferg/corelog/internal/board/application/port"
	"github.com/jhonsferg/corelog/internal/board/domain"
)

type BoardService struct {
	repository port.Repository
	projects   port.ProjectAuthorizer
	authorizer port.Authorizer
	mover      port.IssueMover
}

func NewBoardService(repository port.Repository, projects port.ProjectAuthorizer, authorizer port.Authorizer, mover port.IssueMover) *BoardService {
	return &BoardService{repository: repository, projects: projects, authorizer: authorizer, mover: mover}
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
	view.CanManageBoard = s.authorizer.Authorize(ctx, port.ActionManageBoard, actorID, globalRole, projectID) == nil
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

// MoveCard mueve una incidencia a la columna indicada. La columna define el estado destino (ADR-004).
// Respeta el limite WIP de la columna destino y delega en el modulo ticket la regla de flujo y permisos.
func (s *BoardService) MoveCard(ctx context.Context, projectID, actorID uuid.UUID, globalRole string, issueID, columnID uuid.UUID) error {
	if _, err := s.projects.Get(ctx, actorID, globalRole, projectID); err != nil {
		return err
	}
	// El permiso va antes que el limite WIP para no revelar el estado de una columna a quien no puede mover.
	if err := s.authorizer.Authorize(ctx, port.ActionMoveCards, actorID, globalRole, projectID); err != nil {
		return err
	}
	board, err := s.repository.FindDefaultByProject(ctx, projectID)
	if err != nil {
		return err
	}
	var target, source *domain.Column
	for i := range board.Columns {
		column := &board.Columns[i]
		if column.ID == columnID {
			target = column
		}
		for _, card := range column.Cards {
			if card.ID == issueID {
				source = column
			}
		}
	}
	if target == nil {
		return domain.ErrColumnNotFound
	}
	if source == nil {
		return domain.ErrCardNotFound
	}
	if source.ID == target.ID {
		return nil
	}
	if target.WIPLimit != nil && len(target.Cards) >= *target.WIPLimit {
		return domain.ErrWIPLimitExceeded
	}
	return s.mover.MoveIssue(ctx, actorID, globalRole, projectID, issueID, string(target.Status))
}

// UpdateColumnWIP fija o quita el limite WIP de una columna. Solo lead y admin (manage_board).
func (s *BoardService) UpdateColumnWIP(ctx context.Context, projectID, actorID uuid.UUID, globalRole string, columnID uuid.UUID, limit *int) error {
	if err := s.authorizer.Authorize(ctx, port.ActionManageBoard, actorID, globalRole, projectID); err != nil {
		return err
	}
	if limit != nil && *limit < 1 {
		return domain.ErrInvalidWIPLimit
	}
	board, err := s.repository.FindDefaultByProject(ctx, projectID)
	if err != nil {
		return err
	}
	found := false
	for _, column := range board.Columns {
		if column.ID == columnID {
			found = true
		}
	}
	if !found {
		return domain.ErrColumnNotFound
	}
	return s.repository.UpdateWIPLimit(ctx, columnID, limit)
}
