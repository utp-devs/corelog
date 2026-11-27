package usecase

import (
	"context"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/jhonsferg/corelog/internal/issue/application/port"
	"github.com/jhonsferg/corelog/internal/issue/domain"
	"github.com/jhonsferg/corelog/internal/platform/apperror"
)

// setType gives an issue a catalog type through the patch rules.
func (f *opsFixture) setType(t *testing.T, id, typeID uuid.UUID) {
	t.Helper()
	if _, err := f.svc.Patch(context.Background(), id, f.lead, "user", port.PatchInput{IssueTypeID: port.Some(typeID)}); err != nil {
		t.Fatalf("set type: %v", err)
	}
}

func TestMoveRequiresMappingForTypesTheTargetLacks(t *testing.T) {
	f := newOpsFixture(t)
	ctx := context.Background()
	// A story is enabled in the target project, so it moves without a mapping.
	story := f.issue(t, f.projA, f.owner, "Historia")
	f.setType(t, story.ID, f.story)
	if res, err := f.svc.MoveIssue(ctx, story.ID, f.lead, "user", port.MoveInput{ProjectID: f.projB}); err != nil || res.Issue.IssueTypeID != f.story {
		t.Fatalf("story move = %v, err %v; want no mapping needed", res, err)
	}

	epic := f.issue(t, f.projA, f.owner, "Epica")
	f.setType(t, epic.ID, f.epic)
	_, err := f.svc.MoveIssue(ctx, epic.ID, f.lead, "user", port.MoveInput{ProjectID: f.projB})
	if apperror.CodeOf(err) != "issue.move_mapping_required" {
		t.Fatalf("epic err = %v, want issue.move_mapping_required", err)
	}
	details := apperror.DetailsOf(err)
	if len(details) != 1 || details[0].Field != "field_mapping.issue_type."+f.epic.String() || !strings.Contains(details[0].Message, f.task.String()) {
		t.Fatalf("details = %+v, want one mapping for the epic listing the accepted types", details)
	}
	stored, _ := f.repo.FindByID(ctx, epic.ID)
	if stored.ProjectID != f.projA {
		t.Fatal("a refused move changed the issue")
	}
}

func TestMoveWithMappingKeepsOldKeyAndReportsDrops(t *testing.T) {
	f := newOpsFixture(t)
	ctx := context.Background()
	root := f.issue(t, f.projA, f.owner, "Mover")
	f.setType(t, root.ID, f.epic)
	label := uuid.New()
	f.ops.labelNames[label] = "deuda"
	f.ops.labelProject[label] = f.projA
	_ = f.ops.AddIssueLabels(ctx, root.ID, []uuid.UUID{label})
	oldKey := root.Key()

	res, err := f.svc.MoveIssue(ctx, root.ID, f.lead, "user", port.MoveInput{
		ProjectID: f.projB,
		Mapping:   domain.MoveMapping{IssueTypes: map[uuid.UUID]uuid.UUID{f.epic: f.task}},
	})
	if err != nil {
		t.Fatalf("MoveIssue: %v", err)
	}
	if res.Issue.ProjectID != f.projB || res.Issue.IssueTypeID != f.task {
		t.Fatalf("moved issue = project %s type %s", res.Issue.ProjectID, res.Issue.IssueTypeID)
	}
	if res.Issue.Key() == oldKey || !strings.HasPrefix(res.Issue.Key(), "OTHR-") {
		t.Fatalf("new key = %q, old %q", res.Issue.Key(), oldKey)
	}
	if len(res.Warnings) != 1 || !strings.Contains(res.Warnings[0], "etiquetas") {
		t.Fatalf("warnings = %v, want the dropped label", res.Warnings)
	}
	if len(res.Moved) != 1 || res.Moved[0].FromKey != oldKey || res.Moved[0].ToKey != res.Issue.Key() {
		t.Fatalf("moved = %+v", res.Moved)
	}

	// The old key still resolves to the moved issue.
	id, err := f.svc.ResolveKey(ctx, f.lead, "user", uuid.Nil, oldKey)
	if err != nil || id != root.ID {
		t.Fatalf("old key resolved to %v, err %v; want %v", id, err, root.ID)
	}
	if _, err := f.svc.ResolveKey(ctx, f.lead, "user", uuid.Nil, "CORE-999999"); apperror.CodeOf(err) != "issue.not_found" {
		t.Fatalf("unknown key err = %v", err)
	}
}

func TestMoveTakesChildrenAlong(t *testing.T) {
	f := newOpsFixture(t)
	ctx := context.Background()
	// The target accepts epics too, so the epic moves without a mapping and its child keeps the parent.
	f.catalog.projectTypes[f.projB] = []uuid.UUID{f.task, f.story, f.epic}
	epic := f.issue(t, f.projA, f.owner, "Epica con hijos")
	f.setType(t, epic.ID, f.epic)
	child, err := f.svc.Create(ctx, port.CreateInput{
		Summary: "Hijo", Description: "d", Priority: domain.PriorityMedium, ReporterID: f.owner, GlobalRole: "user",
		ProjectID: &f.projA, ParentID: &epic.ID,
	})
	if err != nil {
		t.Fatal(err)
	}
	res, err := f.svc.MoveIssue(ctx, epic.ID, f.lead, "user", port.MoveInput{ProjectID: f.projB})
	if err != nil {
		t.Fatalf("MoveIssue: %v", err)
	}
	if len(res.Moved) != 2 {
		t.Fatalf("moved = %+v, want the epic and its child", res.Moved)
	}
	moved, _ := f.repo.FindByID(ctx, child.ID)
	if moved.ProjectID != f.projB || moved.ParentID == nil || *moved.ParentID != epic.ID || !strings.HasPrefix(moved.Key(), "OTHR-") {
		t.Fatalf("child after move = project %s parent %v key %s", moved.ProjectID, moved.ParentID, moved.Key())
	}
}

func TestMoveRejectsSameProjectAndUnknownTypeFromCaller(t *testing.T) {
	f := newOpsFixture(t)
	ctx := context.Background()
	root := f.issue(t, f.projA, f.owner, "Igual")
	if _, err := f.svc.MoveIssue(ctx, root.ID, f.lead, "user", port.MoveInput{ProjectID: f.projA}); apperror.CodeOf(err) != "issue.move_same_project" {
		t.Fatalf("same project err = %v", err)
	}
	bogus := uuid.New()
	_, err := f.svc.MoveIssue(ctx, root.ID, f.lead, "user", port.MoveInput{ProjectID: f.projB, IssueTypeID: &bogus})
	if apperror.CodeOf(err) != "issue.move_mapping_invalid" {
		t.Fatalf("explicit unknown type err = %v, want issue.move_mapping_invalid", err)
	}
}

func TestMoveNeedsCreatePermissionOnTarget(t *testing.T) {
	f := newOpsFixture(t)
	root := f.issue(t, f.projA, f.owner, "Sin permiso")
	_, err := f.svc.MoveIssue(context.Background(), root.ID, f.viewer, "user", port.MoveInput{ProjectID: f.projB})
	if apperror.KindOf(err) != apperror.KindForbidden && apperror.KindOf(err) != apperror.KindNotFound {
		t.Fatalf("viewer move err = %v, want forbidden or not found", err)
	}
}
