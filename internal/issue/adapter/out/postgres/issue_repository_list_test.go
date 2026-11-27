package postgres

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/google/uuid"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/jhonsferg/corelog/internal/issue/application/port"
	"github.com/jhonsferg/corelog/internal/issue/domain"
	"github.com/jhonsferg/corelog/internal/platform/apperror"
	"github.com/jmoiron/sqlx"
)

// newListTestRepository crea un esquema aislado con la tabla issues y devuelve el repositorio sobre el.
func newListTestRepository(t *testing.T) *IssueRepository {
	t.Helper()
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

	schema := "list_test_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	if _, err := db.Exec(`CREATE SCHEMA ` + schema); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = db.Exec(`DROP SCHEMA ` + schema + ` CASCADE`) })
	if _, err := db.Exec(`SET search_path TO ` + schema); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`CREATE TABLE projects (id UUID PRIMARY KEY, key VARCHAR(10) NOT NULL DEFAULT 'TST', next_issue_number INT NOT NULL DEFAULT 1, deleted_at TIMESTAMPTZ)`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`CREATE TABLE issues (
		id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
		project_id UUID NOT NULL,
		key_number INT NOT NULL DEFAULT 0,
		summary VARCHAR(200) NOT NULL,
		description TEXT NOT NULL,
		status VARCHAR(20) NOT NULL,
		priority VARCHAR(20) NOT NULL,
		issue_type VARCHAR(20) NOT NULL DEFAULT 'task',
		parent_id UUID,
		due_date TIMESTAMPTZ,
		story_points NUMERIC(4,1),
		rank VARCHAR(64) NOT NULL DEFAULT 'm',
		closed_at TIMESTAMPTZ,
		reporter_id UUID NOT NULL,
		assignee_id UUID,
		created_at TIMESTAMPTZ NOT NULL,
		updated_at TIMESTAMPTZ NOT NULL
	)`); err != nil {
		t.Fatal(err)
	}
	addCatalogFixtures(t, db)
	return NewIssueRepository(db)
}

// insertProjectRow adds a live project so that issue writes pass the project check.
func insertProjectRow(t *testing.T, db *sqlx.DB, id uuid.UUID) {
	t.Helper()
	if _, err := db.Exec(`INSERT INTO projects (id) VALUES ($1)`, id); err != nil {
		t.Fatal(err)
	}
}

// TestIssueRepositoryCountMatchesFindAllPostgres verifica que Count y FindAll usan la misma
// clausula WHERE: el total cuenta todas las coincidencias y la pagina devuelve solo su parte.
func TestIssueRepositoryCountMatchesFindAllPostgres(t *testing.T) {
	repo := newListTestRepository(t)
	projectID := uuid.New()
	insertProjectRow(t, repo.db, projectID)
	reporterID := uuid.New()
	for i := 0; i < 3; i++ {
		issue, err := domain.NewIssue("Issue", "Description", domain.PriorityLow, reporterID)
		if err != nil {
			t.Fatal(err)
		}
		issue.ProjectID = projectID
		if err := repo.Save(context.Background(), issue); err != nil {
			t.Fatal(err)
		}
	}

	ctx := context.Background()
	page := port.ListFilter{ProjectIDs: []uuid.UUID{projectID}, Page: 1, PageSize: 2}
	items, err := repo.FindAll(ctx, page)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 2 {
		t.Errorf("FindAll page 1 returned %d items, want 2", len(items))
	}
	total, err := repo.Count(ctx, page)
	if err != nil {
		t.Fatal(err)
	}
	if total != 3 {
		t.Errorf("Count = %d, want 3 (all matches, not only the page)", total)
	}

	none := port.ListFilter{ProjectIDs: []uuid.UUID{projectID}, Status: domain.StatusResolved, Page: 1, PageSize: 20}
	zero, err := repo.Count(ctx, none)
	if err != nil {
		t.Fatal(err)
	}
	if zero != 0 {
		t.Errorf("Count with no matches = %d, want 0", zero)
	}
}

// TestIssueRepositoryAssigneeFilterPostgres verifica el filtro de responsable: Unassigned devuelve solo
// las incidencias sin persona asignada y AssigneeID solo las de esa persona.
func TestIssueRepositoryAssigneeFilterPostgres(t *testing.T) {
	repo := newListTestRepository(t)
	ctx := context.Background()
	projectID := uuid.New()
	insertProjectRow(t, repo.db, projectID)
	reporterID := uuid.New()
	assigneeID := uuid.New()

	unassigned, err := domain.NewIssue("Unassigned", "Description", domain.PriorityLow, reporterID)
	if err != nil {
		t.Fatal(err)
	}
	unassigned.ProjectID = projectID
	assigned, err := domain.NewIssue("Assigned", "Description", domain.PriorityLow, reporterID)
	if err != nil {
		t.Fatal(err)
	}
	assigned.ProjectID = projectID
	assigned.AssigneeID = &assigneeID
	for _, issue := range []*domain.Issue{unassigned, assigned} {
		if err := repo.Save(ctx, issue); err != nil {
			t.Fatal(err)
		}
	}

	noneFilter := port.ListFilter{ProjectIDs: []uuid.UUID{projectID}, Unassigned: true, Page: 1, PageSize: 20}
	items, err := repo.FindAll(ctx, noneFilter)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || items[0].ID != unassigned.ID {
		t.Errorf("FindAll unassigned returned %d items, want only the unassigned issue", len(items))
	}
	total, err := repo.Count(ctx, noneFilter)
	if err != nil {
		t.Fatal(err)
	}
	if total != 1 {
		t.Errorf("Count unassigned = %d, want 1", total)
	}

	byAssignee := port.ListFilter{ProjectIDs: []uuid.UUID{projectID}, AssigneeID: &assigneeID, Page: 1, PageSize: 20}
	items, err = repo.FindAll(ctx, byAssignee)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || items[0].ID != assigned.ID {
		t.Errorf("FindAll by assignee returned %d items, want only the assigned issue", len(items))
	}
}

// TestIssueRepositoryHidesIssuesOfDeletedProjectPostgres checks that the issues of a soft-deleted project
// are not readable, listed, counted or writable, while the issues of a live project are unaffected.
func TestIssueRepositoryHidesIssuesOfDeletedProjectPostgres(t *testing.T) {
	repo := newListTestRepository(t)
	ctx := context.Background()
	reporter := uuid.New()
	live, gone := uuid.New(), uuid.New()
	insertProjectRow(t, repo.db, live)
	insertProjectRow(t, repo.db, gone)

	liveIssue, err := domain.NewIssue("Live", "Description", domain.PriorityLow, reporter)
	if err != nil {
		t.Fatal(err)
	}
	liveIssue.ProjectID = live
	goneIssue, err := domain.NewIssue("Gone", "Description", domain.PriorityLow, reporter)
	if err != nil {
		t.Fatal(err)
	}
	goneIssue.ProjectID = gone
	for _, issue := range []*domain.Issue{liveIssue, goneIssue} {
		if err := repo.Save(ctx, issue); err != nil {
			t.Fatal(err)
		}
	}

	if _, err := repo.db.Exec(`UPDATE projects SET deleted_at = now() WHERE id = $1`, gone); err != nil {
		t.Fatal(err)
	}

	if _, err := repo.FindByID(ctx, goneIssue.ID); !errors.Is(err, domain.ErrIssueNotFound) {
		t.Fatalf("FindByID of deleted project = %v, want ErrIssueNotFound", err)
	}
	if _, err := repo.FindByID(ctx, liveIssue.ID); err != nil {
		t.Fatalf("FindByID of live project: %v", err)
	}

	all := port.ListFilter{Page: 1, PageSize: 50}
	items, err := repo.FindAll(ctx, all)
	if err != nil || len(items) != 1 || items[0].ID != liveIssue.ID {
		t.Fatalf("FindAll = %d items, %v; want only the live issue", len(items), err)
	}
	if total, err := repo.Count(ctx, all); err != nil || total != 1 {
		t.Fatalf("Count = %d, %v; want 1", total, err)
	}
	ofGone := port.ListFilter{ProjectIDs: []uuid.UUID{gone}, Page: 1, PageSize: 50}
	if total, err := repo.Count(ctx, ofGone); err != nil || total != 0 {
		t.Fatalf("Count filtered by deleted project = %d, %v; want 0", total, err)
	}
	if backlog, err := repo.ListBacklog(ctx, gone); err != nil || len(backlog) != 0 {
		t.Fatalf("ListBacklog of deleted project = %d, %v; want 0", len(backlog), err)
	}

	goneIssue.Summary = "Changed"
	if err := repo.Update(ctx, goneIssue); !errors.Is(err, domain.ErrIssueNotFound) {
		t.Fatalf("Update of deleted project = %v, want ErrIssueNotFound", err)
	}

	created, err := domain.NewIssue("New", "Description", domain.PriorityLow, reporter)
	if err != nil {
		t.Fatal(err)
	}
	created.ProjectID = gone
	if err := repo.Save(ctx, created); apperror.KindOf(err) != apperror.KindNotFound {
		t.Fatalf("Save into deleted project = %v, want a not found error", err)
	}
}
