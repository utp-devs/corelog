package port

import (
	"context"

	"github.com/google/uuid"

	"github.com/jhonsferg/corelog/internal/issue/domain"
)

// IssueSummary is the display data of an issue that links, children and other lists show.
type IssueSummary struct {
	ID             uuid.UUID
	ProjectID      uuid.UUID
	OrganizationID uuid.UUID
	Key            string
	Summary        string
	Status         domain.Status
	TypeName       string
	PriorityName   string
}

// LinkedIssue is one link of an issue seen from that issue. Outward is true when the viewed issue is the source.
type LinkedIssue struct {
	LinkID  uuid.UUID
	Kind    domain.IssueLinkKind
	Outward bool
	Other   IssueSummary
}

// IssueLinkRepository reads and writes the links between issues. Reads only see live issues.
type IssueLinkRepository interface {
	// FindLinkable returns a live issue with its organization. Not found when the issue is missing or deleted.
	FindLinkable(ctx context.Context, issueID uuid.UUID) (IssueSummary, error)
	// ListLinks returns the links touching the issue whose other end is live, ordered by kind and creation.
	ListLinks(ctx context.Context, issueID uuid.UUID) ([]LinkedIssue, error)
	// FindLinkForIssue returns a link only when the issue is one of its ends. Not found otherwise.
	FindLinkForIssue(ctx context.Context, issueID, linkID uuid.UUID) (*domain.IssueLink, error)
	// PairExists reports whether the two issues are linked with the kind in either direction.
	PairExists(ctx context.Context, a, b uuid.UUID, kind domain.IssueLinkKind) (bool, error)
	// BlocksReach reports whether from reaches to following blocks links in their stored direction.
	BlocksReach(ctx context.Context, from, to uuid.UUID) (bool, error)
	Save(ctx context.Context, link *domain.IssueLink) error
	Delete(ctx context.Context, linkID uuid.UUID) error
}

// ChildrenView is the list of live direct children of an issue with the progress of the done ones.
type ChildrenView struct {
	Children      []*domain.Issue
	AssigneeNames map[uuid.UUID]string
	Progress      domain.ChildProgress
}

// ChildReader lists the live direct children of an issue ordered by rank.
type ChildReader interface {
	ListChildren(ctx context.Context, parentID uuid.UUID) ([]*domain.Issue, error)
}

// VoteRepository records votes. Each write also moves the vote counter of the issue in the same statement.
type VoteRepository interface {
	// CastVote adds the vote of the user. It reports false when the user had already voted.
	CastVote(ctx context.Context, issueID, userID uuid.UUID) (bool, error)
	// WithdrawVote removes the vote of the user. It reports false when the user had not voted.
	WithdrawVote(ctx context.Context, issueID, userID uuid.UUID) (bool, error)
	HasVoted(ctx context.Context, issueID, userID uuid.UUID) (bool, error)
}
