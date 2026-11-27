package usecase

import (
	"testing"

	"github.com/jhonsferg/corelog/internal/project/domain"
)

// TestArchivedProjectRejectsManagerWrites covers the manager bypass of the project service: org
// owners and platform admins skip the permission scheme, so the archive rules must still reject their
// member, label and update writes with 409 project.archived. Restore brings the writes back.
func TestArchivedProjectRejectsManagerWrites(t *testing.T) {
	f := newFixture(t)
	p := createFromTemplate(t, f, f.owner, f.orgA, "ARW", domain.TemplateKanban)
	if _, err := f.svc.Archive(f.ctx, f.owner, "requester", p.ID); err != nil {
		t.Fatalf("Archive: %v", err)
	}

	_, err := f.svc.AddMember(f.ctx, f.owner, "requester", p.ID, f.member, "viewer")
	wantCode(t, err, "project.archived")

	_, err = f.svc.CreateLabel(f.ctx, f.admin, "admin", p.ID, "bug", "#ff0000")
	wantCode(t, err, "project.archived")

	_, err = f.svc.Update(f.ctx, f.owner, "requester", p.ID, domain.UpdateInput{
		Name: "renamed", Type: domain.ProjectTypeKanban, Visibility: domain.VisibilityOpen, LeadID: f.owner,
	})
	wantCode(t, err, "project.archived")

	if _, err := f.svc.Get(f.ctx, f.owner, "requester", p.ID); err != nil {
		t.Fatalf("read archived project: %v", err)
	}

	if _, err := f.svc.Restore(f.ctx, f.owner, "requester", p.ID); err != nil {
		t.Fatalf("Restore: %v", err)
	}
	if _, err := f.svc.AddMember(f.ctx, f.owner, "requester", p.ID, f.member, "viewer"); err != nil {
		t.Fatalf("AddMember after restore: %v", err)
	}
}
