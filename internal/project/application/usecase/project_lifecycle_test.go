package usecase

import (
	"context"
	"testing"

	"github.com/google/uuid"

	"github.com/jhonsferg/corelog/internal/platform/apperror"
	"github.com/jhonsferg/corelog/internal/project/application/port"
	"github.com/jhonsferg/corelog/internal/project/domain"
)

// recordingEvents keeps every published project event.
type recordingEvents struct {
	events []port.ProjectEvent
}

func (r *recordingEvents) PublishProjectEvent(_ context.Context, event port.ProjectEvent) error {
	r.events = append(r.events, event)
	return nil
}

func (r *recordingEvents) types() []string {
	out := make([]string, 0, len(r.events))
	for _, e := range r.events {
		out = append(out, e.Type)
	}
	return out
}

// passThroughTx runs the function without a real transaction and counts the calls.
type passThroughTx struct {
	calls int
}

func (p *passThroughTx) InTx(ctx context.Context, fn func(ctx context.Context) error) error {
	p.calls++
	return fn(ctx)
}

// withLifecycle connects the event recorder and the transaction runner to the fixture service.
func withLifecycle(f *fixture) (*recordingEvents, *passThroughTx) {
	events := &recordingEvents{}
	tx := &passThroughTx{}
	f.svc.WithEvents(events).WithTransactions(tx)
	return events, tx
}

func createFromTemplate(t *testing.T, f *fixture, actor, org uuid.UUID, key string, template domain.Template) *domain.Project {
	t.Helper()
	p, err := f.svc.CreateProject(f.ctx, actor, org, port.CreateInput{
		Key: key, Name: "Project " + key, Template: template, Visibility: domain.VisibilityOpen,
	})
	if err != nil {
		t.Fatalf("CreateProject %s: %v", key, err)
	}
	return p
}

func TestCreateProjectFromTemplateMakesCreatorLead(t *testing.T) {
	f := newFixture(t)
	p := createFromTemplate(t, f, f.owner, f.orgA, "scr", domain.TemplateScrum)

	if p.Key != "SCR" || p.Template != domain.TemplateScrum || p.Type != domain.ProjectTypeScrum {
		t.Fatalf("project = key %q template %q type %q", p.Key, p.Template, p.Type)
	}
	if p.NextIssueNumber != 1 || p.Color != domain.DefaultColor {
		t.Fatalf("next issue %d color %q", p.NextIssueNumber, p.Color)
	}
	role := f.repo.members[p.ID][f.owner].Role
	if role != "lead" {
		t.Fatalf("creator role = %q, want lead", role)
	}
}

func TestCreateProjectInitialMembersAndTeams(t *testing.T) {
	f := newFixture(t)
	teamID := uuid.New()
	p, err := f.svc.CreateProject(f.ctx, f.owner, f.orgA, port.CreateInput{
		Key: "TEAM", Name: "Team", Template: domain.TemplateTaskTracking,
		Visibility: domain.VisibilityPrivate,
		MemberIDs:  []uuid.UUID{f.member, f.member, f.owner},
		TeamIDs:    []uuid.UUID{teamID, teamID},
	})
	if err != nil {
		t.Fatal(err)
	}
	if got := f.repo.members[p.ID][f.member].Role; got != "contributor" {
		t.Fatalf("initial member role = %q, want contributor", got)
	}
	if got := f.repo.members[p.ID][f.owner].Role; got != "lead" {
		t.Fatalf("creator role = %q, want lead (not overwritten by member list)", got)
	}
	if len(f.repo.teamGrants[p.ID]) != 1 {
		t.Fatalf("team grants = %d, want 1 (duplicates dropped)", len(f.repo.teamGrants[p.ID]))
	}
}

func TestCreateProjectRejectsMemberOutsideOrganization(t *testing.T) {
	f := newFixture(t)
	_, err := f.svc.CreateProject(f.ctx, f.owner, f.orgA, port.CreateInput{
		Key: "OUT", Name: "Out", Template: domain.TemplateKanban, MemberIDs: []uuid.UUID{f.outsider},
	})
	wantCode(t, err, "project.user_not_in_org")
}

func TestCreateProjectKeyRules(t *testing.T) {
	f := newFixture(t)
	createFromTemplate(t, f, f.owner, f.orgA, "DUP", domain.TemplateKanban)

	_, err := f.svc.CreateProject(f.ctx, f.owner, f.orgA, port.CreateInput{Key: "dup", Name: "x", Template: domain.TemplateKanban})
	wantCode(t, err, "project.key_taken")

	_, err = f.svc.CreateProject(f.ctx, f.owner, f.orgA, port.CreateInput{Key: "1BAD", Name: "x", Template: domain.TemplateKanban})
	wantCode(t, err, "project.invalid_key")

	// The same key in another organization is free.
	createFromTemplate(t, f, f.outsider, f.orgB, "DUP", domain.TemplateKanban)
}

func TestCreateProjectUnknownTemplateIsRejected(t *testing.T) {
	f := newFixture(t)
	_, err := f.svc.CreateProject(f.ctx, f.owner, f.orgA, port.CreateInput{Key: "TPL", Name: "x", Template: "wiki"})
	wantCode(t, err, "project.invalid_template")
}

func TestKeyAvailableAndDeletedProjectFreesKey(t *testing.T) {
	f := newFixture(t)
	createFromTemplate(t, f, f.owner, f.orgA, "FREE", domain.TemplateKanban)

	available, err := f.svc.KeyAvailable(f.ctx, f.owner, "requester", f.orgA, "free")
	if err != nil || available {
		t.Fatalf("KeyAvailable(taken) = %v, %v; want false", available, err)
	}
	if _, err := f.svc.KeyAvailable(f.ctx, f.owner, "requester", f.orgA, "x"); err == nil {
		t.Fatal("invalid key should fail")
	}
	if _, err := f.svc.KeyAvailable(f.ctx, f.outsider, "requester", f.orgA, "FREE"); err == nil {
		t.Fatal("non member must not query keys of the organization")
	}

	projectID := mustFindKey(t, f, f.orgA, "FREE")
	if err := f.svc.DeleteProject(f.ctx, f.owner, "requester", projectID, "Project FREE"); err != nil {
		t.Fatal(err)
	}
	available, err = f.svc.KeyAvailable(f.ctx, f.owner, "requester", f.orgA, "FREE")
	if err != nil || !available {
		t.Fatalf("KeyAvailable(after delete) = %v, %v; want true", available, err)
	}
	createFromTemplate(t, f, f.owner, f.orgA, "FREE", domain.TemplateKanban)
}

func mustFindKey(t *testing.T, f *fixture, orgID uuid.UUID, key string) uuid.UUID {
	t.Helper()
	for id, p := range f.repo.projects {
		if p.OrganizationID == orgID && p.Key == key && !p.IsDeleted() {
			return id
		}
	}
	t.Fatalf("project %s not found", key)
	return uuid.Nil
}

func TestArchiveRestoreRulesAndEvents(t *testing.T) {
	f := newFixture(t)
	events, _ := withLifecycle(f)
	p := createFromTemplate(t, f, f.owner, f.orgA, "ARC", domain.TemplateKanban)

	archived, err := f.svc.Archive(f.ctx, f.owner, "requester", p.ID)
	if err != nil || !archived.IsArchived() {
		t.Fatalf("Archive = %v, %v", archived, err)
	}
	_, err = f.svc.Archive(f.ctx, f.owner, "requester", p.ID)
	wantCode(t, err, "project.already_archived")

	_, err = f.svc.Update(f.ctx, f.owner, "requester", p.ID, domain.UpdateInput{
		Name: "renamed", Type: domain.ProjectTypeKanban, Visibility: domain.VisibilityOpen, LeadID: f.owner,
	})
	wantCode(t, err, "project.archived")

	restored, err := f.svc.Restore(f.ctx, f.owner, "requester", p.ID)
	if err != nil || restored.IsArchived() {
		t.Fatalf("Restore = %v, %v", restored, err)
	}
	_, err = f.svc.Restore(f.ctx, f.owner, "requester", p.ID)
	wantCode(t, err, "project.not_archived")

	if got := events.types(); len(got) != 2 || got[0] != EventProjectArchived || got[1] != EventProjectRestored {
		t.Fatalf("events = %v", got)
	}
}

func TestArchiveRequiresProjectManagement(t *testing.T) {
	f := newFixture(t)
	p := createFromTemplate(t, f, f.owner, f.orgA, "PRM", domain.TemplateKanban)
	f.addExplicit(t, p.ID, f.member, "viewer")

	_, err := f.svc.Archive(f.ctx, f.member, "requester", p.ID)
	wantError(t, err, apperror.KindForbidden, "")

	// Guests do not see an open project unless they are explicit members; they get not found.
	_, err = f.svc.Archive(f.ctx, f.guest, "requester", p.ID)
	wantError(t, err, apperror.KindNotFound, "project.not_found")
}

func TestDeleteRequiresTypedConfirmation(t *testing.T) {
	f := newFixture(t)
	events, _ := withLifecycle(f)
	p := createFromTemplate(t, f, f.owner, f.orgA, "DEL", domain.TemplateKanban)

	err := f.svc.DeleteProject(f.ctx, f.owner, "requester", p.ID, "DEL")
	wantCode(t, err, "project.confirmation_mismatch")
	if len(events.events) != 0 {
		t.Fatalf("mismatch must not publish events: %v", events.types())
	}

	if err := f.svc.DeleteProject(f.ctx, f.owner, "requester", p.ID, "  Project DEL  "); err != nil {
		t.Fatalf("DeleteProject with the exact name: %v", err)
	}
	_, err = f.svc.Get(f.ctx, f.owner, "requester", p.ID)
	wantError(t, err, apperror.KindNotFound, "project.not_found")
	if got := events.types(); len(got) != 1 || got[0] != EventProjectDeleted {
		t.Fatalf("events = %v", got)
	}
}

func TestDeleteNeedsLeadOrOrgManager(t *testing.T) {
	f := newFixture(t)
	p := createFromTemplate(t, f, f.owner, f.orgA, "NODEL", domain.TemplateKanban)
	f.addExplicit(t, p.ID, f.member, "contributor")

	err := f.svc.DeleteProject(f.ctx, f.member, "requester", p.ID, "Project NODEL")
	wantError(t, err, apperror.KindForbidden, "")

	f.addExplicit(t, p.ID, f.member, "lead")
	if err := f.svc.DeleteProject(f.ctx, f.member, "requester", p.ID, "Project NODEL"); err != nil {
		t.Fatalf("lead delete: %v", err)
	}
}

func TestListProjectsVisibilityAndFilters(t *testing.T) {
	f := newFixture(t)
	open := createFromTemplate(t, f, f.owner, f.orgA, "OPN", domain.TemplateKanban)
	private, err := f.svc.CreateProject(f.ctx, f.owner, f.orgA, port.CreateInput{
		Key: "PRV", Name: "Private", Template: domain.TemplateScrum, Visibility: domain.VisibilityPrivate,
	})
	if err != nil {
		t.Fatal(err)
	}
	f.addExplicit(t, private.ID, f.guest, "viewer")

	page := func(actor uuid.UUID, role string, filter port.ProjectFilter) port.ProjectPage {
		t.Helper()
		result, err := f.svc.ListProjects(f.ctx, actor, role, f.orgA, filter)
		if err != nil {
			t.Fatalf("ListProjects: %v", err)
		}
		return result
	}

	if got := page(f.member, "requester", port.ProjectFilter{}); got.Total != 1 || got.Items[0].ID != open.ID {
		t.Fatalf("member sees %d, want only the open project", got.Total)
	}
	if got := page(f.owner, "requester", port.ProjectFilter{}); got.Total != 2 {
		t.Fatalf("owner sees %d, want 2", got.Total)
	}
	if got := page(f.guest, "requester", port.ProjectFilter{}); got.Total != 1 || got.Items[0].ID != private.ID {
		t.Fatalf("guest sees %d, want only the explicit project", got.Total)
	}
	if got := page(f.owner, "requester", port.ProjectFilter{Template: domain.TemplateScrum}); got.Total != 1 {
		t.Fatalf("template filter = %d, want 1", got.Total)
	}

	if _, err := f.svc.Archive(f.ctx, f.owner, "requester", open.ID); err != nil {
		t.Fatal(err)
	}
	if got := page(f.owner, "requester", port.ProjectFilter{}); got.Total != 1 {
		t.Fatalf("archived excluded by default: got %d", got.Total)
	}
	if got := page(f.owner, "requester", port.ProjectFilter{Archived: port.ArchiveFilterOnly}); got.Total != 1 || got.Items[0].ID != open.ID {
		t.Fatalf("archived only: got %d", got.Total)
	}

	if _, err := f.svc.ListProjects(f.ctx, f.outsider, "requester", f.orgA, port.ProjectFilter{}); err == nil {
		t.Fatal("outsider must not list the organization")
	} else {
		wantError(t, err, apperror.KindNotFound, "organization.not_found")
	}
	_, err = f.svc.ListProjects(f.ctx, f.owner, "requester", f.orgA, port.ProjectFilter{Sort: "password"})
	wantCode(t, err, "project.invalid")
}

func TestUpdateSwitchesScrumAndKanbanOnly(t *testing.T) {
	f := newFixture(t)
	kanban := createFromTemplate(t, f, f.owner, f.orgA, "SWK", domain.TemplateKanban)

	updated, err := f.svc.Update(f.ctx, f.owner, "requester", kanban.ID, domain.UpdateInput{
		Name: "Project SWK", Type: domain.ProjectTypeScrum, Visibility: domain.VisibilityOpen,
		LeadID: f.owner, Color: "Red",
	})
	if err != nil {
		t.Fatal(err)
	}
	if updated.Template != domain.TemplateScrum || updated.Type != domain.ProjectTypeScrum || updated.Color != "red" {
		t.Fatalf("after switch: template %q type %q color %q", updated.Template, updated.Type, updated.Color)
	}

	bugs := createFromTemplate(t, f, f.owner, f.orgA, "BUG", domain.TemplateBugTracking)
	_, err = f.svc.Update(f.ctx, f.owner, "requester", bugs.ID, domain.UpdateInput{
		Name: "Project BUG", Type: domain.ProjectTypeScrum, Visibility: domain.VisibilityOpen, LeadID: f.owner,
	})
	wantCode(t, err, "project.invalid")
}

func TestLegacyCreateStillMapsTypeToTemplate(t *testing.T) {
	f := newFixture(t)
	p, err := f.svc.Create(f.ctx, f.owner, f.orgA, "LEG", "Legacy", "", domain.ProjectTypeScrum, domain.VisibilityOpen)
	if err != nil {
		t.Fatal(err)
	}
	if p.Template != domain.TemplateScrum {
		t.Fatalf("template = %q", p.Template)
	}
}

// wantCode checks only the stable error code.
func wantCode(t *testing.T, err error, code string) {
	t.Helper()
	if err == nil {
		t.Fatalf("got nil error, want code %q", code)
	}
	if got := apperror.CodeOf(err); got != code {
		t.Fatalf("code = %q (%v), want %q", got, err, code)
	}
}
