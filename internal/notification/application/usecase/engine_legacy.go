package usecase

import (
	"context"
	"strings"

	"github.com/google/uuid"

	"github.com/jhonsferg/corelog/internal/notification/application/port"
	"github.com/jhonsferg/corelog/internal/notification/domain"
)

// NotifyLegacy delivers the single-recipient notices other modules raise through the Notifier port (assignment,
// mention and reply) through the scheme. It reports false for a kind it does not know, which the caller stores
// as a plain in-app notification.
func (e *Engine) NotifyLegacy(ctx context.Context, userID uuid.UUID, kind, message, link string) (bool, error) {
	n := port.Notice{Kind: kind, Message: message, Link: link, IssueID: issueIDFromLink(link), SkipVisibility: true}
	switch kind {
	case "issue_assigned":
		n.Event = domain.EventIssueAssigned
		n.Explicit = map[domain.RecipientType][]uuid.UUID{domain.RecipientAssignee: {userID}}
	case "comment_mention":
		n.Event = domain.EventIssueMentioned
		n.Explicit = map[domain.RecipientType][]uuid.UUID{domain.RecipientMentioned: {userID}}
		n.Only = userID
	case "comment_reply":
		n.Event = domain.EventIssueCommented
		n.Explicit = map[domain.RecipientType][]uuid.UUID{domain.RecipientReplyTarget: {userID}}
		n.Only = userID
	default:
		return false, nil
	}
	return true, e.Deliver(ctx, n)
}

// issueIDFromLink extracts the issue id of links like /issues/{id} and /issues/{id}#comment-{id}.
func issueIDFromLink(link string) uuid.UUID {
	rest, ok := strings.CutPrefix(link, "/issues/")
	if !ok {
		return uuid.Nil
	}
	if i := strings.IndexAny(rest, "#?/"); i >= 0 {
		rest = rest[:i]
	}
	id, err := uuid.Parse(rest)
	if err != nil {
		return uuid.Nil
	}
	return id
}
