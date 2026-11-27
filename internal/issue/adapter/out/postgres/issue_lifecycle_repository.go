package postgres

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"

	"github.com/jhonsferg/corelog/internal/issue/application/port"
	"github.com/jhonsferg/corelog/internal/issue/domain"
	"github.com/jhonsferg/corelog/internal/platform/apperror"
	"github.com/jhonsferg/corelog/internal/platform/database"
	"github.com/jhonsferg/corelog/internal/platform/rank"
)

var (
	_ port.ViewReader      = (*IssueRepository)(nil)
	_ port.HierarchyReader = (*IssueRepository)(nil)
	_ port.IssueLifecycle  = (*IssueRepository)(nil)
	_ port.ProjectDefaults = (*IssueRepository)(nil)
)

type viewExtrasRow struct {
	Comments      int        `db:"comments"`
	Attachments   int        `db:"attachments"`
	Children      int        `db:"children"`
	Links         int        `db:"links"`
	Watchers      int        `db:"watchers"`
	Votes         int        `db:"votes"`
	ProjectID     uuid.UUID  `db:"project_id"`
	ProjectKey    string     `db:"project_key"`
	ProjectName   string     `db:"project_name"`
	ParentID      *uuid.UUID `db:"parent_id"`
	ParentNumber  *int       `db:"parent_number"`
	ParentKey     *string    `db:"parent_project_key"`
	ParentSummary *string    `db:"parent_summary"`
	ParentType    *string    `db:"parent_type"`
}

// ViewExtras reads the counters and the breadcrumb of a live issue in one statement.
func (r *IssueRepository) ViewExtras(ctx context.Context, issueID uuid.UUID) (domain.IssueCounts, domain.Breadcrumb, error) {
	var row viewExtrasRow
	err := sqlx.GetContext(ctx, database.Executor(ctx, r.db), &row, `
		SELECT
		  (SELECT COUNT(*) FROM comments c WHERE c.issue_id = i.id AND c.deleted_at IS NULL)::int AS comments,
		  (SELECT COUNT(*) FROM attachments a WHERE a.issue_id = i.id)::int AS attachments,
		  (SELECT COUNT(*) FROM issues ch WHERE ch.parent_id = i.id AND ch.deleted_at IS NULL)::int AS children,
		  (SELECT COUNT(*) FROM issue_links l WHERE l.source_issue_id = i.id OR l.target_issue_id = i.id)::int AS links,
		  (SELECT COUNT(*) FROM watchers w WHERE w.issue_id = i.id)::int AS watchers,
		  i.vote_count AS votes,
		  p.id AS project_id, p.key AS project_key, p.name AS project_name,
		  par.id AS parent_id, par.key_number AS parent_number, pp.key AS parent_project_key,
		  par.summary AS parent_summary, par.issue_type AS parent_type
		FROM issues i
		JOIN projects p ON p.id = i.project_id AND p.deleted_at IS NULL
		LEFT JOIN issues par ON par.id = i.parent_id AND par.deleted_at IS NULL
		LEFT JOIN projects pp ON pp.id = par.project_id
		WHERE i.id = $1 AND i.deleted_at IS NULL`, issueID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return domain.IssueCounts{}, domain.Breadcrumb{}, domain.ErrIssueNotFound
		}
		return domain.IssueCounts{}, domain.Breadcrumb{}, apperror.Wrap(apperror.KindInternal, "failed to load issue view data", err)
	}
	counts := domain.IssueCounts{
		Comments: row.Comments, Attachments: row.Attachments, Children: row.Children,
		Links: row.Links, Watchers: row.Watchers, Votes: row.Votes,
	}
	crumb := domain.Breadcrumb{ProjectID: row.ProjectID, ProjectKey: row.ProjectKey, ProjectName: row.ProjectName}
	if row.ParentID != nil {
		parent := &domain.ParentRef{ID: *row.ParentID, Summary: deref(row.ParentSummary), Type: deref(row.ParentType)}
		if row.ParentNumber != nil {
			parent.Key = (&domain.Issue{ProjectKey: deref(row.ParentKey), KeyNumber: *row.ParentNumber}).Key()
		}
		crumb.Parent = parent
	}
	return counts, crumb, nil
}

// ChildLevels returns the hierarchy level of every live child of the issue.
func (r *IssueRepository) ChildLevels(ctx context.Context, parentID uuid.UUID) ([]int, error) {
	var levels []int
	if err := sqlx.SelectContext(ctx, database.Executor(ctx, r.db), &levels, `
		SELECT DISTINCT it.hierarchy_level::int
		FROM issues ch
		JOIN issue_types it ON it.id = ch.issue_type_id
		WHERE ch.parent_id = $1 AND ch.deleted_at IS NULL`, parentID); err != nil {
		return nil, apperror.Wrap(apperror.KindInternal, "failed to read child levels", err)
	}
	return levels, nil
}

// SoftDelete marks a live issue as deleted. A missing or already deleted issue is not found.
func (r *IssueRepository) SoftDelete(ctx context.Context, id uuid.UUID, at time.Time) error {
	result, err := database.Executor(ctx, r.db).ExecContext(ctx, `
		UPDATE issues SET deleted_at = $2, updated_at = $2
		WHERE id = $1 AND deleted_at IS NULL
		  AND EXISTS (SELECT 1 FROM projects p WHERE p.id = issues.project_id AND p.deleted_at IS NULL)`, id, at)
	if err != nil {
		return apperror.Wrap(apperror.KindInternal, "failed to delete issue", err)
	}
	return requireIssueAffected(result)
}

// Restore clears the deletion mark of a deleted issue in a live project.
func (r *IssueRepository) Restore(ctx context.Context, id uuid.UUID, at time.Time) error {
	result, err := database.Executor(ctx, r.db).ExecContext(ctx, `
		UPDATE issues SET deleted_at = NULL, updated_at = $2
		WHERE id = $1 AND deleted_at IS NOT NULL
		  AND EXISTS (SELECT 1 FROM projects p WHERE p.id = issues.project_id AND p.deleted_at IS NULL)`, id, at)
	if err != nil {
		return apperror.Wrap(apperror.KindInternal, "failed to restore issue", err)
	}
	return requireIssueAffected(result)
}

func requireIssueAffected(result sql.Result) error {
	affected, err := result.RowsAffected()
	if err != nil {
		return apperror.Wrap(apperror.KindInternal, "failed to write issue", err)
	}
	if affected == 0 {
		return domain.ErrIssueNotFound
	}
	return nil
}

// FindDeleted returns a soft-deleted issue of a live project.
func (r *IssueRepository) FindDeleted(ctx context.Context, id uuid.UUID) (*domain.Issue, error) {
	var row issueRow
	if err := sqlx.GetContext(ctx, database.Executor(ctx, r.db), &row, `
		SELECT `+issueColumns+`
		FROM issues WHERE id = $1 AND deleted_at IS NOT NULL
		  AND EXISTS (SELECT 1 FROM projects p WHERE p.id = issues.project_id AND p.deleted_at IS NULL)`, id); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, domain.ErrIssueNotFound
		}
		return nil, apperror.Wrap(apperror.KindInternal, "failed to find deleted issue", err)
	}
	return row.toDomain(), nil
}

// LockForUpdate takes the row lock of a live issue until the surrounding transaction ends.
func (r *IssueRepository) LockForUpdate(ctx context.Context, id uuid.UUID) error {
	var locked uuid.UUID
	if err := database.Executor(ctx, r.db).QueryRowxContext(ctx,
		`SELECT id FROM issues WHERE id = $1 AND deleted_at IS NULL FOR UPDATE`, id).Scan(&locked); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return domain.ErrIssueNotFound
		}
		return apperror.Wrap(apperror.KindInternal, "failed to lock issue", err)
	}
	return nil
}

// NextRank returns a rank after the last live issue of the project.
func (r *IssueRepository) NextRank(ctx context.Context, projectID uuid.UUID) (string, error) {
	var last string
	if err := database.Executor(ctx, r.db).QueryRowxContext(ctx, `
		SELECT COALESCE(MAX(rank COLLATE "C"), '') FROM issues WHERE project_id = $1 AND deleted_at IS NULL`,
		projectID).Scan(&last); err != nil {
		return "", apperror.Wrap(apperror.KindInternal, "failed to read the last rank", err)
	}
	next, err := rank.Between(last, "")
	if err != nil {
		// Legacy ranks such as "000010" are not valid fractional ranks; appending a digit still sorts after them.
		return last + "m", nil
	}
	return next, nil
}

// DefaultAssigneePolicy reads the assignee policy and the lead of a live project.
func (r *IssueRepository) DefaultAssigneePolicy(ctx context.Context, projectID uuid.UUID) (string, *uuid.UUID, error) {
	var policy string
	var lead *uuid.UUID
	if err := database.Executor(ctx, r.db).QueryRowxContext(ctx, `
		SELECT default_assignee_policy, lead_id FROM projects WHERE id = $1 AND deleted_at IS NULL`,
		projectID).Scan(&policy, &lead); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "unassigned", nil, nil
		}
		return "", nil, apperror.Wrap(apperror.KindInternal, "failed to read project defaults", err)
	}
	return policy, lead, nil
}
