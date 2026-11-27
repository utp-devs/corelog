package postgres

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"

	"github.com/jhonsferg/corelog/internal/issue/application/port"
	"github.com/jhonsferg/corelog/internal/issue/domain"
	"github.com/jhonsferg/corelog/internal/platform/apperror"
	"github.com/jhonsferg/corelog/internal/platform/database"
)

type activityRow struct {
	ID        uuid.UUID  `db:"id"`
	IssueID   uuid.UUID  `db:"issue_id"`
	ActorID   uuid.UUID  `db:"actor_id"`
	Action    string     `db:"action"`
	FieldName *string    `db:"field_name"`
	OldValue  *string    `db:"old_value"`
	NewValue  *string    `db:"new_value"`
	BatchID   *uuid.UUID `db:"batch_id"`
	CreatedAt time.Time  `db:"created_at"`
}

type CollaborationRepository struct {
	db *sqlx.DB
}

func NewCollaborationRepository(db *sqlx.DB) *CollaborationRepository {
	return &CollaborationRepository{db: db}
}

var _ port.CollaborationRepository = (*CollaborationRepository)(nil)
var _ port.ActivityBatchWriter = (*CollaborationRepository)(nil)

// SaveComment stores a public comment written by a workflow transition, with its follow of the author, in the
// transaction of ctx when there is one. Comments written by people go through CommentRepository.
func (r *CollaborationRepository) SaveComment(ctx context.Context, comment *domain.Comment) error {
	return database.NewTxManager(r.db).InTx(ctx, func(ctx context.Context) error {
		return saveCommentRow(ctx, r.db, comment)
	})
}

func (r *CollaborationRepository) SaveActivity(ctx context.Context, activity *domain.Activity) error {
	err := database.Executor(ctx, r.db).QueryRowxContext(ctx, `
		INSERT INTO activities (issue_id, actor_id, action, field_name, old_value, new_value, created_at, batch_id)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		RETURNING id`,
		activity.IssueID, activity.ActorID, activity.Action,
		activity.FieldName, activity.OldValue, activity.NewValue, activity.CreatedAt, activity.BatchID,
	).Scan(&activity.ID)
	if err != nil {
		return apperror.Wrap(apperror.KindInternal, "failed to save activity", err)
	}
	return nil
}

// SaveActivities writes the activities of one patch, in order.
func (r *CollaborationRepository) SaveActivities(ctx context.Context, activities []*domain.Activity) error {
	for _, activity := range activities {
		if err := r.SaveActivity(ctx, activity); err != nil {
			return err
		}
	}
	return nil
}

func (r *CollaborationRepository) FindActivities(ctx context.Context, issueID uuid.UUID) ([]*domain.Activity, error) {
	var rows []activityRow
	if err := r.db.SelectContext(ctx, &rows, `
		SELECT id, issue_id, actor_id, action, field_name, old_value, new_value, batch_id, created_at
		FROM activities
		WHERE issue_id = $1
		ORDER BY created_at ASC`,
		issueID,
	); err != nil {
		return nil, apperror.Wrap(apperror.KindInternal, "failed to list activities", err)
	}
	activities := make([]*domain.Activity, 0, len(rows))
	for _, row := range rows {
		activities = append(activities, &domain.Activity{
			ID: row.ID, IssueID: row.IssueID, ActorID: row.ActorID, Action: row.Action,
			FieldName: row.FieldName, OldValue: row.OldValue, NewValue: row.NewValue,
			BatchID: row.BatchID, CreatedAt: row.CreatedAt,
		})
	}
	return activities, nil
}

// saveCommentRow inserts a comment row and its author follow. Comments written by the workflow are public.
func saveCommentRow(ctx context.Context, db *sqlx.DB, comment *domain.Comment) error {
	tx := database.Executor(ctx, db)
	if err := tx.QueryRowxContext(ctx, `
		INSERT INTO comments (issue_id, author_id, body, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id`,
		comment.IssueID, comment.AuthorID, comment.Body, comment.CreatedAt, comment.UpdatedAt,
	).Scan(&comment.ID); err != nil {
		return apperror.Wrap(apperror.KindInternal, "failed to save comment", err)
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO watchers (issue_id, user_id)
		VALUES ($1, $2)
		ON CONFLICT (issue_id, user_id) DO NOTHING`, comment.IssueID, comment.AuthorID); err != nil {
		return apperror.Wrap(apperror.KindInternal, "failed to follow issue after commenting", err)
	}
	return nil
}
