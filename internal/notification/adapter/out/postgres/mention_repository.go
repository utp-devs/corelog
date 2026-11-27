package postgres

import (
	"context"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"

	"github.com/jhonsferg/corelog/internal/notification/application/port"
	"github.com/jhonsferg/corelog/internal/platform/apperror"
)

// MentionRepository keeps the notices sent for comments and reads their mentions.
type MentionRepository struct {
	db *sqlx.DB
}

func NewMentionRepository(db *sqlx.DB) *MentionRepository {
	return &MentionRepository{db: db}
}

var _ port.MentionDeliveries = (*MentionRepository)(nil)

func (r *MentionRepository) NoticeSent(ctx context.Context, commentID, userID uuid.UUID, kind string) (bool, error) {
	var sent bool
	if err := r.db.GetContext(ctx, &sent, `
		SELECT EXISTS (SELECT 1 FROM comment_notices WHERE comment_id = $1 AND user_id = $2 AND kind = $3)`,
		commentID, userID, kind); err != nil {
		return false, apperror.Wrap(apperror.KindInternal, "failed to read comment notices", err)
	}
	return sent, nil
}

func (r *MentionRepository) RecordNotice(ctx context.Context, commentID, userID uuid.UUID, kind string) error {
	if _, err := r.db.ExecContext(ctx, `
		INSERT INTO comment_notices (comment_id, user_id, kind)
		VALUES ($1, $2, $3)
		ON CONFLICT DO NOTHING`, commentID, userID, kind); err != nil {
		return apperror.Wrap(apperror.KindInternal, "failed to record comment notice", err)
	}
	return nil
}

func (r *MentionRepository) IsMentioned(ctx context.Context, commentID, userID uuid.UUID) (bool, error) {
	var mentioned bool
	if err := r.db.GetContext(ctx, &mentioned, `
		SELECT EXISTS (SELECT 1 FROM comment_mentions WHERE comment_id = $1 AND user_id = $2)`,
		commentID, userID); err != nil {
		return false, apperror.Wrap(apperror.KindInternal, "failed to read comment mentions", err)
	}
	return mentioned, nil
}
