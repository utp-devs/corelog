package usecase

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"

	authdomain "github.com/jhonsferg/corelog/internal/authorization/domain"
	"github.com/jhonsferg/corelog/internal/issue/application/port"
	"github.com/jhonsferg/corelog/internal/issue/domain"
	"github.com/jhonsferg/corelog/internal/platform/apperror"
	"github.com/jhonsferg/corelog/internal/platform/clock"
	"github.com/jhonsferg/corelog/internal/platform/events"
)

// permFake is a project authorizer and issue permission checker driven by the real default permission scheme.
type permFake struct {
	scheme authdomain.Scheme
	roles  map[uuid.UUID]authdomain.ProjectRole
	admins map[uuid.UUID]bool
}

func newPermFake() *permFake {
	return &permFake{scheme: authdomain.DefaultScheme(), roles: map[uuid.UUID]authdomain.ProjectRole{}, admins: map[uuid.UUID]bool{}}
}

func (p *permFake) roleOf(actor uuid.UUID) (*authdomain.ProjectRole, []authdomain.Holder) {
	if p.admins[actor] {
		lead := authdomain.ProjectRoleLead
		return &lead, []authdomain.Holder{authdomain.HolderOrgAdmin}
	}
	if role, ok := p.roles[actor]; ok {
		return &role, nil
	}
	return nil, nil
}

func (p *permFake) Can(_ context.Context, actor uuid.UUID, _ string, _ uuid.UUID, permission string, holders ...string) (bool, error) {
	role, specials := p.roleOf(actor)
	for _, h := range holders {
		specials = append(specials, authdomain.Holder(h))
	}
	return authdomain.Allowed(p.scheme, authdomain.Permission(permission), role, specials...), nil
}

func (p *permFake) Effective(_ context.Context, actor uuid.UUID, _ string, _ uuid.UUID) ([]string, error) {
	role, specials := p.roleOf(actor)
	var out []string
	for _, permission := range authdomain.Permissions {
		if authdomain.Allowed(p.scheme, permission, role, specials...) {
			out = append(out, string(permission))
		}
	}
	return out, nil
}

func (p *permFake) Authorize(_ context.Context, action string, actor uuid.UUID, _ string, _ uuid.UUID) error {
	permission, ok := authdomain.PermissionForAction(action)
	role, specials := p.roleOf(actor)
	if !ok || !authdomain.Allowed(p.scheme, permission, role, specials...) {
		return apperror.New(apperror.KindForbidden, "forbidden project operation")
	}
	return nil
}

// lifecycleRepo adds soft delete and the hierarchy reader to the in-memory repository.
type lifecycleRepo struct {
	*fakeIssueRepo
	deleted map[uuid.UUID]time.Time
	ranks   int
}

func newLifecycleRepo() *lifecycleRepo {
	return &lifecycleRepo{fakeIssueRepo: newFakeIssueRepo(), deleted: map[uuid.UUID]time.Time{}}
}

func (r *lifecycleRepo) FindByID(ctx context.Context, id uuid.UUID) (*domain.Issue, error) {
	if _, gone := r.deleted[id]; gone {
		return nil, domain.ErrIssueNotFound
	}
	return r.fakeIssueRepo.FindByID(ctx, id)
}

func (r *lifecycleRepo) FindAll(ctx context.Context, filter port.ListFilter) ([]*domain.Issue, error) {
	all, err := r.fakeIssueRepo.FindAll(ctx, filter)
	var live []*domain.Issue
	for _, issue := range all {
		if _, gone := r.deleted[issue.ID]; !gone {
			live = append(live, issue)
		}
	}
	return live, err
}

func (r *lifecycleRepo) SoftDelete(_ context.Context, id uuid.UUID, at time.Time) error {
	if _, gone := r.deleted[id]; gone {
		return domain.ErrIssueNotFound
	}
	r.deleted[id] = at
	return nil
}

func (r *lifecycleRepo) Restore(_ context.Context, id uuid.UUID, _ time.Time) error {
	if _, gone := r.deleted[id]; !gone {
		return domain.ErrIssueNotFound
	}
	delete(r.deleted, id)
	return nil
}

func (r *lifecycleRepo) FindDeleted(ctx context.Context, id uuid.UUID) (*domain.Issue, error) {
	if _, gone := r.deleted[id]; !gone {
		return nil, domain.ErrIssueNotFound
	}
	return r.fakeIssueRepo.FindByID(ctx, id)
}

func (r *lifecycleRepo) LockForUpdate(ctx context.Context, id uuid.UUID) error {
	_, err := r.FindByID(ctx, id)
	return err
}

func (r *lifecycleRepo) NextRank(context.Context, uuid.UUID) (string, error) {
	r.ranks++
	return "r" + string(rune('a'+r.ranks)), nil
}

func (r *lifecycleRepo) ChildLevels(_ context.Context, parentID uuid.UUID) ([]int, error) {
	var levels []int
	for id, issue := range r.issues {
		if _, gone := r.deleted[id]; !gone && issue.ParentID != nil && *issue.ParentID == parentID {
			levels = append(levels, issue.HierarchyLevel())
		}
	}
	return levels, nil
}

type recordedEvents struct{ list []events.Event }

func (r *recordedEvents) Publish(_ context.Context, e events.Event) error {
	r.list = append(r.list, e)
	return nil
}

type patchFixture struct {
	svc    *IssueService
	repo   *lifecycleRepo
	collab *memCollab
	events *recordedEvents
	perms  *permFake
	clock  *clock.FakeClock
	lead   uuid.UUID
	owner  uuid.UUID // contributor who reported the issue
	other  uuid.UUID // contributor who did not
	viewer uuid.UUID
	admin  uuid.UUID
	issue  *domain.Issue
}

func newPatchFixture(t *testing.T) *patchFixture {
	t.Helper()
	f := &patchFixture{
		repo: newLifecycleRepo(), collab: &memCollab{}, events: &recordedEvents{}, perms: newPermFake(),
		clock: &clock.FakeClock{T: time.Date(2026, 11, 27, 10, 0, 0, 0, time.UTC)},
		lead:  uuid.New(), owner: uuid.New(), other: uuid.New(), viewer: uuid.New(), admin: uuid.New(),
	}
	f.perms.roles[f.lead] = authdomain.ProjectRoleLead
	f.perms.roles[f.owner] = authdomain.ProjectRoleContributor
	f.perms.roles[f.other] = authdomain.ProjectRoleContributor
	f.perms.roles[f.viewer] = authdomain.ProjectRoleViewer
	f.perms.admins[f.admin] = true
	dir := newFakeUserDirectory()
	team := uuid.New()
	for _, id := range []uuid.UUID{f.lead, f.owner, f.other, f.viewer, f.admin} {
		dir.userTeams[id] = &team
	}
	f.svc = NewIssueService(f.repo, dir, f.collab, newFakeWatcherRepository(), f.perms).
		WithPermissions(f.perms).WithEvents(f.events).WithClock(f.clock)
	issue, err := f.svc.Create(context.Background(), port.CreateInput{
		Summary: "Original", Description: "Texto", Priority: domain.PriorityLow, ReporterID: f.owner,
	})
	if err != nil {
		t.Fatal(err)
	}
	f.issue = issue
	f.events.list = nil
	f.collab.activities = nil
	return f
}

func date(y int, m time.Month, d int) time.Time { return time.Date(y, m, d, 0, 0, 0, 0, time.UTC) }

func TestPatch_OneBatchAndOneEventForManyFields(t *testing.T) {
	f := newPatchFixture(t)
	ctx := context.Background()
	f.clock.Advance(time.Minute)
	res, err := f.svc.Patch(ctx, f.issue.ID, f.lead, "user", port.PatchInput{
		Summary:     port.Some("Nuevo resumen"),
		DueDate:     port.Some(date(2026, 12, 15)),
		StoryPoints: port.Some(3.5),
		Flagged:     port.Some(true),
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Changes) != 4 {
		t.Fatalf("changes = %+v, want 4", res.Changes)
	}
	if len(f.collab.activities) != 4 {
		t.Fatalf("activities = %d, want 4", len(f.collab.activities))
	}
	for _, activity := range f.collab.activities {
		if activity.BatchID == nil || *activity.BatchID != res.BatchID {
			t.Fatalf("activity batch = %v, want %v", activity.BatchID, res.BatchID)
		}
	}
	if len(f.events.list) != 1 || f.events.list[0].Type != "issue.updated" {
		t.Fatalf("events = %+v, want one issue.updated", f.events.list)
	}
	payload := f.events.list[0].Payload["changes"].([]map[string]any)
	if len(payload) != 4 {
		t.Fatalf("event changes = %d, want 4", len(payload))
	}
	if !res.Issue.UpdatedAt.Equal(f.clock.T) {
		t.Fatalf("updated_at = %v, want the injected clock %v", res.Issue.UpdatedAt, f.clock.T)
	}

	// An identical patch changes nothing and writes nothing.
	again, err := f.svc.Patch(ctx, f.issue.ID, f.lead, "user", port.PatchInput{Summary: port.Some("Nuevo resumen")})
	if err != nil || len(again.Changes) != 0 || len(f.events.list) != 1 || len(f.collab.activities) != 4 {
		t.Fatalf("no-op patch: err=%v changes=%v events=%d activities=%d", err, again.Changes, len(f.events.list), len(f.collab.activities))
	}
}

func TestPatch_PartialUpdateSemantics(t *testing.T) {
	f := newPatchFixture(t)
	ctx := context.Background()
	due := date(2026, 12, 1)
	if _, err := f.svc.Patch(ctx, f.issue.ID, f.lead, "user", port.PatchInput{
		DueDate: port.Some(due), Description: port.Some("Detalle"), OriginalEstimateSeconds: port.Some(int64(3600)),
	}); err != nil {
		t.Fatal(err)
	}
	// Omitted fields stay, explicit null clears.
	res, err := f.svc.Patch(ctx, f.issue.ID, f.lead, "user", port.PatchInput{
		DueDate: port.Null[time.Time](), Description: port.Null[string](),
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.Issue.DueDate != nil || res.Issue.Description != "" {
		t.Fatalf("null did not clear: due=%v description=%q", res.Issue.DueDate, res.Issue.Description)
	}
	if res.Issue.Summary != "Original" || res.Issue.OriginalEstimateSeconds == nil || *res.Issue.OriginalEstimateSeconds != 3600 {
		t.Fatalf("omitted fields changed: %+v", res.Issue)
	}

	invalid := []struct {
		name  string
		input port.PatchInput
		code  string
	}{
		{"null summary", port.PatchInput{Summary: port.Null[string]()}, "issue.field_not_clearable"},
		{"blank summary", port.PatchInput{Summary: port.Some("   ")}, "issue.summary_required"},
		{"long summary", port.PatchInput{Summary: port.Some(string(make([]rune, 256)))}, "issue.summary_too_long"},
		{"points step", port.PatchInput{StoryPoints: port.Some(1.3)}, "issue.invalid_story_points"},
		{"points range", port.PatchInput{StoryPoints: port.Some(1000.0)}, "issue.invalid_story_points"},
		{"negative estimate", port.PatchInput{RemainingEstimateSeconds: port.Some(int64(-1))}, "issue.invalid_estimate"},
		{"start after due", port.PatchInput{StartDate: port.Some(date(2026, 12, 20)), DueDate: port.Some(date(2026, 12, 10))}, "issue.invalid_dates"},
		{"null flagged", port.PatchInput{Flagged: port.Null[bool]()}, "issue.field_not_clearable"},
	}
	for _, tc := range invalid {
		t.Run(tc.name, func(t *testing.T) {
			_, err := f.svc.Patch(ctx, f.issue.ID, f.lead, "user", tc.input)
			if apperror.KindOf(err) != apperror.KindValidation || apperror.CodeOf(err) != tc.code {
				t.Fatalf("err = %v (kind %v code %q), want validation %s", err, apperror.KindOf(err), apperror.CodeOf(err), tc.code)
			}
		})
	}
}

func TestPatch_PermissionMatrix(t *testing.T) {
	f := newPatchFixture(t)
	ctx := context.Background()
	outsider := uuid.New()
	assignee := f.other
	cases := []struct {
		name  string
		actor uuid.UUID
		input port.PatchInput
		want  apperror.Kind // "" means success
	}{
		{"lead edits", f.lead, port.PatchInput{Summary: port.Some("a")}, ""},
		{"owner contributor edits", f.owner, port.PatchInput{Summary: port.Some("b")}, ""},
		{"other contributor edits", f.other, port.PatchInput{Summary: port.Some("c")}, ""},
		{"viewer cannot edit", f.viewer, port.PatchInput{Summary: port.Some("d")}, apperror.KindForbidden},
		{"org admin edits", f.admin, port.PatchInput{Summary: port.Some("e")}, ""},
		{"outsider sees nothing", outsider, port.PatchInput{Summary: port.Some("f")}, apperror.KindNotFound},
		{"lead assigns", f.lead, port.PatchInput{AssigneeID: port.Some(assignee)}, ""},
		{"viewer cannot assign", f.viewer, port.PatchInput{AssigneeID: port.Some(f.lead)}, apperror.KindForbidden},
		{"viewer cannot edit and assign", f.viewer, port.PatchInput{AssigneeID: port.Null[uuid.UUID]()}, apperror.KindForbidden},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tc.input.HideForbidden = true
			_, err := f.svc.Patch(ctx, f.issue.ID, tc.actor, "user", tc.input)
			if tc.want == "" {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				return
			}
			if apperror.KindOf(err) != tc.want {
				t.Fatalf("kind = %v (%v), want %v", apperror.KindOf(err), err, tc.want)
			}
		})
	}

	t.Run("denied field blocks the whole patch", func(t *testing.T) {
		before, _ := f.repo.FindByID(ctx, f.issue.ID)
		_, err := f.svc.Patch(ctx, f.issue.ID, f.viewer, "user", port.PatchInput{Summary: port.Some("zzz")})
		if apperror.KindOf(err) != apperror.KindForbidden {
			t.Fatalf("err = %v", err)
		}
		after, _ := f.repo.FindByID(ctx, f.issue.ID)
		if after.Summary != before.Summary {
			t.Fatal("a denied patch changed the issue")
		}
	})

	t.Run("edit_own_issue lets only the reporter edit when edit_issue is restricted", func(t *testing.T) {
		scheme, err := authdomain.NewScheme([]authdomain.Grant{
			{Permission: authdomain.PermissionBrowseProject, Holders: []authdomain.Holder{authdomain.HolderLead, authdomain.HolderContributor, authdomain.HolderViewer}},
			{Permission: authdomain.PermissionEditIssue, Holders: []authdomain.Holder{authdomain.HolderLead}},
			{Permission: authdomain.PermissionEditOwnIssue, Holders: []authdomain.Holder{authdomain.HolderReporter}},
			{Permission: authdomain.PermissionManageMembers, Holders: []authdomain.Holder{authdomain.HolderLead}},
			{Permission: authdomain.PermissionEditProject, Holders: []authdomain.Holder{authdomain.HolderLead}},
			{Permission: authdomain.PermissionDeleteProject, Holders: []authdomain.Holder{authdomain.HolderLead}},
		})
		if err != nil {
			t.Fatal(err)
		}
		f.perms.scheme = scheme
		defer func() { f.perms.scheme = authdomain.DefaultScheme() }()
		if _, err := f.svc.Patch(ctx, f.issue.ID, f.owner, "user", port.PatchInput{Summary: port.Some("mio")}); err != nil {
			t.Fatalf("reporter denied: %v", err)
		}
		_, err = f.svc.Patch(ctx, f.issue.ID, f.other, "user", port.PatchInput{Summary: port.Some("ajeno")})
		if apperror.KindOf(err) != apperror.KindForbidden {
			t.Fatalf("non reporter err = %v, want forbidden", err)
		}
	})
}

func TestPatch_StaleUpdate(t *testing.T) {
	f := newPatchFixture(t)
	ctx := context.Background()
	stale := f.issue.UpdatedAt.Add(-time.Hour)
	_, err := f.svc.Patch(ctx, f.issue.ID, f.lead, "user", port.PatchInput{ExpectedUpdatedAt: &stale, Summary: port.Some("x")})
	if apperror.KindOf(err) != apperror.KindConflict || apperror.CodeOf(err) != "issue.stale" {
		t.Fatalf("err = %v, want conflict issue.stale", err)
	}
	current := f.issue.UpdatedAt
	f.clock.Advance(time.Second)
	res, err := f.svc.Patch(ctx, f.issue.ID, f.lead, "user", port.PatchInput{ExpectedUpdatedAt: &current, Summary: port.Some("x")})
	if err != nil {
		t.Fatalf("fresh precondition rejected: %v", err)
	}
	// The same precondition is now stale.
	_, err = f.svc.Patch(ctx, f.issue.ID, f.lead, "user", port.PatchInput{ExpectedUpdatedAt: &current, Summary: port.Some("y")})
	if apperror.CodeOf(err) != "issue.stale" {
		t.Fatalf("second use of a stale precondition: %v (issue updated_at %v)", err, res.Issue.UpdatedAt)
	}
}

func TestPatch_HierarchyRules(t *testing.T) {
	f := newPatchFixture(t)
	ctx := context.Background()
	mk := func(kind domain.IssueType, parent *uuid.UUID) (*domain.Issue, error) {
		return f.svc.Create(ctx, port.CreateInput{Summary: "Item", Description: "Desc", Priority: domain.PriorityLow,
			ReporterID: f.lead, IssueType: kind, ParentID: parent})
	}
	epic, err := mk(domain.IssueTypeEpic, nil)
	if err != nil {
		t.Fatal(err)
	}
	story, err := mk(domain.IssueTypeStory, &epic.ID)
	if err != nil {
		t.Fatalf("a standard issue under an epic must be valid: %v", err)
	}
	sub, err := mk(domain.IssueTypeSubtask, &story.ID)
	if err != nil {
		t.Fatalf("a subtask under a standard issue must be valid: %v", err)
	}

	rejected := []struct {
		name string
		do   func() error
		code string
	}{
		{"epic with parent", func() error { _, err := mk(domain.IssueTypeEpic, &epic.ID); return err }, "issue.hierarchy_epic_parent"},
		{"subtask without parent", func() error { _, err := mk(domain.IssueTypeSubtask, nil); return err }, "issue.hierarchy_subtask_parent_required"},
		{"subtask under epic", func() error { _, err := mk(domain.IssueTypeSubtask, &epic.ID); return err }, "issue.hierarchy_subtask_parent_invalid"},
		{"subtask under subtask", func() error { _, err := mk(domain.IssueTypeSubtask, &sub.ID); return err }, "issue.hierarchy_subtask_parent_invalid"},
		{"standard under standard", func() error { _, err := mk(domain.IssueTypeTask, &story.ID); return err }, "issue.hierarchy_standard_parent_invalid"},
		{"missing parent", func() error { missing := uuid.New(); _, err := mk(domain.IssueTypeTask, &missing); return err }, "issue.parent_not_found"},
		{"self parent", func() error {
			_, err := f.svc.Patch(ctx, story.ID, f.lead, "user", port.PatchInput{ParentID: port.Some(story.ID)})
			return err
		}, "issue.hierarchy_cycle"},
		{"epic becomes child", func() error {
			_, err := f.svc.Patch(ctx, epic.ID, f.lead, "user", port.PatchInput{ParentID: port.Some(story.ID)})
			return err
		}, "issue.hierarchy_cycle"},
		{"epic to task keeps standard child", func() error {
			_, err := f.svc.Patch(ctx, epic.ID, f.lead, "user", port.PatchInput{IssueType: port.Some(domain.IssueTypeTask)})
			return err
		}, "issue.hierarchy_children_invalid"},
		{"story under an epic cannot become a subtask", func() error {
			_, err := f.svc.Patch(ctx, story.ID, f.lead, "user", port.PatchInput{IssueType: port.Some(domain.IssueTypeSubtask)})
			return err
		}, "issue.hierarchy_subtask_parent_invalid"},
		{"loose task with a subtask cannot become an epic", func() error {
			loose, err := mk(domain.IssueTypeTask, nil)
			if err != nil {
				return err
			}
			if _, err := mk(domain.IssueTypeSubtask, &loose.ID); err != nil {
				return err
			}
			_, err = f.svc.Patch(ctx, loose.ID, f.lead, "user", port.PatchInput{IssueType: port.Some(domain.IssueTypeEpic)})
			return err
		}, "issue.hierarchy_children_invalid"},
		{"story to epic keeps its parent", func() error {
			_, err := f.svc.Patch(ctx, story.ID, f.lead, "user", port.PatchInput{IssueType: port.Some(domain.IssueTypeEpic)})
			return err
		}, "issue.hierarchy_epic_parent"},
		{"subtask loses its parent", func() error {
			_, err := f.svc.Patch(ctx, sub.ID, f.lead, "user", port.PatchInput{ParentID: port.Null[uuid.UUID]()})
			return err
		}, "issue.hierarchy_subtask_parent_required"},
	}
	for _, tc := range rejected {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.do()
			if apperror.KindOf(err) != apperror.KindValidation || apperror.CodeOf(err) != tc.code {
				t.Fatalf("err = %v (code %q), want %s", err, apperror.CodeOf(err), tc.code)
			}
		})
	}

	t.Run("moving a story to another epic and clearing it are valid", func(t *testing.T) {
		other, err := mk(domain.IssueTypeEpic, nil)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := f.svc.Patch(ctx, story.ID, f.lead, "user", port.PatchInput{ParentID: port.Some(other.ID)}); err != nil {
			t.Fatal(err)
		}
		if _, err := f.svc.Patch(ctx, story.ID, f.lead, "user", port.PatchInput{ParentID: port.Null[uuid.UUID]()}); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("a parent in another project is rejected", func(t *testing.T) {
		foreign, err := f.svc.Create(ctx, port.CreateInput{Summary: "Otro", Description: "Desc", Priority: domain.PriorityLow,
			ReporterID: f.lead, IssueType: domain.IssueTypeEpic, ProjectID: ptr(uuid.New())})
		if err != nil {
			t.Fatal(err)
		}
		_, err = f.svc.Patch(ctx, story.ID, f.lead, "user", port.PatchInput{ParentID: port.Some(foreign.ID)})
		if apperror.CodeOf(err) != "issue.hierarchy_parent_other_project" {
			t.Fatalf("err = %v", err)
		}
	})
}

func ptr[T any](v T) *T { return &v }

func TestDeleteAndRestore(t *testing.T) {
	f := newPatchFixture(t)
	ctx := context.Background()
	if err := f.svc.Delete(ctx, f.issue.ID, f.other, "user"); apperror.KindOf(err) != apperror.KindForbidden {
		t.Fatalf("contributor delete = %v, want forbidden", err)
	}
	if err := f.svc.Delete(ctx, f.issue.ID, uuid.New(), "user"); apperror.KindOf(err) != apperror.KindNotFound {
		t.Fatalf("outsider delete = %v, want not found", err)
	}
	if err := f.svc.Delete(ctx, f.issue.ID, f.lead, "user"); err != nil {
		t.Fatal(err)
	}
	if _, err := f.svc.Get(ctx, f.issue.ID, f.lead, "user"); apperror.KindOf(err) != apperror.KindNotFound {
		t.Fatalf("get after delete = %v, want not found", err)
	}
	if _, err := f.svc.Patch(ctx, f.issue.ID, f.lead, "user", port.PatchInput{Summary: port.Some("x")}); apperror.KindOf(err) != apperror.KindNotFound {
		t.Fatalf("patch after delete = %v, want not found", err)
	}
	if err := f.svc.Delete(ctx, f.issue.ID, f.lead, "user"); apperror.KindOf(err) != apperror.KindNotFound {
		t.Fatalf("second delete = %v, want not found", err)
	}
	f.perms.roles[f.owner] = authdomain.ProjectRoleContributor
	if _, err := f.svc.Restore(ctx, f.issue.ID, f.other, "user"); apperror.KindOf(err) != apperror.KindForbidden {
		t.Fatalf("contributor restore = %v, want forbidden", err)
	}
	restored, err := f.svc.Restore(ctx, f.issue.ID, f.admin, "user")
	if err != nil || restored.ID != f.issue.ID {
		t.Fatalf("restore: %v", err)
	}
	var types []string
	for _, e := range f.events.list {
		types = append(types, e.Type)
	}
	if len(types) != 2 || types[0] != "issue.deleted" || types[1] != "issue.restored" {
		t.Fatalf("events = %v", types)
	}
}

type viewReaderFake struct{}

func (viewReaderFake) ViewExtras(context.Context, uuid.UUID) (domain.IssueCounts, domain.Breadcrumb, error) {
	return domain.IssueCounts{Comments: 2, Children: 3, Watchers: 1}, domain.Breadcrumb{ProjectKey: "CORE", ProjectName: "CoreLog",
		Parent: &domain.ParentRef{Key: "CORE-1", Summary: "Epica", Type: "epic"}}, nil
}

func TestGetView(t *testing.T) {
	f := newPatchFixture(t)
	ctx := context.Background()
	f.svc.viewReader = viewReaderFake{}

	lead, err := f.svc.GetView(ctx, f.issue.ID, f.lead, "user")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"edit_issue", "assign_issue", "comment_issue", "delete_issue"} {
		if !contains(lead.Permissions, want) {
			t.Errorf("lead permissions %v miss %s", lead.Permissions, want)
		}
	}
	if lead.Counts.Comments != 2 || lead.Counts.Children != 3 || lead.Breadcrumb.Parent == nil || lead.Breadcrumb.Parent.Key != "CORE-1" {
		t.Fatalf("view extras = %+v %+v", lead.Counts, lead.Breadcrumb)
	}
	if len(lead.Transitions) == 0 || lead.Transitions[0].To != domain.StatusInProgress {
		t.Fatalf("transitions = %+v, want the legacy open transitions", lead.Transitions)
	}

	viewer, err := f.svc.GetView(ctx, f.issue.ID, f.viewer, "user")
	if err != nil {
		t.Fatal(err)
	}
	if contains(viewer.Permissions, "edit_issue") || contains(viewer.Permissions, "delete_issue") || len(viewer.Transitions) != 0 {
		t.Fatalf("viewer view = %v %v", viewer.Permissions, viewer.Transitions)
	}

	owner, err := f.svc.GetView(ctx, f.issue.ID, f.owner, "user")
	if err != nil || !contains(owner.Permissions, "edit_issue") || contains(owner.Permissions, "delete_issue") {
		t.Fatalf("owner permissions = %v err=%v", owner.Permissions, err)
	}
	if _, err := f.svc.GetView(ctx, f.issue.ID, uuid.New(), "user"); apperror.KindOf(err) != apperror.KindForbidden {
		t.Fatalf("outsider view = %v", err)
	}
}

func contains(list []string, want string) bool {
	for _, item := range list {
		if item == want {
			return true
		}
	}
	return false
}

type projectDefaultsFake struct{ lead uuid.UUID }

func (d projectDefaultsFake) DefaultAssigneePolicy(context.Context, uuid.UUID) (string, *uuid.UUID, error) {
	return "lead", &d.lead, nil
}

type requiredFake struct{ keys []string }

func (r requiredFake) RequiredSystemFields(context.Context, uuid.UUID, uuid.UUID) ([]string, error) {
	return r.keys, nil
}

func TestCreateDefaultsWatchersRankAndRequiredFields(t *testing.T) {
	f := newPatchFixture(t)
	ctx := context.Background()
	watchers := newFakeWatcherRepository()
	svc := NewIssueService(f.repo, newFakeUserDirectory(), f.collab, watchers, f.perms).
		WithPermissions(f.perms).WithEvents(f.events).WithClock(f.clock).
		WithProjectDefaults(projectDefaultsFake{lead: f.lead}).
		WithRequiredFields(requiredFake{keys: []string{"summary", "due_date"}})

	input := port.CreateInput{Summary: "Con politica", Description: "Desc", Priority: domain.PriorityLow, ReporterID: f.owner}
	input.Provided = map[string]bool{"summary": true, "description": true}
	if _, err := svc.Create(ctx, input); apperror.KindOf(err) != apperror.KindValidation || apperror.CodeOf(err) != "validation.failed" {
		t.Fatalf("missing required due_date: %v", err)
	}

	due := date(2026, 12, 31)
	input.DueDate = &due
	input.Provided["due_date"] = true
	created, err := svc.Create(ctx, input)
	if err != nil {
		t.Fatal(err)
	}
	if created.AssigneeID == nil || *created.AssigneeID != f.lead {
		t.Fatalf("assignee = %v, want the project lead", created.AssigneeID)
	}
	if created.Rank == "" || created.Rank == "m" {
		t.Fatalf("rank = %q, want one allocated at the end", created.Rank)
	}
	for _, user := range []uuid.UUID{f.owner, f.lead} {
		if state, _ := watchers.State(ctx, created.ID, user); !state.IsWatching {
			t.Fatalf("user %v does not watch the new issue", user)
		}
	}
	if len(f.events.list) == 0 || f.events.list[len(f.events.list)-1].Type != "issue.created" {
		t.Fatalf("events = %+v", f.events.list)
	}

	t.Run("choosing an assignee needs assign_issue", func(t *testing.T) {
		in := port.CreateInput{Summary: "Asignada", Description: "Desc", Priority: domain.PriorityLow, ReporterID: f.viewer, AssigneeID: &f.lead}
		if _, err := svc.Create(ctx, in); apperror.KindOf(err) == "" {
			t.Fatal("expected an error: a viewer cannot create issues")
		}
	})
}

func TestListExcludesDeleted(t *testing.T) {
	f := newPatchFixture(t)
	ctx := context.Background()
	f.svc.projectRoles = nil
	if err := f.svc.Delete(ctx, f.issue.ID, f.lead, "user"); err != nil {
		t.Fatal(err)
	}
	res, err := f.svc.List(ctx, f.lead, "admin", port.ListFilter{})
	if err != nil {
		t.Fatal(err)
	}
	for _, item := range res.Items {
		if item.ID == f.issue.ID {
			t.Fatal("deleted issue listed")
		}
	}
}
