package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"

	"github.com/jhonsferg/corelog/internal/issue/application/port"
	"github.com/jhonsferg/corelog/internal/issue/domain"
	"github.com/jhonsferg/corelog/internal/platform/apperror"
	"github.com/jhonsferg/corelog/internal/platform/database"
)

// commentColumns lists the comment columns read by the repository, with the c alias.
const commentColumns = `c.id, c.issue_id, c.author_id, c.parent_id, c.body, c.visibility, c.internal_role,
	c.created_at, c.updated_at, c.edited_at, c.deleted_at`

type commentRow struct {
	ID           uuid.UUID  `db:"id"`
	IssueID      uuid.UUID  `db:"issue_id"`
	AuthorID     uuid.UUID  `db:"author_id"`
	ParentID     *uuid.UUID `db:"parent_id"`
	Body         string     `db:"body"`
	Visibility   string     `db:"visibility"`
	InternalRole *string    `db:"internal_role"`
	CreatedAt    time.Time  `db:"created_at"`
	UpdatedAt    time.Time  `db:"updated_at"`
	EditedAt     *time.Time `db:"edited_at"`
	DeletedAt    *time.Time `db:"deleted_at"`
}

func (row commentRow) toDomain() *domain.Comment {
	comment := &domain.Comment{
		ID: row.ID, IssueID: row.IssueID, AuthorID: row.AuthorID, ParentID: row.ParentID, Body: row.Body,
		Visibility: domain.CommentVisibility(row.Visibility), CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
		EditedAt: row.EditedAt, DeletedAt: row.DeletedAt,
	}
	if row.InternalRole != nil {
		comment.InternalRole, _ = domain.ParseInternalRole(*row.InternalRole)
	}
	return comment
}

// readableSQL keeps the comments that the viewer at the level of param can read. Public comments are readable
// by every viewer; internal ones need the level of their minimum role (contributor 2, lead 3).
func readableSQL(alias, param string) string {
	return fmt.Sprintf(`(%[1]s.visibility = 'public' OR %[2]s >= CASE %[1]s.internal_role
		WHEN 'lead' THEN 3 WHEN 'contributor' THEN 2 ELSE 4 END)`, alias, param)
}

// CommentRepository persists comments, mentions, revisions and reactions.
type CommentRepository struct {
	db *sqlx.DB
}

func NewCommentRepository(db *sqlx.DB) *CommentRepository {
	return &CommentRepository{db: db}
}

var _ port.CommentStore = (*CommentRepository)(nil)

func (r *CommentRepository) FindComment(ctx context.Context, id uuid.UUID) (*domain.Comment, error) {
	var row commentRow
	if err := r.db.GetContext(ctx, &row, `SELECT `+commentColumns+` FROM comments c WHERE c.id = $1`, id); err != nil {
		if err == sql.ErrNoRows {
			return nil, domain.ErrCommentNotFound
		}
		return nil, apperror.Wrap(apperror.KindInternal, "failed to find comment", err)
	}
	return row.toDomain(), nil
}

func (r *CommentRepository) InsertComment(ctx context.Context, comment *domain.Comment) error {
	return database.NewTxManager(r.db).InTx(ctx, func(ctx context.Context) error {
		tx := database.Executor(ctx, r.db)
		var parent any
		if comment.ParentID != nil {
			parent = *comment.ParentID
		}
		var internalRole any
		if comment.Visibility == domain.CommentVisibilityInternal {
			internalRole = comment.InternalRole.Name()
		}
		if err := tx.QueryRowxContext(ctx, `
			INSERT INTO comments (issue_id, author_id, parent_id, body, visibility, internal_role, created_at, updated_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
			RETURNING id`,
			comment.IssueID, comment.AuthorID, parent, comment.Body, string(comment.Visibility), internalRole,
			comment.CreatedAt, comment.UpdatedAt,
		).Scan(&comment.ID); err != nil {
			return apperror.Wrap(apperror.KindInternal, "failed to save comment", err)
		}
		if err := replaceMentionRows(ctx, tx, comment.ID, comment.Mentions); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO watchers (issue_id, user_id)
			VALUES ($1, $2)
			ON CONFLICT (issue_id, user_id) DO NOTHING`, comment.IssueID, comment.AuthorID); err != nil {
			return apperror.Wrap(apperror.KindInternal, "failed to follow issue after commenting", err)
		}
		return nil
	})
}

func (r *CommentRepository) UpdateCommentBody(ctx context.Context, comment *domain.Comment, previousBody string, editedBy uuid.UUID) error {
	if comment.EditedAt == nil {
		return apperror.New(apperror.KindInternal, "edited comment without edit time")
	}
	return database.NewTxManager(r.db).InTx(ctx, func(ctx context.Context) error {
		tx := database.Executor(ctx, r.db)
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO comment_revisions (comment_id, body, edited_by, edited_at)
			VALUES ($1, $2, $3, $4)`, comment.ID, previousBody, editedBy, *comment.EditedAt); err != nil {
			return apperror.Wrap(apperror.KindInternal, "failed to save comment revision", err)
		}
		result, err := tx.ExecContext(ctx, `
			UPDATE comments SET body = $1, updated_at = $2, edited_at = $3
			WHERE id = $4 AND deleted_at IS NULL`,
			comment.Body, comment.UpdatedAt, *comment.EditedAt, comment.ID)
		if err != nil {
			return apperror.Wrap(apperror.KindInternal, "failed to update comment", err)
		}
		return requireRow(result, "failed to update comment")
	})
}

func (r *CommentRepository) SoftDeleteComment(ctx context.Context, comment *domain.Comment) error {
	if comment.DeletedAt == nil {
		return apperror.New(apperror.KindInternal, "deleted comment without delete time")
	}
	return database.NewTxManager(r.db).InTx(ctx, func(ctx context.Context) error {
		tx := database.Executor(ctx, r.db)
		for _, stmt := range []string{
			`DELETE FROM comment_revisions WHERE comment_id = $1`,
			`DELETE FROM comment_reactions WHERE comment_id = $1`,
			`DELETE FROM comment_mentions WHERE comment_id = $1`,
		} {
			if _, err := tx.ExecContext(ctx, stmt, comment.ID); err != nil {
				return apperror.Wrap(apperror.KindInternal, "failed to clear deleted comment", err)
			}
		}
		result, err := tx.ExecContext(ctx, `
			UPDATE comments SET body = '', deleted_at = $1, updated_at = $2
			WHERE id = $3 AND deleted_at IS NULL`,
			*comment.DeletedAt, comment.UpdatedAt, comment.ID)
		if err != nil {
			return apperror.Wrap(apperror.KindInternal, "failed to delete comment", err)
		}
		return requireRow(result, "failed to delete comment")
	})
}

func (r *CommentRepository) ReplaceMentions(ctx context.Context, commentID uuid.UUID, userIDs []uuid.UUID) error {
	return database.NewTxManager(r.db).InTx(ctx, func(ctx context.Context) error {
		return replaceMentionRows(ctx, database.Executor(ctx, r.db), commentID, userIDs)
	})
}

// replaceMentionRows keeps the mentions of the comment equal to userIDs: missing ones are inserted and the others
// removed.
func replaceMentionRows(ctx context.Context, tx sqlx.ExtContext, commentID uuid.UUID, userIDs []uuid.UUID) error {
	if _, err := tx.ExecContext(ctx, `
		DELETE FROM comment_mentions
		WHERE comment_id = $1 AND NOT (user_id = ANY($2::uuid[]))`, commentID, commentIDStrings(userIDs)); err != nil {
		return apperror.Wrap(apperror.KindInternal, "failed to replace comment mentions", err)
	}
	for _, userID := range userIDs {
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO comment_mentions (comment_id, user_id)
			VALUES ($1, $2)
			ON CONFLICT DO NOTHING`, commentID, userID); err != nil {
			return apperror.Wrap(apperror.KindInternal, "failed to save comment mention", err)
		}
	}
	return nil
}

func (r *CommentRepository) MentionedUserIDs(ctx context.Context, commentID uuid.UUID) ([]uuid.UUID, error) {
	ids := []uuid.UUID{}
	if err := r.db.SelectContext(ctx, &ids, `
		SELECT user_id FROM comment_mentions WHERE comment_id = $1 ORDER BY user_id`, commentID); err != nil {
		return nil, apperror.Wrap(apperror.KindInternal, "failed to list comment mentions", err)
	}
	return ids, nil
}

func (r *CommentRepository) ListMentions(ctx context.Context, commentIDs []uuid.UUID) (map[uuid.UUID][]uuid.UUID, error) {
	out := make(map[uuid.UUID][]uuid.UUID, len(commentIDs))
	if len(commentIDs) == 0 {
		return out, nil
	}
	var rows []struct {
		CommentID uuid.UUID `db:"comment_id"`
		UserID    uuid.UUID `db:"user_id"`
	}
	if err := r.db.SelectContext(ctx, &rows, `
		SELECT comment_id, user_id FROM comment_mentions
		WHERE comment_id = ANY($1::uuid[])
		ORDER BY comment_id, created_at, user_id`, commentIDStrings(commentIDs)); err != nil {
		return nil, apperror.Wrap(apperror.KindInternal, "failed to list comment mentions", err)
	}
	for _, row := range rows {
		out[row.CommentID] = append(out[row.CommentID], row.UserID)
	}
	return out, nil
}

func (r *CommentRepository) ListRevisions(ctx context.Context, commentID uuid.UUID) ([]domain.CommentRevision, error) {
	var rows []struct {
		ID        uuid.UUID `db:"id"`
		CommentID uuid.UUID `db:"comment_id"`
		Body      string    `db:"body"`
		EditedBy  uuid.UUID `db:"edited_by"`
		EditedAt  time.Time `db:"edited_at"`
	}
	if err := r.db.SelectContext(ctx, &rows, `
		SELECT id, comment_id, body, edited_by, edited_at
		FROM comment_revisions
		WHERE comment_id = $1
		ORDER BY edited_at DESC, id DESC`, commentID); err != nil {
		return nil, apperror.Wrap(apperror.KindInternal, "failed to list comment revisions", err)
	}
	out := make([]domain.CommentRevision, 0, len(rows))
	for _, row := range rows {
		out = append(out, domain.CommentRevision{
			ID: row.ID, CommentID: row.CommentID, Body: row.Body, EditedBy: row.EditedBy, EditedAt: row.EditedAt,
		})
	}
	return out, nil
}

func (r *CommentRepository) ListThreadRoots(ctx context.Context, issueID uuid.UUID, viewer domain.CommentLevel, sort string, after *domain.CommentCursor, limit int) ([]*domain.Comment, error) {
	direction, comparison := "ASC", ">"
	if sort == domain.CommentSortDesc {
		direction, comparison = "DESC", "<"
	}
	args := []any{issueID, int(viewer)}
	where := `c.issue_id = $1 AND c.parent_id IS NULL AND ` + readableSQL("c", "$2") + `
		AND (c.deleted_at IS NULL OR EXISTS (
			SELECT 1 FROM comments r
			WHERE r.parent_id = c.id AND r.deleted_at IS NULL AND ` + readableSQL("r", "$2") + `))`
	if after != nil {
		args = append(args, after.CreatedAt, after.ID)
		where += fmt.Sprintf(" AND (c.created_at, c.id) %s ($%d, $%d)", comparison, len(args)-1, len(args))
	}
	args = append(args, limit)
	query := fmt.Sprintf(`SELECT %s FROM comments c WHERE %s
		ORDER BY c.created_at %[3]s, c.id %[3]s LIMIT $%d`, commentColumns, where, direction, len(args))
	var rows []commentRow
	if err := r.db.SelectContext(ctx, &rows, query, args...); err != nil {
		return nil, apperror.Wrap(apperror.KindInternal, "failed to list comments", err)
	}
	return rowsToComments(rows), nil
}

func (r *CommentRepository) ListReplies(ctx context.Context, issueID uuid.UUID, rootIDs []uuid.UUID, viewer domain.CommentLevel) ([]*domain.Comment, error) {
	if len(rootIDs) == 0 {
		return []*domain.Comment{}, nil
	}
	var rows []commentRow
	if err := r.db.SelectContext(ctx, &rows, `
		SELECT `+commentColumns+` FROM comments c
		WHERE c.issue_id = $1 AND c.parent_id = ANY($2::uuid[]) AND c.deleted_at IS NULL AND `+readableSQL("c", "$3")+`
		ORDER BY c.created_at ASC, c.id ASC`, issueID, commentIDStrings(rootIDs), int(viewer)); err != nil {
		return nil, apperror.Wrap(apperror.KindInternal, "failed to list comment replies", err)
	}
	return rowsToComments(rows), nil
}

func (r *CommentRepository) ReactionCounts(ctx context.Context, commentIDs []uuid.UUID, viewerID uuid.UUID) (map[uuid.UUID][]port.ReactionCount, error) {
	out := make(map[uuid.UUID][]port.ReactionCount, len(commentIDs))
	if len(commentIDs) == 0 {
		return out, nil
	}
	var rows []struct {
		CommentID uuid.UUID `db:"comment_id"`
		Emoji     string    `db:"emoji"`
		Total     int       `db:"total"`
		Mine      bool      `db:"mine"`
	}
	if err := r.db.SelectContext(ctx, &rows, `
		SELECT comment_id, emoji, COUNT(*)::int AS total, BOOL_OR(user_id = $2) AS mine
		FROM comment_reactions
		WHERE comment_id = ANY($1::uuid[])
		GROUP BY comment_id, emoji`, commentIDStrings(commentIDs), viewerID); err != nil {
		return nil, apperror.Wrap(apperror.KindInternal, "failed to count reactions", err)
	}
	byComment := make(map[uuid.UUID]map[string]port.ReactionCount, len(rows))
	for _, row := range rows {
		if byComment[row.CommentID] == nil {
			byComment[row.CommentID] = map[string]port.ReactionCount{}
		}
		byComment[row.CommentID][row.Emoji] = port.ReactionCount{Emoji: row.Emoji, Count: row.Total, ViewerReacted: row.Mine}
	}
	// The fixed reaction order keeps the response stable.
	for commentID, counts := range byComment {
		for _, emoji := range domain.CommentReactions {
			if count, ok := counts[emoji]; ok {
				out[commentID] = append(out[commentID], count)
			}
		}
	}
	return out, nil
}

func (r *CommentRepository) SetReaction(ctx context.Context, commentID, userID uuid.UUID, emoji string, on bool) error {
	if on {
		if _, err := r.db.ExecContext(ctx, `
			INSERT INTO comment_reactions (comment_id, user_id, emoji)
			VALUES ($1, $2, $3)
			ON CONFLICT DO NOTHING`, commentID, userID, emoji); err != nil {
			return apperror.Wrap(apperror.KindInternal, "failed to add reaction", err)
		}
		return nil
	}
	if _, err := r.db.ExecContext(ctx, `
		DELETE FROM comment_reactions WHERE comment_id = $1 AND user_id = $2 AND emoji = $3`,
		commentID, userID, emoji); err != nil {
		return apperror.Wrap(apperror.KindInternal, "failed to remove reaction", err)
	}
	return nil
}

// loadCommentRows loads the comments with the given ids of one issue, deleted ones included, in one statement.
func loadCommentRows(ctx context.Context, db *sqlx.DB, issueID uuid.UUID, ids []uuid.UUID) (map[uuid.UUID]*domain.Comment, error) {
	out := make(map[uuid.UUID]*domain.Comment, len(ids))
	if len(ids) == 0 {
		return out, nil
	}
	var rows []commentRow
	if err := db.SelectContext(ctx, &rows, `
		SELECT `+commentColumns+` FROM comments c
		WHERE c.issue_id = $1 AND c.id = ANY($2::uuid[])`, issueID, commentIDStrings(ids)); err != nil {
		return nil, apperror.Wrap(apperror.KindInternal, "failed to load comments", err)
	}
	for _, row := range rows {
		out[row.ID] = row.toDomain()
	}
	return out, nil
}

func rowsToComments(rows []commentRow) []*domain.Comment {
	out := make([]*domain.Comment, 0, len(rows))
	for _, row := range rows {
		out = append(out, row.toDomain())
	}
	return out
}

func commentIDStrings(ids []uuid.UUID) []string {
	out := make([]string, 0, len(ids))
	for _, id := range ids {
		out = append(out, id.String())
	}
	return out
}

// requireRow maps a statement that must touch one row: no row means the comment was deleted or missing.
func requireRow(result sql.Result, message string) error {
	affected, err := result.RowsAffected()
	if err != nil {
		return apperror.Wrap(apperror.KindInternal, message, err)
	}
	if affected == 0 {
		return domain.ErrCommentNotFound
	}
	return nil
}
