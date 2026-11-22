package domain

import (
	"testing"

	"github.com/google/uuid"
)

func TestNewDefaultBoardMapsProjectStatusesInOrder(t *testing.T) {
	board, err := NewDefaultBoard(uuid.New(), TypeKanban)
	if err != nil {
		t.Fatal(err)
	}
	want := []IssueStatus{StatusOpen, StatusInProgress, StatusResolved, StatusClosed}
	if !board.IsDefault || board.Type != TypeKanban || len(board.Columns) != len(want) {
		t.Fatalf("unexpected default board: %+v", board)
	}
	for i, column := range board.Columns {
		if column.BoardID != board.ID || column.Position != i || column.Status != want[i] {
			t.Errorf("column %d=%+v, want board=%s position=%d status=%s", i, column, board.ID, i, want[i])
		}
	}
}

func TestNewDefaultBoardRejectsInvalidProjectOrType(t *testing.T) {
	for _, tc := range []struct {
		name      string
		projectID uuid.UUID
		boardType Type
	}{
		{name: "missing project", boardType: TypeKanban},
		{name: "invalid type", projectID: uuid.New(), boardType: "unknown"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := NewDefaultBoard(tc.projectID, tc.boardType); err != ErrInvalidBoard {
				t.Fatalf("error=%v, want %v", err, ErrInvalidBoard)
			}
		})
	}
}
