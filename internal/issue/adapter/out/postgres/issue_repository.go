package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jmoiron/sqlx"

	"github.com/jhonsferg/corelog/internal/issue/application/port"
	"github.com/jhonsferg/corelog/internal/issue/domain"
	"github.com/jhonsferg/corelog/internal/platform/apperror"
	"github.com/jhonsferg/corelog/internal/platform/database"
)

const pgForeignKeyViolation = "23503"

type IssueRepository struct {
	db *sqlx.DB
}

func NewIssueRepository(db *sqlx.DB) *IssueRepository {
	return &IssueRepository{db: db}
}

var _ port.Repository = (*IssueRepository)(nil)
var _ port.AssigneeWatcherRepository = (*IssueRepository)(nil)

// Save inserts the issue. The project counter is advanced and the row is inserted in the same transaction,
// so concurrent creates get distinct, gap-free numbers and a failed insert does not burn a number.
func (r *IssueRepository) Save(ctx context.Context, t *domain.Issue) error {
	return database.NewTxManager(r.db).InTx(ctx, func(ctx context.Context) error {
		return r.save(ctx, t)
	})
}

func (r *IssueRepository) save(ctx context.Context, t *domain.Issue) error {
	exec := database.Executor(ctx, r.db)

	if t.ProjectID == domain.DefaultProjectID {
		// El proyecto por defecto puede no existir en una BD recien migrada. Se crea en la
		// organizacion mas antigua; si no hay ninguna, no se crea y el INSERT del issue falla con FK.
		if _, err := exec.ExecContext(ctx, `
			INSERT INTO projects (
				id, organization_id, key, name, description, type, lead_id, visibility, archived_at, created_at, updated_at
			)
			SELECT $1, o.id, 'CORE', 'CoreLog', 'Proyecto por defecto', 'kanban', $2, 'open', NULL, $3, $3
			FROM organizations o
			WHERE o.deleted_at IS NULL
			ORDER BY o.created_at
			LIMIT 1
			ON CONFLICT DO NOTHING`,
			domain.DefaultProjectID, t.ReporterID, t.CreatedAt,
		); err != nil {
			return apperror.Wrap(apperror.KindInternal, "failed to ensure default project", err)
		}
	}

	// A soft-deleted project accepts no new issues: it looks missing, as it does to every other read.
	// The counter update takes the project row lock, which serializes concurrent creates.
	var number int
	var projectKey string
	row := exec.QueryRowxContext(ctx, `
		UPDATE projects
		SET next_issue_number = next_issue_number + 1
		WHERE id = $1 AND deleted_at IS NULL
		RETURNING next_issue_number - 1, key`, t.ProjectID)
	if err := row.Scan(&number, &projectKey); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return apperror.New(apperror.KindNotFound, "project not found")
		}
		return apperror.Wrap(apperror.KindInternal, "failed to allocate issue number", err)
	}
	t.KeyNumber, t.ProjectKey = number, projectKey
	if err := r.fillDefaultCatalogIDs(ctx, exec, t); err != nil {
		return err
	}
	dbRow := fromDomain(t)

	const query = `
		INSERT INTO issues (
			project_id, summary, description, status, priority, issue_type, parent_id,
			due_date, story_points, rank, closed_at, reporter_id, assignee_id, key_number, created_at, updated_at,
			issue_type_id, priority_id, resolution_id, resolved_at, custom,
			original_estimate_seconds, remaining_estimate_seconds, start_date, flagged
		)
		VALUES (
			:project_id, :summary, :description, :status, :priority,
			:issue_type, :parent_id, :due_date, :story_points, :rank, :closed_at,
			:reporter_id, :assignee_id, :key_number, :created_at, :updated_at,
			:issue_type_id, :priority_id, :resolution_id, :resolved_at, CAST(:custom AS JSONB),
			:original_estimate_seconds, :remaining_estimate_seconds, :start_date, :flagged
		)
		RETURNING id`

	id, err := database.InsertReturningID(ctx, exec, query, dbRow)
	if err != nil {
		if isForeignKeyViolationOn(err, "fk_issues_project") {
			return apperror.New(apperror.KindNotFound, "project not found")
		}
		if isForeignKeyViolation(err) {
			return domain.ErrReporterNotFound
		}
		return apperror.Wrap(apperror.KindInternal, "failed to save issue", err)
	}
	t.ID = id
	return nil
}

// fillDefaultCatalogIDs gives a new issue the organization default type and priority when the caller did not
// choose them. A new issue is never done, so it gets no resolution.
func (r *IssueRepository) fillDefaultCatalogIDs(ctx context.Context, exec sqlx.ExtContext, t *domain.Issue) error {
	if t.IssueTypeID == uuid.Nil {
		if err := exec.QueryRowxContext(ctx, `
			SELECT it.id FROM issue_types it
			JOIN projects p ON p.organization_id = it.organization_id
			WHERE p.id = $1 AND it.is_default`, t.ProjectID).Scan(&t.IssueTypeID); err != nil {
			return apperror.Wrap(apperror.KindInternal, "failed to find default issue type", err)
		}
	}
	if t.PriorityID == uuid.Nil {
		if err := exec.QueryRowxContext(ctx, `
			SELECT pr.id FROM priorities pr
			JOIN projects p ON p.organization_id = pr.organization_id
			WHERE p.id = $1 AND pr.is_default`, t.ProjectID).Scan(&t.PriorityID); err != nil {
			return apperror.Wrap(apperror.KindInternal, "failed to find default priority", err)
		}
	}
	return nil
}

// FindKeyCandidates returns the issues with the given project key and number that the user can reach through
// an organization membership, a direct project membership or a team grant. When allOrgs is true (platform
// administrator) the reachability test is skipped. The caller applies project visibility.
func (r *IssueRepository) FindKeyCandidates(ctx context.Context, projectKey string, number int, orgID, userID uuid.UUID, allOrgs bool) ([]*domain.Issue, error) {
	var org any
	if orgID != uuid.Nil {
		org = orgID
	}
	var rows []issueRow
	if err := sqlx.SelectContext(ctx, database.Executor(ctx, r.db), &rows, `
		SELECT `+issueColumns+`
		FROM issues
		WHERE key_number = $2 AND deleted_at IS NULL
		  AND EXISTS (
		      SELECT 1 FROM projects p
		      WHERE p.id = issues.project_id AND p.key = $1 AND p.deleted_at IS NULL
		        AND ($4::uuid IS NULL OR p.organization_id = $4::uuid)
		        AND (
		            $3::boolean
		            OR EXISTS (SELECT 1 FROM organization_members om
		                       WHERE om.organization_id = p.organization_id AND om.user_id = $5)
		            OR EXISTS (SELECT 1 FROM project_members pm WHERE pm.project_id = p.id AND pm.user_id = $5)
		            OR EXISTS (SELECT 1 FROM project_team_roles ptr
		                       JOIN team_members tm ON tm.team_id = ptr.team_id
		                       WHERE ptr.project_id = p.id AND tm.user_id = $5)
		        )
		  )
		ORDER BY created_at ASC, id ASC`, projectKey, number, allOrgs, org, userID); err != nil {
		return nil, apperror.Wrap(apperror.KindInternal, "failed to find issue by key", err)
	}
	issues := make([]*domain.Issue, 0, len(rows))
	for _, row := range rows {
		issues = append(issues, row.toDomain())
	}
	return issues, nil
}

func (r *IssueRepository) FindByID(ctx context.Context, id uuid.UUID) (*domain.Issue, error) {
	var row issueRow

	const query = `
		SELECT ` + issueColumns + `
		FROM issues WHERE id = $1 AND deleted_at IS NULL AND EXISTS (SELECT 1 FROM projects p WHERE p.id = issues.project_id AND p.deleted_at IS NULL)`

	if err := sqlx.GetContext(ctx, database.Executor(ctx, r.db), &row, query, id); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, domain.ErrIssueNotFound
		}
		return nil, apperror.Wrap(apperror.KindInternal, "failed to find issue", err)
	}

	return row.toDomain(), nil
}

// listWhere arma la clausula WHERE compartida por la consulta de datos y el conteo,
// para que ambas devuelvan exactamente el mismo conjunto de issues.
func listWhere(filter port.ListFilter) (string, []any) {
	query := " WHERE 1 = 1 AND deleted_at IS NULL AND EXISTS (SELECT 1 FROM projects p WHERE p.id = issues.project_id AND p.deleted_at IS NULL)"
	args := []any{}
	if len(filter.ProjectIDs) > 0 {
		placeholders := make([]string, 0, len(filter.ProjectIDs))
		for _, projectID := range filter.ProjectIDs {
			args = append(args, projectID)
			placeholders = append(placeholders, fmt.Sprintf("$%d", len(args)))
		}
		query += " AND project_id IN (" + strings.Join(placeholders, ", ") + ")"
	}

	if filter.Status != "" {
		args = append(args, filter.Status)
		query += fmt.Sprintf(" AND status = $%d", len(args))
	}
	if filter.Priority != "" {
		args = append(args, filter.Priority)
		query += fmt.Sprintf(" AND priority = $%d", len(args))
	}
	if filter.Query != "" {
		args = append(args, "%"+filter.Query+"%")
		query += fmt.Sprintf(" AND (summary ILIKE $%d OR description ILIKE $%d)", len(args), len(args))
	}
	if filter.IssueType != "" {
		args = append(args, filter.IssueType)
		query += fmt.Sprintf(" AND issue_type = $%d", len(args))
	}
	if filter.LabelID != uuid.Nil {
		args = append(args, filter.LabelID)
		query += fmt.Sprintf(" AND EXISTS (SELECT 1 FROM issue_labels il WHERE il.issue_id = issues.id AND il.label_id = $%d)", len(args))
	}
	if filter.AssigneeID != nil {
		args = append(args, *filter.AssigneeID)
		query += fmt.Sprintf(" AND assignee_id = $%d", len(args))
	}
	if filter.Unassigned {
		query += " AND assignee_id IS NULL"
	}
	if filter.DueFrom != nil {
		args = append(args, *filter.DueFrom)
		query += fmt.Sprintf(" AND due_date >= $%d", len(args))
	}
	if filter.DueTo != nil {
		args = append(args, *filter.DueTo)
		query += fmt.Sprintf(" AND due_date <= $%d", len(args))
	}
	return query, args
}

// Count devuelve cuantos issues cumplen el filtro, sin paginar.
func (r *IssueRepository) Count(ctx context.Context, filter port.ListFilter) (int, error) {
	where, args := listWhere(filter)
	var total int
	if err := r.db.GetContext(ctx, &total, "SELECT COUNT(*) FROM issues"+where, args...); err != nil {
		return 0, apperror.Wrap(apperror.KindInternal, "failed to count issues", err)
	}
	return total, nil
}

func (r *IssueRepository) FindAll(ctx context.Context, filter port.ListFilter) ([]*domain.Issue, error) {
	where, args := listWhere(filter)
	query := `
		SELECT ` + issueColumns + `
		FROM issues` + where

	query += " ORDER BY rank ASC, created_at DESC"

	args = append(args, filter.PageSize)
	query += fmt.Sprintf(" LIMIT $%d", len(args))

	args = append(args, (filter.Page-1)*filter.PageSize)
	query += fmt.Sprintf(" OFFSET $%d", len(args))

	var rows []issueRow
	if err := r.db.SelectContext(ctx, &rows, query, args...); err != nil {
		return nil, apperror.Wrap(apperror.KindInternal, "failed to list issues", err)
	}

	issues := make([]*domain.Issue, 0, len(rows))
	for _, row := range rows {
		issues = append(issues, row.toDomain())
	}
	return issues, nil
}

func (r *IssueRepository) Update(ctx context.Context, t *domain.Issue) error {
	row := fromDomain(t)

	const query = `
		UPDATE issues
		SET summary = :summary, description = :description, status = :status, priority = :priority,
		    issue_type = :issue_type, parent_id = :parent_id, due_date = :due_date,
		    story_points = :story_points, rank = :rank, closed_at = :closed_at,
		    assignee_id = :assignee_id, updated_at = :updated_at,
		    issue_type_id = :issue_type_id, priority_id = :priority_id,
		    resolution_id = :resolution_id, resolved_at = :resolved_at,
		    custom = CAST(:custom AS JSONB),
		    original_estimate_seconds = :original_estimate_seconds, remaining_estimate_seconds = :remaining_estimate_seconds,
		    start_date = :start_date, flagged = :flagged
		WHERE id = :id AND deleted_at IS NULL AND EXISTS (SELECT 1 FROM projects p WHERE p.id = issues.project_id AND p.deleted_at IS NULL)`

	result, err := sqlx.NamedExecContext(ctx, database.Executor(ctx, r.db), query, row)
	if err != nil {
		if isForeignKeyViolation(err) {
			return domain.ErrAssigneeNotFound
		}
		return apperror.Wrap(apperror.KindInternal, "failed to update issue", err)
	}

	affected, err := result.RowsAffected()
	if err != nil {
		return apperror.Wrap(apperror.KindInternal, "failed to update issue", err)
	}
	if affected == 0 {
		return domain.ErrIssueNotFound
	}

	return nil
}

// UpdateAndWatchAssignee guarda la asignacion y deja a la persona asignada siguiendo la incidencia. Usa la
// transaccion de ctx si existe; si no, abre una propia para que las dos escrituras no queden a medias.
func (r *IssueRepository) UpdateAndWatchAssignee(ctx context.Context, t *domain.Issue, assigneeID uuid.UUID) error {
	return database.NewTxManager(r.db).InTx(ctx, func(ctx context.Context) error {
		return r.updateAndWatchAssignee(ctx, t, assigneeID)
	})
}

func (r *IssueRepository) updateAndWatchAssignee(ctx context.Context, t *domain.Issue, assigneeID uuid.UUID) error {
	exec := database.Executor(ctx, r.db)
	row := fromDomain(t)
	const query = `
		UPDATE issues
		SET summary = :summary, description = :description, status = :status, priority = :priority,
		    issue_type = :issue_type, parent_id = :parent_id, due_date = :due_date,
		    story_points = :story_points, rank = :rank, closed_at = :closed_at,
		    assignee_id = :assignee_id, updated_at = :updated_at,
		    issue_type_id = :issue_type_id, priority_id = :priority_id,
		    resolution_id = :resolution_id, resolved_at = :resolved_at,
		    custom = CAST(:custom AS JSONB),
		    original_estimate_seconds = :original_estimate_seconds, remaining_estimate_seconds = :remaining_estimate_seconds,
		    start_date = :start_date, flagged = :flagged
		WHERE id = :id AND deleted_at IS NULL AND EXISTS (SELECT 1 FROM projects p WHERE p.id = issues.project_id AND p.deleted_at IS NULL)`
	result, err := sqlx.NamedExecContext(ctx, exec, query, row)
	if err != nil {
		if isForeignKeyViolation(err) {
			return domain.ErrAssigneeNotFound
		}
		return apperror.Wrap(apperror.KindInternal, "failed to assign issue", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return apperror.Wrap(apperror.KindInternal, "failed to assign issue", err)
	}
	if affected == 0 {
		return domain.ErrIssueNotFound
	}
	if _, err := exec.ExecContext(ctx, `
		INSERT INTO watchers(issue_id, user_id) VALUES($1, $2)
		ON CONFLICT(issue_id, user_id) DO NOTHING`, t.ID, assigneeID); err != nil {
		if isForeignKeyViolation(err) {
			return domain.ErrAssigneeNotFound
		}
		return apperror.Wrap(apperror.KindInternal, "failed to follow issue after assignment", err)
	}
	return nil
}

func isForeignKeyViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == pgForeignKeyViolation
}

func isForeignKeyViolationOn(err error, constraint string) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == pgForeignKeyViolation && pgErr.ConstraintName == constraint
}
