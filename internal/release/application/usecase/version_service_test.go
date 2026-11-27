package usecase

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/jhonsferg/corelog/internal/platform/apperror"
	"github.com/jhonsferg/corelog/internal/platform/clock"
	"github.com/jhonsferg/corelog/internal/platform/events"
	"github.com/jhonsferg/corelog/internal/release/application/port"
	"github.com/jhonsferg/corelog/internal/release/domain"
)

var fixedNow = time.Date(2026, 11, 27, 9, 0, 0, 0, time.UTC)

// fakeRepo keeps versions and issue links in memory. Issue categories are set per issue by the test.
type fakeRepo struct {
	versions  map[uuid.UUID]*domain.Version
	links     map[[3]uuid.UUID]bool // issue, version, kind (kind as uuid-less key via kindKey)
	linkKinds map[linkKey]bool
	issues    map[uuid.UUID]issueRow
}

type linkKey struct {
	issue, version uuid.UUID
	kind           string
}

type issueRow struct {
	project  uuid.UUID
	category string
	points   float64
	typeName string
	key      string
	summary  string
}

func newFakeRepo() *fakeRepo {
	return &fakeRepo{versions: map[uuid.UUID]*domain.Version{}, linkKinds: map[linkKey]bool{}, issues: map[uuid.UUID]issueRow{}}
}

func (r *fakeRepo) link(issue, version uuid.UUID, kind string) {
	r.linkKinds[linkKey{issue, version, kind}] = true
}

func (r *fakeRepo) has(issue, version uuid.UUID, kind string) bool {
	return r.linkKinds[linkKey{issue, version, kind}]
}

func (r *fakeRepo) add(v *domain.Version) {
	v.ID = uuid.New()
	r.versions[v.ID] = v
}

func (r *fakeRepo) Create(_ context.Context, v *domain.Version) error {
	for _, other := range r.versions {
		if other.ProjectID == v.ProjectID && strings.EqualFold(other.Name, v.Name) {
			return domain.ErrVersionNameTaken
		}
	}
	r.add(v)
	return nil
}

func (r *fakeRepo) Save(_ context.Context, v *domain.Version) error {
	if _, ok := r.versions[v.ID]; !ok {
		return domain.ErrVersionNotFound
	}
	copied := *v
	r.versions[v.ID] = &copied
	return nil
}

func (r *fakeRepo) FindByID(_ context.Context, id uuid.UUID) (*domain.Version, error) {
	v, ok := r.versions[id]
	if !ok {
		return nil, domain.ErrVersionNotFound
	}
	copied := *v
	return &copied, nil
}

func (r *fakeRepo) LockByID(ctx context.Context, id uuid.UUID) (*domain.Version, error) {
	return r.FindByID(ctx, id)
}

func (r *fakeRepo) List(_ context.Context, projectID uuid.UUID, status *domain.Status) ([]*domain.Version, error) {
	out := []*domain.Version{}
	for _, v := range r.versions {
		if v.ProjectID != projectID || (status != nil && v.Status != *status) {
			continue
		}
		copied := *v
		out = append(out, &copied)
	}
	return out, nil
}

func (r *fakeRepo) Progress(_ context.Context, ids []uuid.UUID) (map[uuid.UUID][]domain.CategoryCount, error) {
	out := map[uuid.UUID][]domain.CategoryCount{}
	for key := range r.linkKinds {
		if key.kind != "fix" || !contains(ids, key.version) {
			continue
		}
		row := r.issues[key.issue]
		out[key.version] = append(out[key.version], domain.CategoryCount{Category: row.category, Issues: 1, Points: row.points})
	}
	return out, nil
}

func contains(ids []uuid.UUID, id uuid.UUID) bool {
	for _, v := range ids {
		if v == id {
			return true
		}
	}
	return false
}

func (r *fakeRepo) NameTaken(_ context.Context, projectID uuid.UUID, name string, except uuid.UUID) (bool, error) {
	for id, v := range r.versions {
		if id != except && v.ProjectID == projectID && strings.EqualFold(v.Name, name) {
			return true, nil
		}
	}
	return false, nil
}

func (r *fakeRepo) Delete(_ context.Context, id uuid.UUID) error {
	if _, ok := r.versions[id]; !ok {
		return domain.ErrVersionNotFound
	}
	delete(r.versions, id)
	for key := range r.linkKinds {
		if key.version == id {
			delete(r.linkKinds, key)
		}
	}
	return nil
}

func (r *fakeRepo) MoveLinks(_ context.Context, from, to uuid.UUID) error {
	for key := range r.linkKinds {
		if key.version == from {
			r.linkKinds[linkKey{key.issue, to, key.kind}] = true
		}
	}
	return nil
}

func (r *fakeRepo) unresolved(version uuid.UUID) []uuid.UUID {
	var out []uuid.UUID
	for key := range r.linkKinds {
		if key.version == version && key.kind == "fix" && r.issues[key.issue].category != domain.CategoryDone {
			out = append(out, key.issue)
		}
	}
	return out
}

func (r *fakeRepo) UnresolvedFixCount(_ context.Context, id uuid.UUID) (int, error) {
	return len(r.unresolved(id)), nil
}

func (r *fakeRepo) MoveUnresolvedFixes(_ context.Context, from, to uuid.UUID) (int, error) {
	moved := r.unresolved(from)
	for _, issue := range moved {
		r.linkKinds[linkKey{issue, to, "fix"}] = true
		delete(r.linkKinds, linkKey{issue, from, "fix"})
	}
	return len(moved), nil
}

func (r *fakeRepo) NotesIssues(_ context.Context, id uuid.UUID) ([]domain.NoteIssue, error) {
	out := []domain.NoteIssue{}
	for key := range r.linkKinds {
		if key.version != id || key.kind != "fix" {
			continue
		}
		row := r.issues[key.issue]
		out = append(out, domain.NoteIssue{ID: key.issue, Key: row.key, Summary: row.summary, TypeName: row.typeName, Done: row.category == domain.CategoryDone})
	}
	return out, nil
}

// fakeAuth grants browse to every member and manage to leads. Unknown actors are outsiders.
type fakeAuth struct {
	roles    map[uuid.UUID]map[uuid.UUID]string // project -> actor -> role
	archived map[uuid.UUID]bool
}

func (a fakeAuth) Authorize(_ context.Context, action string, actor uuid.UUID, _ string, project uuid.UUID) error {
	role, member := a.roles[project][actor]
	if !member {
		return apperror.New(apperror.KindForbidden, "forbidden")
	}
	switch action {
	case port.ActionViewProject:
		return nil
	case port.ActionManageVersions:
		if a.archived[project] {
			return apperror.WithCode(apperror.KindConflict, "project.archived", "archived")
		}
		if role != "lead" {
			return apperror.New(apperror.KindForbidden, "forbidden")
		}
		return nil
	}
	return apperror.New(apperror.KindForbidden, "unknown action")
}

type fixture struct {
	svc      *VersionService
	repo     *fakeRepo
	auth     fakeAuth
	events   *events.MemoryPublisher
	project  uuid.UUID
	other    uuid.UUID
	lead     uuid.UUID
	contrib  uuid.UUID
	viewer   uuid.UUID
	outsider uuid.UUID
}

func newFixture(t *testing.T) *fixture {
	t.Helper()
	f := &fixture{
		repo: newFakeRepo(), project: uuid.New(), other: uuid.New(),
		lead: uuid.New(), contrib: uuid.New(), viewer: uuid.New(), outsider: uuid.New(),
		events: events.NewMemoryPublisher(),
	}
	f.auth = fakeAuth{roles: map[uuid.UUID]map[uuid.UUID]string{
		f.project: {f.lead: "lead", f.contrib: "contributor", f.viewer: "viewer"},
		f.other:   {f.lead: "lead"},
	}, archived: map[uuid.UUID]bool{}}
	f.svc = NewVersionService(f.repo, f.auth, passTx{}, f.events, clock.NewFixed(fixedNow))
	return f
}

type passTx struct{}

func (passTx) InTx(ctx context.Context, fn func(ctx context.Context) error) error { return fn(ctx) }

func (f *fixture) issue(version uuid.UUID, kind, category string, points float64, key string) uuid.UUID {
	id := uuid.New()
	f.repo.issues[id] = issueRow{project: f.project, category: category, points: points, typeName: "Historia", key: key, summary: "Resumen " + key}
	f.repo.link(id, version, kind)
	return id
}

func (f *fixture) version(t *testing.T, name string) *domain.Version {
	t.Helper()
	v, err := f.svc.Create(context.Background(), f.lead, "user", f.project, port.CreateInput{Name: name})
	if err != nil {
		t.Fatalf("create %s: %v", name, err)
	}
	return v
}

func errCode(err error) string { return apperror.CodeOf(err) }

func TestCreateValidatesAndRespectsRoles(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	if _, err := f.svc.Create(ctx, f.outsider, "user", f.project, port.CreateInput{Name: "1.0"}); apperror.KindOf(err) != apperror.KindNotFound {
		t.Fatalf("outsider create: %v", err)
	}
	if _, err := f.svc.Create(ctx, f.viewer, "user", f.project, port.CreateInput{Name: "1.0"}); apperror.KindOf(err) != apperror.KindForbidden {
		t.Fatalf("viewer create: %v", err)
	}
	if _, err := f.svc.Create(ctx, f.contrib, "user", f.project, port.CreateInput{Name: "1.0"}); apperror.KindOf(err) != apperror.KindForbidden {
		t.Fatalf("contributor create: %v", err)
	}
	f.version(t, "1.0")
	if _, err := f.svc.Create(ctx, f.lead, "user", f.project, port.CreateInput{Name: " 1.0 "}); errCode(err) != "version.name_taken" {
		t.Fatalf("duplicate name: %v", err)
	}
	if _, err := f.svc.Create(ctx, f.lead, "user", f.project, port.CreateInput{Name: ""}); errCode(err) != "version.invalid" {
		t.Fatalf("empty name: %v", err)
	}
}

func TestReadsNeedBrowseAndHideOutsiders(t *testing.T) {
	f := newFixture(t)
	v := f.version(t, "1.0")
	ctx := context.Background()
	if _, err := f.svc.Get(ctx, f.viewer, "user", v.ID); err != nil {
		t.Fatalf("viewer read: %v", err)
	}
	if _, err := f.svc.Get(ctx, f.outsider, "user", v.ID); errCode(err) != "version.not_found" {
		t.Fatalf("outsider read should be 404, got %v", err)
	}
	if _, err := f.svc.List(ctx, f.outsider, "user", f.project, port.ListFilter{}); errCode(err) != "version.not_found" {
		t.Fatalf("outsider list should be 404, got %v", err)
	}
	if _, err := f.svc.Get(ctx, f.lead, "user", uuid.New()); errCode(err) != "version.not_found" {
		t.Fatalf("missing version: %v", err)
	}
}

func TestArchivedProjectRejectsWrites(t *testing.T) {
	f := newFixture(t)
	v := f.version(t, "1.0")
	f.auth.archived[f.project] = true
	_, err := f.svc.Update(context.Background(), f.lead, "user", v.ID, domain.VersionPatch{Name: strPtr("x")})
	if apperror.KindOf(err) != apperror.KindConflict || errCode(err) != "project.archived" {
		t.Fatalf("write on archived project: %v", err)
	}
}

func strPtr(s string) *string { return &s }

func TestReleaseLifecycleAndProgress(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	v := f.version(t, "1.0")
	f.issue(v.ID, "fix", domain.CategoryDone, 3, "CORE-1")
	f.issue(v.ID, "fix", domain.CategoryTodo, 5, "CORE-2")

	got, err := f.svc.Get(ctx, f.viewer, "user", v.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Progress.IssuesTotal != 2 || got.Progress.IssuesDone != 1 || got.Progress.PointsTotal != 8 {
		t.Fatalf("progress %+v", got.Progress)
	}

	res, err := f.svc.Release(ctx, f.lead, "user", v.ID, port.ReleaseInput{Move: port.MoveTarget{Mode: MoveNone}})
	if err != nil {
		t.Fatal(err)
	}
	if res.Version.Status != domain.StatusReleased || res.UnresolvedIssues != 1 || res.MovedIssues != 0 {
		t.Fatalf("release result %+v", res)
	}
	if _, err := f.svc.Release(ctx, f.lead, "user", v.ID, port.ReleaseInput{}); errCode(err) != "version.invalid_transition" {
		t.Fatalf("second release: %v", err)
	}
	if _, err := f.svc.Archive(ctx, f.lead, "user", v.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := f.svc.Archive(ctx, f.lead, "user", v.ID); errCode(err) != "version.archived" {
		t.Fatalf("archive twice: %v", err)
	}
	if n := len(f.events.Events()); n != 1 {
		t.Fatalf("released events = %d, want 1", n)
	}
}

func TestReleaseMovesUnresolvedToNewVersion(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	v := f.version(t, "2.0")
	done := f.issue(v.ID, "fix", domain.CategoryDone, 1, "CORE-1")
	open := f.issue(v.ID, "fix", domain.CategoryInProgress, 2, "CORE-2")
	affects := f.issue(v.ID, "affects", domain.CategoryTodo, 0, "CORE-3")

	res, err := f.svc.Release(ctx, f.lead, "user", v.ID, port.ReleaseInput{Move: port.MoveTarget{Mode: MoveNew}})
	if err != nil {
		t.Fatal(err)
	}
	if res.NextVersion == nil || res.NextVersion.Name != "Siguiente a 2.0" || res.NextVersion.Status != domain.StatusUnreleased {
		t.Fatalf("next version %+v", res.NextVersion)
	}
	if res.MovedIssues != 1 || res.UnresolvedIssues != 0 {
		t.Fatalf("moved %d unresolved %d", res.MovedIssues, res.UnresolvedIssues)
	}
	if !f.repo.has(open, res.NextVersion.ID, "fix") || f.repo.has(open, v.ID, "fix") {
		t.Fatal("open issue not moved")
	}
	if f.repo.has(done, res.NextVersion.ID, "fix") || !f.repo.has(done, v.ID, "fix") {
		t.Fatal("done issue must stay in the released version")
	}
	if !f.repo.has(affects, v.ID, "affects") {
		t.Fatal("affects links stay")
	}
}

func TestReleaseMovesToExistingTargetAndRejectsBadTargets(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	source := f.version(t, "3.0")
	target := f.version(t, "3.1")
	released := f.version(t, "2.9")
	open := f.issue(source.ID, "fix", domain.CategoryTodo, 1, "CORE-5")
	if _, err := f.svc.Release(ctx, f.lead, "user", released.ID, port.ReleaseInput{}); err != nil {
		t.Fatal(err)
	}

	// Targets from another project, the source itself or a released version are refused.
	otherProject := uuid.New()
	f.auth.roles[otherProject] = map[uuid.UUID]string{f.lead: "lead"}
	foreign, _ := f.svc.Create(ctx, f.lead, "user", otherProject, port.CreateInput{Name: "x"})
	for _, bad := range []uuid.UUID{foreign.ID, source.ID, released.ID, uuid.New()} {
		_, err := f.svc.Release(ctx, f.lead, "user", source.ID, port.ReleaseInput{Move: port.MoveTarget{Mode: MoveVersion, VersionID: bad}})
		if errCode(err) != "version.move_target_invalid" {
			t.Fatalf("target %s: %v", bad, err)
		}
	}
	if v, _ := f.repo.FindByID(ctx, source.ID); v.Status != domain.StatusUnreleased {
		t.Fatal("failed release must not change the version")
	}

	res, err := f.svc.Release(ctx, f.lead, "user", source.ID, port.ReleaseInput{Move: port.MoveTarget{Mode: MoveVersion, VersionID: target.ID}})
	if err != nil {
		t.Fatal(err)
	}
	if res.MovedIssues != 1 || !f.repo.has(open, target.ID, "fix") {
		t.Fatalf("moved %d", res.MovedIssues)
	}
}

func TestReleaseWithUnknownMoveMode(t *testing.T) {
	f := newFixture(t)
	v := f.version(t, "4.0")
	_, err := f.svc.Release(context.Background(), f.lead, "user", v.ID, port.ReleaseInput{Move: port.MoveTarget{Mode: "bogus"}})
	if errCode(err) != "version.move_target_invalid" {
		t.Fatalf("code %v", err)
	}
}

func TestDeleteMovesLinksWhenAsked(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	doomed := f.version(t, "5.0")
	keep := f.version(t, "5.1")
	issue := f.issue(doomed.ID, "fix", domain.CategoryTodo, 1, "CORE-7")
	if err := f.svc.Delete(ctx, f.viewer, "user", doomed.ID, nil); apperror.KindOf(err) != apperror.KindForbidden {
		t.Fatalf("viewer delete: %v", err)
	}
	if err := f.svc.Delete(ctx, f.lead, "user", doomed.ID, &keep.ID); err != nil {
		t.Fatal(err)
	}
	if !f.repo.has(issue, keep.ID, "fix") {
		t.Fatal("links not moved before delete")
	}
	if err := f.svc.Delete(ctx, f.lead, "user", doomed.ID, nil); errCode(err) != "version.not_found" {
		t.Fatalf("second delete: %v", err)
	}
}

func TestUpdateRenameConflictAndUnrelease(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	a := f.version(t, "6.0")
	f.version(t, "6.1")
	if _, err := f.svc.Update(ctx, f.lead, "user", a.ID, domain.VersionPatch{Name: strPtr("6.1")}); errCode(err) != "version.name_taken" {
		t.Fatalf("rename onto taken name: %v", err)
	}
	if _, err := f.svc.Release(ctx, f.lead, "user", a.ID, port.ReleaseInput{}); err != nil {
		t.Fatal(err)
	}
	reopen := domain.StatusUnreleased
	got, err := f.svc.Update(ctx, f.lead, "user", a.ID, domain.VersionPatch{Status: &reopen})
	if err != nil || got.Status != domain.StatusUnreleased {
		t.Fatalf("unrelease: %v %+v", err, got)
	}
}

func TestNotesGroupFixIssuesOnly(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	v := f.version(t, "7.0")
	f.issue(v.ID, "fix", domain.CategoryDone, 1, "CORE-10")
	f.issue(v.ID, "affects", domain.CategoryTodo, 0, "CORE-11")
	notes, err := f.svc.Notes(ctx, f.viewer, "user", v.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(notes.Groups) != 1 || len(notes.Groups[0].Issues) != 1 || notes.Groups[0].Issues[0].Key != "CORE-10" {
		t.Fatalf("groups %+v", notes.Groups)
	}
	if !strings.Contains(notes.Markdown, "- CORE-10 Resumen CORE-10") {
		t.Fatalf("markdown %q", notes.Markdown)
	}
	if _, err := f.svc.Notes(ctx, f.outsider, "user", v.ID); errCode(err) != "version.not_found" {
		t.Fatalf("outsider notes: %v", err)
	}
}

func TestListFilterRejectsUnknownStatus(t *testing.T) {
	f := newFixture(t)
	bad := domain.Status("nope")
	if _, err := f.svc.List(context.Background(), f.viewer, "user", f.project, port.ListFilter{Status: &bad}); errCode(err) != "version.status_invalid" {
		t.Fatalf("code %v", err)
	}
}
