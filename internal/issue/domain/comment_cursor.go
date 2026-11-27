package domain

import (
	"time"

	"github.com/google/uuid"
)

// CommentCursor marks the last root comment of a page. Pages are ordered by creation time, then id.
type CommentCursor struct {
	CreatedAt time.Time `json:"created_at"`
	ID        uuid.UUID `json:"id"`
}

// Comment sort orders of the thread list.
const (
	CommentSortAsc  = "asc"
	CommentSortDesc = "desc"
)

// ParseCommentSort validates the sort query value. An empty value means ascending.
func ParseCommentSort(raw string) (string, error) {
	switch raw {
	case "", CommentSortAsc:
		return CommentSortAsc, nil
	case CommentSortDesc:
		return CommentSortDesc, nil
	default:
		return "", ErrCommentSortInvalid
	}
}

// NormalizeCommentLimit applies the default (50) and the maximum (100) of the thread list.
func NormalizeCommentLimit(limit int, given bool) (int, error) {
	if !given {
		return 50, nil
	}
	if limit < 1 || limit > 100 {
		return 0, ErrCommentLimitInvalid
	}
	return limit, nil
}
