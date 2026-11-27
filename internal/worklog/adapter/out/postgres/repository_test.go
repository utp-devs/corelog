package postgres

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/jhonsferg/corelog/internal/platform/apperror"
	"github.com/jhonsferg/corelog/internal/platform/database/testdb"
	"github.com/jhonsferg/corelog/internal/worklog/application/port"
	"github.com/jhonsferg/corelog/internal/worklog/domain"
)

const testOrg = "00000000-0000-4000-8000-000000000002"

// setup creates a user, a project and an issue in the default organization.
func setup(t *testing.T) (*Repository, uuid.UUID, uuid.UUID, uuid.UUID) {
	t.Helper()
	db := testdb.New(t)
	repo := NewRepository(db)
	author, project, issue := uuid.New(), uuid.New(), uuid.New()
	mustExec := func(q string, args ...any) {
		t.Helper()
		if _, err := db.Exec(q, args...); err != nil {
			t.Fatalf("%v\n%s", err, q)
		}
	}
	mustExec(`INSERT INTO users (id, name, display_name, email, password_hash, role) VALUES ($1, 'wl', 'Worker', $2, 'x', 'user')`, author, author.String()+"@example.test")
	mustExec(`INSERT INTO organization_members (organization_id, user_id, role) VALUES ($1, $2, 'member')`, testOrg, author)
	mustExec(`INSERT INTO projects (id, organization_id, key, name, type, template, lead_id, visibility) VALUES ($1, $2, 'WRK', 'Work', 'scrum', 'scrum', $3, 'open')`, project, testOrg, author)
	mustExec(`INSERT INTO issues (id, project_id, summary, description, status, priority, reporter_id, issue_type, key_number, rank, issue_type_id, priority_id, original_estimate_seconds, remaining_estimate_seconds)
		SELECT $1, p.id, 'Trabajo', '', 'open', 'medium', $2, 'task', 1, 'm',
		       (SELECT id FROM issue_types WHERE organization_id = p.organization_id AND legacy_key = 'task'),
		       (SELECT id FROM priorities WHERE organization_id = p.organization_id AND legacy_key = 'medium'),
		       28800, 28800
		FROM projects p WHERE p.id = $3`, issue, author, project)
	return repo, author, project, issue
}

func TestRepositoryWorklogLifecycle(t *testing.T) {
	repo, author, _, issue := setup(t)
	ctx := context.Background()
	it, err := repo.IssueTime(ctx, issue, true)
	if err != nil {
		t.Fatalf("IssueTime: %v", err)
	}
	if it.IssueKey != "WRK-1" || it.Original == nil || *it.Original != 28800 || it.Spent != 0 {
		t.Fatalf("issue time = %+v", it)
	}

	w := domain.Worklog{IssueID: issue, AuthorID: author, StartedAt: time.Now().UTC().Truncate(time.Second), TimeSpentSeconds: 3600, Description: "primero", RemainingMode: domain.RemainingAuto, RemainingAdjustSeconds: 3600}
	if err := repo.InsertWorklog(ctx, &w); err != nil {
		t.Fatalf("InsertWorklog: %v", err)
	}
	if w.ID == uuid.Nil {
		t.Fatalf("id not assigned")
	}
	if err := repo.RecomputeSpent(ctx, issue); err != nil {
		t.Fatal(err)
	}
	if err := repo.SetRemaining(ctx, issue, ptrInt(25200)); err != nil {
		t.Fatal(err)
	}
	got, err := repo.Worklog(ctx, w.ID, false)
	if err != nil || got.IssueKey != "WRK-1" || got.AuthorName != "Worker" || got.Description != "primero" {
		t.Fatalf("Worklog = %+v %v", got, err)
	}
	list, err := repo.ListIssueWorklogs(ctx, issue)
	if err != nil || len(list) != 1 {
		t.Fatalf("list = %d %v", len(list), err)
	}
	it, _ = repo.IssueTime(ctx, issue, false)
	if it.Spent != 3600 || *it.Remaining != 25200 {
		t.Fatalf("aggregate after insert = %+v", it)
	}

	w.TimeSpentSeconds = 7200
	w.Description = "editado"
	if err := repo.UpdateWorklog(ctx, &w); err != nil {
		t.Fatalf("UpdateWorklog: %v", err)
	}
	if err := repo.RecomputeSpent(ctx, issue); err != nil {
		t.Fatal(err)
	}
	it, _ = repo.IssueTime(ctx, issue, false)
	if it.Spent != 7200 {
		t.Fatalf("aggregate after update = %d", it.Spent)
	}

	if err := repo.SoftDeleteWorklog(ctx, w.ID, time.Now().UTC()); err != nil {
		t.Fatalf("SoftDeleteWorklog: %v", err)
	}
	if err := repo.RecomputeSpent(ctx, issue); err != nil {
		t.Fatal(err)
	}
	it, _ = repo.IssueTime(ctx, issue, false)
	if it.Spent != 0 {
		t.Fatalf("aggregate after delete = %d", it.Spent)
	}
	if _, err := repo.Worklog(ctx, w.ID, false); apperror.CodeOf(err) != domain.CodeWorklogNotFound {
		t.Fatalf("deleted worklog code = %v", err)
	}
	if err := repo.SoftDeleteWorklog(ctx, w.ID, time.Now()); apperror.CodeOf(err) != domain.CodeWorklogNotFound {
		t.Fatalf("second delete = %v", err)
	}
}

func TestRepositoryMissingIssueIsNotFound(t *testing.T) {
	repo, _, _, _ := setup(t)
	if _, err := repo.IssueTime(context.Background(), uuid.New(), false); apperror.CodeOf(err) != domain.CodeIssueNotFound {
		t.Fatalf("missing issue code = %v", err)
	}
}

func TestRepositoryWindowAndOrgSettings(t *testing.T) {
	repo, author, _, issue := setup(t)
	ctx := context.Background()
	base := time.Date(2026, 11, 20, 12, 0, 0, 0, time.UTC)
	for i, hours := range []int64{1, 2, 3} {
		w := domain.Worklog{IssueID: issue, AuthorID: author, StartedAt: base.AddDate(0, 0, i), TimeSpentSeconds: hours * 3600, RemainingMode: domain.RemainingLeave}
		if err := repo.InsertWorklog(ctx, &w); err != nil {
			t.Fatal(err)
		}
	}
	rows, err := repo.Window(ctx, port.Window{Lo: base, Hi: base.AddDate(0, 0, 2), AuthorID: &author})
	if err != nil || len(rows) != 2 {
		t.Fatalf("window = %d %v", len(rows), err)
	}
	other := uuid.New()
	rows, _ = repo.Window(ctx, port.Window{Lo: base, Hi: base.AddDate(0, 0, 5), AuthorID: &other})
	if len(rows) != 0 {
		t.Fatalf("other author must see nothing")
	}

	// Organization settings: the default organization has no time keys, so defaults apply.
	times, err := repo.OrgTimes(ctx, []uuid.UUID{uuid.MustParse(testOrg)})
	if err != nil {
		t.Fatal(err)
	}
	z := times[uuid.MustParse(testOrg)]
	if z.Location == nil || z.Config != domain.DefaultConfig() {
		t.Fatalf("org time defaults = %+v", z)
	}
}

func TestRepositoryAddActivities(t *testing.T) {
	repo, author, _, issue := setup(t)
	old, neu := "3600", "7200"
	err := repo.AddActivities(context.Background(), []port.Activity{{
		IssueID: issue, ActorID: author, Action: "field_changed", Field: "remaining_estimate_seconds",
		OldValue: &old, NewValue: &neu, At: time.Now().UTC(), BatchID: uuid.New(),
	}})
	if err != nil {
		t.Fatalf("AddActivities: %v", err)
	}
}

func ptrInt(v int64) *int64 { return &v }
