package usecase

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/jhonsferg/corelog/internal/issue/application/port"
	"github.com/jhonsferg/corelog/internal/issue/domain"
	"github.com/jhonsferg/corelog/internal/platform/apperror"
	"github.com/jhonsferg/corelog/internal/platform/database"
)

// failingActivityCollab falla al guardar el historial y conserva el resto de la colaboracion en memoria.
type failingActivityCollab struct {
	memCollab
	err error
}

func (f *failingActivityCollab) SaveActivity(context.Context, *domain.Activity) error {
	return f.err
}

type assignTxFixture struct {
	svc      *IssueService
	tx       *database.FakeTxManager
	repo     *fakeIssueRepo
	collab   *memCollab
	issue    *domain.Issue
	assigner uuid.UUID
	assignee uuid.UUID
}

func newAssignTxFixture(t *testing.T, collab port.CollaborationRepository, tx *database.FakeTxManager) assignTxFixture {
	t.Helper()
	ctx := context.Background()
	repo := newFakeIssueRepo()
	dir := newFakeUserDirectory()
	team := uuid.New()
	assigner, assignee := uuid.New(), uuid.New()
	dir.userTeams[assigner] = &team
	dir.userTeams[assignee] = &team
	svc := NewIssueService(repo, dir, contributorRoleResolver(), newFakeWatcherRepository(), collab).WithTxManager(tx)
	issue, err := svc.Create(ctx, port.CreateInput{Summary: "Asignar", Description: "Desc", Priority: domain.PriorityLow, ReporterID: assigner})
	if err != nil {
		t.Fatal(err)
	}
	tx.Calls = 0
	mem, _ := collab.(*memCollab)
	if mem != nil {
		mem.activities = nil
	}
	return assignTxFixture{svc: svc, tx: tx, repo: repo, collab: mem, issue: issue, assigner: assigner, assignee: assignee}
}

func TestIssueService_AssignRunsWritesInOneTransaction(t *testing.T) {
	ctx := context.Background()
	collab := &memCollab{}
	tx := &database.FakeTxManager{}
	fx := newAssignTxFixture(t, collab, tx)

	if _, err := fx.svc.Assign(ctx, fx.issue.ID, fx.assigner, fx.assignee, "admin"); err != nil {
		t.Fatalf("assign: %v", err)
	}
	if tx.Calls != 1 {
		t.Fatalf("InTx calls = %d, want 1", tx.Calls)
	}
	stored, _ := fx.repo.FindByID(ctx, fx.issue.ID)
	if stored.AssigneeID == nil || *stored.AssigneeID != fx.assignee {
		t.Fatalf("stored assignee = %v, want %v", stored.AssigneeID, fx.assignee)
	}
	if len(collab.activities) != 1 {
		t.Fatalf("activities = %d, want 1", len(collab.activities))
	}
}

func TestIssueService_AssignRecordsAssigneeActivity(t *testing.T) {
	ctx := context.Background()
	collab := &memCollab{}
	fx := newAssignTxFixture(t, collab, &database.FakeTxManager{})

	if _, err := fx.svc.Assign(ctx, fx.issue.ID, fx.assigner, fx.assignee, "admin"); err != nil {
		t.Fatalf("first assign: %v", err)
	}
	activity := collab.activities[0]
	if activity.Action != "assignee_changed" || activity.ActorID != fx.assigner || activity.IssueID != fx.issue.ID {
		t.Errorf("activity = %+v, want assignee_changed by assigner on issue", activity)
	}
	if activity.FieldName == nil || *activity.FieldName != "assignee_id" {
		t.Errorf("field name = %v, want assignee_id", activity.FieldName)
	}
	if activity.OldValue != nil {
		t.Errorf("old value = %v, want nil when assigning from empty", *activity.OldValue)
	}
	if activity.NewValue == nil || *activity.NewValue != fx.assignee.String() {
		t.Errorf("new value = %v, want %v", activity.NewValue, fx.assignee)
	}

	// Reasignar a otra persona registra la anterior como old_value.
	next := uuid.New()
	team := uuid.New()
	fx.svc.directory.(*fakeUserDirectory).userTeams[next] = &team
	fx.svc.directory.(*fakeUserDirectory).userTeams[fx.assigner] = &team
	collab.activities = nil
	if _, err := fx.svc.Assign(ctx, fx.issue.ID, fx.assigner, next, "admin"); err != nil {
		t.Fatalf("reassign: %v", err)
	}
	if len(collab.activities) != 1 {
		t.Fatalf("reassign activities = %d, want 1", len(collab.activities))
	}
	if old := collab.activities[0].OldValue; old == nil || *old != fx.assignee.String() {
		t.Errorf("reassign old value = %v, want %v", old, fx.assignee)
	}
}

func TestIssueService_AssignSameAssigneeRecordsNoActivity(t *testing.T) {
	ctx := context.Background()
	collab := &memCollab{}
	fx := newAssignTxFixture(t, collab, &database.FakeTxManager{})

	if _, err := fx.svc.Assign(ctx, fx.issue.ID, fx.assigner, fx.assignee, "admin"); err != nil {
		t.Fatalf("assign: %v", err)
	}
	collab.activities = nil
	if _, err := fx.svc.Assign(ctx, fx.issue.ID, fx.assigner, fx.assignee, "admin"); err != nil {
		t.Fatalf("assign again: %v", err)
	}
	if len(collab.activities) != 0 {
		t.Errorf("activities = %d, want none when the assignee does not change", len(collab.activities))
	}
}

func TestIssueService_AssignReturnsActivityFailure(t *testing.T) {
	ctx := context.Background()
	boom := apperror.New(apperror.KindInternal, "activity store down")
	collab := &failingActivityCollab{err: boom}
	tx := &database.FakeTxManager{}
	fx := newAssignTxFixture(t, collab, tx)

	_, err := fx.svc.Assign(ctx, fx.issue.ID, fx.assigner, fx.assignee, "admin")
	if !errors.Is(err, boom) {
		t.Fatalf("assign err = %v, want activity failure", err)
	}
	if tx.Calls != 1 {
		t.Errorf("InTx calls = %d, want 1", tx.Calls)
	}
}

func TestIssueService_AssignFailsWhenTransactionCannotBegin(t *testing.T) {
	ctx := context.Background()
	collab := &memCollab{}
	tx := &database.FakeTxManager{FailBegin: true}
	fx := newAssignTxFixture(t, collab, tx)

	if _, err := fx.svc.Assign(ctx, fx.issue.ID, fx.assigner, fx.assignee, "admin"); err == nil {
		t.Fatal("assign succeeded without a transaction")
	}
	stored, _ := fx.repo.FindByID(ctx, fx.issue.ID)
	if stored.AssigneeID != nil {
		t.Errorf("assignee = %v, want nil after failed begin", stored.AssigneeID)
	}
	if len(collab.activities) != 0 {
		t.Errorf("activities = %d, want none", len(collab.activities))
	}
}

func TestIssueService_UnassignRunsWritesInOneTransaction(t *testing.T) {
	ctx := context.Background()
	collab := &memCollab{}
	tx := &database.FakeTxManager{}
	fx := newAssignTxFixture(t, collab, tx)
	if _, err := fx.svc.Assign(ctx, fx.issue.ID, fx.assigner, fx.assignee, "admin"); err != nil {
		t.Fatalf("assign: %v", err)
	}
	tx.Calls = 0
	collab.activities = nil

	if _, err := fx.svc.Unassign(ctx, fx.issue.ID, fx.assigner, "admin"); err != nil {
		t.Fatalf("unassign: %v", err)
	}
	if tx.Calls != 1 {
		t.Fatalf("InTx calls = %d, want 1", tx.Calls)
	}
	if len(collab.activities) != 1 {
		t.Fatalf("activities = %d, want 1", len(collab.activities))
	}
}

func TestIssueService_UnassignReturnsActivityFailure(t *testing.T) {
	ctx := context.Background()
	boom := apperror.New(apperror.KindInternal, "activity store down")
	collab := &failingActivityCollab{err: boom}
	fx := newAssignTxFixture(t, collab, &database.FakeTxManager{})
	// Asignar con un repositorio de historial sano y luego fallar solo al desasignar.
	fx.svc.collaboration = &memCollab{}
	if _, err := fx.svc.Assign(ctx, fx.issue.ID, fx.assigner, fx.assignee, "admin"); err != nil {
		t.Fatalf("assign: %v", err)
	}
	fx.svc.collaboration = collab

	_, err := fx.svc.Unassign(ctx, fx.issue.ID, fx.assigner, "admin")
	if !errors.Is(err, boom) {
		t.Fatalf("unassign err = %v, want activity failure", err)
	}
}
