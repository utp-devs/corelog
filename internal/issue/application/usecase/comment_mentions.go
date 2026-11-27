package usecase

import (
	"context"

	"github.com/google/uuid"

	"github.com/jhonsferg/corelog/internal/issue/application/port"
	"github.com/jhonsferg/corelog/internal/issue/domain"
)

// mentionRejectedCode is the stable code of a mention that was kept as plain text.
const mentionRejectedCode = "issue.comment_mention_invalid"

// mentionRejectedMessage is shared by unknown users and users without access, so it reveals nothing about them.
const mentionRejectedMessage = "no se puede mencionar a este usuario en este comentario"

// resolvedMentions are the mentions of a body that the author may keep, and the ones that were rejected.
type resolvedMentions struct {
	ids      []uuid.UUID
	warnings []port.MentionWarning
}

// resolveMentions checks each distinct mention token of the body. A mention is valid when the user can browse the
// project and, for internal comments, reaches the minimum role. Self mentions are ignored, invalid ones become
// warnings and stay as text in the body.
func (s *CommentService) resolveMentions(ctx context.Context, projectID, authorID uuid.UUID, body string, visibility domain.CommentVisibility, role domain.CommentLevel) (resolvedMentions, error) {
	out := resolvedMentions{ids: []uuid.UUID{}}
	tokens := domain.ExtractMentions(body)
	targets := make([]domain.MentionToken, 0, len(tokens))
	for _, token := range tokens {
		if token.UserID != authorID {
			targets = append(targets, token)
		}
	}
	if len(targets) > domain.CommentMaxMentions {
		return out, domain.ErrCommentTooManyMentions
	}
	for _, token := range targets {
		level, err := commentLevelOf(ctx, s.perms, token.UserID, "user", projectID)
		if err != nil {
			return out, err
		}
		allowed := level != domain.CommentLevelNone
		if visibility == domain.CommentVisibilityInternal {
			allowed = domain.CanReadComment(level, visibility, role)
		}
		if !allowed {
			out.warnings = append(out.warnings, port.MentionWarning{UserID: token.UserID, Code: mentionRejectedCode, Message: mentionRejectedMessage})
			continue
		}
		out.ids = append(out.ids, token.UserID)
	}
	return out, nil
}

// newlyMentioned returns the ids of next that are not in previous, keeping the order of next.
func newlyMentioned(previous, next []uuid.UUID) []uuid.UUID {
	seen := make(map[uuid.UUID]bool, len(previous))
	for _, id := range previous {
		seen[id] = true
	}
	out := []uuid.UUID{}
	for _, id := range next {
		if !seen[id] {
			out = append(out, id)
		}
	}
	return out
}
