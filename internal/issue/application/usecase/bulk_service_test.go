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

func TestBulkPartialFailureReportsEachIssue(t *testing.T) {
	f := newOpsFixture(t)
	ctx := context.Background()
	mine := f.issue(t, f.projA, f.owner, "Mia")
	// Reported by someone else: the caller holds no edit_own_issue on it, so the project gate decides.
	locked := f.issue(t, f.projB, f.other, "Bloqueada")
	f.gate.denyEdit[f.projB] = true
	missing := uuid.New()

	res, err := f.svc.BulkChange(ctx, f.owner, "user", port.BulkInput{
		Refs: []string{mine.ID.String(), locked.ID.String(), missing.String()}, Action: domain.BulkSetPriority, PriorityID: f.high,
	})
	if err != nil {
		t.Fatalf("BulkChange: %v", err)
	}
	if len(res.Succeeded) != 1 || res.Succeeded[0].ID != mine.ID || res.Succeeded[0].PriorityID != f.high {
		t.Fatalf("succeeded = %+v, want only the editable issue with the new priority", res.Succeeded)
	}
	if len(res.Failed) != 2 {
		t.Fatalf("failed = %+v, want two", res.Failed)
	}
	if res.Failed[0].Ref != locked.ID.String() || res.Failed[0].Code != "issue.forbidden" {
		t.Fatalf("forbidden failure = %+v", res.Failed[0])
	}
	if res.Failed[1].Ref != missing.String() || res.Failed[1].Code != "issue.not_found" {
		t.Fatalf("missing failure = %+v", res.Failed[1])
	}
	stored, _ := f.repo.FindByID(ctx, locked.ID)
	if stored.PriorityID == f.high {
		t.Fatal("forbidden issue changed anyway")
	}
	if len(f.events.list) != 1 || f.events.list[0].Type != "issue.updated" {
		t.Fatalf("events = %+v, want one issue.updated for the changed issue", f.events.list)
	}
}

func TestBulkRejectsMalformedRequests(t *testing.T) {
	f := newOpsFixture(t)
	ctx := context.Background()
	many := make([]string, domain.MaxBulkIssues+1)
	for i := range many {
		many[i] = uuid.NewString()
	}
	cases := []struct {
		name string
		in   port.BulkInput
		code string
	}{
		{"empty", port.BulkInput{Action: domain.BulkDelete}, "issue.bulk_empty"},
		{"too many", port.BulkInput{Refs: many, Action: domain.BulkDelete}, "issue.bulk_too_many"},
		{"assign without assignee", port.BulkInput{Refs: []string{uuid.NewString()}, Action: domain.BulkAssign}, "issue.bulk_params_invalid"},
		{"priority without id", port.BulkInput{Refs: []string{uuid.NewString()}, Action: domain.BulkSetPriority}, "issue.bulk_params_invalid"},
		{"labels empty", port.BulkInput{Refs: []string{uuid.NewString()}, Action: domain.BulkSetLabels}, "issue.bulk_params_invalid"},
		{"field without edits", port.BulkInput{Refs: []string{uuid.NewString()}, Action: domain.BulkSetField}, "issue.bulk_params_invalid"},
		{"bad status", port.BulkInput{Refs: []string{uuid.NewString()}, Action: domain.BulkTransition, Status: "nope"}, "issue.invalid_status"},
		{"move without project", port.BulkInput{Refs: []string{uuid.NewString()}, Action: domain.BulkMove}, "issue.bulk_params_invalid"},
		{"unknown action", port.BulkInput{Refs: []string{uuid.NewString()}, Action: "explode"}, "issue.bulk_action_unknown"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := f.svc.BulkChange(ctx, f.lead, "user", tc.in)
			if apperror.KindOf(err) != apperror.KindValidation || apperror.CodeOf(err) != tc.code {
				t.Fatalf("err = %v (code %q), want validation %s", err, apperror.CodeOf(err), tc.code)
			}
		})
	}
	if _, err := domain.ParseBulkAction("explode"); err == nil {
		t.Fatal("unknown action parsed")
	}
}

func TestBulkDedupesRepeatedReferences(t *testing.T) {
	f := newOpsFixture(t)
	issue := f.issue(t, f.projA, f.owner, "Una vez")
	res, err := f.svc.BulkChange(context.Background(), f.lead, "user", port.BulkInput{
		Refs: []string{issue.ID.String(), issue.ID.String()}, Action: domain.BulkSetPriority, PriorityID: f.high,
	})
	if err != nil || len(res.Succeeded) != 1 || len(res.Failed) != 0 {
		t.Fatalf("res = %+v, err = %v; want one success", res, err)
	}
}

func TestBulkLabelsNeedLabelsOfTheIssueProject(t *testing.T) {
	f := newOpsFixture(t)
	ctx := context.Background()
	local := uuid.New()
	f.ops.labelNames[local] = "backend"
	f.ops.labelProject[local] = f.projA
	inA := f.issue(t, f.projA, f.owner, "En A")
	inB := f.issue(t, f.projB, f.owner, "En B")

	res, err := f.svc.BulkChange(ctx, f.lead, "user", port.BulkInput{
		Refs: []string{inA.ID.String(), inB.ID.String()}, Action: domain.BulkSetLabels, LabelsAdd: []uuid.UUID{local},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Succeeded) != 1 || res.Succeeded[0].ID != inA.ID {
		t.Fatalf("succeeded = %+v", res.Succeeded)
	}
	if len(res.Failed) != 1 || res.Failed[0].Code != "issue.label_not_in_project" {
		t.Fatalf("failed = %+v", res.Failed)
	}
	names, _ := f.ops.IssueLabelNames(ctx, inA.ID)
	if strings.Join(names, ",") != "backend" {
		t.Fatalf("labels of A = %v", names)
	}
	if len(f.events.list) != 1 {
		t.Fatalf("events = %d, want one for the label change", len(f.events.list))
	}
}

func TestBulkAssignFollowsSingleAssignRules(t *testing.T) {
	f := newOpsFixture(t)
	ctx := context.Background()
	issue := f.issue(t, f.projA, f.owner, "Asignar")
	outsider := uuid.New()

	res, err := f.svc.BulkChange(ctx, f.lead, "user", port.BulkInput{
		Refs: []string{issue.ID.String()}, Action: domain.BulkAssign, Assignee: port.Some(outsider),
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Failed) != 1 || res.Failed[0].Code != domain.ErrCrossTeamAssignment.Code {
		t.Fatalf("assign to another team = %+v, want %s", res.Failed, domain.ErrCrossTeamAssignment.Code)
	}

	res, err = f.svc.BulkChange(ctx, f.lead, "user", port.BulkInput{
		Refs: []string{issue.ID.String()}, Action: domain.BulkAssign, Assignee: port.Some(f.other),
	})
	if err != nil || len(res.Succeeded) != 1 || res.Succeeded[0].AssigneeID == nil || *res.Succeeded[0].AssigneeID != f.other {
		t.Fatalf("assign to a teammate = %+v, err %v", res, err)
	}
	if res.Succeeded[0].Status != domain.StatusOpen {
		t.Fatalf("status after assign = %s, want it unchanged like the single endpoint", res.Succeeded[0].Status)
	}
}

func TestBulkDeleteSoftDeletesEachIssue(t *testing.T) {
	f := newOpsFixture(t)
	ctx := context.Background()
	a := f.issue(t, f.projA, f.owner, "Borrar A")
	b := f.issue(t, f.projA, f.owner, "Borrar B")
	res, err := f.svc.BulkChange(ctx, f.lead, "user", port.BulkInput{Refs: []string{a.ID.String(), b.ID.String()}, Action: domain.BulkDelete})
	if err != nil || len(res.Succeeded) != 2 || len(res.Failed) != 0 {
		t.Fatalf("delete = %+v, err %v", res, err)
	}
	if _, err := f.repo.FindByID(ctx, a.ID); apperror.CodeOf(err) != "issue.not_found" {
		t.Fatalf("deleted issue still readable: %v", err)
	}
}
