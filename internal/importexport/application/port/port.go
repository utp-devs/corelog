// Package port declares the import/export service and the narrow ports it needs.
package port

import (
	"context"
	"io"
	"time"

	"github.com/google/uuid"

	"github.com/jhonsferg/corelog/internal/importexport/domain"
)

// Job is an import job.
type Job struct {
	ID            uuid.UUID
	ProjectID     uuid.UUID
	UserID        uuid.UUID
	Source        string
	Status        string
	TotalRows     int
	ProcessedRows int
	CreatedRows   int
	ErrorRows     int
	DryRun        bool
	FileName      string
	Mapping       map[string]string
	FileRef       string
	Errors        []domain.RowError
	CreatedAt     time.Time
	FinishedAt    *time.Time
}

// Progress is the counters written while a job runs.
type Progress struct {
	Processed int
	Created   int
	Errors    int
	Report    []domain.RowError
}

// Repository stores import jobs.
type Repository interface {
	Create(ctx context.Context, j *Job) error
	Get(ctx context.Context, projectID, id uuid.UUID) (Job, error)
	List(ctx context.Context, projectID uuid.UUID, limit int) ([]Job, error)
	// Queue stores the final mapping and moves a validating job to queued. It returns domain.ErrAlreadyStart for
	// any other status.
	Queue(ctx context.Context, projectID, id uuid.UUID, mapping map[string]string, dryRun bool) error
	// Claim moves up to limit queued jobs to running and returns them.
	Claim(ctx context.Context, limit int) ([]Job, error)
	SaveProgress(ctx context.Context, id uuid.UUID, p Progress) error
	Finish(ctx context.Context, id uuid.UUID, status string, p Progress, at time.Time) error
}

// Files stores uploaded files.
type Files interface {
	Save(ctx context.Context, key string, r io.Reader) error
	Open(ctx context.Context, key string) (io.ReadCloser, error)
	Delete(ctx context.Context, key string) error
}

// ImportRow is one row handed to the issue creation adapter.
type ImportRow struct {
	ProjectID uuid.UUID
	ActorID   uuid.UUID
	Role      string
	Row       domain.Row
	// DryRun validates references (users, catalogs, parent) without creating the issue.
	DryRun bool
}

// RowImporter creates one issue through the issue use cases. It returns the issue key ("" in dry runs). Errors
// with a public message are reported in the row error report.
type RowImporter interface {
	Import(ctx context.Context, in ImportRow) (string, error)
}

// Authorizer is the project permission check.
type Authorizer interface {
	Authorize(ctx context.Context, action string, actorID uuid.UUID, globalRole string, projectID uuid.UUID) error
}

// Searcher resolves a CoreQL text to the ids of the issues the actor can see in a project.
type Searcher interface {
	Find(ctx context.Context, actorID uuid.UUID, role string, projectID uuid.UUID, query string, limit int) ([]uuid.UUID, error)
}

// ExportRow is the flat data of one exported issue.
type ExportRow struct {
	Key           string
	Summary       string
	Description   string
	Type          string
	Priority      string
	Status        string
	AssigneeEmail string
	ReporterEmail string
	Labels        []string
	Created       time.Time
	Updated       time.Time
	DueDate       *time.Time
	StoryPoints   *float64
}

// ProjectDump is the full-fidelity backup of a project.
type ProjectDump struct {
	Version    string           `json:"version"`
	ExportedAt time.Time        `json:"exported_at"`
	Project    map[string]any   `json:"project"`
	Issues     []map[string]any `json:"issues"`
	Truncated  bool             `json:"truncated"`
}

// Reader reads data for exports.
type Reader interface {
	// Rows returns the export rows of the issues, keeping the order of ids.
	Rows(ctx context.Context, projectID uuid.UUID, ids []uuid.UUID) ([]ExportRow, error)
	// ProjectKey returns the project key for file names.
	ProjectKey(ctx context.Context, projectID uuid.UUID) (string, error)
	// Dump reads the project backup with the size caps applied.
	Dump(ctx context.Context, projectID uuid.UUID, now time.Time) (ProjectDump, error)
}

// Preview is the answer of the upload step.
type Preview struct {
	JobID            uuid.UUID
	Columns          []string
	SampleRows       [][]string
	SuggestedMapping map[string]string
	TotalRows        int
	Delimiter        string
}

// Service is the in-port.
type Service interface {
	Upload(ctx context.Context, actorID uuid.UUID, role string, projectID uuid.UUID, fileName string, data []byte) (Preview, error)
	Start(ctx context.Context, actorID uuid.UUID, role string, projectID, jobID uuid.UUID, mapping map[string]string, dryRun bool) (Job, error)
	Jobs(ctx context.Context, actorID uuid.UUID, role string, projectID uuid.UUID) ([]Job, error)
	GetJob(ctx context.Context, actorID uuid.UUID, role string, projectID, jobID uuid.UUID) (Job, error)
	// ErrorReport writes the row errors of a job as CSV.
	ErrorReport(ctx context.Context, actorID uuid.UUID, role string, projectID, jobID uuid.UUID, w io.Writer) error
	// ExportIssues writes the CSV of the issues matching the CoreQL filter (all issues when empty) and returns
	// the project key for the file name.
	ExportIssues(ctx context.Context, actorID uuid.UUID, role string, projectID uuid.UUID, query string, w io.Writer) (string, error)
	ExportProject(ctx context.Context, actorID uuid.UUID, role string, projectID uuid.UUID) (ProjectDump, string, error)
}
