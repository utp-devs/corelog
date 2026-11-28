package postgres

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"

	"github.com/jhonsferg/corelog/internal/issue/application/port"
	"github.com/jhonsferg/corelog/internal/issue/domain"
	"github.com/jhonsferg/corelog/internal/platform/apperror"
	"github.com/jhonsferg/corelog/internal/platform/database"
)

type issueLinkRow struct {
	ID            uuid.UUID `db:"id"`
	SourceIssueID uuid.UUID `db:"source_issue_id"`
	TargetIssueID uuid.UUID `db:"target_issue_id"`
	Kind          string    `db:"kind"`
	CreatedBy     uuid.UUID `db:"created_by"`
	CreatedAt     time.Time `db:"created_at"`
}

type linkedIssueRow struct {
	LinkID         uuid.UUID `db:"link_id"`
	Kind           string    `db:"kind"`
	Outward        bool      `db:"outward"`
	OtherID        uuid.UUID `db:"other_id"`
	OtherProjectID uuid.UUID `db:"other_project_id"`
	OrganizationID uuid.UUID `db:"organization_id"`
	ProjectKey     string    `db:"project_key"`
	KeyNumber      int       `db:"key_number"`
	Summary        string    `db:"summary"`
	Status         string    `db:"status"`
	TypeName       string    `db:"type_name"`
	PriorityName   string    `db:"priority_name"`
}

type linkableRow struct {
	ProjectID      uuid.UUID `db:"project_id"`
	OrganizationID uuid.UUID `db:"organization_id"`
	ProjectKey     string    `db:"project_key"`
	KeyNumber      int       `db:"key_number"`
	Summary        string    `db:"summary"`
	Status         string    `db:"status"`
	TypeName       string    `db:"type_name"`
	PriorityName   string    `db:"priority_name"`
}

// IssueLinkRepository stores the links between issues. Every read excludes soft-deleted issues and projects.
type IssueLinkRepository struct {
	db *sqlx.DB
}

func NewIssueLinkRepository(db *sqlx.DB) *IssueLinkRepository {
	return &IssueLinkRepository{db: db}
}

var _ port.IssueLinkRepository = (*IssueLinkRepository)(nil)

// FindLinkable returns a live issue with its organization and the display data used by the link rules.
func (r *IssueLinkRepository) FindLinkable(ctx context.Context, issueID uuid.UUID) (port.IssueSummary, error) {
	var row linkableRow
	err := database.Executor(ctx, r.db).QueryRowxContext(ctx, `
		SELECT i.project_id, p.organization_id, p.key AS project_key, i.key_number, i.summary, i.status,
		       COALESCE(it.name, '') AS type_name, COALESCE(pr.name, '') AS priority_name
		FROM issues i
		JOIN projects p ON p.id = i.project_id AND p.deleted_at IS NULL
		LEFT JOIN issue_types it ON it.id = i.issue_type_id
		LEFT JOIN priorities pr ON pr.id = i.priority_id
		WHERE i.id = $1 AND i.deleted_at IS NULL`, issueID).Scan(
		&row.ProjectID, &row.OrganizationID, &row.ProjectKey, &row.KeyNumber, &row.Summary, &row.Status,
		&row.TypeName, &row.PriorityName)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return port.IssueSummary{}, domain.ErrIssueNotFound
		}
		return port.IssueSummary{}, apperror.Wrap(apperror.KindInternal, "failed to read issue for link", err)
	}
	return port.IssueSummary{
		ID: issueID, ProjectID: row.ProjectID, OrganizationID: row.OrganizationID,
		Key:     (&domain.Issue{ProjectKey: row.ProjectKey, KeyNumber: row.KeyNumber}).Key(),
		Summary: row.Summary, Status: domain.Status(row.Status), TypeName: row.TypeName, PriorityName: row.PriorityName,
	}, nil
}

// ListLinks returns the links of the issue whose other end is live, ordered by kind and creation.
func (r *IssueLinkRepository) ListLinks(ctx context.Context, issueID uuid.UUID) ([]port.LinkedIssue, error) {
	var rows []linkedIssueRow
	if err := sqlx.SelectContext(ctx, database.Executor(ctx, r.db), &rows, `
		SELECT l.id AS link_id, l.kind, (l.source_issue_id = $1) AS outward,
		       o.id AS other_id, o.project_id AS other_project_id, p.organization_id, p.key AS project_key,
		       o.key_number, o.summary, o.status,
		       COALESCE(it.name, '') AS type_name, COALESCE(pr.name, '') AS priority_name
		FROM issue_links l
		JOIN issues o ON o.id = CASE WHEN l.source_issue_id = $1 THEN l.target_issue_id ELSE l.source_issue_id END
		JOIN projects p ON p.id = o.project_id AND p.deleted_at IS NULL
		LEFT JOIN issue_types it ON it.id = o.issue_type_id
		LEFT JOIN priorities pr ON pr.id = o.priority_id
		WHERE (l.source_issue_id = $1 OR l.target_issue_id = $1) AND o.deleted_at IS NULL AND l.deleted_at IS NULL
		ORDER BY l.kind, l.created_at, l.id`, issueID); err != nil {
		return nil, apperror.Wrap(apperror.KindInternal, "failed to list issue links", err)
	}
	out := make([]port.LinkedIssue, 0, len(rows))
	for _, row := range rows {
		out = append(out, port.LinkedIssue{
			LinkID: row.LinkID, Kind: domain.IssueLinkKind(row.Kind), Outward: row.Outward,
			Other: port.IssueSummary{
				ID: row.OtherID, ProjectID: row.OtherProjectID, OrganizationID: row.OrganizationID,
				Key:     (&domain.Issue{ProjectKey: row.ProjectKey, KeyNumber: row.KeyNumber}).Key(),
				Summary: row.Summary, Status: domain.Status(row.Status), TypeName: row.TypeName, PriorityName: row.PriorityName,
			},
		})
	}
	return out, nil
}

// FindLinkForIssue returns the link only when the issue is one of its ends.
func (r *IssueLinkRepository) FindLinkForIssue(ctx context.Context, issueID, linkID uuid.UUID) (*domain.IssueLink, error) {
	var row issueLinkRow
	if err := sqlx.GetContext(ctx, database.Executor(ctx, r.db), &row, `
		SELECT id, source_issue_id, target_issue_id, kind, created_by, created_at
		FROM issue_links
		WHERE id = $1 AND deleted_at IS NULL AND (source_issue_id = $2 OR target_issue_id = $2)`, linkID, issueID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, domain.ErrIssueLinkNotFound
		}
		return nil, apperror.Wrap(apperror.KindInternal, "failed to find issue link", err)
	}
	return &domain.IssueLink{
		ID: row.ID, SourceIssueID: row.SourceIssueID, TargetIssueID: row.TargetIssueID,
		Kind: domain.IssueLinkKind(row.Kind), CreatedBy: row.CreatedBy, CreatedAt: row.CreatedAt,
	}, nil
}

// PairExists reports whether the two issues already share a link of the kind, in either direction.
func (r *IssueLinkRepository) PairExists(ctx context.Context, a, b uuid.UUID, kind domain.IssueLinkKind) (bool, error) {
	var exists bool
	if err := database.Executor(ctx, r.db).QueryRowxContext(ctx, `
		SELECT EXISTS (
		  SELECT 1 FROM issue_links
		  WHERE kind = $3 AND deleted_at IS NULL AND ((source_issue_id = $1 AND target_issue_id = $2) OR (source_issue_id = $2 AND target_issue_id = $1)))`,
		a, b, string(kind)).Scan(&exists); err != nil {
		return false, apperror.Wrap(apperror.KindInternal, "failed to check issue link", err)
	}
	return exists, nil
}

// BlocksReach follows blocks links from one issue and reports whether the other one is reachable. UNION keeps the
// walk finite even when the stored data already contains a cycle.
func (r *IssueLinkRepository) BlocksReach(ctx context.Context, from, to uuid.UUID) (bool, error) {
	var reached bool
	if err := database.Executor(ctx, r.db).QueryRowxContext(ctx, `
		WITH RECURSIVE reach(id) AS (
		  SELECT l.target_issue_id FROM issue_links l WHERE l.kind = 'blocks' AND l.deleted_at IS NULL AND l.source_issue_id = $1
		  UNION
		  SELECT l.target_issue_id FROM issue_links l JOIN reach rc ON l.source_issue_id = rc.id WHERE l.kind = 'blocks' AND l.deleted_at IS NULL
		)
		SELECT EXISTS (SELECT 1 FROM reach WHERE id = $2)`, from, to).Scan(&reached); err != nil {
		return false, apperror.Wrap(apperror.KindInternal, "failed to walk blocking links", err)
	}
	return reached, nil
}

// Save inserts the link. The unique (source, target, kind) constraint maps to a duplicate conflict.
func (r *IssueLinkRepository) Save(ctx context.Context, link *domain.IssueLink) error {
	err := database.Executor(ctx, r.db).QueryRowxContext(ctx, `
		INSERT INTO issue_links (source_issue_id, target_issue_id, kind, created_at, created_by)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id`,
		link.SourceIssueID, link.TargetIssueID, string(link.Kind), link.CreatedAt, link.CreatedBy).Scan(&link.ID)
	if err != nil {
		if strings.Contains(err.Error(), "duplicate key") || strings.Contains(err.Error(), "23505") {
			return domain.ErrDuplicateLink
		}
		return apperror.Wrap(apperror.KindInternal, "failed to save issue link", err)
	}
	return nil
}

// Delete logically deletes the link by id; the row stays for history. The caller has checked that the link touches the issue.
func (r *IssueLinkRepository) Delete(ctx context.Context, linkID uuid.UUID) error {
	result, err := database.Executor(ctx, r.db).ExecContext(ctx, `UPDATE issue_links SET deleted_at = now() WHERE id = $1 AND deleted_at IS NULL`, linkID)
	if err != nil {
		return apperror.Wrap(apperror.KindInternal, "failed to delete issue link", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return apperror.Wrap(apperror.KindInternal, "failed to delete issue link", err)
	}
	if affected == 0 {
		return domain.ErrIssueLinkNotFound
	}
	return nil
}
