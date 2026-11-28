// Package usecase implements CSV import (upload, mapping, asynchronous batches) and the exports.
package usecase

import (
	"bytes"
	"context"
	"io"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/jhonsferg/corelog/internal/importexport/application/port"
	"github.com/jhonsferg/corelog/internal/importexport/domain"
	"github.com/jhonsferg/corelog/internal/platform/apperror"
	"github.com/jhonsferg/corelog/internal/platform/clock"
	"github.com/jhonsferg/corelog/internal/platform/events"
	"github.com/jhonsferg/corelog/internal/platform/filestorage"
)

// Permissions used by the module (names of the permission catalog).
const (
	ActionImport = "edit_project"
	ActionExport = "export_issues"
	ActionBackup = "edit_project"
	// batchSize is how many rows are processed between progress writes.
	batchSize = 200
	// maxExportIssues caps one CSV export.
	maxExportIssues = 10000
)

// Deps are the collaborators of the service.
type Deps struct {
	Repo      port.Repository
	Files     port.Files
	Importer  port.RowImporter
	Auth      port.Authorizer
	Searcher  port.Searcher
	Reader    port.Reader
	Publisher events.Publisher // optional
	Clock     clock.Clock
}

// Service implements port.Service and the background runner.
type Service struct{ Deps }

var _ port.Service = (*Service)(nil)

// NewService returns the service.
func NewService(d Deps) *Service { return &Service{Deps: d} }

// Upload parses the file, stores it and returns the preview. The job waits in "validating" for its mapping.
func (s *Service) Upload(ctx context.Context, actorID uuid.UUID, role string, projectID uuid.UUID, fileName string, data []byte) (port.Preview, error) {
	if err := s.Auth.Authorize(ctx, ActionImport, actorID, role, projectID); err != nil {
		return port.Preview{}, err
	}
	table, err := domain.ParseCSV(data)
	if err != nil {
		return port.Preview{}, err
	}
	key, err := filestorage.NewKey()
	if err != nil {
		return port.Preview{}, apperror.Wrap(apperror.KindInternal, "no se pudo guardar el archivo", err)
	}
	if err := s.Files.Save(ctx, key, bytes.NewReader(data)); err != nil {
		return port.Preview{}, apperror.Wrap(apperror.KindInternal, "no se pudo guardar el archivo", err)
	}
	suggested := domain.SuggestMapping(table.Header)
	job := port.Job{ProjectID: projectID, UserID: actorID, Source: "csv", Status: domain.StatusValidating, TotalRows: len(table.Rows),
		FileName: cleanName(fileName), Mapping: suggested, FileRef: key, CreatedAt: s.Clock.Now()}
	if err := s.Repo.Create(ctx, &job); err != nil {
		_ = s.Files.Delete(ctx, job.FileRef)
		return port.Preview{}, err
	}
	sample := table.Rows
	if len(sample) > domain.PreviewRows {
		sample = sample[:domain.PreviewRows]
	}
	rows := make([][]string, len(sample))
	for i, r := range sample {
		rows[i] = make([]string, len(r))
		for j, c := range r {
			rows[i][j] = domain.Sanitize(c)
		}
	}
	return port.Preview{JobID: job.ID, Columns: table.Header, SampleRows: rows, SuggestedMapping: suggested,
		TotalRows: len(table.Rows), Delimiter: string(table.Delimiter)}, nil
}

func cleanName(n string) string {
	n = domain.Sanitize(n)
	if i := strings.LastIndexAny(n, `/\`); i >= 0 {
		n = n[i+1:]
	}
	if len(n) > 200 {
		n = n[:200]
	}
	return n
}

// Start fixes the column mapping and queues the job. The mapping is validated against the stored file header.
func (s *Service) Start(ctx context.Context, actorID uuid.UUID, role string, projectID, jobID uuid.UUID, mapping map[string]string, dryRun bool) (port.Job, error) {
	if err := s.Auth.Authorize(ctx, ActionImport, actorID, role, projectID); err != nil {
		return port.Job{}, err
	}
	job, err := s.Repo.Get(ctx, projectID, jobID)
	if err != nil {
		return port.Job{}, err
	}
	if job.Status != domain.StatusValidating {
		return port.Job{}, domain.ErrAlreadyStart
	}
	table, err := s.load(ctx, job)
	if err != nil {
		return port.Job{}, err
	}
	if err := domain.ValidateMapping(mapping, table.Header); err != nil {
		return port.Job{}, err
	}
	if err := s.Repo.Queue(ctx, projectID, jobID, mapping, dryRun); err != nil {
		return port.Job{}, err
	}
	return s.Repo.Get(ctx, projectID, jobID)
}

func (s *Service) load(ctx context.Context, job port.Job) (domain.Table, error) {
	f, err := s.Files.Open(ctx, job.FileRef)
	if err != nil {
		return domain.Table{}, apperror.Wrap(apperror.KindInternal, "no se pudo leer el archivo importado", err)
	}
	defer func() { _ = f.Close() }()
	data, err := io.ReadAll(io.LimitReader(f, domain.MaxFileBytes+1))
	if err != nil {
		return domain.Table{}, apperror.Wrap(apperror.KindInternal, "no se pudo leer el archivo importado", err)
	}
	return domain.ParseCSV(data)
}

// Jobs lists the import jobs of a project.
func (s *Service) Jobs(ctx context.Context, actorID uuid.UUID, role string, projectID uuid.UUID) ([]port.Job, error) {
	if err := s.Auth.Authorize(ctx, ActionImport, actorID, role, projectID); err != nil {
		return nil, err
	}
	return s.Repo.List(ctx, projectID, 50)
}

// GetJob returns one job with its progress.
func (s *Service) GetJob(ctx context.Context, actorID uuid.UUID, role string, projectID, jobID uuid.UUID) (port.Job, error) {
	if err := s.Auth.Authorize(ctx, ActionImport, actorID, role, projectID); err != nil {
		return port.Job{}, err
	}
	return s.Repo.Get(ctx, projectID, jobID)
}

// ErrorReport writes the row error report as CSV.
func (s *Service) ErrorReport(ctx context.Context, actorID uuid.UUID, role string, projectID, jobID uuid.UUID, w io.Writer) error {
	job, err := s.GetJob(ctx, actorID, role, projectID, jobID)
	if err != nil {
		return err
	}
	if len(job.Errors) == 0 {
		return domain.ErrNoReport
	}
	rows := make([][]string, len(job.Errors))
	for i, e := range job.Errors {
		rows[i] = []string{strconv.Itoa(e.Row), strings.Join(e.Errors, "; ")}
	}
	return domain.WriteCSV(w, []string{"fila", "errores"}, rows)
}

// RunQueued processes every queued job. It is called by the background loop and by tests.
func (s *Service) RunQueued(ctx context.Context) error {
	jobs, err := s.Repo.Claim(ctx, 5)
	if err != nil {
		return err
	}
	for _, job := range jobs {
		s.process(ctx, job)
	}
	return nil
}

// Run calls RunQueued every interval until ctx is done.
func (s *Service) Run(ctx context.Context, interval time.Duration) {
	t := time.NewTicker(interval)
	defer t.Stop()
	for {
		if err := s.RunQueued(ctx); err != nil && ctx.Err() == nil {
			slog.Error("import run failed", "error", err)
		}
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		}
	}
}

// process runs one claimed job in batches, validating every row and creating issues through the importer.
func (s *Service) process(ctx context.Context, job port.Job) {
	fail := func(msg string, p port.Progress) {
		p.Report = append(p.Report, domain.RowError{Row: 0, Errors: []string{msg}})
		_ = s.Repo.Finish(ctx, job.ID, domain.StatusFailed, p, s.Clock.Now())
	}
	table, err := s.load(ctx, job)
	if err != nil {
		fail("no se pudo leer el archivo", port.Progress{})
		return
	}
	var p port.Progress
	for i, rec := range table.Rows {
		if ctx.Err() != nil {
			return
		}
		number := i + 1
		row, problems := domain.ParseRow(number, table.Header, rec, job.Mapping)
		if len(problems) == 0 {
			key, err := s.Importer.Import(ctx, port.ImportRow{ProjectID: job.ProjectID, ActorID: job.UserID, Role: roleOf(ctx), Row: row, DryRun: job.DryRun})
			if key != "" || (err == nil && job.DryRun) {
				p.Created++
			}
			if err != nil {
				problems = []string{publicMessage(err)}
			}
		}
		p.Processed++
		if len(problems) > 0 {
			p.Errors++
			if len(p.Report) < domain.MaxErrorReports {
				p.Report = append(p.Report, domain.RowError{Row: number, Errors: problems})
			}
		}
		if p.Processed%batchSize == 0 {
			_ = s.Repo.SaveProgress(ctx, job.ID, p)
		}
	}
	if err := s.Repo.Finish(ctx, job.ID, domain.StatusDone, p, s.Clock.Now()); err != nil {
		slog.Error("import job could not be finished", "job_id", job.ID.String(), "error", err)
		return
	}
	s.publishDone(ctx, job, p)
}

// roleOf is the platform role imports run with: the importing user is a regular user, so permissions are the
// ones of that user in the project.
func roleOf(context.Context) string { return "user" }

func publicMessage(err error) string {
	if apperror.KindOf(err) != apperror.KindInternal {
		return err.Error()
	}
	return "error interno al crear la incidencia"
}

func (s *Service) publishDone(ctx context.Context, job port.Job, p port.Progress) {
	if s.Publisher == nil || job.DryRun {
		return
	}
	actor, project, id := job.UserID, job.ProjectID, job.ID
	_ = s.Publisher.Publish(ctx, events.Event{ProjectID: &project, ActorID: &actor, EntityID: &id, Type: "import.completed",
		EntityType: "import_job", CreatedAt: s.Clock.Now(),
		Payload: map[string]any{"created": p.Created, "errors": p.Errors, "total": job.TotalRows}})
}

// exportHeader is the column list of the issue CSV.
var exportHeader = []string{"Clave", "Resumen", "Descripcion", "Tipo", "Prioridad", "Estado", "Responsable", "Informador",
	"Etiquetas", "Creada", "Actualizada", "Vencimiento", "Puntos"}

// ExportIssues writes the issues the filter matches, visible to the actor, as CSV with formula protection.
func (s *Service) ExportIssues(ctx context.Context, actorID uuid.UUID, role string, projectID uuid.UUID, query string, w io.Writer) (string, error) {
	if err := s.Auth.Authorize(ctx, ActionExport, actorID, role, projectID); err != nil {
		return "", err
	}
	ids, err := s.Searcher.Find(ctx, actorID, role, projectID, strings.TrimSpace(query), maxExportIssues)
	if err != nil {
		return "", err
	}
	rows, err := s.Reader.Rows(ctx, projectID, ids)
	if err != nil {
		return "", err
	}
	out := make([][]string, len(rows))
	for i, r := range rows {
		out[i] = []string{r.Key, r.Summary, r.Description, r.Type, r.Priority, r.Status, r.AssigneeEmail, r.ReporterEmail,
			strings.Join(r.Labels, ","), r.Created.UTC().Format(time.RFC3339), r.Updated.UTC().Format(time.RFC3339),
			dateOrEmpty(r.DueDate), pointsOrEmpty(r.StoryPoints)}
	}
	key, err := s.Reader.ProjectKey(ctx, projectID)
	if err != nil {
		return "", err
	}
	return key, domain.WriteCSV(w, exportHeader, out)
}

func dateOrEmpty(t *time.Time) string {
	if t == nil {
		return ""
	}
	return t.UTC().Format("2006-01-02")
}

func pointsOrEmpty(p *float64) string {
	if p == nil {
		return ""
	}
	return strconv.FormatFloat(*p, 'f', -1, 64)
}

// ExportProject returns the full backup of a project. Only users who can edit the project (leads and
// organization administrators) may take it.
func (s *Service) ExportProject(ctx context.Context, actorID uuid.UUID, role string, projectID uuid.UUID) (port.ProjectDump, string, error) {
	if err := s.Auth.Authorize(ctx, ActionBackup, actorID, role, projectID); err != nil {
		return port.ProjectDump{}, "", err
	}
	dump, err := s.Reader.Dump(ctx, projectID, s.Clock.Now())
	if err != nil {
		return port.ProjectDump{}, "", err
	}
	key, err := s.Reader.ProjectKey(ctx, projectID)
	return dump, key, err
}
