// Package postgres stores import jobs and reads the data of the exports.
package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"

	"github.com/jhonsferg/corelog/internal/importexport/application/port"
	"github.com/jhonsferg/corelog/internal/importexport/domain"
	"github.com/jhonsferg/corelog/internal/platform/apperror"
	"github.com/jhonsferg/corelog/internal/platform/database"
)

// Repository is the PostgreSQL adapter of port.Repository.
type Repository struct{ db *sqlx.DB }

var _ port.Repository = (*Repository)(nil)

// NewRepository returns a repository bound to db.
func NewRepository(db *sqlx.DB) *Repository { return &Repository{db: db} }

func (r *Repository) ext(ctx context.Context) sqlx.ExtContext { return database.Executor(ctx, r.db) }

func internal(msg string, err error) error { return apperror.Wrap(apperror.KindInternal, msg, err) }

type jobRow struct {
	ID         uuid.UUID  `db:"id"`
	ProjectID  uuid.UUID  `db:"project_id"`
	UserID     uuid.UUID  `db:"user_id"`
	Source     string     `db:"source"`
	Status     string     `db:"status"`
	Total      int        `db:"total_rows"`
	Processed  int        `db:"processed_rows"`
	Created    int        `db:"created_rows"`
	Errors     int        `db:"error_rows"`
	DryRun     bool       `db:"dry_run"`
	FileName   string     `db:"file_name"`
	Mapping    []byte     `db:"mapping"`
	FileRef    string     `db:"file_ref"`
	Report     []byte     `db:"error_report"`
	CreatedAt  time.Time  `db:"created_at"`
	FinishedAt *time.Time `db:"finished_at"`
}

const jobColumns = `id, project_id, user_id, source, status, total_rows, processed_rows, created_rows, error_rows, dry_run,
	file_name, mapping, file_ref, error_report, created_at, finished_at`

func (j jobRow) job() (port.Job, error) {
	out := port.Job{ID: j.ID, ProjectID: j.ProjectID, UserID: j.UserID, Source: j.Source, Status: j.Status, TotalRows: j.Total,
		ProcessedRows: j.Processed, CreatedRows: j.Created, ErrorRows: j.Errors, DryRun: j.DryRun, FileName: j.FileName,
		FileRef: j.FileRef, CreatedAt: j.CreatedAt, FinishedAt: j.FinishedAt}
	if err := json.Unmarshal(j.Mapping, &out.Mapping); err != nil {
		return out, internal("importacion corrupta: mapeo", err)
	}
	if err := json.Unmarshal(j.Report, &out.Errors); err != nil {
		return out, internal("importacion corrupta: informe", err)
	}
	return out, nil
}

// Create inserts a job and fills its id.
func (r *Repository) Create(ctx context.Context, j *port.Job) error {
	mapping, _ := json.Marshal(j.Mapping)
	err := sqlx.GetContext(ctx, r.ext(ctx), &j.ID, `
		INSERT INTO import_jobs (project_id, user_id, source, status, total_rows, file_name, mapping, file_ref, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7::jsonb, $8, $9) RETURNING id`,
		j.ProjectID, j.UserID, j.Source, j.Status, j.TotalRows, j.FileName, string(mapping), j.FileRef, j.CreatedAt)
	if err != nil {
		return internal("no se pudo crear la importacion", err)
	}
	return nil
}

// Get returns a job of the project.
func (r *Repository) Get(ctx context.Context, projectID, id uuid.UUID) (port.Job, error) {
	var row jobRow
	err := sqlx.GetContext(ctx, r.ext(ctx), &row, `SELECT `+jobColumns+` FROM import_jobs WHERE id = $1 AND project_id = $2`, id, projectID)
	if errors.Is(err, sql.ErrNoRows) {
		return port.Job{}, domain.ErrJobNotFound
	}
	if err != nil {
		return port.Job{}, internal("no se pudo leer la importacion", err)
	}
	return row.job()
}

// List returns the latest jobs of the project.
func (r *Repository) List(ctx context.Context, projectID uuid.UUID, limit int) ([]port.Job, error) {
	var rows []jobRow
	if err := sqlx.SelectContext(ctx, r.ext(ctx), &rows,
		`SELECT `+jobColumns+` FROM import_jobs WHERE project_id = $1 ORDER BY created_at DESC, id LIMIT $2`, projectID, limit); err != nil {
		return nil, internal("no se pudieron listar las importaciones", err)
	}
	out := make([]port.Job, 0, len(rows))
	for _, row := range rows {
		j, err := row.job()
		if err != nil {
			return nil, err
		}
		out = append(out, j)
	}
	return out, nil
}

// Queue stores the mapping and queues a validating job.
func (r *Repository) Queue(ctx context.Context, projectID, id uuid.UUID, mapping map[string]string, dryRun bool) error {
	raw, _ := json.Marshal(mapping)
	res, err := r.ext(ctx).ExecContext(ctx, `
		UPDATE import_jobs SET status = 'queued', started = TRUE, mapping = $3::jsonb, dry_run = $4
		WHERE id = $1 AND project_id = $2 AND status = 'validating'`, id, projectID, string(raw), dryRun)
	if err != nil {
		return internal("no se pudo encolar la importacion", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return domain.ErrAlreadyStart
	}
	return nil
}

// Claim moves queued jobs to running with row locks that skip jobs another worker holds.
func (r *Repository) Claim(ctx context.Context, limit int) ([]port.Job, error) {
	var rows []jobRow
	err := sqlx.SelectContext(ctx, r.ext(ctx), &rows, `
		UPDATE import_jobs SET status = 'running'
		WHERE id IN (SELECT id FROM import_jobs WHERE status = 'queued' ORDER BY created_at LIMIT $1 FOR UPDATE SKIP LOCKED)
		RETURNING `+jobColumns, limit)
	if err != nil {
		return nil, internal("no se pudieron reservar las importaciones", err)
	}
	out := make([]port.Job, 0, len(rows))
	for _, row := range rows {
		j, err := row.job()
		if err != nil {
			return nil, err
		}
		out = append(out, j)
	}
	return out, nil
}

// SaveProgress stores the counters of a running job.
func (r *Repository) SaveProgress(ctx context.Context, id uuid.UUID, p port.Progress) error {
	report, _ := json.Marshal(nonNil(p.Report))
	_, err := r.ext(ctx).ExecContext(ctx, `
		UPDATE import_jobs SET processed_rows = $2, created_rows = $3, error_rows = $4, error_report = $5::jsonb WHERE id = $1`,
		id, p.Processed, p.Created, p.Errors, string(report))
	if err != nil {
		return internal("no se pudo guardar el progreso", err)
	}
	return nil
}

// Finish closes a job with its final counters.
func (r *Repository) Finish(ctx context.Context, id uuid.UUID, status string, p port.Progress, at time.Time) error {
	report, _ := json.Marshal(nonNil(p.Report))
	_, err := r.ext(ctx).ExecContext(ctx, `
		UPDATE import_jobs SET status = $2, processed_rows = $3, created_rows = $4, error_rows = $5, error_report = $6::jsonb,
		       finished_at = $7 WHERE id = $1`, id, status, p.Processed, p.Created, p.Errors, string(report), at)
	if err != nil {
		return internal("no se pudo cerrar la importacion", err)
	}
	return nil
}

func nonNil(r []domain.RowError) []domain.RowError {
	if r == nil {
		return []domain.RowError{}
	}
	return r
}
