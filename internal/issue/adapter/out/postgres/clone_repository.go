package postgres

import (
	"context"

	"github.com/google/uuid"

	"github.com/jhonsferg/corelog/internal/platform/apperror"
	"github.com/jhonsferg/corelog/internal/platform/database"
)

// CopyWatchers makes every watcher of fromID also watch toID.
func (r *OperationsRepository) CopyWatchers(ctx context.Context, fromID, toID uuid.UUID) error {
	if _, err := database.Executor(ctx, r.db).ExecContext(ctx, `
		INSERT INTO watchers (issue_id, user_id)
		SELECT $2, user_id FROM watchers WHERE issue_id = $1
		ON CONFLICT (issue_id, user_id) DO NOTHING`, fromID, toID); err != nil {
		return apperror.Wrap(apperror.KindInternal, "failed to copy watchers", err)
	}
	return nil
}

// CopyProjectScoped copies the labels and the components of fromID to toID. Both issues must be in one project.
func (r *OperationsRepository) CopyProjectScoped(ctx context.Context, fromID, toID uuid.UUID) error {
	exec := database.Executor(ctx, r.db)
	if _, err := exec.ExecContext(ctx, `
		INSERT INTO issue_labels (issue_id, label_id)
		SELECT $2, label_id FROM issue_labels WHERE issue_id = $1
		ON CONFLICT (issue_id, label_id) DO NOTHING`, fromID, toID); err != nil {
		return apperror.Wrap(apperror.KindInternal, "failed to copy labels", err)
	}
	if _, err := exec.ExecContext(ctx, `
		INSERT INTO issue_components (issue_id, component_id)
		SELECT $2, component_id FROM issue_components WHERE issue_id = $1
		ON CONFLICT (issue_id, component_id) DO NOTHING`, fromID, toID); err != nil {
		return apperror.Wrap(apperror.KindInternal, "failed to copy components", err)
	}
	return nil
}

// CopyLinks copies the live links of fromID to toID with their kind and direction. Clones links are not copied.
func (r *OperationsRepository) CopyLinks(ctx context.Context, fromID, toID, actorID uuid.UUID) error {
	if _, err := database.Executor(ctx, r.db).ExecContext(ctx, `
		INSERT INTO issue_links (id, source_issue_id, target_issue_id, kind, created_at, created_by)
		SELECT gen_random_uuid(),
		       CASE WHEN l.source_issue_id = $1 THEN $2 ELSE l.source_issue_id END,
		       CASE WHEN l.target_issue_id = $1 THEN $2 ELSE l.target_issue_id END,
		       l.kind, now(), $3
		FROM issue_links l
		JOIN issues other ON other.id = (CASE WHEN l.source_issue_id = $1 THEN l.target_issue_id ELSE l.source_issue_id END)
		WHERE (l.source_issue_id = $1 OR l.target_issue_id = $1)
		  AND other.deleted_at IS NULL
		  AND l.kind <> 'clones'
		ON CONFLICT DO NOTHING`, fromID, toID, actorID); err != nil {
		return apperror.Wrap(apperror.KindInternal, "failed to copy links", err)
	}
	return nil
}
