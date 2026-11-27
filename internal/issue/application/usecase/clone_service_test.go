package usecase

import (
	"context"
	"testing"

	"github.com/google/uuid"

	"github.com/jhonsferg/corelog/internal/issue/application/port"
	"github.com/jhonsferg/corelog/internal/issue/domain"
	"github.com/jhonsferg/corelog/internal/platform/apperror"
)

func TestCloneCopiesFieldsAndLinksBackToOriginal(t *testing.T) {
	f := newOpsFixture(t)
	ctx := context.Background()
	original := f.issue(t, f.projA, f.owner, "Original")

	clone, err := f.svc.CloneIssue(ctx, original.ID, f.other, "user", port.CloneInput{})
	if err != nil {
		t.Fatalf("CloneIssue: %v", err)
	}
	if clone.ID == original.ID || clone.Key() == original.Key() {
		t.Fatalf("clone reuses the original identity: %s", clone.Key())
	}
	if clone.Summary != "Copia de Original" || clone.ReporterID != f.other || clone.Status != domain.StatusOpen {
		t.Fatalf("clone = %+v, want default summary, caller as reporter, open", clone)
	}
	if len(f.links.links) != 1 || f.links.links[0].Kind != domain.LinkKindClones ||
		f.links.links[0].SourceIssueID != clone.ID || f.links.links[0].TargetIssueID != original.ID {
		t.Fatalf("links = %+v, want one clones link from the copy to the original", f.links.links)
	}
	if f.ops.copies != 1 {
		t.Fatalf("copies = %d; only the project-scoped copy (labels and components) runs without options", f.ops.copies)
	}
}

func TestCloneOptionsAndSummary(t *testing.T) {
	f := newOpsFixture(t)
	ctx := context.Background()
	original := f.issue(t, f.projA, f.owner, "Con opciones")

	clone, err := f.svc.CloneIssue(ctx, original.ID, f.lead, "user", port.CloneInput{
		Summary: "Nueva", Include: []string{domain.CloneIncludeLinks, domain.CloneIncludeWatchers},
	})
	if err != nil || clone.Summary != "Nueva" {
		t.Fatalf("clone with summary = %+v, err %v", clone, err)
	}
	if f.ops.copies != 3 {
		t.Fatalf("copies = %d, want links and watchers copied plus the project-scoped copy", f.ops.copies)
	}

	_, err = f.svc.CloneIssue(ctx, original.ID, f.lead, "user", port.CloneInput{Include: []string{"comments"}})
	if apperror.CodeOf(err) != "issue.clone_option_invalid" {
		t.Fatalf("unknown option err = %v", err)
	}
}

func TestCloneToAnotherProjectNeedsCreatePermission(t *testing.T) {
	f := newOpsFixture(t)
	original := f.issue(t, f.projA, f.owner, "Del origen")

	_, err := f.svc.CloneIssue(context.Background(), original.ID, f.viewer, "user", port.CloneInput{ProjectID: f.projB})
	if apperror.KindOf(err) != apperror.KindForbidden {
		t.Fatalf("viewer clone err = %v, want forbidden", err)
	}
}

func TestCloneOfInvisibleIssueLooksMissing(t *testing.T) {
	f := newOpsFixture(t)
	_, err := f.svc.CloneIssue(context.Background(), uuid.New(), f.lead, "user", port.CloneInput{})
	if apperror.KindOf(err) != apperror.KindNotFound {
		t.Fatalf("clone of missing issue err = %v, want not found", err)
	}
}
