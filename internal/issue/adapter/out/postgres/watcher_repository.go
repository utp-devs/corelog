package postgres

import (
	"context"

	"github.com/google/uuid"
	"github.com/jhonsferg/corelog/internal/issue/application/port"
	"github.com/jhonsferg/corelog/internal/platform/apperror"
	"github.com/jhonsferg/corelog/internal/platform/database"
	"github.com/jmoiron/sqlx"
)

type WatcherRepository struct{ db *sqlx.DB }

func NewWatcherRepository(db *sqlx.DB) *WatcherRepository { return &WatcherRepository{db: db} }

var _ port.IssueWatcherRepository = (*WatcherRepository)(nil)

func (r *WatcherRepository) State(ctx context.Context, issueID, userID uuid.UUID) (port.WatcherState, error) {
	var state port.WatcherState
	if err := r.db.QueryRowxContext(ctx, `
		SELECT EXISTS(SELECT 1 FROM watchers WHERE issue_id=$1 AND user_id=$2),
		       (SELECT COUNT(*) FROM watchers WHERE issue_id=$1)`, issueID, userID).
		Scan(&state.IsWatching, &state.Count); err != nil {
		return port.WatcherState{}, apperror.Wrap(apperror.KindInternal, "failed to get watcher state", err)
	}
	return state, nil
}

func (r *WatcherRepository) Add(ctx context.Context, issueID, userID uuid.UUID) error {
	if _, err := database.Executor(ctx, r.db).ExecContext(ctx, `
		INSERT INTO watchers(issue_id, user_id) VALUES($1, $2)
		ON CONFLICT(issue_id, user_id) DO NOTHING`, issueID, userID); err != nil {
		return apperror.Wrap(apperror.KindInternal, "failed to follow issue", err)
	}
	return nil
}

func (r *WatcherRepository) Remove(ctx context.Context, issueID, userID uuid.UUID) error {
	if _, err := database.Executor(ctx, r.db).ExecContext(ctx, `DELETE FROM watchers WHERE issue_id=$1 AND user_id=$2`, issueID, userID); err != nil {
		return apperror.Wrap(apperror.KindInternal, "failed to unfollow issue", err)
	}
	return nil
}
