package usecase

import (
	"bytes"
	"context"
	"errors"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/jhonsferg/corelog/internal/importexport/application/port"
	"github.com/jhonsferg/corelog/internal/importexport/domain"
	"github.com/jhonsferg/corelog/internal/platform/apperror"
	"github.com/jhonsferg/corelog/internal/platform/clock"
	"github.com/jhonsferg/corelog/internal/platform/events"
)

type fakeRepo struct{ jobs map[uuid.UUID]*port.Job }

func (r *fakeRepo) Create(_ context.Context, j *port.Job) error {
	j.ID = uuid.New()
	cp := *j
	r.jobs[j.ID] = &cp
	return nil
}
func (r *fakeRepo) Get(_ context.Context, p, id uuid.UUID) (port.Job, error) {
	if j, ok := r.jobs[id]; ok && j.ProjectID == p {
		return *j, nil
	}
	return port.Job{}, domain.ErrJobNotFound
}
func (r *fakeRepo) List(_ context.Context, p uuid.UUID, _ int) ([]port.Job, error) {
	var out []port.Job
	for _, j := range r.jobs {
		if j.ProjectID == p {
			out = append(out, *j)
		}
	}
	return out, nil
}
func (r *fakeRepo) Queue(_ context.Context, p, id uuid.UUID, m map[string]string, dry bool) error {
	j, ok := r.jobs[id]
	if !ok || j.ProjectID != p || j.Status != domain.StatusValidating {
		return domain.ErrAlreadyStart
	}
	j.Status, j.Mapping, j.DryRun = domain.StatusQueued, m, dry
	return nil
}
func (r *fakeRepo) Claim(context.Context, int) ([]port.Job, error) {
	var out []port.Job
	for _, j := range r.jobs {
		if j.Status == domain.StatusQueued {
			j.Status = domain.StatusRunning
			out = append(out, *j)
		}
	}
	return out, nil
}
func (r *fakeRepo) SaveProgress(_ context.Context, id uuid.UUID, p port.Progress) error {
	r.jobs[id].ProcessedRows = p.Processed
	return nil
}
func (r *fakeRepo) Finish(_ context.Context, id uuid.UUID, status string, p port.Progress, at time.Time) error {
	j := r.jobs[id]
	j.Status, j.ProcessedRows, j.CreatedRows, j.ErrorRows, j.Errors, j.FinishedAt = status, p.Processed, p.Created, p.Errors, p.Report, &at
	return nil
}

type memFiles map[string][]byte

func (m memFiles) Save(_ context.Context, k string, r io.Reader) error {
	b, _ := io.ReadAll(r)
	m[k] = b
	return nil
}
func (m memFiles) Open(_ context.Context, k string) (io.ReadCloser, error) {
	b, ok := m[k]
	if !ok {
		return nil, errors.New("missing")
	}
	return io.NopCloser(bytes.NewReader(b)), nil
}
func (m memFiles) Delete(_ context.Context, k string) error { delete(m, k); return nil }

type fakeImporter struct {
	rows    []domain.Row
	dry     []bool
	reject  string
	partial string
}

func (f *fakeImporter) Import(_ context.Context, in port.ImportRow) (string, error) {
	f.rows = append(f.rows, in.Row)
	f.dry = append(f.dry, in.DryRun)
	if in.Row.Summary == f.reject {
		return "", apperror.New(apperror.KindValidation, "tipo de incidencia desconocido: X")
	}
	if in.Row.Summary == "boom" {
		return "", errors.New("pq: connection secret")
	}
	if in.Row.Summary == f.partial {
		return "K-9", errors.New("creada K-9, pero no se pudo pasar al estado")
	}
	if in.DryRun {
		return "", nil
	}
	return "K-1", nil
}

type fakeAuth struct{ allow map[string]bool }

func (a fakeAuth) Authorize(_ context.Context, action string, _ uuid.UUID, _ string, _ uuid.UUID) error {
	if a.allow[action] {
		return nil
	}
	return apperror.New(apperror.KindForbidden, "forbidden")
}

type fakeSearch struct {
	query string
	ids   []uuid.UUID
}

func (s *fakeSearch) Find(_ context.Context, _ uuid.UUID, _ string, _ uuid.UUID, q string, _ int) ([]uuid.UUID, error) {
	s.query = q
	return s.ids, nil
}

type fakeReader struct{ rows []port.ExportRow }

func (r fakeReader) Rows(context.Context, uuid.UUID, []uuid.UUID) ([]port.ExportRow, error) {
	return r.rows, nil
}
func (r fakeReader) ProjectKey(context.Context, uuid.UUID) (string, error) { return "CORE", nil }
func (r fakeReader) Dump(_ context.Context, _ uuid.UUID, now time.Time) (port.ProjectDump, error) {
	return port.ProjectDump{Version: "v1", ExportedAt: now}, nil
}

type recorder struct{ events []events.Event }

func (r *recorder) Publish(_ context.Context, e events.Event) error {
	r.events = append(r.events, e)
	return nil
}

type env struct {
	svc      *Service
	repo     *fakeRepo
	files    memFiles
	importer *fakeImporter
	search   *fakeSearch
	pub      *recorder
	project  uuid.UUID
	actor    uuid.UUID
}

func newEnv(rows []port.ExportRow) *env {
	e := &env{repo: &fakeRepo{jobs: map[uuid.UUID]*port.Job{}}, files: memFiles{}, importer: &fakeImporter{}, search: &fakeSearch{},
		pub: &recorder{}, project: uuid.New(), actor: uuid.New()}
	e.svc = NewService(Deps{Repo: e.repo, Files: e.files, Importer: e.importer, Auth: fakeAuth{allow: map[string]bool{ActionImport: true, ActionExport: true}},
		Searcher: e.search, Reader: fakeReader{rows: rows}, Publisher: e.pub, Clock: clock.NewFixed(time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC))})
	return e
}

const sample = "Summary,Description,Labels\nUno,d1,a\nDos,d2,b\nboom,d3,\nrechazada,d4,\n,sin resumen,\n"

func TestUploadPreviewStartAndRun(t *testing.T) {
	e := newEnv(nil)
	ctx := context.Background()
	e.importer.reject = "rechazada"
	pv, err := e.svc.Upload(ctx, e.actor, "user", e.project, `C:\temp\..\jira.csv`, []byte(sample))
	if err != nil {
		t.Fatal(err)
	}
	if pv.TotalRows != 5 || len(pv.SampleRows) != domain.PreviewRows || pv.SuggestedMapping[domain.FieldSummary] != "Summary" || pv.Delimiter != "," || len(e.files) != 1 {
		t.Fatalf("preview %+v", pv)
	}
	job, _ := e.svc.GetJob(ctx, e.actor, "user", e.project, pv.JobID)
	if job.Status != domain.StatusValidating || job.FileName != "jira.csv" {
		t.Fatalf("job %+v", job)
	}
	// Bad mappings are refused and do not start the job.
	if _, err := e.svc.Start(ctx, e.actor, "user", e.project, pv.JobID, map[string]string{domain.FieldSummary: "Nope"}, false); apperror.KindOf(err) != apperror.KindValidation {
		t.Fatalf("bad mapping: %v", err)
	}
	if _, err := e.svc.Start(ctx, e.actor, "user", e.project, uuid.New(), pv.SuggestedMapping, false); !errors.Is(err, domain.ErrJobNotFound) {
		t.Fatalf("unknown job: %v", err)
	}
	started, err := e.svc.Start(ctx, e.actor, "user", e.project, pv.JobID, pv.SuggestedMapping, false)
	if err != nil || started.Status != domain.StatusQueued {
		t.Fatalf("start: %+v %v", started, err)
	}
	if _, err := e.svc.Start(ctx, e.actor, "user", e.project, pv.JobID, pv.SuggestedMapping, false); !errors.Is(err, domain.ErrAlreadyStart) {
		t.Fatalf("second start: %v", err)
	}
	if err := e.svc.RunQueued(ctx); err != nil {
		t.Fatal(err)
	}
	done, _ := e.svc.GetJob(ctx, e.actor, "user", e.project, pv.JobID)
	if done.Status != domain.StatusDone || done.TotalRows != 5 || done.ProcessedRows != 5 || done.CreatedRows != 2 || done.ErrorRows != 3 || done.FinishedAt == nil {
		t.Fatalf("done %+v", done)
	}
	byRow := map[int]string{}
	for _, re := range done.Errors {
		byRow[re.Row] = strings.Join(re.Errors, ";")
	}
	if !strings.Contains(byRow[4], "tipo de incidencia") || byRow[5] != "el resumen es obligatorio" || strings.Contains(byRow[3], "secret") {
		t.Fatalf("report %v (internal errors must not leak)", byRow)
	}
	if len(e.pub.events) != 1 || e.pub.events[0].Type != "import.completed" {
		t.Fatalf("events %v", e.pub.events)
	}
	// Error report as CSV.
	var buf bytes.Buffer
	if err := e.svc.ErrorReport(ctx, e.actor, "user", e.project, pv.JobID, &buf); err != nil || !strings.Contains(buf.String(), "fila,errores") || !strings.Contains(buf.String(), "tipo de incidencia") {
		t.Fatalf("report %q %v", buf.String(), err)
	}
	if jobs, _ := e.svc.Jobs(ctx, e.actor, "user", e.project); len(jobs) != 1 {
		t.Fatalf("jobs %v", jobs)
	}
}

func TestDryRunCreatesNothingAndPartialRowsAreReported(t *testing.T) {
	e := newEnv(nil)
	ctx := context.Background()
	e.importer.partial = "Dos"
	pv, _ := e.svc.Upload(ctx, e.actor, "user", e.project, "a.csv", []byte("Summary\nUno\nDos\n"))
	_, _ = e.svc.Start(ctx, e.actor, "user", e.project, pv.JobID, pv.SuggestedMapping, true)
	_ = e.svc.RunQueued(ctx)
	for _, d := range e.importer.dry {
		if !d {
			t.Fatal("dry run must be passed to the importer")
		}
	}
	job, _ := e.svc.GetJob(ctx, e.actor, "user", e.project, pv.JobID)
	if job.Status != domain.StatusDone || job.ErrorRows != 1 || job.CreatedRows != 2 || !job.DryRun {
		t.Fatalf("dry run job %+v", job)
	}
	if len(e.pub.events) != 0 {
		t.Fatal("dry runs are not audited as imports")
	}
	// Without errors there is no report.
	e2 := newEnv(nil)
	pv, _ = e2.svc.Upload(ctx, e2.actor, "user", e2.project, "a.csv", []byte("Summary\nUno\n"))
	_, _ = e2.svc.Start(ctx, e2.actor, "user", e2.project, pv.JobID, pv.SuggestedMapping, false)
	_ = e2.svc.RunQueued(ctx)
	if err := e2.svc.ErrorReport(ctx, e2.actor, "user", e2.project, pv.JobID, io.Discard); !errors.Is(err, domain.ErrNoReport) {
		t.Fatalf("report without errors: %v", err)
	}
}

func TestMissingFileFailsTheJob(t *testing.T) {
	e := newEnv(nil)
	ctx := context.Background()
	pv, _ := e.svc.Upload(ctx, e.actor, "user", e.project, "a.csv", []byte("Summary\nUno\n"))
	_, _ = e.svc.Start(ctx, e.actor, "user", e.project, pv.JobID, pv.SuggestedMapping, false)
	for k := range e.files {
		delete(e.files, k)
	}
	_ = e.svc.RunQueued(ctx)
	if job, _ := e.svc.GetJob(ctx, e.actor, "user", e.project, pv.JobID); job.Status != domain.StatusFailed {
		t.Fatalf("job %+v", job)
	}
}

func TestLimitsAndPermissions(t *testing.T) {
	e := newEnv(nil)
	ctx := context.Background()
	if _, err := e.svc.Upload(ctx, e.actor, "user", e.project, "a.csv", bytes.Repeat([]byte("x"), domain.MaxFileBytes+1)); err == nil {
		t.Fatal("oversized file accepted")
	}
	if _, err := e.svc.Upload(ctx, e.actor, "user", e.project, "a.csv", []byte("\xff")); err == nil {
		t.Fatal("invalid encoding accepted")
	}
	if len(e.files) != 0 || len(e.repo.jobs) != 0 {
		t.Fatal("rejected uploads must leave nothing behind")
	}
	denied := NewService(Deps{Repo: e.repo, Files: e.files, Importer: e.importer, Auth: fakeAuth{}, Searcher: e.search, Reader: fakeReader{}, Clock: clock.NewFixed(time.Now())})
	if _, err := denied.Upload(ctx, e.actor, "user", e.project, "a.csv", []byte(sample)); apperror.KindOf(err) != apperror.KindForbidden {
		t.Fatalf("upload: %v", err)
	}
	if _, err := denied.Start(ctx, e.actor, "user", e.project, uuid.New(), nil, false); err == nil {
		t.Fatal("start")
	}
	if _, err := denied.Jobs(ctx, e.actor, "user", e.project); err == nil {
		t.Fatal("jobs")
	}
	if _, err := denied.GetJob(ctx, e.actor, "user", e.project, uuid.New()); err == nil {
		t.Fatal("get job")
	}
	if _, err := denied.ExportIssues(ctx, e.actor, "user", e.project, "", io.Discard); err == nil {
		t.Fatal("export")
	}
	if _, _, err := denied.ExportProject(ctx, e.actor, "user", e.project); err == nil {
		t.Fatal("backup")
	}
	// A job of another project is invisible.
	pv, _ := e.svc.Upload(ctx, e.actor, "user", e.project, "a.csv", []byte("Summary\nUno\n"))
	if _, err := e.svc.GetJob(ctx, e.actor, "user", uuid.New(), pv.JobID); !errors.Is(err, domain.ErrJobNotFound) {
		t.Fatalf("cross-project job: %v", err)
	}
}

func TestExportNeutralizesFormulas(t *testing.T) {
	due := time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)
	points := 2.5
	rows := []port.ExportRow{
		{Key: "CORE-1", Summary: "=HYPERLINK(\"http://evil\")", Description: "+cmd|' /C calc'!A0", Type: "Bug", Priority: "High", Status: "Open",
			AssigneeEmail: "@admin", Labels: []string{"a", "-b"}, Created: due, Updated: due, DueDate: &due, StoryPoints: &points},
		{Key: "CORE-2", Summary: "Normal, con coma", Created: due, Updated: due},
	}
	e := newEnv(rows)
	e.search.ids = []uuid.UUID{uuid.New(), uuid.New()}
	var buf bytes.Buffer
	key, err := e.svc.ExportIssues(context.Background(), e.actor, "user", e.project, "  priority = High ", &buf)
	if err != nil || key != "CORE" || e.search.query != "priority = High" {
		t.Fatalf("export: %q %v query %q", key, err, e.search.query)
	}
	out := buf.String()
	for _, want := range []string{"'=HYPERLINK", "'+cmd", "'@admin", "2026-08-01", "2.5", `"Normal, con coma"`, "a,-b"} {
		if !strings.Contains(out, want) {
			t.Errorf("export lacks %q:\n%s", want, out)
		}
	}
	for _, bad := range []string{",=HYPERLINK", "\n=HYPERLINK", ",+cmd", ",@admin", "\"=HYPERLINK"} {
		if strings.Contains(out, bad) {
			t.Errorf("export contains an executable cell %q", bad)
		}
	}
	dump, key, err := e.svc.ExportProject(context.Background(), e.actor, "user", e.project)
	if err != nil || key != "CORE" || dump.Version != "v1" {
		t.Fatalf("dump %+v %v", dump, err)
	}
}
