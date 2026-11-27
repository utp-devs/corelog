package usecase

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/jhonsferg/corelog/internal/issue/application/port"
	"github.com/jhonsferg/corelog/internal/issue/domain"
	"github.com/jhonsferg/corelog/internal/platform/apperror"
)

// TestIssueService_Unassign cubre desasignar: permiso, historial, aviso e idempotencia.
func TestIssueService_Unassign(t *testing.T) {
	ctx := context.Background()

	setup := func(t *testing.T, role *string) (*IssueService, *fakeIssueRepo, *memCollab, *fakeNotifier, *domain.Issue, uuid.UUID, uuid.UUID) {
		t.Helper()
		repo := newFakeIssueRepo()
		dir := newFakeUserDirectory()
		team := uuid.New()
		assigner, assignee := uuid.New(), uuid.New()
		dir.userTeams[assigner] = &team
		dir.userTeams[assignee] = &team
		collab := &memCollab{}
		notifier := &fakeNotifier{}
		svc := NewIssueService(repo, dir, fakeProjectRoleResolver{role: role}, newFakeWatcherRepository(), collab).WithNotifier(notifier)
		issue, err := svc.Create(ctx, port.CreateInput{Summary: "Desasignar", Description: "Desc", Priority: domain.PriorityLow, ReporterID: assigner})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := svc.Assign(ctx, issue.ID, assigner, assignee, "admin"); err != nil {
			t.Fatalf("assign: %v", err)
		}
		notifier.calls = nil
		collab.activities = nil
		return svc, repo, collab, notifier, issue, assigner, assignee
	}

	contributor := "contributor"
	viewer := "viewer"

	t.Run("clears the assignee and records the previous assignee in history", func(t *testing.T) {
		svc, repo, collab, _, issue, actor, assignee := setup(t, &contributor)

		got, err := svc.Unassign(ctx, issue.ID, actor, "admin")
		if err != nil {
			t.Fatalf("unassign: %v", err)
		}
		if got.AssigneeID != nil {
			t.Errorf("returned assignee = %v, want nil", got.AssigneeID)
		}
		stored, _ := repo.FindByID(ctx, issue.ID)
		if stored.AssigneeID != nil {
			t.Errorf("stored assignee = %v, want nil", stored.AssigneeID)
		}
		if len(collab.activities) != 1 {
			t.Fatalf("activities = %d, want 1", len(collab.activities))
		}
		activity := collab.activities[0]
		if activity.Action != "assignee_changed" || activity.ActorID != actor || activity.IssueID != issue.ID {
			t.Errorf("activity = %+v, want assignee_changed by actor on issue", activity)
		}
		if activity.FieldName == nil || *activity.FieldName != "assignee_id" {
			t.Errorf("field name = %v, want assignee_id", activity.FieldName)
		}
		if activity.OldValue == nil || *activity.OldValue != assignee.String() {
			t.Errorf("old value = %v, want %v", activity.OldValue, assignee)
		}
		if activity.NewValue != nil {
			t.Errorf("new value = %v, want nil", *activity.NewValue)
		}
	})

	t.Run("does not notify anyone", func(t *testing.T) {
		svc, _, _, notifier, issue, actor, _ := setup(t, &contributor)
		if _, err := svc.Unassign(ctx, issue.ID, actor, "admin"); err != nil {
			t.Fatalf("unassign: %v", err)
		}
		if len(notifier.calls) != 0 {
			t.Errorf("notifications = %v, want none", notifier.calls)
		}
	})

	t.Run("is idempotent when already unassigned", func(t *testing.T) {
		svc, _, collab, notifier, issue, actor, _ := setup(t, &contributor)
		if _, err := svc.Unassign(ctx, issue.ID, actor, "admin"); err != nil {
			t.Fatalf("first unassign: %v", err)
		}
		collab.activities = nil
		got, err := svc.Unassign(ctx, issue.ID, actor, "admin")
		if err != nil {
			t.Fatalf("second unassign: %v", err)
		}
		if got.AssigneeID != nil {
			t.Errorf("assignee = %v, want nil", got.AssigneeID)
		}
		if len(collab.activities) != 0 {
			t.Errorf("second unassign recorded %d activities, want none", len(collab.activities))
		}
		if len(notifier.calls) != 0 {
			t.Errorf("notifications = %v, want none", notifier.calls)
		}
	})

	t.Run("requires the same permission as assigning", func(t *testing.T) {
		_, repo, collab, _, issue, actor, assignee := setup(t, &contributor)
		viewerSvc := NewIssueService(repo, newFakeUserDirectory(), fakeProjectRoleResolver{role: &viewer}, newFakeWatcherRepository(), collab)
		_, err := viewerSvc.Unassign(ctx, issue.ID, actor, "viewer")
		if apperror.KindOf(err) != apperror.KindForbidden {
			t.Fatalf("got %v, want forbidden", err)
		}
		stored, _ := repo.FindByID(ctx, issue.ID)
		if stored.AssigneeID == nil || *stored.AssigneeID != assignee {
			t.Errorf("denied unassign changed assignee to %v", stored.AssigneeID)
		}
		if len(collab.activities) != 0 {
			t.Errorf("denied unassign recorded activity")
		}
	})

	t.Run("returns not found for unknown issue", func(t *testing.T) {
		svc, _, _, _, _, actor, _ := setup(t, &contributor)
		if _, err := svc.Unassign(ctx, uuid.New(), actor, "admin"); !errors.Is(err, domain.ErrIssueNotFound) {
			t.Fatalf("got %v, want ErrIssueNotFound", err)
		}
	})
}
