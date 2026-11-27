package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"

	"github.com/jhonsferg/corelog/internal/issue/application/port"
	"github.com/jhonsferg/corelog/internal/issue/domain"
	"github.com/jhonsferg/corelog/internal/platform/apperror"
	"github.com/jhonsferg/corelog/internal/platform/database"
)

// OperationsRepository implements the out-ports of the bulk, clone, move and export operations.
type OperationsRepository struct {
	db *sqlx.DB
}

// NewOperationsRepository returns the adapter of the bulk, clone, move and export operations.
func NewOperationsRepository(db *sqlx.DB) *OperationsRepository {
	return &OperationsRepository{db: db}
}

var _ port.OperationsRepository = (*OperationsRepository)(nil)

// Relocate moves the issue to the target project. It takes the next number of the target counter, keeps the old
// key as an alias and removes the labels and components that belong to the source project. Run it inside the
// transaction of the move.
func (r *OperationsRepository) Relocate(ctx context.Context, issueID, sourceProjectID uuid.UUID, sourceNumber int, targetProjectID uuid.UUID) (port.RelocateResult, error) {
	exec := database.Executor(ctx, r.db)
	var result port.RelocateResult
	err := exec.QueryRowxContext(ctx, `
		UPDATE projects
		SET next_issue_number = next_issue_number + 1
		WHERE id = $1 AND deleted_at IS NULL
		RETURNING next_issue_number - 1, key`, targetProjectID).Scan(&result.KeyNumber, &result.ProjectKey)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return port.RelocateResult{}, apperror.New(apperror.KindNotFound, "project not found")
		}
		return port.RelocateResult{}, apperror.Wrap(apperror.KindInternal, "failed to allocate issue number", err)
	}
	if _, err := exec.ExecContext(ctx, `
		INSERT INTO issue_key_aliases (source_project_id, key_number, issue_id)
		VALUES ($1, $2, $3)
		ON CONFLICT (source_project_id, key_number) DO UPDATE SET issue_id = EXCLUDED.issue_id`,
		sourceProjectID, sourceNumber, issueID); err != nil {
		return port.RelocateResult{}, apperror.Wrap(apperror.KindInternal, "failed to keep the previous key", err)
	}
	res, err := exec.ExecContext(ctx, `
		UPDATE issues SET project_id = $2, key_number = $3
		WHERE id = $1 AND deleted_at IS NULL`, issueID, targetProjectID, result.KeyNumber)
	if err != nil {
		return port.RelocateResult{}, apperror.Wrap(apperror.KindInternal, "failed to move issue", err)
	}
	if affected, err := res.RowsAffected(); err != nil || affected == 0 {
		return port.RelocateResult{}, domain.ErrIssueNotFound
	}
	labels, err := exec.ExecContext(ctx, `
		DELETE FROM issue_labels il USING labels l
		WHERE il.issue_id = $1 AND l.id = il.label_id AND l.project_id <> $2`, issueID, targetProjectID)
	if err != nil {
		return port.RelocateResult{}, apperror.Wrap(apperror.KindInternal, "failed to drop labels of the source project", err)
	}
	result.DroppedLabels = rowsOf(labels)
	components, err := exec.ExecContext(ctx, `
		DELETE FROM issue_components ic USING components c
		WHERE ic.issue_id = $1 AND c.id = ic.component_id AND c.project_id <> $2`, issueID, targetProjectID)
	if err != nil {
		return port.RelocateResult{}, apperror.Wrap(apperror.KindInternal, "failed to drop components of the source project", err)
	}
	result.DroppedComponents = rowsOf(components)
	return result, nil
}

// FindAliasIssues returns the live issues that had the key projectKey-number. The source project must still
// exist under that key; orgID restricts the search when it is not Nil.
func (r *OperationsRepository) FindAliasIssues(ctx context.Context, projectKey string, number int, orgID uuid.UUID) ([]uuid.UUID, error) {
	var org any
	if orgID != uuid.Nil {
		org = orgID
	}
	var ids []uuid.UUID
	if err := sqlx.SelectContext(ctx, database.Executor(ctx, r.db), &ids, `
		SELECT DISTINCT a.issue_id
		FROM issue_key_aliases a
		JOIN projects p ON p.id = a.source_project_id AND p.deleted_at IS NULL
		JOIN issues i ON i.id = a.issue_id AND i.deleted_at IS NULL
		WHERE p.key = $1 AND a.key_number = $2 AND ($3::uuid IS NULL OR p.organization_id = $3::uuid)
		ORDER BY a.issue_id`, projectKey, number, org); err != nil {
		return nil, apperror.Wrap(apperror.KindInternal, "failed to find issue by previous key", err)
	}
	return ids, nil
}

// rowsOf counts the rows an exec affected, zero when the driver cannot tell.
func rowsOf(res sql.Result) int {
	n, err := res.RowsAffected()
	if err != nil {
		return 0
	}
	return int(n)
}

// idList renders the placeholders of an IN list and their arguments, starting after offset arguments.
func idList(ids []uuid.UUID, offset int) (string, []any) {
	placeholders := make([]string, 0, len(ids))
	args := make([]any, 0, len(ids))
	for i, id := range ids {
		placeholders = append(placeholders, fmt.Sprintf("$%d", offset+i+1))
		args = append(args, id)
	}
	return strings.Join(placeholders, ", "), args
}
