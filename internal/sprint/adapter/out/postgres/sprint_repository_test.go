package postgres

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/google/uuid"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/jmoiron/sqlx"

	"github.com/jhonsferg/corelog/internal/sprint/domain"
)

// TestSprintRepositoryPostgres recorre los metodos del repositorio contra PostgreSQL en un esquema temporal
// con las tablas minimas: projects, sprints (migracion 000017) e issues con sprint_id.
func TestSprintRepositoryPostgres(t *testing.T) {
	dsn := os.Getenv("CORELOG_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("set CORELOG_TEST_DATABASE_URL to run PostgreSQL integration tests")
	}
	db, err := sqlx.Connect("pgx", dsn)
	if err != nil {
		t.Fatal(err)
	}
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = db.Close() })

	schema := "sprint_repo_test_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	if _, err := db.Exec(`CREATE SCHEMA ` + schema); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = db.Exec(`DROP SCHEMA ` + schema + ` CASCADE`) })
	if _, err := db.Exec(`SET search_path TO ` + schema); err != nil {
		t.Fatal(err)
	}
	for _, statement := range []string{
		`CREATE TABLE projects (id UUID PRIMARY KEY, deleted_at TIMESTAMPTZ)`,
		`CREATE TABLE sprints (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			project_id UUID NOT NULL REFERENCES projects (id) ON DELETE CASCADE,
			name VARCHAR(80) NOT NULL,
			goal VARCHAR(500) NOT NULL DEFAULT '',
			status VARCHAR(10) NOT NULL DEFAULT 'planned' CHECK (status IN ('planned', 'active', 'closed')),
			start_date DATE, end_date DATE, closed_at TIMESTAMPTZ,
			created_at TIMESTAMPTZ NOT NULL DEFAULT now(), updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
			CHECK (start_date IS NULL OR end_date IS NULL OR end_date >= start_date))`,
		`CREATE UNIQUE INDEX idx_sprints_one_active_per_project ON sprints (project_id) WHERE status = 'active'`,
		`CREATE TABLE issues (
			id UUID PRIMARY KEY, project_id UUID NOT NULL REFERENCES projects (id),
			summary TEXT NOT NULL, status TEXT NOT NULL, priority TEXT NOT NULL DEFAULT 'medium',
			rank VARCHAR(64) NOT NULL DEFAULT 'm', story_points DOUBLE PRECISION, closed_at TIMESTAMPTZ,
			sprint_id UUID REFERENCES sprints (id) ON DELETE SET NULL,
			updated_at TIMESTAMPTZ NOT NULL DEFAULT now(), deleted_at TIMESTAMPTZ)`,
	} {
		if _, err := db.Exec(statement); err != nil {
			t.Fatal(err)
		}
	}

	project, other := uuid.New(), uuid.New()
	for _, id := range []uuid.UUID{project, other} {
		if _, err := db.Exec(`INSERT INTO projects (id) VALUES ($1)`, id); err != nil {
			t.Fatal(err)
		}
	}
	repo := NewSprintRepository(db)
	ctx := context.Background()

	first, err := domain.NewSprint(project, "Sprint 1", "Meta", nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := repo.Save(ctx, first); err != nil || first.ID == uuid.Nil {
		t.Fatalf("Save = %v, id %s", err, first.ID)
	}
	second, _ := domain.NewSprint(project, "Sprint 2", "", nil, nil)
	if err := repo.Save(ctx, second); err != nil {
		t.Fatal(err)
	}

	if _, err := repo.FindByID(ctx, uuid.New()); !errors.Is(err, domain.ErrSprintNotFound) {
		t.Fatalf("FindByID unknown = %v, want ErrSprintNotFound", err)
	}

	if err := first.Start(); err != nil {
		t.Fatal(err)
	}
	if err := repo.Update(ctx, first); err != nil {
		t.Fatalf("Update: %v", err)
	}
	active, err := repo.HasActive(ctx, project, second.ID)
	if err != nil || !active {
		t.Fatalf("HasActive(other sprint) = %v, %v; want true", active, err)
	}
	active, _ = repo.HasActive(ctx, project, first.ID)
	if active {
		t.Fatal("HasActive must ignore the sprint itself")
	}
	list, err := repo.ListByProject(ctx, project)
	if err != nil || len(list) != 2 {
		t.Fatalf("ListByProject = %d, %v; want 2", len(list), err)
	}

	issue, closedIssue, foreign := uuid.New(), uuid.New(), uuid.New()
	for _, row := range []struct {
		id      uuid.UUID
		project uuid.UUID
		status  string
		points  float64
	}{{issue, project, "open", 3}, {closedIssue, project, "closed", 5}, {foreign, other, "open", 1}} {
		if _, err := db.Exec(`INSERT INTO issues (id, project_id, summary, status, story_points) VALUES ($1, $2, 'x', $3, $4)`,
			row.id, row.project, row.status, row.points); err != nil {
			t.Fatal(err)
		}
	}
	if err := repo.SetIssueSprint(ctx, project, issue, &first.ID); err != nil {
		t.Fatalf("assign issue: %v", err)
	}
	if err := repo.SetIssueSprint(ctx, project, closedIssue, &first.ID); err != nil {
		t.Fatalf("assign closed issue: %v", err)
	}
	if err := repo.SetIssueSprint(ctx, project, foreign, &first.ID); !errors.Is(err, domain.ErrIssueNotInProject) {
		t.Fatalf("issue of another project = %v, want ErrIssueNotInProject", err)
	}
	refs, err := repo.ListIssues(ctx, first.ID)
	if err != nil || len(refs) != 2 {
		t.Fatalf("ListIssues = %d, %v; want 2", len(refs), err)
	}

	if err := first.Close(); err != nil {
		t.Fatal(err)
	}
	if err := repo.MoveUnfinished(ctx, first.ID, &second.ID); err != nil {
		t.Fatalf("MoveUnfinished: %v", err)
	}
	if err := repo.Update(ctx, first); err != nil {
		t.Fatal(err)
	}
	moved, _ := repo.ListIssues(ctx, second.ID)
	if len(moved) != 1 || moved[0].ID != issue {
		t.Fatalf("unfinished issue should move to the target sprint, got %d", len(moved))
	}

	velocity, err := repo.Velocity(ctx, project, 5)
	if err != nil || len(velocity) != 1 || velocity[0].Points != 5 {
		t.Fatalf("Velocity = %+v, %v; want one sprint with 5 completed points", velocity, err)
	}

	if err := repo.RemoveIssueFromSprint(ctx, project, second.ID, issue); err != nil {
		t.Fatalf("RemoveIssueFromSprint: %v", err)
	}
	if err := repo.RemoveIssueFromSprint(ctx, project, second.ID, issue); !errors.Is(err, domain.ErrIssueNotInProject) {
		t.Fatalf("removing twice = %v, want ErrIssueNotInProject", err)
	}

	// Issues of a soft-deleted project leave sprint reads and writes: the sprint looks empty and its
	// issues cannot be moved or removed.
	gone, goneSprint, goneIssue := uuid.New(), uuid.New(), uuid.New()
	if _, err := db.Exec(`INSERT INTO projects (id, deleted_at) VALUES ($1, now())`, gone); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO sprints (id, project_id, name, status) VALUES ($1, $2, 'Borrado', 'active')`, goneSprint, gone); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO issues (id, project_id, summary, status, sprint_id) VALUES ($1, $2, 'x', 'open', $3)`, goneIssue, gone, goneSprint); err != nil {
		t.Fatal(err)
	}
	if refs, err := repo.ListIssues(ctx, goneSprint); err != nil || len(refs) != 0 {
		t.Fatalf("ListIssues of deleted project = %d, %v; want 0", len(refs), err)
	}
	if err := repo.RemoveIssueFromSprint(ctx, gone, goneSprint, goneIssue); !errors.Is(err, domain.ErrIssueNotInProject) {
		t.Fatalf("remove issue of deleted project = %v, want ErrIssueNotInProject", err)
	}
	if err := repo.MoveUnfinished(ctx, goneSprint, nil); err != nil {
		t.Fatalf("MoveUnfinished on deleted project: %v", err)
	}
	var stillIn uuid.UUID
	if err := db.Get(&stillIn, `SELECT sprint_id FROM issues WHERE id = $1`, goneIssue); err != nil || stillIn != goneSprint {
		t.Fatalf("issue of deleted project moved: sprint = %v, err %v", stillIn, err)
	}

	// Soft-deleted issues behave as not found: they leave sprint reads, and cannot be assigned, moved or removed.
	removed, removedSprint := uuid.New(), second.ID
	if _, err := db.Exec(`INSERT INTO issues (id, project_id, summary, status, sprint_id, deleted_at) VALUES ($1, $2, 'x', 'open', $3, now())`,
		removed, project, removedSprint); err != nil {
		t.Fatal(err)
	}
	if refs, err := repo.ListIssues(ctx, removedSprint); err != nil || len(refs) != 0 {
		t.Fatalf("ListIssues with a soft-deleted issue = %d, %v; want 0", len(refs), err)
	}
	if err := repo.SetIssueSprint(ctx, project, removed, &first.ID); !errors.Is(err, domain.ErrIssueNotInProject) {
		t.Fatalf("assign soft-deleted issue = %v, want ErrIssueNotInProject", err)
	}
	if err := repo.RemoveIssueFromSprint(ctx, project, removedSprint, removed); !errors.Is(err, domain.ErrIssueNotInProject) {
		t.Fatalf("remove soft-deleted issue = %v, want ErrIssueNotInProject", err)
	}
	if err := repo.MoveUnfinished(ctx, removedSprint, &first.ID); err != nil {
		t.Fatalf("MoveUnfinished: %v", err)
	}
	var kept uuid.UUID
	if err := db.Get(&kept, `SELECT sprint_id FROM issues WHERE id = $1`, removed); err != nil || kept != removedSprint {
		t.Fatalf("soft-deleted issue moved: sprint = %v, err %v", kept, err)
	}
}
