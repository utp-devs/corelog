package postgres

import (
	"context"
	"testing"

	"github.com/google/uuid"

	"github.com/jhonsferg/corelog/internal/issue/application/port"
	"github.com/jhonsferg/corelog/internal/issue/domain"
	"github.com/jhonsferg/corelog/internal/platform/apperror"
	"github.com/jhonsferg/corelog/internal/platform/database/testdb"
)

// catalogTestOrg returns an organization with its default catalogs and one project with one user.
func catalogTestOrg(t *testing.T, repo *CatalogRepository) (uuid.UUID, uuid.UUID, uuid.UUID) {
	t.Helper()
	db := repo.db
	ctx := context.Background()
	orgID := keyTestOrg(t, db)
	if err := repo.SeedDefaults(ctx, orgID); err != nil {
		t.Fatal(err)
	}
	lead := keyTestUser(t, db, orgID)
	project := keyTestProject(t, db, orgID, lead, "CAT")
	return orgID, project, lead
}

func catalogByName(t *testing.T, repo *CatalogRepository, org uuid.UUID, kind domain.CatalogKind, name string) *domain.CatalogItem {
	t.Helper()
	items, err := repo.ListItems(context.Background(), org, kind)
	if err != nil {
		t.Fatal(err)
	}
	for _, item := range items {
		if item.Name == name {
			return item
		}
	}
	t.Fatalf("catalog %s has no item %q", kind, name)
	return nil
}

func TestCatalogDefaultsAndLookups(t *testing.T) {
	db := testdb.New(t)
	repo := NewCatalogRepository(db)
	ctx := context.Background()
	org, project, _ := catalogTestOrg(t, repo)

	types, err := repo.ListItems(ctx, org, domain.CatalogIssueType)
	if err != nil || len(types) != 5 {
		t.Fatalf("issue types = %d, err %v; want 5", len(types), err)
	}
	if got := catalogByName(t, repo, org, domain.CatalogIssueType, "Tarea"); !got.IsDefault || got.LegacyKey != "task" {
		t.Errorf("Tarea = default %v legacy %q", got.IsDefault, got.LegacyKey)
	}

	defaultType, err := repo.Select(ctx, project, domain.CatalogIssueType, port.CatalogSelector{Default: true})
	if err != nil || defaultType.Name != "Tarea" {
		t.Fatalf("default type = %v, err %v", defaultType, err)
	}
	legacyPriority, err := repo.Select(ctx, project, domain.CatalogPriority, port.CatalogSelector{LegacyKey: "critical"})
	if err != nil || legacyPriority.Name != "Máxima" {
		t.Fatalf("critical = %v, err %v", legacyPriority, err)
	}
	lowest, err := repo.Select(ctx, project, domain.CatalogPriority, port.CatalogSelector{LegacyKey: "low"})
	if err != nil || lowest.Name != "Baja" {
		t.Fatalf("low = %v, err %v; the first low item by position wins", lowest, err)
	}
	resolution, err := repo.Select(ctx, project, domain.CatalogResolution, port.CatalogSelector{Default: true})
	if err != nil || resolution.Name != "Hecho" {
		t.Fatalf("default resolution = %v, err %v", resolution, err)
	}
}

func TestCatalogDefaultTypeHonoursProjectConfiguration(t *testing.T) {
	db := testdb.New(t)
	repo := NewCatalogRepository(db)
	ctx := context.Background()
	org, project, _ := catalogTestOrg(t, repo)

	epic := catalogByName(t, repo, org, domain.CatalogIssueType, "Épica")
	subtask := catalogByName(t, repo, org, domain.CatalogIssueType, "Subtarea")
	story := catalogByName(t, repo, org, domain.CatalogIssueType, "Historia")
	if err := repo.ReplaceProjectIssueTypes(ctx, project, []uuid.UUID{epic.ID, subtask.ID, story.ID}); err != nil {
		t.Fatal(err)
	}

	items, configured, err := repo.ProjectIssueTypes(ctx, project)
	if err != nil || !configured || len(items) != 3 {
		t.Fatalf("project types = %d configured=%v err=%v", len(items), configured, err)
	}
	def, err := repo.Select(ctx, project, domain.CatalogIssueType, port.CatalogSelector{Default: true})
	if err != nil || def.Name != "Historia" {
		t.Fatalf("default on restricted project = %v, err %v; want the first enabled standard type", def, err)
	}
	enabled, err := repo.IssueTypeEnabled(ctx, project, story.ID)
	if err != nil || !enabled {
		t.Errorf("Historia enabled = %v, err %v", enabled, err)
	}
	tarea := catalogByName(t, repo, org, domain.CatalogIssueType, "Tarea")
	enabled, err = repo.IssueTypeEnabled(ctx, project, tarea.ID)
	if err != nil || enabled {
		t.Errorf("Tarea enabled = %v, err %v; want disabled", enabled, err)
	}
}

func TestCatalogDeleteInUseIsRefusedAndMoveIssuesUnblocksIt(t *testing.T) {
	db := testdb.New(t)
	repo := NewCatalogRepository(db)
	ctx := context.Background()
	org, project, lead := catalogTestOrg(t, repo)
	issues := NewIssueRepository(db)

	bug := catalogByName(t, repo, org, domain.CatalogIssueType, "Error")
	tarea := catalogByName(t, repo, org, domain.CatalogIssueType, "Tarea")
	issue := newKeyTestIssue(project, lead, "Con error")
	issue.IssueTypeID = bug.ID
	issue.IssueType = domain.IssueTypeBug
	if err := issues.Save(ctx, issue); err != nil {
		t.Fatal(err)
	}

	if err := repo.DeleteItem(ctx, bug); apperror.CodeOf(err) != "catalog.in_use" {
		t.Fatalf("delete in use = %v, want catalog.in_use", err)
	}
	if _, err := repo.MoveIssues(ctx, bug, tarea); err != nil {
		t.Fatalf("move: %v", err)
	}
	found, err := issues.FindByID(ctx, issue.ID)
	if err != nil {
		t.Fatal(err)
	}
	if found.IssueTypeID != tarea.ID || found.IssueType != domain.IssueTypeTask {
		t.Errorf("moved issue type = %s, legacy %s", found.IssueTypeID, found.IssueType)
	}
	if found.IssueTypeRef == nil || found.IssueTypeRef.Name != "Tarea" {
		t.Errorf("issue type ref = %+v, want Tarea", found.IssueTypeRef)
	}
	if err := repo.DeleteItem(ctx, bug); err != nil {
		t.Fatalf("delete after move: %v", err)
	}
	// Logical delete: the issue keeps showing the name, the catalog hides the item and its name is reusable.
	if items, _ := repo.ListItems(ctx, org, domain.CatalogIssueType); true {
		for _, it := range items {
			if it.ID == bug.ID {
				t.Fatal("deleted catalog item listed")
			}
		}
	}
	var stamped int
	if err := db.Get(&stamped, `SELECT count(*) FROM issue_types WHERE id = $1 AND deleted_at IS NOT NULL`, bug.ID); err != nil || stamped != 1 {
		t.Fatalf("catalog row must be kept with deleted_at set: %d %v", stamped, err)
	}
	again := &domain.CatalogItem{OrganizationID: org, Kind: domain.CatalogIssueType, Name: bug.Name, HierarchyLevel: 0, Icon: "bug", Color: "red"}
	if err := repo.InsertItem(ctx, again); err != nil {
		t.Fatalf("name of a deleted item must be reusable: %v", err)
	}
}

func TestCatalogNamesAreUniquePerOrganization(t *testing.T) {
	db := testdb.New(t)
	repo := NewCatalogRepository(db)
	ctx := context.Background()
	org, _, _ := catalogTestOrg(t, repo)

	dup := &domain.CatalogItem{OrganizationID: org, Kind: domain.CatalogResolution, Name: "hecho"}
	if err := repo.InsertItem(ctx, dup); apperror.CodeOf(err) != "catalog.name_taken" {
		t.Fatalf("duplicate name (case-insensitive) = %v, want catalog.name_taken", err)
	}
	second := &domain.CatalogItem{OrganizationID: org, Kind: domain.CatalogIssueType, Name: "Tarea", HierarchyLevel: 0, Icon: "task", Color: "blue"}
	if err := repo.InsertItem(ctx, second); apperror.CodeOf(err) != "catalog.name_taken" {
		t.Fatalf("duplicate type name = %v", err)
	}
	extraEpic := &domain.CatalogItem{OrganizationID: org, Kind: domain.CatalogIssueType, Name: "Otra", HierarchyLevel: domain.HierarchyEpic, Icon: "epic", Color: "purple"}
	if err := repo.InsertItem(ctx, extraEpic); apperror.CodeOf(err) != "catalog.hierarchy_level_taken" {
		t.Fatalf("second epic-level type = %v, want catalog.hierarchy_level_taken", err)
	}
}

func TestCatalogItemsAreIsolatedByOrganization(t *testing.T) {
	db := testdb.New(t)
	repo := NewCatalogRepository(db)
	ctx := context.Background()
	orgA, projectA, _ := catalogTestOrg(t, repo)
	orgB := keyTestOrg(t, db)
	if err := repo.SeedDefaults(ctx, orgB); err != nil {
		t.Fatal(err)
	}
	itemA := catalogByName(t, repo, orgA, domain.CatalogResolution, "Descartado")

	if _, err := repo.FindItem(ctx, orgB, domain.CatalogResolution, itemA.ID); apperror.CodeOf(err) != "catalog.not_found" {
		t.Fatalf("foreign item lookup = %v, want catalog.not_found", err)
	}
	itemsB, err := repo.ListItems(ctx, orgB, domain.CatalogResolution)
	if err != nil {
		t.Fatal(err)
	}
	for _, item := range itemsB {
		if item.ID == itemA.ID {
			t.Fatal("organization B lists organization A's resolution")
		}
	}
	if _, err := repo.Select(ctx, projectA, domain.CatalogResolution, port.CatalogSelector{ID: itemA.ID}); err != nil {
		t.Fatalf("own item select: %v", err)
	}
	if _, err := repo.Select(ctx, projectA, domain.CatalogResolution, port.CatalogSelector{ID: uuid.New()}); apperror.CodeOf(err) != "catalog.not_found" {
		t.Fatalf("unknown id select = %v", err)
	}
}
