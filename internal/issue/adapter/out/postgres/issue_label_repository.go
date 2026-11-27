package postgres

import (
	"context"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"

	"github.com/jhonsferg/corelog/internal/issue/application/port"
	"github.com/jhonsferg/corelog/internal/platform/apperror"
	"github.com/jhonsferg/corelog/internal/platform/database"
	"github.com/jhonsferg/corelog/internal/platform/palette"
)

// IssueLabelRepository reads and writes the labels of issues. It joins the transaction of the context when
// there is one.
type IssueLabelRepository struct {
	db *sqlx.DB
}

func NewIssueLabelRepository(db *sqlx.DB) *IssueLabelRepository {
	return &IssueLabelRepository{db: db}
}

var _ port.IssueLabelRepository = (*IssueLabelRepository)(nil)

// IssueLabelIDs returns the sorted ids of the labels of the issue.
func (r *IssueLabelRepository) IssueLabelIDs(ctx context.Context, issueID uuid.UUID) ([]uuid.UUID, error) {
	var ids []uuid.UUID
	if err := sqlx.SelectContext(ctx, database.Executor(ctx, r.db), &ids, `
		SELECT label_id FROM issue_labels WHERE issue_id = $1 ORDER BY label_id`, issueID); err != nil {
		return nil, apperror.Wrap(apperror.KindInternal, "failed to read issue labels", err)
	}
	return ids, nil
}

// ResolveLabelNames finds or creates the labels of the names in one statement. Names match the stored names
// exactly, as the unique key of the labels table does.
func (r *IssueLabelRepository) ResolveLabelNames(ctx context.Context, projectID uuid.UUID, names []string) ([]uuid.UUID, error) {
	if len(names) == 0 {
		return []uuid.UUID{}, nil
	}
	type resolved struct {
		ID   uuid.UUID `db:"id"`
		Name string    `db:"name"`
	}
	var rows []resolved
	if err := sqlx.SelectContext(ctx, database.Executor(ctx, r.db), &rows, `
		INSERT INTO labels (project_id, name, color)
		SELECT $1, n, $3 FROM unnest($2::text[]) AS n
		ON CONFLICT (project_id, name) DO UPDATE SET name = EXCLUDED.name
		RETURNING id, name`, projectID, names, palette.Default); err != nil {
		return nil, apperror.Wrap(apperror.KindInternal, "failed to resolve labels", err)
	}
	byName := make(map[string]uuid.UUID, len(rows))
	for _, row := range rows {
		byName[row.Name] = row.ID
	}
	ids := make([]uuid.UUID, 0, len(names))
	for _, name := range names {
		id, ok := byName[name]
		if !ok {
			return nil, apperror.New(apperror.KindInternal, "a label could not be resolved")
		}
		ids = append(ids, id)
	}
	return ids, nil
}

// ReplaceIssueLabels sets the labels of the issue to exactly labelIDs.
func (r *IssueLabelRepository) ReplaceIssueLabels(ctx context.Context, issueID uuid.UUID, labelIDs []uuid.UUID) error {
	exec := database.Executor(ctx, r.db)
	if _, err := exec.ExecContext(ctx, `DELETE FROM issue_labels WHERE issue_id = $1`, issueID); err != nil {
		return apperror.Wrap(apperror.KindInternal, "failed to clear issue labels", err)
	}
	if len(labelIDs) == 0 {
		return nil
	}
	ids := make([]string, 0, len(labelIDs))
	for _, id := range labelIDs {
		ids = append(ids, id.String())
	}
	if _, err := exec.ExecContext(ctx, `
		INSERT INTO issue_labels (issue_id, label_id)
		SELECT $1, label_id FROM unnest($2::uuid[]) AS label_id`, issueID, ids); err != nil {
		return apperror.Wrap(apperror.KindInternal, "failed to save issue labels", err)
	}
	return nil
}
