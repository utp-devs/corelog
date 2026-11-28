// Package postgres implements the release repository with explicit, parameterised SQL.
package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jmoiron/sqlx"

	"github.com/jhonsferg/corelog/internal/platform/apperror"
	"github.com/jhonsferg/corelog/internal/platform/database"
	"github.com/jhonsferg/corelog/internal/release/application/port"
	"github.com/jhonsferg/corelog/internal/release/domain"
)

const pgUniqueViolation = "23505"

// Category of the status an issue is in, read through the database function that maps legacy and workflow statuses.
const issueCategory = `corelog_issue_category(i.status_id, i.status)`

// Repository is the sqlx implementation of port.Repository.
type Repository struct {
	db *sqlx.DB
}

// NewRepository builds the repository. It joins the transaction of the context when there is one.
func NewRepository(db *sqlx.DB) *Repository {
	return &Repository{db: db}
}

var _ port.Repository = (*Repository)(nil)

type versionRow struct {
	ID          uuid.UUID  `db:"id"`
	ProjectID   uuid.UUID  `db:"project_id"`
	Name        string     `db:"name"`
	Description string     `db:"description"`
	StartDate   *time.Time `db:"start_date"`
	ReleaseDate *time.Time `db:"release_date"`
	Status      string     `db:"status"`
	ReleasedAt  *time.Time `db:"released_at"`
	ArchivedAt  *time.Time `db:"archived_at"`
	CreatedAt   time.Time  `db:"created_at"`
	UpdatedAt   time.Time  `db:"updated_at"`
}

const versionColumns = `id, project_id, name, description, start_date, release_date, status, released_at, archived_at, created_at, updated_at`

func (r versionRow) toDomain() *domain.Version {
	return &domain.Version{
		ID: r.ID, ProjectID: r.ProjectID, Name: r.Name, Description: r.Description,
		StartDate: r.StartDate, ReleaseDate: r.ReleaseDate, Status: domain.Status(r.Status),
		ReleasedAt: r.ReleasedAt, ArchivedAt: r.ArchivedAt, CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt,
	}
}

func mapWriteError(err error, message string) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == pgUniqueViolation {
		return domain.ErrVersionNameTaken
	}
	return apperror.Wrap(apperror.KindInternal, message, err)
}

func (r *Repository) Create(ctx context.Context, v *domain.Version) error {
	id, err := database.InsertReturningID(ctx, database.Executor(ctx, r.db), `
		INSERT INTO versions (project_id, name, description, start_date, release_date, status, released_at, archived_at, created_at, updated_at)
		VALUES (:project_id, :name, :description, :start_date, :release_date, :status, :released_at, :archived_at, :created_at, :updated_at)
		RETURNING id`, map[string]any{
		"project_id": v.ProjectID, "name": v.Name, "description": v.Description,
		"start_date": v.StartDate, "release_date": v.ReleaseDate, "status": string(v.Status),
		"released_at": v.ReleasedAt, "archived_at": v.ArchivedAt, "created_at": v.CreatedAt, "updated_at": v.UpdatedAt,
	})
	if err != nil {
		return mapWriteError(err, "failed to create version")
	}
	v.ID = id
	return nil
}

func (r *Repository) Save(ctx context.Context, v *domain.Version) error {
	_, err := database.Executor(ctx, r.db).ExecContext(ctx, `
		UPDATE versions SET name = $2, description = $3, start_date = $4, release_date = $5, status = $6,
		       released_at = $7, archived_at = $8, updated_at = $9
		WHERE id = $1 AND deleted_at IS NULL`,
		v.ID, v.Name, v.Description, v.StartDate, v.ReleaseDate, string(v.Status), v.ReleasedAt, v.ArchivedAt, v.UpdatedAt)
	if err != nil {
		return mapWriteError(err, "failed to save version")
	}
	return nil
}

func (r *Repository) FindByID(ctx context.Context, id uuid.UUID) (*domain.Version, error) {
	return r.find(ctx, id, "")
}

func (r *Repository) LockByID(ctx context.Context, id uuid.UUID) (*domain.Version, error) {
	return r.find(ctx, id, " FOR UPDATE")
}

func (r *Repository) find(ctx context.Context, id uuid.UUID, suffix string) (*domain.Version, error) {
	var row versionRow
	query := `SELECT ` + versionColumns + ` FROM versions WHERE id = $1 AND deleted_at IS NULL` + suffix
	if err := sqlx.GetContext(ctx, database.Executor(ctx, r.db), &row, query, id); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, domain.ErrVersionNotFound
		}
		return nil, apperror.Wrap(apperror.KindInternal, "failed to read version", err)
	}
	return row.toDomain(), nil
}

func (r *Repository) List(ctx context.Context, projectID uuid.UUID, status *domain.Status) ([]*domain.Version, error) {
	var filter *string
	if status != nil {
		value := string(*status)
		filter = &value
	}
	var rows []versionRow
	if err := sqlx.SelectContext(ctx, database.Executor(ctx, r.db), &rows, `
		SELECT `+versionColumns+` FROM versions
		WHERE project_id = $1 AND deleted_at IS NULL AND ($2::text IS NULL OR status = $2)
		ORDER BY COALESCE(release_date, DATE '9999-12-31') ASC, lower(name) ASC, id ASC`,
		projectID, filter); err != nil {
		return nil, apperror.Wrap(apperror.KindInternal, "failed to list versions", err)
	}
	out := make([]*domain.Version, 0, len(rows))
	for _, row := range rows {
		out = append(out, row.toDomain())
	}
	return out, nil
}

type progressRow struct {
	VersionID uuid.UUID `db:"version_id"`
	Category  string    `db:"category"`
	Issues    int       `db:"issues"`
	Points    float64   `db:"points"`
}

func (r *Repository) Progress(ctx context.Context, versionIDs []uuid.UUID) (map[uuid.UUID][]domain.CategoryCount, error) {
	out := make(map[uuid.UUID][]domain.CategoryCount, len(versionIDs))
	if len(versionIDs) == 0 {
		return out, nil
	}
	ids := make([]string, 0, len(versionIDs))
	for _, id := range versionIDs {
		ids = append(ids, id.String())
	}
	var rows []progressRow
	if err := sqlx.SelectContext(ctx, database.Executor(ctx, r.db), &rows, `
		SELECT iv.version_id, `+issueCategory+` AS category, COUNT(*)::int AS issues,
		       COALESCE(SUM(i.story_points), 0)::float8 AS points
		FROM issue_versions iv
		JOIN issues i ON i.id = iv.issue_id AND i.deleted_at IS NULL
		WHERE iv.version_id = ANY($1::uuid[]) AND iv.kind = 'fix'
		GROUP BY iv.version_id, category`, ids); err != nil {
		return nil, apperror.Wrap(apperror.KindInternal, "failed to read version progress", err)
	}
	for _, row := range rows {
		out[row.VersionID] = append(out[row.VersionID], domain.CategoryCount{Category: row.Category, Issues: row.Issues, Points: row.Points})
	}
	return out, nil
}

func (r *Repository) NameTaken(ctx context.Context, projectID uuid.UUID, name string, except uuid.UUID) (bool, error) {
	var taken bool
	if err := sqlx.GetContext(ctx, database.Executor(ctx, r.db), &taken, `
		SELECT EXISTS (SELECT 1 FROM versions WHERE project_id = $1 AND deleted_at IS NULL AND lower(name) = lower($2) AND id <> $3)`,
		projectID, name, except); err != nil {
		return false, apperror.Wrap(apperror.KindInternal, "failed to check version name", err)
	}
	return taken, nil
}

func (r *Repository) Delete(ctx context.Context, id uuid.UUID) error {
	result, err := database.Executor(ctx, r.db).ExecContext(ctx, `UPDATE versions SET deleted_at = now() WHERE id = $1 AND deleted_at IS NULL`, id)
	if err != nil {
		return apperror.Wrap(apperror.KindInternal, "failed to delete version", err)
	}
	if affected, err := result.RowsAffected(); err != nil {
		return apperror.Wrap(apperror.KindInternal, "failed to delete version", err)
	} else if affected == 0 {
		return domain.ErrVersionNotFound
	}
	return nil
}

func (r *Repository) MoveLinks(ctx context.Context, from, to uuid.UUID) error {
	if _, err := database.Executor(ctx, r.db).ExecContext(ctx, `
		INSERT INTO issue_versions (issue_id, version_id, kind)
		SELECT issue_id, $2, kind FROM issue_versions WHERE version_id = $1
		ON CONFLICT DO NOTHING`, from, to); err != nil {
		return apperror.Wrap(apperror.KindInternal, "failed to move version links", err)
	}
	return nil
}

func (r *Repository) UnresolvedFixCount(ctx context.Context, id uuid.UUID) (int, error) {
	var count int
	if err := sqlx.GetContext(ctx, database.Executor(ctx, r.db), &count, `
		SELECT COUNT(*)::int FROM issue_versions iv
		JOIN issues i ON i.id = iv.issue_id AND i.deleted_at IS NULL
		WHERE iv.version_id = $1 AND iv.kind = 'fix' AND `+issueCategory+` <> 'done'`, id); err != nil {
		return 0, apperror.Wrap(apperror.KindInternal, "failed to count unresolved issues", err)
	}
	return count, nil
}

// MoveUnresolvedFixes moves the unfinished fix issues of from to the target, in two statements of the same transaction.
func (r *Repository) MoveUnresolvedFixes(ctx context.Context, from, to uuid.UUID) (int, error) {
	exec := database.Executor(ctx, r.db)
	count, err := r.UnresolvedFixCount(ctx, from)
	if err != nil {
		return 0, err
	}
	if _, err := exec.ExecContext(ctx, `
		INSERT INTO issue_versions (issue_id, version_id, kind)
		SELECT iv.issue_id, $2, 'fix' FROM issue_versions iv
		JOIN issues i ON i.id = iv.issue_id AND i.deleted_at IS NULL
		WHERE iv.version_id = $1 AND iv.kind = 'fix' AND `+issueCategory+` <> 'done'
		ON CONFLICT DO NOTHING`, from, to); err != nil {
		return 0, apperror.Wrap(apperror.KindInternal, "failed to move unresolved issues", err)
	}
	if _, err := exec.ExecContext(ctx, `
		DELETE FROM issue_versions iv USING issues i
		WHERE i.id = iv.issue_id AND iv.version_id = $1 AND iv.kind = 'fix' AND `+issueCategory+` <> 'done'`, from); err != nil {
		return 0, apperror.Wrap(apperror.KindInternal, "failed to remove moved issues", err)
	}
	return count, nil
}

type noteRow struct {
	ID         uuid.UUID `db:"id"`
	ProjectKey string    `db:"project_key"`
	KeyNumber  int       `db:"key_number"`
	Summary    string    `db:"summary"`
	TypeName   string    `db:"type_name"`
	Done       bool      `db:"done"`
}

func (r *Repository) NotesIssues(ctx context.Context, id uuid.UUID) ([]domain.NoteIssue, error) {
	var rows []noteRow
	if err := sqlx.SelectContext(ctx, database.Executor(ctx, r.db), &rows, `
		SELECT i.id, p.key AS project_key, i.key_number, i.summary, COALESCE(it.name, '') AS type_name,
		       (`+issueCategory+` = 'done') AS done
		FROM issue_versions iv
		JOIN issues i ON i.id = iv.issue_id AND i.deleted_at IS NULL
		JOIN projects p ON p.id = i.project_id AND p.deleted_at IS NULL
		LEFT JOIN issue_types it ON it.id = i.issue_type_id
		WHERE iv.version_id = $1 AND iv.kind = 'fix'
		ORDER BY i.key_number ASC, i.id ASC`, id); err != nil {
		return nil, apperror.Wrap(apperror.KindInternal, "failed to read release notes", err)
	}
	out := make([]domain.NoteIssue, 0, len(rows))
	for _, row := range rows {
		key := ""
		if row.KeyNumber > 0 {
			key = fmt.Sprintf("%s-%d", row.ProjectKey, row.KeyNumber)
		}
		out = append(out, domain.NoteIssue{ID: row.ID, Key: key, Summary: row.Summary, TypeName: row.TypeName, Done: row.Done})
	}
	return out, nil
}
