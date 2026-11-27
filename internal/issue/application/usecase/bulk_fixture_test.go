package usecase

import (
	"context"
	"fmt"
	"sort"
	"testing"
	"time"

	"github.com/google/uuid"

	authdomain "github.com/jhonsferg/corelog/internal/authorization/domain"
	"github.com/jhonsferg/corelog/internal/issue/application/port"
	"github.com/jhonsferg/corelog/internal/issue/domain"
	"github.com/jhonsferg/corelog/internal/platform/clock"
)

// projectGate wraps the permission fake and denies edit_issue on the projects listed in denyEdit.
type projectGate struct {
	*permFake
	denyEdit map[uuid.UUID]bool
}

func (g projectGate) Can(ctx context.Context, actor uuid.UUID, role string, projectID uuid.UUID, permission string, holders ...string) (bool, error) {
	if permission == "edit_issue" && g.denyEdit[projectID] {
		return false, nil
	}
	return g.permFake.Can(ctx, actor, role, projectID, permission, holders...)
}

// opsRepo adds the child reader to the lifecycle repository.
type opsRepo struct {
	*lifecycleRepo
}

func (r *opsRepo) ListChildren(_ context.Context, parentID uuid.UUID) ([]*domain.Issue, error) {
	var out []*domain.Issue
	for id, issue := range r.issues {
		if _, gone := r.deleted[id]; !gone && issue.ParentID != nil && *issue.ParentID == parentID {
			out = append(out, issue)
		}
	}
	return out, nil
}

// opsFake is the operations out-port in memory: keys of moved issues, labels and copies.
type opsFake struct {
	projectKeys  map[uuid.UUID]string
	nextNumber   map[uuid.UUID]int
	aliases      map[string]uuid.UUID
	labelNames   map[uuid.UUID]string
	labelProject map[uuid.UUID]uuid.UUID
	issueLabels  map[uuid.UUID]map[uuid.UUID]bool
	copies       int
}

func newOpsFake() *opsFake {
	return &opsFake{
		projectKeys:  map[uuid.UUID]string{},
		nextNumber:   map[uuid.UUID]int{},
		aliases:      map[string]uuid.UUID{},
		labelNames:   map[uuid.UUID]string{},
		labelProject: map[uuid.UUID]uuid.UUID{},
		issueLabels:  map[uuid.UUID]map[uuid.UUID]bool{},
	}
}

var _ port.OperationsRepository = (*opsFake)(nil)

func (f *opsFake) Relocate(_ context.Context, issueID, sourceProjectID uuid.UUID, sourceNumber int, target uuid.UUID) (port.RelocateResult, error) {
	if f.nextNumber[target] == 0 {
		f.nextNumber[target] = 1000
	}
	f.nextNumber[target]++
	f.aliases[fmt.Sprintf("%s-%d", f.projectKeys[sourceProjectID], sourceNumber)] = issueID
	dropped := 0
	for label := range f.issueLabels[issueID] {
		if f.labelProject[label] != target {
			delete(f.issueLabels[issueID], label)
			dropped++
		}
	}
	return port.RelocateResult{KeyNumber: f.nextNumber[target], ProjectKey: f.projectKeys[target], DroppedLabels: dropped}, nil
}

func (f *opsFake) FindAliasIssues(_ context.Context, projectKey string, number int, _ uuid.UUID) ([]uuid.UUID, error) {
	if id, ok := f.aliases[fmt.Sprintf("%s-%d", projectKey, number)]; ok {
		return []uuid.UUID{id}, nil
	}
	return nil, nil
}

func (f *opsFake) CopyWatchers(context.Context, uuid.UUID, uuid.UUID) error {
	f.copies++
	return nil
}

func (f *opsFake) CopyProjectScoped(context.Context, uuid.UUID, uuid.UUID) error {
	f.copies++
	return nil
}

func (f *opsFake) CopyLinks(context.Context, uuid.UUID, uuid.UUID, uuid.UUID) error {
	f.copies++
	return nil
}

func (f *opsFake) ProjectLabelIDs(_ context.Context, projectID uuid.UUID, ids []uuid.UUID) (map[uuid.UUID]string, error) {
	out := map[uuid.UUID]string{}
	for _, id := range ids {
		if f.labelProject[id] == projectID {
			out[id] = f.labelNames[id]
		}
	}
	return out, nil
}

func (f *opsFake) IssueLabelNames(_ context.Context, issueID uuid.UUID) ([]string, error) {
	var names []string
	for label := range f.issueLabels[issueID] {
		names = append(names, f.labelNames[label])
	}
	sort.Strings(names)
	return names, nil
}

func (f *opsFake) AddIssueLabels(_ context.Context, issueID uuid.UUID, ids []uuid.UUID) error {
	if f.issueLabels[issueID] == nil {
		f.issueLabels[issueID] = map[uuid.UUID]bool{}
	}
	for _, id := range ids {
		f.issueLabels[issueID][id] = true
	}
	return nil
}

func (f *opsFake) RemoveIssueLabels(_ context.Context, issueID uuid.UUID, ids []uuid.UUID) error {
	for _, id := range ids {
		delete(f.issueLabels[issueID], id)
	}
	return nil
}

func (f *opsFake) LabelNamesByIssue(_ context.Context, ids []uuid.UUID) (map[uuid.UUID][]string, error) {
	out := map[uuid.UUID][]string{}
	for _, id := range ids {
		names, _ := f.IssueLabelNames(context.Background(), id)
		if len(names) > 0 {
			out[id] = names
		}
	}
	return out, nil
}

// opsFixture is a service with two projects in one organization, a catalog and the operations fakes.
type opsFixture struct {
	svc      *IssueService
	repo     *opsRepo
	ops      *opsFake
	events   *recordedEvents
	collab   *memCollab
	links    *linkFakeRepo
	perms    *permFake
	gate     projectGate
	catalog  *memCatalog
	clock    *clock.FakeClock
	org      uuid.UUID
	projA    uuid.UUID
	projB    uuid.UUID
	lead     uuid.UUID
	owner    uuid.UUID
	other    uuid.UUID
	viewer   uuid.UUID
	stranger uuid.UUID
	task     uuid.UUID
	epic     uuid.UUID
	story    uuid.UUID
	subtask  uuid.UUID
	medium   uuid.UUID
	high     uuid.UUID
}

func newOpsFixture(t *testing.T) *opsFixture {
	t.Helper()
	f := &opsFixture{
		repo:   &opsRepo{lifecycleRepo: newLifecycleRepo()},
		ops:    newOpsFake(),
		events: &recordedEvents{},
		collab: &memCollab{},
		links:  newLinkFakeRepo(),
		perms:  newPermFake(),
		clock:  &clock.FakeClock{T: time.Date(2026, 11, 27, 10, 0, 0, 0, time.UTC)},
		org:    uuid.New(), projA: uuid.New(), projB: uuid.New(),
		lead: uuid.New(), owner: uuid.New(), other: uuid.New(), viewer: uuid.New(), stranger: uuid.New(),
	}
	f.perms.roles[f.lead] = authdomain.ProjectRoleLead
	f.perms.roles[f.owner] = authdomain.ProjectRoleContributor
	f.perms.roles[f.other] = authdomain.ProjectRoleContributor
	f.perms.roles[f.viewer] = authdomain.ProjectRoleViewer
	f.ops.projectKeys[f.projA] = "CORE"
	f.ops.projectKeys[f.projB] = "OTHR"
	f.gate = projectGate{permFake: f.perms, denyEdit: map[uuid.UUID]bool{}}

	dir := newFakeUserDirectory()
	team := uuid.New()
	for _, id := range []uuid.UUID{f.lead, f.owner, f.other, f.viewer} {
		dir.userTeams[id] = &team
	}
	f.catalog = f.seedCatalog()
	f.svc = NewIssueService(f.repo, dir, f.collab, newFakeWatcherRepository(), f.links, f.gate).
		WithPermissions(f.gate).WithEvents(f.events).WithClock(f.clock).WithCatalog(f.catalog)
	f.svc.WithOperations(f.ops, nil, nil)
	return f
}

// seedCatalog gives the organization three issue types and two priorities. Project B enables only the task and
// the story, so an epic or a subtask moved there needs a mapping.
func (f *opsFixture) seedCatalog() *memCatalog {
	cat := newMemCatalog()
	cat.projects[f.projA] = f.org
	cat.projects[f.projB] = f.org
	add := func(kind domain.CatalogKind, name, legacy string, level int, isDefault bool) uuid.UUID {
		item := &domain.CatalogItem{ID: uuid.New(), OrganizationID: f.org, Kind: kind, Name: name, LegacyKey: legacy, HierarchyLevel: level, IsDefault: isDefault}
		cat.items[item.ID] = item
		return item.ID
	}
	f.task = add(domain.CatalogIssueType, "Tarea", "task", domain.HierarchyStandard, true)
	f.story = add(domain.CatalogIssueType, "Historia", "story", domain.HierarchyStandard, false)
	f.epic = add(domain.CatalogIssueType, "Epica", "epic", domain.HierarchyEpic, false)
	f.subtask = add(domain.CatalogIssueType, "Subtarea", "subtask", domain.HierarchySubtask, false)
	f.medium = add(domain.CatalogPriority, "Media", "medium", 0, true)
	f.high = add(domain.CatalogPriority, "Alta", "high", 0, false)
	cat.projectTypes[f.projB] = []uuid.UUID{f.task, f.story}
	return cat
}

// issue creates an issue in project with the caller as reporter.
func (f *opsFixture) issue(t *testing.T, project, reporter uuid.UUID, summary string) *domain.Issue {
	t.Helper()
	created, err := f.svc.Create(context.Background(), port.CreateInput{
		Summary: summary, Description: "Detalle", Priority: domain.PriorityMedium, ReporterID: reporter,
		GlobalRole: "user", ProjectID: &project,
	})
	if err != nil {
		t.Fatalf("create %q: %v", summary, err)
	}
	f.events.list = nil
	f.collab.activities = nil
	return created
}
