package usecase

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/jhonsferg/corelog/internal/platform/apperror"
	"github.com/jhonsferg/corelog/internal/ticket/application/port"
	"github.com/jhonsferg/corelog/internal/ticket/domain"
)

// TestTicketService_Unassign cubre desasignar: permiso, historial, aviso e idempotencia.
func TestTicketService_Unassign(t *testing.T) {
	ctx := context.Background()

	setup := func(t *testing.T, role *string) (*TicketService, *fakeTicketRepo, *memCollab, *fakeNotifier, *domain.Ticket, uuid.UUID, uuid.UUID) {
		t.Helper()
		repo := newFakeTicketRepo()
		dir := newFakeUserDirectory()
		team := uuid.New()
		assigner, assignee := uuid.New(), uuid.New()
		dir.userTeams[assigner] = &team
		dir.userTeams[assignee] = &team
		collab := &memCollab{}
		notifier := &fakeNotifier{}
		svc := NewTicketService(repo, dir, fakeProjectRoleResolver{role: role}, newFakeWatcherRepository(), collab).WithNotifier(notifier)
		ticket, err := svc.Create(ctx, port.CreateInput{Title: "Desasignar", Description: "Desc", Priority: domain.PriorityLow, RequesterID: assigner})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := svc.Assign(ctx, ticket.ID, assigner, assignee, "admin"); err != nil {
			t.Fatalf("assign: %v", err)
		}
		notifier.calls = nil
		collab.activities = nil
		return svc, repo, collab, notifier, ticket, assigner, assignee
	}

	contributor := "contributor"
	viewer := "viewer"

	t.Run("clears the assignee and records the previous assignee in history", func(t *testing.T) {
		svc, repo, collab, _, ticket, actor, assignee := setup(t, &contributor)

		got, err := svc.Unassign(ctx, ticket.ID, actor, "admin")
		if err != nil {
			t.Fatalf("unassign: %v", err)
		}
		if got.AssigneeID != nil {
			t.Errorf("returned assignee = %v, want nil", got.AssigneeID)
		}
		stored, _ := repo.FindByID(ctx, ticket.ID)
		if stored.AssigneeID != nil {
			t.Errorf("stored assignee = %v, want nil", stored.AssigneeID)
		}
		if len(collab.activities) != 1 {
			t.Fatalf("activities = %d, want 1", len(collab.activities))
		}
		activity := collab.activities[0]
		if activity.Action != "assignee_changed" || activity.ActorID != actor || activity.IssueID != ticket.ID {
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
		svc, _, _, notifier, ticket, actor, _ := setup(t, &contributor)
		if _, err := svc.Unassign(ctx, ticket.ID, actor, "admin"); err != nil {
			t.Fatalf("unassign: %v", err)
		}
		if len(notifier.calls) != 0 {
			t.Errorf("notifications = %v, want none", notifier.calls)
		}
	})

	t.Run("is idempotent when already unassigned", func(t *testing.T) {
		svc, _, collab, notifier, ticket, actor, _ := setup(t, &contributor)
		if _, err := svc.Unassign(ctx, ticket.ID, actor, "admin"); err != nil {
			t.Fatalf("first unassign: %v", err)
		}
		collab.activities = nil
		got, err := svc.Unassign(ctx, ticket.ID, actor, "admin")
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
		_, repo, collab, _, ticket, actor, assignee := setup(t, &contributor)
		viewerSvc := NewTicketService(repo, newFakeUserDirectory(), fakeProjectRoleResolver{role: &viewer}, newFakeWatcherRepository(), collab)
		_, err := viewerSvc.Unassign(ctx, ticket.ID, actor, "viewer")
		if apperror.KindOf(err) != apperror.KindForbidden {
			t.Fatalf("got %v, want forbidden", err)
		}
		stored, _ := repo.FindByID(ctx, ticket.ID)
		if stored.AssigneeID == nil || *stored.AssigneeID != assignee {
			t.Errorf("denied unassign changed assignee to %v", stored.AssigneeID)
		}
		if len(collab.activities) != 0 {
			t.Errorf("denied unassign recorded activity")
		}
	})

	t.Run("returns not found for unknown ticket", func(t *testing.T) {
		svc, _, _, _, _, actor, _ := setup(t, &contributor)
		if _, err := svc.Unassign(ctx, uuid.New(), actor, "admin"); !errors.Is(err, domain.ErrTicketNotFound) {
			t.Fatalf("got %v, want ErrTicketNotFound", err)
		}
	})
}
