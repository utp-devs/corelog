package usecase

import (
	"context"

	"github.com/google/uuid"

	"github.com/jhonsferg/corelog/internal/platform/apperror"
)

// permVoteIssue is the permission a caller needs to vote on an issue. Without a permission checker the browse
// permission alone is enough.
const permVoteIssue = "vote_issue"

// VoteIssue records the vote of the caller. Voting on an issue the caller reported or is assigned is allowed.
func (s *IssueService) VoteIssue(ctx context.Context, issueID, actorID uuid.UUID, globalRole string) error {
	return s.setVote(ctx, issueID, actorID, globalRole, true)
}

// UnvoteIssue removes the vote of the caller.
func (s *IssueService) UnvoteIssue(ctx context.Context, issueID, actorID uuid.UUID, globalRole string) error {
	return s.setVote(ctx, issueID, actorID, globalRole, false)
}

// setVote checks visibility and the vote permission, then writes the vote and its counter in one transaction.
func (s *IssueService) setVote(ctx context.Context, issueID, actorID uuid.UUID, globalRole string, cast bool) error {
	issue, err := s.repo.FindByID(ctx, issueID)
	if err != nil {
		return err
	}
	if err := s.requireVisible(ctx, issue.ProjectID, actorID, globalRole); err != nil {
		return err
	}
	if s.perms != nil {
		ok, err := s.perms.Can(ctx, actorID, globalRole, issue.ProjectID, permVoteIssue, holdersFor(issue, actorID)...)
		if err != nil {
			return err
		}
		if !ok {
			return forbiddenIssueOperation()
		}
	}
	if s.votes == nil {
		return apperror.New(apperror.KindInternal, "el almacenamiento de votos no esta disponible")
	}
	return s.tx.InTx(ctx, func(ctx context.Context) error {
		var err error
		if cast {
			_, err = s.votes.CastVote(ctx, issueID, actorID)
		} else {
			_, err = s.votes.WithdrawVote(ctx, issueID, actorID)
		}
		return err
	})
}
