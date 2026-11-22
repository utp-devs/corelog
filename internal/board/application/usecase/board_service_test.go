package usecase

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"

	"github.com/jhonsferg/corelog/internal/board/application/port"
	"github.com/jhonsferg/corelog/internal/board/domain"
	"github.com/jhonsferg/corelog/internal/platform/apperror"
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

func (s *boardRepositoryStub) UpdateWIPLimit(context.Context, uuid.UUID, *int) error {
	return nil
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
	service := NewBoardService(repository, projectAuthorizerStub{}, fakeAuthorizer{}, &fakeMover{})

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
	service := NewBoardService(repository, projectAuthorizerStub{err: accessErr}, fakeAuthorizer{}, &fakeMover{})

	_, err := service.Get(context.Background(), uuid.New(), uuid.New(), "requester")
	if !errors.Is(err, accessErr) {
		t.Fatalf("Get() error = %v, want %v", err, accessErr)
	}
	if repository.calls != 0 {
		t.Fatalf("board repository called %d times after denied project access", repository.calls)
	}
}

var _ port.Repository = (*boardRepositoryStub)(nil)

type fakeBoardRepo struct {
	board    *domain.Board
	wipSet   bool
	wipValue *int
}

func (r *fakeBoardRepo) FindDefaultByProject(context.Context, uuid.UUID) (*domain.Board, error) {
	return r.board, nil
}

func (r *fakeBoardRepo) UpdateWIPLimit(_ context.Context, _ uuid.UUID, limit *int) error {
	r.wipSet = true
	r.wipValue = limit
	return nil
}

type fakeProjects struct{}

func (fakeProjects) Get(context.Context, uuid.UUID, string, uuid.UUID) (*projectdomain.Project, error) {
	return &projectdomain.Project{}, nil
}

type fakeAuthorizer struct{ err error }

func (f fakeAuthorizer) Authorize(context.Context, string, uuid.UUID, string, uuid.UUID) error {
	return f.err
}

type fakeMover struct {
	calls  int
	status string
}

func (m *fakeMover) MoveIssue(_ context.Context, _ uuid.UUID, _ string, _, _ uuid.UUID, status string) error {
	m.calls++
	m.status = status
	return nil
}

// kanbanFixture: columna "abierto" con una incidencia y sin limite; columna "en curso" con limite 1
// y ya llena con otra incidencia.
type kanbanFixture struct {
	board          *domain.Board
	openColumn     uuid.UUID
	progressColumn uuid.UUID
	cardInOpen     uuid.UUID
	cardInProgress uuid.UUID
}

func newKanbanFixture() kanbanFixture {
	one := 1
	f := kanbanFixture{openColumn: uuid.New(), progressColumn: uuid.New(), cardInOpen: uuid.New(), cardInProgress: uuid.New()}
	f.board = &domain.Board{ID: uuid.New(), Columns: []domain.Column{
		{ID: f.openColumn, Status: domain.StatusOpen, Cards: []domain.IssueCard{{ID: f.cardInOpen, Status: domain.StatusOpen}}},
		{ID: f.progressColumn, Status: domain.StatusInProgress, WIPLimit: &one, Cards: []domain.IssueCard{{ID: f.cardInProgress, Status: domain.StatusInProgress}}},
	}}
	return f
}

func TestBoardServiceMoveCard(t *testing.T) {
	ctx := context.Background()
	actor, project := uuid.New(), uuid.New()

	t.Run("moving into a full column is a wip conflict and does not move", func(t *testing.T) {
		f := newKanbanFixture()
		mover := &fakeMover{}
		svc := NewBoardService(&fakeBoardRepo{board: f.board}, fakeProjects{}, fakeAuthorizer{}, mover)
		err := svc.MoveCard(ctx, project, actor, "agent", f.cardInOpen, f.progressColumn)
		if !errors.Is(err, domain.ErrWIPLimitExceeded) {
			t.Fatalf("want wip limit error, got %v", err)
		}
		if mover.calls != 0 {
			t.Fatal("issue must not move when the column is full")
		}
	})

	t.Run("a viewer gets forbidden before the wip limit is checked", func(t *testing.T) {
		f := newKanbanFixture()
		mover := &fakeMover{}
		forbidden := apperror.New(apperror.KindForbidden, "forbidden project operation")
		svc := NewBoardService(&fakeBoardRepo{board: f.board}, fakeProjects{}, fakeAuthorizer{err: forbidden}, mover)
		if err := svc.MoveCard(ctx, project, actor, "requester", f.cardInOpen, f.progressColumn); apperror.KindOf(err) != apperror.KindForbidden {
			t.Fatalf("want forbidden, got %v", err)
		}
		if mover.calls != 0 {
			t.Fatal("issue must not move without permission")
		}
	})

	t.Run("moving into a free column delegates the status of that column", func(t *testing.T) {
		f := newKanbanFixture()
		f.board.Columns[1].WIPLimit = nil
		mover := &fakeMover{}
		svc := NewBoardService(&fakeBoardRepo{board: f.board}, fakeProjects{}, fakeAuthorizer{}, mover)
		if err := svc.MoveCard(ctx, project, actor, "agent", f.cardInOpen, f.progressColumn); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if mover.calls != 1 || mover.status != string(domain.StatusInProgress) {
			t.Fatalf("mover calls=%d status=%q, want one call to in_progress", mover.calls, mover.status)
		}
	})

	t.Run("moving within the same column is a no-op", func(t *testing.T) {
		f := newKanbanFixture()
		mover := &fakeMover{}
		svc := NewBoardService(&fakeBoardRepo{board: f.board}, fakeProjects{}, fakeAuthorizer{}, mover)
		if err := svc.MoveCard(ctx, project, actor, "agent", f.cardInOpen, f.openColumn); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if mover.calls != 0 {
			t.Fatal("same-column move must not call the mover")
		}
	})

	t.Run("unknown card and unknown column are not found", func(t *testing.T) {
		f := newKanbanFixture()
		svc := NewBoardService(&fakeBoardRepo{board: f.board}, fakeProjects{}, fakeAuthorizer{}, &fakeMover{})
		if err := svc.MoveCard(ctx, project, actor, "agent", uuid.New(), f.openColumn); !errors.Is(err, domain.ErrCardNotFound) {
			t.Fatalf("unknown card: got %v", err)
		}
		if err := svc.MoveCard(ctx, project, actor, "agent", f.cardInOpen, uuid.New()); !errors.Is(err, domain.ErrColumnNotFound) {
			t.Fatalf("unknown column: got %v", err)
		}
	})
}

func TestBoardServiceUpdateColumnWIP(t *testing.T) {
	ctx := context.Background()
	actor, project := uuid.New(), uuid.New()
	two := 2

	t.Run("lead sets the limit", func(t *testing.T) {
		f := newKanbanFixture()
		repo := &fakeBoardRepo{board: f.board}
		svc := NewBoardService(repo, fakeProjects{}, fakeAuthorizer{}, &fakeMover{})
		if err := svc.UpdateColumnWIP(ctx, project, actor, "agent", f.openColumn, &two); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !repo.wipSet || repo.wipValue == nil || *repo.wipValue != 2 {
			t.Fatalf("limit not stored: %+v", repo)
		}
	})

	t.Run("non managers are forbidden and nothing is stored", func(t *testing.T) {
		f := newKanbanFixture()
		repo := &fakeBoardRepo{board: f.board}
		forbidden := apperror.New(apperror.KindForbidden, "forbidden project operation")
		svc := NewBoardService(repo, fakeProjects{}, fakeAuthorizer{err: forbidden}, &fakeMover{})
		if err := svc.UpdateColumnWIP(ctx, project, actor, "agent", f.openColumn, &two); apperror.KindOf(err) != apperror.KindForbidden {
			t.Fatalf("want forbidden, got %v", err)
		}
		if repo.wipSet {
			t.Fatal("limit must not be stored without permission")
		}
	})

	t.Run("a limit below one is invalid", func(t *testing.T) {
		f := newKanbanFixture()
		zero := 0
		svc := NewBoardService(&fakeBoardRepo{board: f.board}, fakeProjects{}, fakeAuthorizer{}, &fakeMover{})
		if err := svc.UpdateColumnWIP(ctx, project, actor, "agent", f.openColumn, &zero); !errors.Is(err, domain.ErrInvalidWIPLimit) {
			t.Fatalf("want invalid wip limit, got %v", err)
		}
	})

}
