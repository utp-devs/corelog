package domain

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestCanReadCommentMatrix(t *testing.T) {
	tests := []struct {
		name       string
		viewer     CommentLevel
		visibility CommentVisibility
		role       CommentLevel
		want       bool
	}{
		{"viewer reads public", CommentLevelViewer, CommentVisibilityPublic, CommentLevelNone, true},
		{"no access reads nothing public", CommentLevelNone, CommentVisibilityPublic, CommentLevelNone, false},
		{"viewer never reads contributor internal", CommentLevelViewer, CommentVisibilityInternal, CommentLevelContributor, false},
		{"contributor reads contributor internal", CommentLevelContributor, CommentVisibilityInternal, CommentLevelContributor, true},
		{"contributor not lead internal", CommentLevelContributor, CommentVisibilityInternal, CommentLevelLead, false},
		{"lead reads lead internal", CommentLevelLead, CommentVisibilityInternal, CommentLevelLead, true},
		{"lead reads contributor internal", CommentLevelLead, CommentVisibilityInternal, CommentLevelContributor, true},
		{"internal without role is unreadable", CommentLevelLead, CommentVisibilityInternal, CommentLevelNone, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := CanReadComment(tt.viewer, tt.visibility, tt.role); got != tt.want {
				t.Fatalf("CanReadComment = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestParseInternalRole(t *testing.T) {
	for _, raw := range []string{"contributor", "lead"} {
		if _, err := ParseInternalRole(raw); err != nil {
			t.Errorf("%s rejected: %v", raw, err)
		}
	}
	for _, raw := range []string{"viewer", "owner", "", "Lead"} {
		if _, err := ParseInternalRole(raw); err == nil {
			t.Errorf("%q accepted", raw)
		}
	}
}

func TestCommentReactionSet(t *testing.T) {
	if len(CommentReactions) != 6 {
		t.Fatalf("reaction set has %d emojis, want 6", len(CommentReactions))
	}
	if !IsCommentReaction("thumbs_up") || IsCommentReaction("laugh") {
		t.Fatal("reaction membership is wrong")
	}
}

func TestCommentBodyLifecycle(t *testing.T) {
	at := time.Date(2026, 11, 27, 10, 0, 0, 0, time.UTC)
	issue, author := uuid.New(), uuid.New()

	if _, err := NewComment(issue, author, "   ", at); !errors.Is(err, ErrInvalidComment) {
		t.Fatalf("blank body: %v", err)
	}
	if _, err := NewComment(issue, author, strings.Repeat("a", CommentMaxLength+1), at); !errors.Is(err, ErrCommentTooLong) {
		t.Fatalf("too long body: %v", err)
	}
	c, err := NewComment(issue, author, strings.Repeat("ñ", CommentMaxLength), at)
	if err != nil {
		t.Fatalf("limit rune count: %v", err)
	}
	if c.IsEdited() || c.IsDeleted() || c.IsReply() {
		t.Fatal("new comment must be public, live and not edited")
	}

	later := at.Add(time.Minute)
	if err := c.Update("  nuevo  ", later); err != nil {
		t.Fatal(err)
	}
	if c.Body != "nuevo" || c.EditedAt == nil || !c.EditedAt.Equal(later) || !c.IsEdited() {
		t.Fatalf("update did not mark the edit: %+v", c)
	}
	if err := c.Update(" ", later); !errors.Is(err, ErrInvalidComment) {
		t.Fatalf("blank update: %v", err)
	}

	c.Delete(later.Add(time.Minute))
	if !c.IsDeleted() || c.Body != "" {
		t.Fatalf("delete must clear the body: %+v", c)
	}
}

func TestCommentSortAndLimit(t *testing.T) {
	if sort, err := ParseCommentSort(""); err != nil || sort != CommentSortAsc {
		t.Fatalf("default sort = %q, %v", sort, err)
	}
	if _, err := ParseCommentSort("random"); !errors.Is(err, ErrCommentSortInvalid) {
		t.Fatalf("invalid sort: %v", err)
	}
	if limit, err := NormalizeCommentLimit(0, false); err != nil || limit != 50 {
		t.Fatalf("default limit = %d, %v", limit, err)
	}
	for _, bad := range []int{0, 101, -3} {
		if _, err := NormalizeCommentLimit(bad, true); !errors.Is(err, ErrCommentLimitInvalid) {
			t.Errorf("limit %d accepted", bad)
		}
	}
}
