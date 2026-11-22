package postgres

import (
	"context"
	"database/sql"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"

	"github.com/jhonsferg/corelog/internal/platform/apperror"
	"github.com/jhonsferg/corelog/internal/ticket/application/port"
	"github.com/jhonsferg/corelog/internal/ticket/domain"
)

type commentRow struct {
	ID        uuid.UUID  `db:"id"`
	IssueID   uuid.UUID  `db:"issue_id"`
	AuthorID  uuid.UUID  `db:"author_id"`
	Body      string     `db:"body"`
	CreatedAt time.Time  `db:"created_at"`
	UpdatedAt time.Time  `db:"updated_at"`
	DeletedAt *time.Time `db:"deleted_at"`
}

type activityRow struct {
	ID        uuid.UUID `db:"id"`
	IssueID   uuid.UUID `db:"issue_id"`
	ActorID   uuid.UUID `db:"actor_id"`
	Action    string    `db:"action"`
	FieldName *string   `db:"field_name"`
	OldValue  *string   `db:"old_value"`
	NewValue  *string   `db:"new_value"`
	CreatedAt time.Time `db:"created_at"`
}

type CollaborationRepository struct {
	db *sqlx.DB
}

func NewCollaborationRepository(db *sqlx.DB) *CollaborationRepository {
	return &CollaborationRepository{db: db}
}

var _ port.CollaborationRepository = (*CollaborationRepository)(nil)

func (r *CollaborationRepository) FindComment(ctx context.Context, commentID uuid.UUID) (*domain.Comment, error) {
	var row commentRow
	if err := r.db.GetContext(ctx, &row, `
		SELECT id, issue_id, author_id, body, created_at, updated_at, deleted_at
		FROM comments WHERE id = $1`, commentID); err != nil {
		if err == sql.ErrNoRows {
			return nil, apperror.New(apperror.KindNotFound, "comment not found")
		}
		return nil, apperror.Wrap(apperror.KindInternal, "failed to find comment", err)
	}
	return &domain.Comment{
		ID: row.ID, IssueID: row.IssueID, AuthorID: row.AuthorID, Body: row.Body,
		CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt, DeletedAt: row.DeletedAt,
	}, nil
}

func (r *CollaborationRepository) SaveComment(ctx context.Context, comment *domain.Comment) error {
	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return apperror.Wrap(apperror.KindInternal, "failed to begin comment transaction", err)
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO comments (id, issue_id, author_id, body, created_at, updated_at, deleted_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)`,
		comment.ID, comment.IssueID, comment.AuthorID, comment.Body,
		comment.CreatedAt, comment.UpdatedAt, comment.DeletedAt,
	); err != nil {
		return apperror.Wrap(apperror.KindInternal, "failed to save comment", err)
	}
	for _, mentionedID := range comment.Mentions {
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO comment_mentions (comment_id, user_id)
			VALUES ($1, $2)
			ON CONFLICT DO NOTHING`,
			comment.ID, mentionedID,
		); err != nil {
			return apperror.Wrap(apperror.KindInternal, "failed to save comment mention", err)
		}
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO watchers (issue_id, user_id)
		VALUES ($1, $2)
		ON CONFLICT (issue_id, user_id) DO NOTHING`, comment.IssueID, comment.AuthorID); err != nil {
		return apperror.Wrap(apperror.KindInternal, "failed to follow issue after commenting", err)
	}
	if err := tx.Commit(); err != nil {
		return apperror.Wrap(apperror.KindInternal, "failed to commit comment", err)
	}
	return nil
}

func (r *CollaborationRepository) FindComments(ctx context.Context, issueID uuid.UUID) ([]*domain.Comment, error) {
	var rows []commentRow
	if err := r.db.SelectContext(ctx, &rows, `
		SELECT id, issue_id, author_id, body, created_at, updated_at, deleted_at
		FROM comments
		WHERE issue_id = $1 AND deleted_at IS NULL
		ORDER BY created_at ASC`,
		issueID,
	); err != nil {
		return nil, apperror.Wrap(apperror.KindInternal, "failed to list comments", err)
	}

	comments := make([]*domain.Comment, 0, len(rows))
	for _, row := range rows {
		comments = append(comments, &domain.Comment{
			ID: row.ID, IssueID: row.IssueID, AuthorID: row.AuthorID, Body: row.Body,
			CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt, DeletedAt: row.DeletedAt,
		})
	}
	return comments, nil
}

func (r *CollaborationRepository) UpdateComment(ctx context.Context, comment *domain.Comment) error {
	result, err := r.db.ExecContext(ctx, `
		UPDATE comments
		SET body = $1, updated_at = $2
		WHERE id = $3 AND author_id = $4 AND deleted_at IS NULL`,
		comment.Body, comment.UpdatedAt, comment.ID, comment.AuthorID,
	)
	if err != nil {
		return apperror.Wrap(apperror.KindInternal, "failed to update comment", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return apperror.Wrap(apperror.KindInternal, "failed to update comment", err)
	}
	if affected == 0 {
		return apperror.New(apperror.KindNotFound, "comment not found")
	}
	return nil
}

func (r *CollaborationRepository) DeleteComment(ctx context.Context, comment *domain.Comment) error {
	result, err := r.db.ExecContext(ctx, `
		UPDATE comments
		SET deleted_at = $1, updated_at = $2
		WHERE id = $3 AND author_id = $4 AND deleted_at IS NULL`,
		comment.DeletedAt, comment.UpdatedAt, comment.ID, comment.AuthorID,
	)
	if err != nil {
		return apperror.Wrap(apperror.KindInternal, "failed to delete comment", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return apperror.Wrap(apperror.KindInternal, "failed to delete comment", err)
	}
	if affected == 0 {
		return apperror.New(apperror.KindNotFound, "comment not found")
	}
	return nil
}

func (r *CollaborationRepository) SaveActivity(ctx context.Context, activity *domain.Activity) error {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO activities (id, issue_id, actor_id, action, field_name, old_value, new_value, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
		activity.ID, activity.IssueID, activity.ActorID, activity.Action,
		activity.FieldName, activity.OldValue, activity.NewValue, activity.CreatedAt,
	)
	if err != nil {
		return apperror.Wrap(apperror.KindInternal, "failed to save activity", err)
	}
	return nil
}

func (r *CollaborationRepository) FindActivities(ctx context.Context, issueID uuid.UUID) ([]*domain.Activity, error) {
	var rows []activityRow
	if err := r.db.SelectContext(ctx, &rows, `
		SELECT id, issue_id, actor_id, action, field_name, old_value, new_value, created_at
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
			CreatedAt: row.CreatedAt,
		})
	}
	return activities, nil
}

func (r *CollaborationRepository) FindMentionIDs(ctx context.Context, issueID uuid.UUID, names []string) ([]uuid.UUID, error) {
	if len(names) == 0 {
		return []uuid.UUID{}, nil
	}
	patterns := make([]string, 0, len(names))
	args := make([]any, 0, len(names)*2+1)
	args = append(args, issueID)
	for _, name := range names {
		nameArg := "$" + strconv.Itoa(len(args)+1)
		emailArg := "$" + strconv.Itoa(len(args)+2)
		patterns = append(patterns, "(LOWER(u.name) LIKE LOWER("+nameArg+") OR LOWER(u.email) LIKE LOWER("+emailArg+"))")
		args = append(args, "%"+strings.ToLower(name)+"%", "%"+strings.ToLower(name)+"%")
	}
	var ids []uuid.UUID
	query := `
		SELECT DISTINCT u.id
		FROM users u
		JOIN project_members pm ON pm.user_id = u.id
		JOIN issues i ON i.project_id = pm.project_id
		WHERE i.id = $1 AND (` + strings.Join(patterns, " OR ") + `)`
	if err := r.db.SelectContext(ctx, &ids, query, args...); err != nil {
		return nil, apperror.Wrap(apperror.KindInternal, "failed to resolve comment mentions", err)
	}
	return ids, nil
}
