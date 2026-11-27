package postgres

import (
	"context"
	"testing"

	"github.com/google/uuid"

	"github.com/jhonsferg/corelog/internal/issue/domain"
	"github.com/jhonsferg/corelog/internal/platform/database/testdb"
)

func newTestField(org uuid.UUID, name string, typ domain.FieldType) *domain.CustomField {
	return &domain.CustomField{
		OrganizationID: org,
		Name:           name,
		Type:           typ,
		Options:        []domain.FieldOption{{ID: "alta", Label: "Alta", Color: "red", Position: 0}, {ID: "baja", Label: "Baja", Position: 1, Archived: true}},
	}
}

func TestCustomFieldDefinitionsAreScopedToTheirOrganization(t *testing.T) {
	db := testdb.New(t)
	repo := NewCustomFieldRepository(db)
	ctx := context.Background()
	orgA := keyTestOrg(t, db)
	orgB := keyTestOrg(t, db)

	field := newTestField(orgA, "Severidad", domain.FieldSelect)
	min := 1.5
	field.NumberMin = &min
	if err := repo.InsertField(ctx, field); err != nil {
		t.Fatalf("insert: %v", err)
	}
	if field.ID == uuid.Nil {
		t.Fatal("insert must set the id")
	}

	fields, err := repo.ListFields(ctx, orgA)
	if err != nil || len(fields) != 1 {
		t.Fatalf("list org A = %d, err %v", len(fields), err)
	}
	got := fields[0]
	if len(got.Options) != 2 || got.Options[0].ID != "alta" || !got.Options[1].Archived || got.Options[0].Color != "red" {
		t.Errorf("options round trip = %+v", got.Options)
	}
	if got.NumberMin == nil || *got.NumberMin != 1.5 || got.NumberMax != nil {
		t.Errorf("settings round trip min=%v max=%v", got.NumberMin, got.NumberMax)
	}

	if list, err := repo.ListFields(ctx, orgB); err != nil || len(list) != 0 {
		t.Errorf("org B must not see org A fields: %d, err %v", len(list), err)
	}
	if _, err := repo.FindField(ctx, orgB, field.ID); err != domain.ErrCustomFieldNotFound {
		t.Errorf("find from org B = %v, want not found", err)
	}
	if err := repo.DeleteField(ctx, &domain.CustomField{ID: field.ID, OrganizationID: orgB}); err != domain.ErrCustomFieldNotFound {
		t.Errorf("delete from org B = %v, want not found", err)
	}
}

func TestCustomFieldNamesAreUniquePerOrganizationIgnoringCase(t *testing.T) {
	db := testdb.New(t)
	repo := NewCustomFieldRepository(db)
	ctx := context.Background()
	orgA := keyTestOrg(t, db)
	orgB := keyTestOrg(t, db)

	if err := repo.InsertField(ctx, newTestField(orgA, "Severidad", domain.FieldText)); err != nil {
		t.Fatal(err)
	}
	if err := repo.InsertField(ctx, newTestField(orgA, "SEVERIDAD", domain.FieldText)); err != domain.ErrCustomFieldNameTaken {
		t.Errorf("same name in org A = %v, want name taken", err)
	}
	if err := repo.InsertField(ctx, newTestField(orgB, "Severidad", domain.FieldText)); err != nil {
		t.Errorf("same name in org B must be allowed: %v", err)
	}
}

func TestCustomFieldContextsAndScope(t *testing.T) {
	db := testdb.New(t)
	catalogs := NewCatalogRepository(db)
	repo := NewCustomFieldRepository(db)
	ctx := context.Background()
	org, project, _ := catalogTestOrg(t, catalogs)
	taskType := catalogByName(t, catalogs, org, domain.CatalogIssueType, "Tarea")

	field := newTestField(org, "Fecha limite", domain.FieldDate)
	field.Options = nil
	if err := repo.InsertField(ctx, field); err != nil {
		t.Fatal(err)
	}
	projectID, typeID := project, taskType.ID
	contexts := []domain.FieldContext{
		{ProjectID: &projectID, IssueTypeID: &typeID, Required: true, DefaultValue: "2026-12-31", Position: 0},
		{ProjectID: &projectID, Position: 1},
	}
	if err := repo.ReplaceContexts(ctx, field, contexts); err != nil {
		t.Fatalf("replace: %v", err)
	}
	fields, err := repo.ListFields(ctx, org)
	if err != nil || len(fields) != 1 || len(fields[0].Contexts) != 2 {
		t.Fatalf("contexts = %+v, err %v", fields, err)
	}
	stored := fields[0].Contexts[0]
	if !stored.Required || stored.DefaultValue != "2026-12-31" || stored.IssueTypeID == nil || *stored.IssueTypeID != typeID {
		t.Errorf("stored context = %+v", stored)
	}

	scope := domain.BuildScope(fields, nil, projectID, typeID)
	if len(scope) != 1 || !scope[0].Required || scope[0].DefaultValue != "2026-12-31" {
		t.Errorf("scope = %+v", scope)
	}

	duplicate := []domain.FieldContext{{ProjectID: &projectID, IssueTypeID: &typeID}, {ProjectID: &projectID, IssueTypeID: &typeID}}
	if err := repo.ReplaceContexts(ctx, field, duplicate); err != domain.ErrCustomFieldContextDup {
		t.Errorf("duplicate contexts = %v, want context duplicate", err)
	}
	// A failed replace keeps the previous contexts.
	if fields, _ := repo.ListFields(ctx, org); len(fields[0].Contexts) != 2 {
		t.Errorf("failed replace must roll back, contexts = %d", len(fields[0].Contexts))
	}
}

func TestProjectLayoutRoundTripAndIsolation(t *testing.T) {
	db := testdb.New(t)
	catalogs := NewCatalogRepository(db)
	repo := NewCustomFieldRepository(db)
	ctx := context.Background()
	org, project, _ := catalogTestOrg(t, catalogs)
	taskType := catalogByName(t, catalogs, org, domain.CatalogIssueType, "Tarea")

	if _, configured, err := repo.FindLayout(ctx, project, taskType.ID); err != nil || configured {
		t.Fatalf("no layout yet: configured %v err %v", configured, err)
	}
	field := newTestField(org, "Cliente", domain.FieldText)
	if err := repo.InsertField(ctx, field); err != nil {
		t.Fatal(err)
	}
	items := []domain.LayoutItem{
		{Kind: domain.LayoutCustom, FieldID: field.ID, Required: true},
		{Kind: domain.LayoutSystem, Key: "summary", Required: true},
	}
	if err := repo.SaveLayout(ctx, project, taskType.ID, items); err != nil {
		t.Fatalf("save: %v", err)
	}
	got, configured, err := repo.FindLayout(ctx, project, taskType.ID)
	if err != nil || !configured || len(got) != 2 || got[0].FieldID != field.ID || !got[0].Required || got[1].Key != "summary" {
		t.Errorf("layout = %+v configured %v err %v", got, configured, err)
	}
	if err := repo.SaveLayout(ctx, project, taskType.ID, items[1:]); err != nil {
		t.Fatal(err)
	}
	if got, _, _ := repo.FindLayout(ctx, project, taskType.ID); len(got) != 1 {
		t.Errorf("save must replace the layout, got %d entries", len(got))
	}

	otherOrg := keyTestOrg(t, db)
	if ok, err := repo.IssueTypeInOrganization(ctx, otherOrg, taskType.ID); err != nil || ok {
		t.Errorf("issue type of another org = %v, err %v", ok, err)
	}
}

func TestCustomFieldValuesOnIssuesAreStoredAndCounted(t *testing.T) {
	db := testdb.New(t)
	catalogs := NewCatalogRepository(db)
	repo := NewCustomFieldRepository(db)
	issues := NewIssueRepository(db)
	ctx := context.Background()
	org, project, lead := catalogTestOrg(t, catalogs)
	otherOrg := keyTestOrg(t, db)
	if err := catalogs.SeedDefaults(ctx, otherOrg); err != nil {
		t.Fatal(err)
	}
	otherProject := keyTestProject(t, db, otherOrg, keyTestUser(t, db, otherOrg), "OTHX")

	field := newTestField(org, "Severidad", domain.FieldSelect)
	if err := repo.InsertField(ctx, field); err != nil {
		t.Fatal(err)
	}

	issue, err := domain.NewIssue("Resumen", "Descripcion", domain.PriorityMedium, lead)
	if err != nil {
		t.Fatal(err)
	}
	issue.ProjectID = project
	issue.Custom = map[string]any{field.ID.String(): "alta"}
	if err := issues.Save(ctx, issue); err != nil {
		t.Fatalf("save issue: %v", err)
	}
	stored, err := issues.FindByID(ctx, issue.ID)
	if err != nil || stored.Custom[field.ID.String()] != "alta" {
		t.Fatalf("custom round trip = %+v, err %v", stored, err)
	}

	issue.Custom = map[string]any{field.ID.String(): []string{"x"}}
	if err := issues.Update(ctx, issue); err != nil {
		t.Fatal(err)
	}
	if got, _ := issues.FindByID(ctx, issue.ID); got.Custom == nil {
		t.Error("update must keep the custom map")
	}

	issue.Custom = map[string]any{field.ID.String(): "alta"}
	if err := issues.Update(ctx, issue); err != nil {
		t.Fatal(err)
	}
	if n, err := repo.CountOptionUsage(ctx, org, field.ID, "alta"); err != nil || n != 1 {
		t.Errorf("usage in org = %d, err %v, want 1", n, err)
	}
	if n, _ := repo.CountOptionUsage(ctx, otherOrg, field.ID, "alta"); n != 0 {
		t.Errorf("usage seen from another org = %d, want 0", n)
	}

	// Values of issues in another organization are not touched by a removal here.
	other, _ := domain.NewIssue("Otro", "Descripcion", domain.PriorityMedium, lead)
	other.ProjectID = otherProject
	other.Custom = map[string]any{field.ID.String(): "alta"}
	if err := issues.Save(ctx, other); err != nil {
		t.Fatal(err)
	}
	if err := repo.RemoveValues(ctx, org, field.ID); err != nil {
		t.Fatal(err)
	}
	if got, _ := issues.FindByID(ctx, issue.ID); len(got.Custom) != 0 {
		t.Errorf("removed field must leave the issue values, got %+v", got.Custom)
	}
	if got, _ := issues.FindByID(ctx, other.ID); got.Custom[field.ID.String()] != "alta" {
		t.Errorf("values of another organization must stay, got %+v", got.Custom)
	}
	if err := repo.DeleteField(ctx, field); err != nil {
		t.Fatal(err)
	}
}
