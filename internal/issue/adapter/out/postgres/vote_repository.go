package postgres

import (
	"context"

	"github.com/google/uuid"

	"github.com/jhonsferg/corelog/internal/issue/application/port"
	"github.com/jhonsferg/corelog/internal/platform/apperror"
	"github.com/jhonsferg/corelog/internal/platform/database"
)

var _ port.VoteRepository = (*IssueRepository)(nil)

// CastVote inserts the vote and bumps the counter in one statement, so the counter cannot drift from the vote rows
// even under concurrent votes: the counter update waits on the issue row. Soft-deleted issues take no votes.
func (r *IssueRepository) CastVote(ctx context.Context, issueID, userID uuid.UUID) (bool, error) {
	var changed int
	if err := database.Executor(ctx, r.db).QueryRowxContext(ctx, `
		WITH ins AS (
		  INSERT INTO issue_votes (issue_id, user_id, created_at)
		  SELECT i.id, $2, now() FROM issues i WHERE i.id = $1 AND i.deleted_at IS NULL
		  ON CONFLICT (issue_id, user_id) DO NOTHING
		  RETURNING 1
		), upd AS (
		  UPDATE issues SET vote_count = vote_count + (SELECT COUNT(*) FROM ins)
		  WHERE id = $1 AND (SELECT COUNT(*) FROM ins) > 0
		  RETURNING 1
		)
		SELECT (SELECT COUNT(*) FROM upd)::int`, issueID, userID).Scan(&changed); err != nil {
		return false, apperror.Wrap(apperror.KindInternal, "failed to record vote", err)
	}
	return changed > 0, nil
}

// WithdrawVote deletes the vote and lowers the counter in one statement.
func (r *IssueRepository) WithdrawVote(ctx context.Context, issueID, userID uuid.UUID) (bool, error) {
	var changed int
	if err := database.Executor(ctx, r.db).QueryRowxContext(ctx, `
		WITH del AS (
		  DELETE FROM issue_votes WHERE issue_id = $1 AND user_id = $2
		  RETURNING 1
		), upd AS (
		  UPDATE issues SET vote_count = vote_count - (SELECT COUNT(*) FROM del)
		  WHERE id = $1 AND (SELECT COUNT(*) FROM del) > 0
		  RETURNING 1
		)
		SELECT (SELECT COUNT(*) FROM upd)::int`, issueID, userID).Scan(&changed); err != nil {
		return false, apperror.Wrap(apperror.KindInternal, "failed to withdraw vote", err)
	}
	return changed > 0, nil
}

// HasVoted reports whether the user has a vote on the issue.
func (r *IssueRepository) HasVoted(ctx context.Context, issueID, userID uuid.UUID) (bool, error) {
	var voted bool
	if err := database.Executor(ctx, r.db).QueryRowxContext(ctx,
		`SELECT EXISTS (SELECT 1 FROM issue_votes WHERE issue_id = $1 AND user_id = $2)`, issueID, userID).Scan(&voted); err != nil {
		return false, apperror.Wrap(apperror.KindInternal, "failed to read vote", err)
	}
	return voted, nil
}
