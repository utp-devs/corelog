package postgres

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/jhonsferg/corelog/internal/importexport/application/port"
	"github.com/jhonsferg/corelog/internal/importexport/domain"
	"github.com/jhonsferg/corelog/internal/platform/database/testdb"
)

const orgID = "00000000-0000-4000-8000-000000000002"

func TestRepositoryJobs(t *testing.T) {
	db := testdb.New(t)
	ctx := context.Background()
	user, project := uuid.New(), uuid.New()
	if _, err := db.Exec(`INSERT INTO users (id, name, display_name, email, password_hash, role) VALUES ($1, 'u', 'u', 'u@example.test', 'x', 'user')`, user); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO projects (id, organization_id, key, name, type, template, lead_id, visibility) VALUES ($1, $2, 'IMP', 'Import', 'kanban', 'kanban', $3, 'open')`, project, orgID, user); err != nil {
		t.Fatal(err)
	}
	repo := NewRepository(db)
	now := time.Now().UTC().Truncate(time.Microsecond)
	job := port.Job{ProjectID: project, UserID: user, Source: "csv", Status: domain.StatusValidating, TotalRows: 3, FileName: "a.csv",
		Mapping: map[string]string{"summary": "Summary"}, FileRef: "abc", CreatedAt: now}
	if err := repo.Create(ctx, &job); err != nil || job.ID == uuid.Nil {
		t.Fatalf("create %v", err)
	}
	got, err := repo.Get(ctx, project, job.ID)
	if err != nil || got.Status != domain.StatusValidating || got.Mapping["summary"] != "Summary" || got.TotalRows != 3 || len(got.Errors) != 0 {
		t.Fatalf("get %+v %v", got, err)
	}
	if _, err := repo.Get(ctx, uuid.New(), job.ID); !errors.Is(err, domain.ErrJobNotFound) {
		t.Fatalf("other project: %v", err)
	}
	if claimed, _ := repo.Claim(ctx, 5); len(claimed) != 0 {
		t.Fatal("a job that was not started must not be claimed")
	}
	if err := repo.Queue(ctx, project, job.ID, map[string]string{"summary": "Resumen"}, true); err != nil {
		t.Fatal(err)
	}
	if err := repo.Queue(ctx, project, job.ID, nil, false); !errors.Is(err, domain.ErrAlreadyStart) {
		t.Fatalf("second queue: %v", err)
	}
	claimed, err := repo.Claim(ctx, 5)
	if err != nil || len(claimed) != 1 || claimed[0].Status != domain.StatusRunning || !claimed[0].DryRun || claimed[0].Mapping["summary"] != "Resumen" {
		t.Fatalf("claim %+v %v", claimed, err)
	}
	if again, _ := repo.Claim(ctx, 5); len(again) != 0 {
		t.Fatal("job claimed twice")
	}
	progress := port.Progress{Processed: 2, Created: 1, Errors: 1, Report: []domain.RowError{{Row: 2, Errors: []string{"mal"}}}}
	if err := repo.SaveProgress(ctx, job.ID, progress); err != nil {
		t.Fatal(err)
	}
	if mid, _ := repo.Get(ctx, project, job.ID); mid.ProcessedRows != 2 || mid.Status != domain.StatusRunning || len(mid.Errors) != 1 {
		t.Fatalf("progress %+v", mid)
	}
	if err := repo.Finish(ctx, job.ID, domain.StatusDone, port.Progress{Processed: 3, Created: 2, Errors: 1}, now.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	done, _ := repo.Get(ctx, project, job.ID)
	if done.Status != domain.StatusDone || done.CreatedRows != 2 || done.FinishedAt == nil || len(done.Errors) != 0 {
		t.Fatalf("done %+v", done)
	}
	if list, err := repo.List(ctx, project, 10); err != nil || len(list) != 1 {
		t.Fatalf("list %v %v", list, err)
	}
	if list, _ := repo.List(ctx, uuid.New(), 10); len(list) != 0 {
		t.Fatal("other project list")
	}
}

func TestReader(t *testing.T) {
	db := testdb.New(t)
	ctx := context.Background()
	user, project, issue, other := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	if _, err := db.Exec(`INSERT INTO users (id, name, display_name, email, password_hash, role) VALUES ($1, 'u', 'u', 'u@example.test', 'x', 'user')`, user); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO projects (id, organization_id, key, name, type, template, lead_id, visibility) VALUES ($1, $2, 'RDR', 'Reader', 'kanban', 'kanban', $3, 'open')`, project, orgID, user); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`SELECT corelog_ensure_workflow($1)`, project); err != nil {
		t.Fatal(err)
	}
	var typeID, prioID, statusID uuid.UUID
	_ = db.Get(&typeID, `SELECT id FROM issue_types WHERE organization_id = $1 AND legacy_key = 'task'`, orgID)
	_ = db.Get(&prioID, `SELECT id FROM priorities WHERE organization_id = $1 AND legacy_key = 'high'`, orgID)
	_ = db.Get(&statusID, `SELECT id FROM statuses WHERE organization_id = $1 ORDER BY position LIMIT 1`, orgID)
	for i, id := range []uuid.UUID{issue, other} {
		if _, err := db.Exec(`INSERT INTO issues (id, summary, description, status, priority, reporter_id, project_id, issue_type, rank, key_number, issue_type_id, priority_id, status_id)
			VALUES ($1, $2, 'd', 'open', 'high', $3, $4, 'task', 'm', $5, $6, $7, $8)`, id, []string{"=Formula", "Segunda"}[i], user, project, i+1, typeID, prioID, statusID); err != nil {
			t.Fatal(err)
		}
	}
	var labelID uuid.UUID
	if err := db.Get(&labelID, `INSERT INTO labels (project_id, name, color) VALUES ($1, 'bug', '#fff') RETURNING id`, project); err != nil {
		t.Fatal(err)
	}
	_, _ = db.Exec(`INSERT INTO issue_labels (issue_id, label_id) VALUES ($1, $2)`, issue, labelID)
	_, _ = db.Exec(`INSERT INTO comments (issue_id, author_id, body) VALUES ($1, $2, 'hola')`, issue, user)
	_, _ = db.Exec(`INSERT INTO worklogs (issue_id, author_id, started_at, time_spent_seconds, description, remaining_mode, remaining_adjust_seconds) VALUES ($1, $2, now(), 60, 'w', 'auto', 0)`, issue, user)
	_, _ = db.Exec(`INSERT INTO issue_links (source_issue_id, target_issue_id, kind, created_by) VALUES ($1, $2, 'relates', $3)`, issue, other, user)

	r := NewReader(db)
	if key, err := r.ProjectKey(ctx, project); err != nil || key != "RDR" {
		t.Fatalf("key %q %v", key, err)
	}
	if _, err := r.ProjectKey(ctx, uuid.New()); err == nil {
		t.Fatal("unknown project")
	}
	rows, err := r.Rows(ctx, project, []uuid.UUID{other, issue, uuid.New()})
	if err != nil || len(rows) != 2 || rows[0].Key != "RDR-2" || rows[1].Key != "RDR-1" || rows[1].Summary != "=Formula" || len(rows[1].Labels) != 1 || rows[1].ReporterEmail != "u@example.test" {
		t.Fatalf("rows %+v %v", rows, err)
	}
	if rows, _ := r.Rows(ctx, project, nil); len(rows) != 0 {
		t.Fatal("no ids, no rows")
	}
	if rows, _ := r.Rows(ctx, uuid.New(), []uuid.UUID{issue}); len(rows) != 0 {
		t.Fatal("rows of another project")
	}
	dump, err := r.Dump(ctx, project, time.Now())
	if err != nil || dump.Version != "v1" || dump.Project["key"] != "RDR" || len(dump.Issues) != 2 || dump.Truncated {
		t.Fatalf("dump %+v %v", dump, err)
	}
	first := dump.Issues[0]
	if len(first["comments"].([]map[string]any)) != 1 || len(first["worklogs"].([]map[string]any)) != 1 || len(first["links"].([]map[string]any)) != 1 {
		t.Fatalf("related rows %+v", first)
	}
	if _, err := r.Dump(ctx, uuid.New(), time.Now()); err == nil {
		t.Fatal("unknown project dump")
	}
}
