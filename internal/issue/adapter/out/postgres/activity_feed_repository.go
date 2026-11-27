package postgres

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"

	"github.com/jhonsferg/corelog/internal/issue/application/port"
	"github.com/jhonsferg/corelog/internal/issue/domain"
	"github.com/jhonsferg/corelog/internal/platform/apperror"
)

// ActivityFeedRepository reads the comments and history rows behind the activity feed.
type ActivityFeedRepository struct {
	db *sqlx.DB
}

func NewActivityFeedRepository(db *sqlx.DB) *ActivityFeedRepository {
	return &ActivityFeedRepository{db: db}
}

var _ port.ActivityFeedReader = (*ActivityFeedRepository)(nil)

type feedRefRow struct {
	Kind   string    `db:"kind"`
	ItemID uuid.UUID `db:"item_id"`
	At     time.Time `db:"at"`
}

// referenceTables maps reference kinds to their catalog table. Only these names are interpolated in SQL.
var referenceTables = map[string]string{
	port.ReferencePriority:    "priorities",
	port.ReferenceIssueType:   "issue_types",
	port.ReferenceLabel:       "labels",
	port.ReferenceCustomField: "custom_fields",
}

// FeedRefs lists the entries of the feed. A history entry is the group of activity rows that share a batch id
// (rows without a batch form an entry of their own, keyed by their id).
func (r *ActivityFeedRepository) FeedRefs(ctx context.Context, issueID uuid.UUID, filter string, after *domain.ActivityCursor, limit int) ([]port.FeedRef, error) {
	var parts []string
	if filter != domain.ActivityFilterComments {
		parts = append(parts, `
			SELECT 'history' AS kind, COALESCE(a.batch_id, a.id) AS item_id, MAX(a.created_at) AS at
			FROM activities a
			WHERE a.issue_id = $1
			GROUP BY COALESCE(a.batch_id, a.id)`)
	}
	if filter != domain.ActivityFilterHistory {
		parts = append(parts, `
			SELECT 'comment' AS kind, c.id AS item_id, c.created_at AS at
			FROM comments c
			WHERE c.issue_id = $1 AND c.deleted_at IS NULL`)
	}
	if len(parts) == 0 {
		return nil, apperror.New(apperror.KindValidation, "invalid activity filter")
	}
	args := []any{issueID}
	query := "SELECT kind, item_id, at FROM (" + strings.Join(parts, " UNION ALL ") + ") feed"
	if after != nil {
		args = append(args, after.At, after.Kind, after.ID)
		query += " WHERE (at, kind, item_id) < ($2, $3, $4)"
	}
	args = append(args, limit)
	query += fmt.Sprintf(" ORDER BY at DESC, kind DESC, item_id DESC LIMIT $%d", len(args))

	var rows []feedRefRow
	if err := r.db.SelectContext(ctx, &rows, query, args...); err != nil {
		return nil, apperror.Wrap(apperror.KindInternal, "failed to list activity", err)
	}
	refs := make([]port.FeedRef, 0, len(rows))
	for _, row := range rows {
		refs = append(refs, port.FeedRef{Kind: row.Kind, ID: row.ItemID, At: row.At})
	}
	return refs, nil
}

// FeedContent loads the rows of the entries with two statements at most, whatever the number of entries.
func (r *ActivityFeedRepository) FeedContent(ctx context.Context, issueID uuid.UUID, refs []port.FeedRef) (port.FeedContent, error) {
	content := port.FeedContent{
		Comments:   map[uuid.UUID]*domain.Comment{},
		Activities: map[uuid.UUID][]*domain.Activity{},
	}
	var commentIDs, batchIDs []string
	for _, ref := range refs {
		if ref.Kind == domain.FeedKindComment {
			commentIDs = append(commentIDs, ref.ID.String())
		} else {
			batchIDs = append(batchIDs, ref.ID.String())
		}
	}
	if len(commentIDs) > 0 {
		var rows []commentRow
		if err := r.db.SelectContext(ctx, &rows, `
			SELECT id, issue_id, author_id, body, created_at, updated_at, deleted_at
			FROM comments
			WHERE issue_id = $1 AND id = ANY($2::uuid[])`, issueID, commentIDs); err != nil {
			return content, apperror.Wrap(apperror.KindInternal, "failed to load comments", err)
		}
		for _, row := range rows {
			content.Comments[row.ID] = &domain.Comment{
				ID: row.ID, IssueID: row.IssueID, AuthorID: row.AuthorID, Body: row.Body,
				CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt, DeletedAt: row.DeletedAt,
			}
		}
	}
	if len(batchIDs) > 0 {
		var rows []activityRow
		if err := r.db.SelectContext(ctx, &rows, `
			SELECT id, issue_id, actor_id, action, field_name, old_value, new_value, batch_id, created_at
			FROM activities
			WHERE issue_id = $1 AND COALESCE(batch_id, id) = ANY($2::uuid[])
			ORDER BY created_at ASC, id ASC`, issueID, batchIDs); err != nil {
			return content, apperror.Wrap(apperror.KindInternal, "failed to load activity", err)
		}
		for _, row := range rows {
			activity := &domain.Activity{
				ID: row.ID, IssueID: row.IssueID, ActorID: row.ActorID, Action: row.Action,
				FieldName: row.FieldName, OldValue: row.OldValue, NewValue: row.NewValue,
				BatchID: row.BatchID, CreatedAt: row.CreatedAt,
			}
			key := row.ID
			if row.BatchID != nil {
				key = *row.BatchID
			}
			content.Activities[key] = append(content.Activities[key], activity)
		}
	}
	return content, nil
}

type nameRow struct {
	ID   uuid.UUID `db:"id"`
	Name string    `db:"name"`
}

// ReferenceNames returns the current names of the catalog rows with the given ids.
func (r *ActivityFeedRepository) ReferenceNames(ctx context.Context, kind string, ids []uuid.UUID) (map[uuid.UUID]string, error) {
	table, ok := referenceTables[kind]
	if !ok {
		return nil, apperror.New(apperror.KindValidation, "unknown reference kind")
	}
	names := make(map[uuid.UUID]string, len(ids))
	if len(ids) == 0 {
		return names, nil
	}
	strIDs := make([]string, 0, len(ids))
	for _, id := range ids {
		strIDs = append(strIDs, id.String())
	}
	var rows []nameRow
	query := fmt.Sprintf("SELECT id, name FROM %s WHERE id = ANY($1::uuid[])", table)
	if err := r.db.SelectContext(ctx, &rows, query, strIDs); err != nil {
		return nil, apperror.Wrap(apperror.KindInternal, "failed to resolve names", err)
	}
	for _, row := range rows {
		names[row.ID] = row.Name
	}
	return names, nil
}
