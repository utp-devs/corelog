package usecase

import (
	"context"
	"sort"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/jhonsferg/corelog/internal/issue/application/port"
	"github.com/jhonsferg/corelog/internal/issue/domain"
	"github.com/jhonsferg/corelog/internal/platform/apperror"
	"github.com/jhonsferg/corelog/internal/platform/clock"
	"github.com/jhonsferg/corelog/internal/platform/database"
)

// memCatalog is an in-memory catalog store. It also implements port.IssueCatalog.
type memCatalog struct {
	items        map[uuid.UUID]*domain.CatalogItem
	usage        map[uuid.UUID]int
	projects     map[uuid.UUID]uuid.UUID
	projectTypes map[uuid.UUID][]uuid.UUID
	moved        int
}

func newMemCatalog() *memCatalog {
	return &memCatalog{
		items:        map[uuid.UUID]*domain.CatalogItem{},
		usage:        map[uuid.UUID]int{},
		projects:     map[uuid.UUID]uuid.UUID{},
		projectTypes: map[uuid.UUID][]uuid.UUID{},
	}
}

var (
	_ port.CatalogStore = (*memCatalog)(nil)
	_ port.IssueCatalog = (*memCatalog)(nil)
)

func (m *memCatalog) add(item domain.CatalogItem) *domain.CatalogItem {
	if item.ID == uuid.Nil {
		item.ID = uuid.New()
	}
	stored := item
	m.items[stored.ID] = &stored
	return &stored
}

func (m *memCatalog) LockOrganization(context.Context, uuid.UUID) error { return nil }

func (m *memCatalog) ListItems(_ context.Context, org uuid.UUID, kind domain.CatalogKind) ([]*domain.CatalogItem, error) {
	var out []*domain.CatalogItem
	for _, item := range m.items {
		if item.OrganizationID == org && item.Kind == kind {
			copied := *item
			copied.Usage = m.usage[item.ID]
			out = append(out, &copied)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Position != out[j].Position {
			return out[i].Position < out[j].Position
		}
		return out[i].Name < out[j].Name
	})
	return out, nil
}

func (m *memCatalog) FindItem(_ context.Context, org uuid.UUID, kind domain.CatalogKind, id uuid.UUID) (*domain.CatalogItem, error) {
	item, ok := m.items[id]
	if !ok || item.OrganizationID != org || item.Kind != kind {
		return nil, domain.ErrCatalogNotFound
	}
	copied := *item
	copied.Usage = m.usage[id]
	return &copied, nil
}

func (m *memCatalog) InsertItem(_ context.Context, item *domain.CatalogItem) error {
	if item.ID == uuid.Nil {
		item.ID = uuid.New()
	}
	stored := *item
	m.items[item.ID] = &stored
	return nil
}

func (m *memCatalog) UpdateItem(_ context.Context, item *domain.CatalogItem) error {
	stored := *item
	m.items[item.ID] = &stored
	return nil
}

func (m *memCatalog) DeleteItem(_ context.Context, item *domain.CatalogItem) error {
	if m.usage[item.ID] > 0 {
		return domain.ErrCatalogInUse
	}
	delete(m.items, item.ID)
	return nil
}

func (m *memCatalog) SetDefaultItem(_ context.Context, org uuid.UUID, kind domain.CatalogKind, id uuid.UUID) error {
	for _, item := range m.items {
		if item.OrganizationID == org && item.Kind == kind {
			item.IsDefault = item.ID == id
		}
	}
	return nil
}

func (m *memCatalog) SetPositions(_ context.Context, org uuid.UUID, kind domain.CatalogKind, ids []uuid.UUID) error {
	for i, id := range ids {
		if item, ok := m.items[id]; ok && item.OrganizationID == org && item.Kind == kind {
			item.Position = i
		}
	}
	return nil
}

func (m *memCatalog) MoveIssues(_ context.Context, from, target *domain.CatalogItem) (int64, error) {
	n := m.usage[from.ID]
	m.usage[target.ID] += n
	m.usage[from.ID] = 0
	m.moved += n
	return int64(n), nil
}

func (m *memCatalog) ProjectOrganization(_ context.Context, projectID uuid.UUID) (uuid.UUID, error) {
	org, ok := m.projects[projectID]
	if !ok {
		return uuid.Nil, apperror.New(apperror.KindNotFound, "project not found")
	}
	return org, nil
}

func (m *memCatalog) ProjectIssueTypes(_ context.Context, projectID uuid.UUID) ([]*domain.CatalogItem, bool, error) {
	ids, configured := m.projectTypes[projectID]
	var out []*domain.CatalogItem
	if configured {
		for _, id := range ids {
			copied := *m.items[id]
			out = append(out, &copied)
		}
		return out, true, nil
	}
	org := m.projects[projectID]
	items, _ := m.ListItems(context.Background(), org, domain.CatalogIssueType)
	return items, false, nil
}

func (m *memCatalog) ReplaceProjectIssueTypes(_ context.Context, projectID uuid.UUID, ids []uuid.UUID) error {
	m.projectTypes[projectID] = ids
	return nil
}

func (m *memCatalog) SeedDefaults(context.Context, uuid.UUID) error { return nil }

// Select resolves an item of the project's organization, the way the repository does.
func (m *memCatalog) Select(ctx context.Context, projectID uuid.UUID, kind domain.CatalogKind, sel port.CatalogSelector) (*domain.CatalogItem, error) {
	org, ok := m.projects[projectID]
	if !ok {
		return nil, domain.ErrCatalogNotFound
	}
	items, _ := m.ListItems(ctx, org, kind)
	if kind == domain.CatalogIssueType && sel.ID == uuid.Nil && sel.LegacyKey == "" {
		// Default types: standard types the project enables, the organization default first.
		for _, item := range items {
			if item.IsDefault && item.HierarchyLevel == domain.HierarchyStandard {
				if enabled, _ := m.IssueTypeEnabled(ctx, projectID, item.ID); enabled {
					return item, nil
				}
			}
		}
		for _, item := range items {
			if item.HierarchyLevel == domain.HierarchyStandard {
				if enabled, _ := m.IssueTypeEnabled(ctx, projectID, item.ID); enabled {
					return item, nil
				}
			}
		}
		return nil, domain.ErrCatalogNotFound
	}
	for _, item := range items {
		switch {
		case sel.ID != uuid.Nil && item.ID == sel.ID:
			return item, nil
		case sel.LegacyKey != "" && item.LegacyKey == sel.LegacyKey:
			return item, nil
		case sel.Default && sel.ID == uuid.Nil && sel.LegacyKey == "" && item.IsDefault:
			return item, nil
		}
	}
	return nil, domain.ErrCatalogNotFound
}

func (m *memCatalog) IssueTypeEnabled(_ context.Context, projectID, typeID uuid.UUID) (bool, error) {
	ids, configured := m.projectTypes[projectID]
	if !configured {
		return true, nil
	}
	for _, id := range ids {
		if id == typeID {
			return true, nil
		}
	}
	return false, nil
}

// roleChecker grants roles per user and organization. Missing entries are outsiders.
type roleChecker struct {
	roles map[uuid.UUID]string
}

var orgRank = map[string]int{"guest": 1, "member": 2, "admin": 3, "owner": 4}

func (r roleChecker) RequireOrgRole(_ context.Context, userID, _ uuid.UUID, min string) error {
	role, ok := r.roles[userID]
	if !ok {
		return apperror.WithCode(apperror.KindNotFound, "org.not_found", "organization not found")
	}
	if orgRank[role] < orgRank[min] {
		return apperror.WithCode(apperror.KindForbidden, "org.forbidden", "forbidden")
	}
	return nil
}

// denyAuthorizer refuses every project action when deny is set.
type denyAuthorizer struct{ deny bool }

func (a denyAuthorizer) Authorize(context.Context, string, uuid.UUID, string, uuid.UUID) error {
	if a.deny {
		return apperror.New(apperror.KindForbidden, "forbidden project operation")
	}
	return nil
}

type catalogFixture struct {
	svc     *CatalogService
	store   *memCatalog
	org     uuid.UUID
	project uuid.UUID
	owner   uuid.UUID
	admin   uuid.UUID
	member  uuid.UUID
	outside uuid.UUID
	types   map[string]*domain.CatalogItem
	prios   map[string]*domain.CatalogItem
	res     map[string]*domain.CatalogItem
}

func newCatalogFixture(t *testing.T) *catalogFixture {
	t.Helper()
	f := &catalogFixture{
		store:   newMemCatalog(),
		org:     uuid.New(),
		project: uuid.New(),
		owner:   uuid.New(),
		admin:   uuid.New(),
		member:  uuid.New(),
		outside: uuid.New(),
		types:   map[string]*domain.CatalogItem{},
		prios:   map[string]*domain.CatalogItem{},
		res:     map[string]*domain.CatalogItem{},
	}
	f.store.projects[f.project] = f.org
	roles := roleChecker{roles: map[uuid.UUID]string{f.owner: "owner", f.admin: "admin", f.member: "member"}}
	f.svc = NewCatalogService(f.store, roles, denyAuthorizer{}, &database.FakeTxManager{},
		clock.NewFixed(time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)))

	typeRows := []domain.CatalogItem{
		{Name: "Épica", Icon: "epic", Color: "purple", HierarchyLevel: domain.HierarchyEpic, Position: 0, LegacyKey: "epic"},
		{Name: "Historia", Icon: "story", Color: "green", Position: 1, LegacyKey: "story"},
		{Name: "Tarea", Icon: "task", Color: "blue", Position: 2, IsDefault: true, LegacyKey: "task"},
		{Name: "Error", Icon: "bug", Color: "red", Position: 3, LegacyKey: "bug"},
		{Name: "Subtarea", Icon: "subtask", Color: "teal", HierarchyLevel: domain.HierarchySubtask, Position: 4, LegacyKey: "subtask"},
	}
	for _, row := range typeRows {
		row.Kind, row.OrganizationID = domain.CatalogIssueType, f.org
		f.types[row.Name] = f.store.add(row)
	}
	prioRows := []domain.CatalogItem{
		{Name: "Máxima", Icon: "chevrons-up", Color: "red", Position: 0, LegacyKey: "critical"},
		{Name: "Alta", Icon: "arrow-up", Color: "orange", Position: 1, LegacyKey: "high"},
		{Name: "Media", Icon: "minus", Color: "amber", Position: 2, IsDefault: true, LegacyKey: "medium"},
		{Name: "Baja", Icon: "arrow-down", Color: "blue", Position: 3, LegacyKey: "low"},
	}
	for _, row := range prioRows {
		row.Kind, row.OrganizationID = domain.CatalogPriority, f.org
		f.prios[row.Name] = f.store.add(row)
	}
	resRows := []domain.CatalogItem{
		{Name: "Hecho", Position: 0, IsDefault: true},
		{Name: "Descartado", Position: 1},
	}
	for _, row := range resRows {
		row.Kind, row.OrganizationID = domain.CatalogResolution, f.org
		f.res[row.Name] = f.store.add(row)
	}
	return f
}

func codeOf(t *testing.T, err error, kind apperror.Kind, code string) {
	t.Helper()
	if err == nil {
		t.Fatalf("want %s, got nil", code)
	}
	if apperror.KindOf(err) != kind {
		t.Errorf("kind = %s, want %s (%v)", apperror.KindOf(err), kind, err)
	}
	if apperror.CodeOf(err) != code {
		t.Errorf("code = %s, want %s", apperror.CodeOf(err), code)
	}
}

func TestCatalogReadsNeedMembershipAndWritesNeedAdmin(t *testing.T) {
	f := newCatalogFixture(t)
	ctx := context.Background()

	items, err := f.svc.ListCatalog(ctx, f.member, f.org, domain.CatalogIssueType)
	if err != nil || len(items) != 5 {
		t.Fatalf("member list = %d items, err %v", len(items), err)
	}
	_, err = f.svc.ListCatalog(ctx, f.outside, f.org, domain.CatalogIssueType)
	codeOf(t, err, apperror.KindNotFound, "org.not_found")

	_, err = f.svc.CreateCatalogItem(ctx, f.member, f.org, domain.CatalogPriority, port.CatalogInput{Name: "Crítica", Icon: "flag", Color: "red"})
	codeOf(t, err, apperror.KindForbidden, "org.forbidden")

	if _, err := f.svc.CreateCatalogItem(ctx, f.admin, f.org, domain.CatalogPriority, port.CatalogInput{Name: "Crítica", Icon: "flag", Color: "red"}); err != nil {
		t.Fatalf("admin create: %v", err)
	}
}

func TestCreateCatalogItemValidatesAndKeepsNamesAndHierarchyUnique(t *testing.T) {
	f := newCatalogFixture(t)
	ctx := context.Background()

	_, err := f.svc.CreateCatalogItem(ctx, f.admin, f.org, domain.CatalogIssueType, port.CatalogInput{Name: "tarea", Icon: "task", Color: "blue"})
	codeOf(t, err, apperror.KindConflict, "catalog.name_taken")

	_, err = f.svc.CreateCatalogItem(ctx, f.admin, f.org, domain.CatalogIssueType, port.CatalogInput{Name: "Otra epica", Icon: "epic", Color: "purple", HierarchyLevel: domain.HierarchyEpic})
	codeOf(t, err, apperror.KindConflict, "catalog.hierarchy_level_taken")

	_, err = f.svc.CreateCatalogItem(ctx, f.admin, f.org, domain.CatalogIssueType, port.CatalogInput{Name: "Mala", Icon: "unicorn", Color: "blue"})
	codeOf(t, err, apperror.KindValidation, "catalog.invalid_icon")

	created, err := f.svc.CreateCatalogItem(ctx, f.admin, f.org, domain.CatalogIssueType, port.CatalogInput{Name: "Mejora", Icon: "star", Color: "indigo", IsDefault: true})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if !created.IsDefault || created.Position != 5 {
		t.Errorf("created default=%v position=%d, want default at position 5", created.IsDefault, created.Position)
	}
	if f.store.items[f.types["Tarea"].ID].IsDefault {
		t.Error("previous default must be cleared")
	}
}

func TestUpdateCatalogItemRejectsNameTaken(t *testing.T) {
	f := newCatalogFixture(t)
	name := "Historia"
	_, err := f.svc.UpdateCatalogItem(context.Background(), f.admin, f.org, domain.CatalogIssueType, f.types["Error"].ID,
		port.CatalogPatch{Name: &name})
	codeOf(t, err, apperror.KindConflict, "catalog.name_taken")
}

func TestDeleteInUseNeedsReassignAndMovesIssues(t *testing.T) {
	f := newCatalogFixture(t)
	ctx := context.Background()
	f.store.usage[f.types["Error"].ID] = 3
	f.store.usage[f.types["Tarea"].ID] = 1

	err := f.svc.DeleteCatalogItem(ctx, f.admin, f.org, domain.CatalogIssueType, f.types["Error"].ID, nil)
	codeOf(t, err, apperror.KindConflict, "catalog.in_use")
	if _, ok := f.store.items[f.types["Error"].ID]; !ok {
		t.Fatal("item deleted despite usage")
	}

	target := f.types["Tarea"].ID
	if err := f.svc.DeleteCatalogItem(ctx, f.admin, f.org, domain.CatalogIssueType, f.types["Error"].ID, &target); err != nil {
		t.Fatalf("delete with reassign: %v", err)
	}
	if f.store.moved != 3 || f.store.usage[target] != 4 {
		t.Errorf("moved = %d, target usage = %d; want 3 and 4", f.store.moved, f.store.usage[target])
	}
	if _, ok := f.store.items[f.types["Error"].ID]; ok {
		t.Error("item still present after delete")
	}
}

func TestDeleteRejectsIncompatibleReassignTarget(t *testing.T) {
	f := newCatalogFixture(t)
	f.store.usage[f.types["Error"].ID] = 1
	subtask := f.types["Subtarea"].ID
	err := f.svc.DeleteCatalogItem(context.Background(), f.admin, f.org, domain.CatalogIssueType, f.types["Error"].ID, &subtask)
	codeOf(t, err, apperror.KindValidation, "catalog.reassign_target_invalid")
}

func TestDeleteKeepsRequiredItems(t *testing.T) {
	f := newCatalogFixture(t)
	ctx := context.Background()
	err := f.svc.DeleteCatalogItem(ctx, f.admin, f.org, domain.CatalogIssueType, f.types["Épica"].ID, nil)
	codeOf(t, err, apperror.KindValidation, "catalog.last_required_item")
	err = f.svc.DeleteCatalogItem(ctx, f.admin, f.org, domain.CatalogIssueType, f.types["Subtarea"].ID, nil)
	codeOf(t, err, apperror.KindValidation, "catalog.last_required_item")

	// A priority can go while another remains.
	if err := f.svc.DeleteCatalogItem(ctx, f.admin, f.org, domain.CatalogPriority, f.prios["Baja"].ID, nil); err != nil {
		t.Fatalf("delete unused priority: %v", err)
	}
}

func TestDeletingDefaultPromotesFirstRemainingItem(t *testing.T) {
	f := newCatalogFixture(t)
	if err := f.svc.DeleteCatalogItem(context.Background(), f.admin, f.org, domain.CatalogPriority, f.prios["Media"].ID, nil); err != nil {
		t.Fatalf("delete default: %v", err)
	}
	if !f.store.items[f.prios["Máxima"].ID].IsDefault {
		t.Error("first remaining priority should become the default")
	}
}

func TestReorderNeedsEveryItemOnce(t *testing.T) {
	f := newCatalogFixture(t)
	ctx := context.Background()
	ids := []uuid.UUID{f.prios["Baja"].ID, f.prios["Alta"].ID}
	_, err := f.svc.ReorderCatalog(ctx, f.admin, f.org, domain.CatalogPriority, ids)
	codeOf(t, err, apperror.KindValidation, "catalog.order_mismatch")

	ids = []uuid.UUID{f.prios["Baja"].ID, f.prios["Alta"].ID, f.prios["Media"].ID, f.prios["Máxima"].ID}
	items, err := f.svc.ReorderCatalog(ctx, f.admin, f.org, domain.CatalogPriority, ids)
	if err != nil {
		t.Fatalf("reorder: %v", err)
	}
	if items[0].ID != f.prios["Baja"].ID {
		t.Errorf("first item = %s, want Baja", items[0].Name)
	}
}

func TestProjectIssueTypesRequireStandardEpicAndSubtask(t *testing.T) {
	f := newCatalogFixture(t)
	ctx := context.Background()
	admin := f.admin

	// Without configuration every organization type applies.
	items, configured, err := f.svc.ProjectIssueTypes(ctx, admin, "user", f.project)
	if err != nil || configured || len(items) != 5 {
		t.Fatalf("default project types = %d configured=%v err=%v", len(items), configured, err)
	}

	missingSubtask := []uuid.UUID{f.types["Épica"].ID, f.types["Tarea"].ID}
	_, err = f.svc.ReplaceProjectIssueTypes(ctx, admin, "user", f.project, missingSubtask)
	codeOf(t, err, apperror.KindValidation, "project.issue_types_required")

	ok := []uuid.UUID{f.types["Tarea"].ID, f.types["Épica"].ID, f.types["Subtarea"].ID}
	saved, err := f.svc.ReplaceProjectIssueTypes(ctx, admin, "user", f.project, ok)
	if err != nil || len(saved) != 3 {
		t.Fatalf("replace = %d items, err %v", len(saved), err)
	}
}

func TestProjectIssueTypesNeedProjectPermission(t *testing.T) {
	f := newCatalogFixture(t)
	f.svc.authorizer = denyAuthorizer{deny: true}
	_, err := f.svc.ReplaceProjectIssueTypes(context.Background(), f.admin, "user", f.project, nil)
	codeOf(t, err, apperror.KindForbidden, "")
}
