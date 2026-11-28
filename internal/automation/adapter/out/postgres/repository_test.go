package postgres

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/jhonsferg/corelog/internal/automation/domain"
	"github.com/jhonsferg/corelog/internal/platform/database/testdb"
)

const orgID = "00000000-0000-4000-8000-000000000002"

func TestRepositoryRulesAndRuns(t *testing.T) {
	db := testdb.New(t)
	ctx := context.Background()
	org := uuid.MustParse(orgID)
	user, project := uuid.New(), uuid.New()
	if _, err := db.Exec(`INSERT INTO users (id, name, display_name, email, password_hash, role) VALUES ($1, 'u', 'u', 'u@example.test', 'x', 'user')`, user); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO projects (id, organization_id, key, name, type, template, lead_id, visibility) VALUES ($1, $2, 'AUT', 'Auto', 'kanban', 'kanban', $3, 'open')`, project, org, user); err != nil {
		t.Fatal(err)
	}
	repo := NewRepository(db)
	now := time.Now().UTC().Truncate(time.Microsecond)

	if got, ok, err := repo.ProjectOrg(ctx, project); err != nil || !ok || got != org {
		t.Fatalf("project org %v %v %v", got, ok, err)
	}
	if _, ok, _ := repo.ProjectOrg(ctx, uuid.New()); ok {
		t.Fatal("unknown project")
	}

	rule := domain.Rule{OrgID: org, ProjectID: &project, Name: "r", Enabled: true, CreatedBy: &user, CreatedAt: now,
		Trigger:    domain.Trigger{Type: domain.TriggerIssueTransitioned, To: "Done"},
		Conditions: []domain.Condition{{Type: domain.ConditionFieldEq, Field: "status", Value: "Done"}},
		Actions:    []domain.Action{{Type: domain.ActionAddLabel, Label: "x"}}}
	if err := repo.Create(ctx, &rule); err != nil || rule.ID == uuid.Nil {
		t.Fatalf("create %v", err)
	}
	scheduled := domain.Rule{OrgID: org, ProjectID: &project, Name: "s", Enabled: true, CreatedAt: now,
		Trigger: domain.Trigger{Type: domain.TriggerScheduled, EveryMinutes: 10, Query: "status = Open"}, Actions: []domain.Action{{Type: domain.ActionAddLabel, Label: "y"}}}
	if err := repo.Create(ctx, &scheduled); err != nil {
		t.Fatal(err)
	}
	disabled := domain.Rule{OrgID: org, ProjectID: &project, Name: "d", Enabled: false, CreatedAt: now, Trigger: domain.Trigger{Type: domain.TriggerIssueCreated},
		Actions: []domain.Action{{Type: domain.ActionAddLabel, Label: "z"}}}
	if err := repo.Create(ctx, &disabled); err != nil {
		t.Fatal(err)
	}
	orgWide := domain.Rule{OrgID: org, Name: "wide", Enabled: true, CreatedAt: now, Trigger: domain.Trigger{Type: domain.TriggerIssueCreated}, Actions: []domain.Action{{Type: domain.ActionAddLabel, Label: "w"}}}
	if err := repo.Create(ctx, &orgWide); err != nil {
		t.Fatal(err)
	}

	got, err := repo.Get(ctx, rule.ID)
	if err != nil || got.Name != "r" || got.Trigger.To != "Done" || len(got.Conditions) != 1 || got.Actions[0].Label != "x" || got.Version != 1 || got.CreatedBy == nil {
		t.Fatalf("get %+v %v", got, err)
	}
	if _, err := repo.Get(ctx, uuid.New()); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("unknown: %v", err)
	}
	if list, _ := repo.ListByProject(ctx, project); len(list) != 3 {
		t.Fatalf("list %d (the organization-wide rule is not part of the project list)", len(list))
	}
	if active, _ := repo.ActiveFor(ctx, org, project); len(active) != 3 { // rule, scheduled, org-wide; the disabled one is out
		t.Fatalf("active %d", len(active))
	}
	if active, _ := repo.ActiveFor(ctx, uuid.New(), project); len(active) != 0 {
		t.Fatal("other organization")
	}
	if sched, _ := repo.ActiveScheduled(ctx); len(sched) != 1 || sched[0].Name != "s" {
		t.Fatalf("scheduled %v", sched)
	}

	// Versioned update.
	got.Name, got.UpdatedAt = "renamed", now.Add(time.Second)
	if err := repo.Update(ctx, &got); err != nil || got.Version != 2 {
		t.Fatalf("update %+v %v", got, err)
	}
	stale := got
	stale.Version = 1
	if err := repo.Update(ctx, &stale); !errors.Is(err, domain.ErrVersionConflict) {
		t.Fatalf("stale update: %v", err)
	}
	if on, err := repo.SetEnabled(ctx, disabled.ID, true, now); err != nil || !on.Enabled {
		t.Fatalf("enable %+v %v", on, err)
	}
	if _, err := repo.SetEnabled(ctx, uuid.New(), true, now); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("enable unknown: %v", err)
	}

	// Runs update the counters, skipped ones do not.
	issue := uuid.New()
	event := int64(5)
	for _, status := range []string{domain.RunSuccess, domain.RunFailed, domain.RunSkipped} {
		run := domain.Run{RuleID: rule.ID, EventID: &event, IssueID: &issue, Status: status, Detail: status, CreatedAt: now.Add(time.Minute)}
		if err := repo.RecordRun(ctx, &run); err != nil || run.ID == uuid.Nil {
			t.Fatalf("record %v", err)
		}
	}
	after, _ := repo.Get(ctx, rule.ID)
	if after.RunCount != 2 || after.LastRunAt == nil {
		t.Fatalf("counters %+v", after)
	}
	if n, _ := repo.CountRunsSince(ctx, rule.ID, now); n != 2 {
		t.Fatalf("executed runs %d", n)
	}
	if n, _ := repo.CountRunsSince(ctx, rule.ID, now.Add(time.Hour)); n != 0 {
		t.Fatal("runs outside the window counted")
	}
	if runs, _ := repo.ListRuns(ctx, rule.ID, "", 10); len(runs) != 3 {
		t.Fatalf("runs %d", len(runs))
	}
	if runs, _ := repo.ListRuns(ctx, rule.ID, domain.RunFailed, 10); len(runs) != 1 || runs[0].IssueID == nil || *runs[0].IssueID != issue {
		t.Fatalf("filtered runs %v", runs)
	}
	if err := repo.TouchScheduled(ctx, scheduled.ID, now.Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	if s, _ := repo.Get(ctx, scheduled.ID); s.LastRunAt == nil || !s.LastRunAt.Equal(now.Add(time.Hour)) {
		t.Fatalf("touched %+v", s)
	}

	if err := repo.Delete(ctx, rule.ID); err != nil {
		t.Fatal(err)
	}
	if runs, _ := repo.ListRuns(ctx, rule.ID, "", 10); len(runs) != 0 {
		t.Fatal("runs of a deleted rule must be hidden")
	}
	var kept, stamped int
	if err := db.Get(&kept, `SELECT count(*) FROM automation_runs WHERE rule_id = $1`, rule.ID); err != nil || kept == 0 {
		t.Fatalf("runs must be kept for statistics: %d %v", kept, err)
	}
	if err := db.Get(&stamped, `SELECT count(*) FROM automation_rules WHERE id = $1 AND deleted_at IS NOT NULL`, rule.ID); err != nil || stamped != 1 {
		t.Fatalf("the rule must be kept with deleted_at set: %d %v", stamped, err)
	}
	if _, err := repo.Get(ctx, rule.ID); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("deleted rule visible through Get: %v", err)
	}
	if err := repo.Delete(ctx, rule.ID); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("second delete: %v", err)
	}
}
