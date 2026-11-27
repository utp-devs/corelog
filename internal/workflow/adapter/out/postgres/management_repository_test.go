package postgres_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"

	"github.com/jhonsferg/corelog/internal/platform/apperror"
	"github.com/jhonsferg/corelog/internal/platform/database"
	"github.com/jhonsferg/corelog/internal/platform/database/testdb"
	"github.com/jhonsferg/corelog/internal/workflow/adapter/out/postgres"
	"github.com/jhonsferg/corelog/internal/workflow/application/port"
	"github.com/jhonsferg/corelog/internal/workflow/domain"
)

// statusOf returns the organization status of the project workflow with the name.
func statusOf(t *testing.T, repo *postgres.Repository, project uuid.UUID, name string) port.OrgStatus {
	t.Helper()
	record, err := repo.FindWorkflow(context.Background(), project)
	if err != nil {
		t.Fatal(err)
	}
	list, err := repo.ListStatuses(context.Background(), uuid.MustParse(defaultOrg))
	if err != nil {
		t.Fatal(err)
	}
	for _, s := range record.Workflow.Statuses {
		if s.Name == name {
			for _, org := range list {
				if org.ID.String() == s.ID {
					return org
				}
			}
		}
	}
	t.Fatalf("status %q not in workflow", name)
	return port.OrgStatus{}
}

func insertIssue(t *testing.T, db *sqlx.DB, project, status uuid.UUID) uuid.UUID {
	t.Helper()
	id := uuid.New()
	reporter := insertActor(t, db)
	if _, err := db.Exec(`INSERT INTO issues (id, project_id, key_number, issue_type, issue_type_id, summary, description, status, priority, priority_id, rank, created_at, status_id, reporter_id)
		VALUES ($1, $2, (SELECT COALESCE(max(key_number), 0) + 1 FROM issues WHERE project_id = $2), 'task',
			(SELECT id FROM issue_types LIMIT 1), 'Repository card', '', 'open', 'low',
			(SELECT id FROM priorities LIMIT 1), 'n', now(), $3, $4)`, id, project, status, reporter); err != nil {
		t.Fatal(err)
	}
	return id
}

func insertActor(t *testing.T, db *sqlx.DB) uuid.UUID {
	t.Helper()
	id := uuid.New()
	if _, err := db.Exec(`INSERT INTO users (id, name, display_name, email, password_hash, role) VALUES ($1, 'Actor', 'Actor', $2, 'x', 'user')`,
		id, id.String()+"@example.test"); err != nil {
		t.Fatal(err)
	}
	return id
}

func issueRow(t *testing.T, db *sqlx.DB, id uuid.UUID) (status string, statusID uuid.UUID, closed bool) {
	t.Helper()
	var row struct {
		Status   string     `db:"status"`
		StatusID uuid.UUID  `db:"status_id"`
		ClosedAt *time.Time `db:"closed_at"`
	}
	if err := sqlx.Get(db, &row, `SELECT status, status_id, closed_at FROM issues WHERE id = $1`, id); err != nil {
		t.Fatal(err)
	}
	return row.Status, row.StatusID, row.ClosedAt != nil
}

func TestMigrateIssuesWritesHistoryActivityAndLegacyMirror(t *testing.T) {
	db := testdb.New(t)
	repo := postgres.NewRepository(db)
	ctx := context.Background()
	project := insertProject(t, db, "scrum")
	actor := insertActor(t, db)
	review := statusOf(t, repo, project, "En revisión")
	done := statusOf(t, repo, project, "Hecho")

	first := insertIssue(t, db, project, review.ID)
	second := insertIssue(t, db, project, review.ID)
	at := time.Date(2026, 11, 27, 12, 0, 0, 0, time.UTC)
	moved, err := repo.MigrateIssues(ctx, port.MigrationScope{OrganizationID: uuid.MustParse(defaultOrg), ProjectID: project}, review, done, actor, at)
	if err != nil {
		t.Fatal(err)
	}
	if moved != 2 {
		t.Fatalf("moved = %d, want 2", moved)
	}

	for _, id := range []uuid.UUID{first, second} {
		status, statusID, closed := issueRow(t, db, id)
		if statusID != done.ID || status != "closed" || !closed {
			t.Fatalf("issue %s = status %q (%s), closed %v; want the done status with legacy closed", id, status, statusID, closed)
		}
	}
	var history int
	if err := sqlx.Get(db, &history, `SELECT count(*) FROM issue_status_history WHERE to_status_id = $1 AND from_status_id = $2 AND changed_by = $3 AND transition_id IS NULL`,
		done.ID, review.ID, actor); err != nil {
		t.Fatal(err)
	}
	if history != 2 {
		t.Fatalf("history rows = %d, want 2", history)
	}
	var activities []struct {
		Action   string `db:"action"`
		NewValue string `db:"new_value"`
	}
	if err := sqlx.Select(db, &activities, `SELECT action, new_value FROM activities WHERE actor_id = $1`, actor); err != nil {
		t.Fatal(err)
	}
	if len(activities) != 2 || activities[0].Action != "status_changed" || activities[0].NewValue != "Hecho" {
		t.Fatalf("activities = %+v, want two status_changed rows", activities)
	}
}

func TestMigrateIssuesStaysInsideTheScope(t *testing.T) {
	db := testdb.New(t)
	repo := postgres.NewRepository(db)
	ctx := context.Background()
	project := insertProject(t, db, "scrum")
	other := insertProject(t, db, "scrum")
	actor := insertActor(t, db)
	review := statusOf(t, repo, project, "En revisión")
	done := statusOf(t, repo, project, "Hecho")
	otherReview := statusOf(t, repo, other, "En revisión")
	_ = otherReview
	insertIssue(t, db, other, review.ID)

	moved, err := repo.MigrateIssues(ctx, port.MigrationScope{OrganizationID: uuid.MustParse(defaultOrg), ProjectID: project}, review, done, actor, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if moved != 0 {
		t.Fatalf("moved %d issues of another project", moved)
	}
}

func TestRefreshLegacyStatusKeepsTheStatusWhenCategoryChanges(t *testing.T) {
	db := testdb.New(t)
	repo := postgres.NewRepository(db)
	ctx := context.Background()
	project := insertProject(t, db, "scrum")
	progress := statusOf(t, repo, project, "En curso")
	issue := insertIssue(t, db, project, progress.ID)
	if status, _, _ := issueRow(t, db, issue); status != "in_progress" {
		t.Fatalf("legacy status before = %q", status)
	}

	err := database.NewTxManager(db).InTx(ctx, func(ctx context.Context) error {
		progress.Category = domain.CategoryDone
		if err := repo.UpdateStatus(ctx, progress); err != nil {
			return err
		}
		return repo.RefreshLegacyStatus(ctx, progress.ID, domain.CategoryDone, time.Now())
	})
	if err != nil {
		t.Fatal(err)
	}
	status, statusID, closed := issueRow(t, db, issue)
	if status != "closed" || statusID != progress.ID || !closed {
		t.Fatalf("after category change: legacy %q, status %s, closed %v; the status id must stay", status, statusID, closed)
	}
	// A later writer that sets the legacy value on its own still goes through the normal mapping.
	if _, err := db.Exec(`UPDATE issues SET status = 'open' WHERE id = $1`, issue); err != nil {
		t.Fatal(err)
	}
	if _, statusID, _ = issueRow(t, db, issue); statusID == progress.ID {
		t.Fatal("legacy writes must keep the mapping to a status of their category")
	}
}

func TestStatusUsageWorkflowsUsingAndProjectContext(t *testing.T) {
	db := testdb.New(t)
	repo := postgres.NewRepository(db)
	ctx := context.Background()
	project := insertProject(t, db, "kanban")
	org := uuid.MustParse(defaultOrg)
	progress := statusOf(t, repo, project, "En curso")

	issues, history, err := repo.StatusUsage(ctx, org, progress.ID)
	if err != nil || issues != 0 || history != 0 {
		t.Fatalf("usage of an unused status = %d, %d, %v", issues, history, err)
	}
	insertIssue(t, db, project, progress.ID)
	if issues, _, _ = repo.StatusUsage(ctx, org, progress.ID); issues != 1 {
		t.Fatalf("issues = %d, want 1", issues)
	}
	// Usage is scoped to the organization of the status.
	if issues, _, _ = repo.StatusUsage(ctx, uuid.New(), progress.ID); issues != 0 {
		t.Fatalf("usage from another organization = %d", issues)
	}

	workflows, err := repo.WorkflowsUsing(ctx, org, progress.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(workflows) != 1 || workflows[0].ProjectID != project {
		t.Fatalf("workflows using the status = %+v", workflows)
	}
	if none, _ := repo.WorkflowsUsing(ctx, uuid.New(), progress.ID); len(none) != 0 {
		t.Fatal("workflows of another organization are visible")
	}

	ref, err := repo.ProjectContext(ctx, project)
	if err != nil || ref.Template != "kanban" || ref.OrganizationID != org {
		t.Fatalf("project context = %+v, %v", ref, err)
	}
	if _, err := repo.ProjectContext(ctx, uuid.New()); apperror.CodeOf(err) != "project.not_found" {
		t.Fatalf("unknown project: %v", err)
	}
	if _, err := repo.LockWorkflow(ctx, uuid.New()); apperror.CodeOf(err) != "workflow.not_found" {
		t.Fatalf("lock of an unknown workflow: %v", err)
	}
}

func TestRewriteStatusHistoryAndPositions(t *testing.T) {
	db := testdb.New(t)
	repo := postgres.NewRepository(db)
	ctx := context.Background()
	project := insertProject(t, db, "scrum")
	actor := insertActor(t, db)
	org := uuid.MustParse(defaultOrg)
	review := statusOf(t, repo, project, "En revisión")
	done := statusOf(t, repo, project, "Hecho")
	insertIssue(t, db, project, review.ID)
	if _, err := repo.MigrateIssues(ctx, port.MigrationScope{OrganizationID: org, ProjectID: project}, review, done, actor, time.Now()); err != nil {
		t.Fatal(err)
	}
	if err := repo.RewriteStatusHistory(ctx, review.ID, done.ID); err != nil {
		t.Fatal(err)
	}
	var left int
	if err := sqlx.Get(db, &left, `SELECT count(*) FROM issue_status_history WHERE from_status_id = $1 OR to_status_id = $1`, review.ID); err != nil {
		t.Fatal(err)
	}
	if left != 0 {
		t.Fatalf("history rows still reference the rewritten status: %d", left)
	}

	list, err := repo.ListStatuses(ctx, org)
	if err != nil || len(list) < 2 {
		t.Fatalf("list = %d, %v", len(list), err)
	}
	reversed := []uuid.UUID{list[1].ID, list[0].ID}
	if err := repo.SetStatusPositions(ctx, org, reversed); err != nil {
		t.Fatal(err)
	}
	after, _ := repo.ListStatuses(ctx, org)
	if after[0].ID != list[1].ID {
		t.Fatalf("positions not stored: first is %s", after[0].ID)
	}
}
