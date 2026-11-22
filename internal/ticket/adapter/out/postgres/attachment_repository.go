package postgres

import (
	"context"
	"database/sql"
	"time"

	"github.com/google/uuid"
	"github.com/jhonsferg/corelog/internal/platform/apperror"
	"github.com/jhonsferg/corelog/internal/ticket/application/port"
	"github.com/jhonsferg/corelog/internal/ticket/domain"
	"github.com/jmoiron/sqlx"
)

type AttachmentRepository struct{ db *sqlx.DB }

func NewAttachmentRepository(db *sqlx.DB) *AttachmentRepository { return &AttachmentRepository{db: db} }

var _ port.AttachmentRepository = (*AttachmentRepository)(nil)

type attachmentRow struct {
	ID          uuid.UUID `db:"id"`
	IssueID     uuid.UUID `db:"issue_id"`
	UploaderID  uuid.UUID `db:"uploader_id"`
	FileName    string    `db:"file_name"`
	ContentType string    `db:"content_type"`
	Size        int64     `db:"size_bytes"`
	StorageKey  uuid.UUID `db:"storage_key"`
	CreatedAt   time.Time `db:"created_at"`
}

func (r *AttachmentRepository) SaveAttachment(ctx context.Context, a *domain.Attachment) error {
	key, err := uuid.Parse(a.StorageKey)
	if err != nil {
		return apperror.Wrap(apperror.KindInternal, "invalid storage key", err)
	}
	_, err = r.db.ExecContext(ctx, `INSERT INTO attachments(id,issue_id,uploader_id,file_name,content_type,size_bytes,storage_key,created_at) VALUES($1,$2,$3,$4,$5,$6,$7,$8)`, a.ID, a.IssueID, a.UploaderID, a.FileName, a.ContentType, a.Size, key, a.CreatedAt)
	if err != nil {
		return apperror.Wrap(apperror.KindInternal, "failed to save attachment", err)
	}
	return nil
}
func (r *AttachmentRepository) FindAttachment(ctx context.Context, issueID, id uuid.UUID) (*domain.Attachment, error) {
	var row attachmentRow
	err := r.db.GetContext(ctx, &row, `SELECT id,issue_id,uploader_id,file_name,content_type,size_bytes,storage_key,created_at FROM attachments WHERE issue_id=$1 AND id=$2`, issueID, id)
	if err == sql.ErrNoRows {
		return nil, apperror.New(apperror.KindNotFound, "attachment not found")
	}
	if err != nil {
		return nil, apperror.Wrap(apperror.KindInternal, "failed to find attachment", err)
	}
	return attachmentFromRow(row), nil
}
func (r *AttachmentRepository) ListAttachments(ctx context.Context, issueID uuid.UUID) ([]*domain.Attachment, error) {
	var rows []attachmentRow
	if err := r.db.SelectContext(ctx, &rows, `SELECT id,issue_id,uploader_id,file_name,content_type,size_bytes,storage_key,created_at FROM attachments WHERE issue_id=$1 ORDER BY created_at,id`, issueID); err != nil {
		return nil, apperror.Wrap(apperror.KindInternal, "failed to list attachments", err)
	}
	out := make([]*domain.Attachment, 0, len(rows))
	for _, row := range rows {
		out = append(out, attachmentFromRow(row))
	}
	return out, nil
}
func (r *AttachmentRepository) DeleteAttachment(ctx context.Context, issueID, id uuid.UUID) error {
	res, err := r.db.ExecContext(ctx, `DELETE FROM attachments WHERE issue_id=$1 AND id=$2`, issueID, id)
	if err != nil {
		return apperror.Wrap(apperror.KindInternal, "failed to delete attachment", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return apperror.Wrap(apperror.KindInternal, "failed to delete attachment", err)
	}
	if n == 0 {
		return apperror.New(apperror.KindNotFound, "attachment not found")
	}
	return nil
}
func attachmentFromRow(r attachmentRow) *domain.Attachment {
	return &domain.Attachment{ID: r.ID, IssueID: r.IssueID, UploaderID: r.UploaderID, FileName: r.FileName, ContentType: r.ContentType, Size: r.Size, StorageKey: r.StorageKey.String(), CreatedAt: r.CreatedAt}
}
