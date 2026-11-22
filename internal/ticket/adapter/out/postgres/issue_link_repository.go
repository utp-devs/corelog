package postgres

import (
	"context"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"

	"github.com/jhonsferg/corelog/internal/platform/apperror"
	"github.com/jhonsferg/corelog/internal/ticket/application/port"
	"github.com/jhonsferg/corelog/internal/ticket/domain"
)

type issueLinkRow struct {
	ID            uuid.UUID `db:"id"`
	SourceIssueID uuid.UUID `db:"source_issue_id"`
	TargetIssueID uuid.UUID `db:"target_issue_id"`
	Kind          string    `db:"kind"`
	CreatedBy     uuid.UUID `db:"created_by"`
	CreatedAt     time.Time `db:"created_at"`
}

type IssueLinkRepository struct {
	db *sqlx.DB
}

func NewIssueLinkRepository(db *sqlx.DB) *IssueLinkRepository {
	return &IssueLinkRepository{db: db}
}

var _ port.IssueLinkRepository = (*IssueLinkRepository)(nil)

func (r *IssueLinkRepository) Save(ctx context.Context, link *domain.IssueLink) error {
	err := r.db.QueryRowContext(ctx, `
		INSERT INTO issue_links (source_issue_id, target_issue_id, kind, created_at, created_by)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id`,
		link.SourceIssueID, link.TargetIssueID, string(link.Kind), link.CreatedAt, link.CreatedBy).Scan(&link.ID)
	if err != nil {
		if strings.Contains(err.Error(), "duplicate key") {
			return domain.ErrDuplicateLink
		}
		return apperror.Wrap(apperror.KindInternal, "failed to save issue link", err)
	}
	return nil
}

func (r *IssueLinkRepository) ListByIssue(ctx context.Context, issueID uuid.UUID) ([]*domain.IssueLink, error) {
	var rows []issueLinkRow
	if err := r.db.SelectContext(ctx, &rows, `
		SELECT id, source_issue_id, target_issue_id, kind, created_by, created_at
		FROM issue_links
		WHERE source_issue_id = $1 OR target_issue_id = $1
		ORDER BY created_at ASC`,
		issueID,
	); err != nil {
		return nil, apperror.Wrap(apperror.KindInternal, "failed to list issue links", err)
	}
	links := make([]*domain.IssueLink, 0, len(rows))
	for _, row := range rows {
		links = append(links, &domain.IssueLink{
			ID: row.ID, SourceIssueID: row.SourceIssueID, TargetIssueID: row.TargetIssueID,
			Kind: domain.IssueLinkKind(row.Kind), CreatedBy: row.CreatedBy, CreatedAt: row.CreatedAt,
		})
	}
	return links, nil
}

func (r *IssueLinkRepository) Delete(ctx context.Context, issueID, linkID uuid.UUID) error {
	result, err := r.db.ExecContext(ctx, `DELETE FROM issue_links WHERE id = $1 AND (source_issue_id = $2 OR target_issue_id = $2)`, linkID, issueID)
	if err != nil {
		return apperror.Wrap(apperror.KindInternal, "failed to delete issue link", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return apperror.Wrap(apperror.KindInternal, "failed to delete issue link", err)
	}
	if affected == 0 {
		return apperror.New(apperror.KindNotFound, "issue link not found")
	}
	return nil
}
