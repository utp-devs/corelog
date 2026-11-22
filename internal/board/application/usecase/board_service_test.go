package usecase

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"

	"github.com/jhonsferg/corelog/internal/board/application/port"
	"github.com/jhonsferg/corelog/internal/board/domain"
	projectdomain "github.com/jhonsferg/corelog/internal/project/domain"
)

type boardRepositoryStub struct {
	board *domain.Board
	err   error
	calls int
}

func (s *boardRepositoryStub) FindDefaultByProject(context.Context, uuid.UUID) (*domain.Board, error) {
	s.calls++
	return s.board, s.err
}

type projectAuthorizerStub struct {
	err error
}

func (s projectAuthorizerStub) Get(context.Context, uuid.UUID, string, uuid.UUID) (*projectdomain.Project, error) {
	return nil, s.err
}

func TestBoardServiceGetReturnsColumnsAndCards(t *testing.T) {
	projectID, boardID, columnID, issueID := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	assigneeName := "Ada Lovelace"
	points := 5.0
	repository := &boardRepositoryStub{board: &domain.Board{
		ID: boardID, ProjectID: projectID, Name: "Default", Type: domain.TypeKanban,
		Columns: []domain.Column{{
			ID: columnID, Name: "In progress", Status: domain.StatusInProgress, Position: 1,
			Cards: []domain.IssueCard{{
				ID: issueID, Type: "story", Summary: "Board view", Status: domain.StatusInProgress,
				Priority: "high", Assignee: &assigneeName, StoryPoints: &points, Rank: "m",
			}},
		}},
	}}
	service := NewBoardService(repository, projectAuthorizerStub{})

	view, err := service.Get(context.Background(), projectID, uuid.New(), "agent")
	if err != nil {
		t.Fatal(err)
	}
	if repository.calls != 1 || view.ID != boardID || len(view.Columns) != 1 || len(view.Columns[0].Cards) != 1 {
		t.Fatalf("unexpected board view: %#v", view)
	}
	if got := view.Columns[0].Cards[0]; got.ID != issueID || got.Assignee == nil || *got.Assignee != assigneeName || got.StoryPoints == nil || *got.StoryPoints != points {
		t.Fatalf("card data not mapped correctly: %#v", got)
	}
}

func TestBoardServiceGetChecksProjectAccessBeforeReadingBoard(t *testing.T) {
	accessErr := errors.New("project access denied")
	repository := &boardRepositoryStub{}
	service := NewBoardService(repository, projectAuthorizerStub{err: accessErr})

	_, err := service.Get(context.Background(), uuid.New(), uuid.New(), "requester")
	if !errors.Is(err, accessErr) {
		t.Fatalf("Get() error = %v, want %v", err, accessErr)
	}
	if repository.calls != 0 {
		t.Fatalf("board repository called %d times after denied project access", repository.calls)
	}
}

var _ port.Repository = (*boardRepositoryStub)(nil)
