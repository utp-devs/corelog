package usecase

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/jhonsferg/corelog/internal/issue/application/port"
	"github.com/jhonsferg/corelog/internal/issue/domain"
	"github.com/jhonsferg/corelog/internal/platform/apperror"
	"github.com/jhonsferg/corelog/internal/platform/clock"
	"github.com/jhonsferg/corelog/internal/platform/database"
)

// cfStore is an in-memory port.CustomFieldStore. Organization ownership is checked the way the database does.
type cfStore struct {
	fields        map[uuid.UUID]*domain.CustomField
	projects      map[uuid.UUID]uuid.UUID
	issueTypes    map[uuid.UUID]uuid.UUID
	layouts       map[string][]domain.LayoutItem
	usage         map[string]int
	removedValues []uuid.UUID
}

func newCFStore() *cfStore {
	return &cfStore{
		fields:     map[uuid.UUID]*domain.CustomField{},
		projects:   map[uuid.UUID]uuid.UUID{},
		issueTypes: map[uuid.UUID]uuid.UUID{},
		layouts:    map[string][]domain.LayoutItem{},
		usage:      map[string]int{},
	}
}

func cloneField(f *domain.CustomField) *domain.CustomField {
	copyField := *f
	copyField.Options = append([]domain.FieldOption(nil), f.Options...)
	copyField.Contexts = append([]domain.FieldContext(nil), f.Contexts...)
	return &copyField
}

func (s *cfStore) ListFields(_ context.Context, org uuid.UUID) ([]*domain.CustomField, error) {
	out := []*domain.CustomField{}
	for _, f := range s.fields {
		if f.OrganizationID == org {
			out = append(out, cloneField(f))
		}
	}
	return out, nil
}

func (s *cfStore) FindField(_ context.Context, org, id uuid.UUID) (*domain.CustomField, error) {
	f, ok := s.fields[id]
	if !ok || f.OrganizationID != org {
		return nil, domain.ErrCustomFieldNotFound
	}
	return cloneField(f), nil
}

func (s *cfStore) nameTaken(org uuid.UUID, name string, except uuid.UUID) bool {
	for id, f := range s.fields {
		if id != except && f.OrganizationID == org && strings.EqualFold(f.Name, name) {
			return true
		}
	}
	return false
}

func (s *cfStore) InsertField(_ context.Context, f *domain.CustomField) error {
	if s.nameTaken(f.OrganizationID, f.Name, uuid.Nil) {
		return domain.ErrCustomFieldNameTaken
	}
	f.ID = uuid.New()
	s.fields[f.ID] = cloneField(f)
	return nil
}

func (s *cfStore) UpdateField(_ context.Context, f *domain.CustomField) error {
	if s.nameTaken(f.OrganizationID, f.Name, f.ID) {
		return domain.ErrCustomFieldNameTaken
	}
	s.fields[f.ID] = cloneField(f)
	return nil
}

func (s *cfStore) DeleteField(_ context.Context, f *domain.CustomField) error {
	delete(s.fields, f.ID)
	return nil
}

func (s *cfStore) ReplaceContexts(_ context.Context, f *domain.CustomField, contexts []domain.FieldContext) error {
	stored := s.fields[f.ID]
	stored.Contexts = contexts
	return nil
}

func (s *cfStore) RemoveValues(_ context.Context, _, fieldID uuid.UUID) error {
	s.removedValues = append(s.removedValues, fieldID)
	return nil
}

func (s *cfStore) CountOptionUsage(_ context.Context, _, fieldID uuid.UUID, optionID string) (int, error) {
	return s.usage[fieldID.String()+"/"+optionID], nil
}

func (s *cfStore) ProjectOrganization(_ context.Context, projectID uuid.UUID) (uuid.UUID, error) {
	org, ok := s.projects[projectID]
	if !ok {
		return uuid.Nil, apperror.New(apperror.KindNotFound, "project not found")
	}
	return org, nil
}

func (s *cfStore) IssueTypeInOrganization(_ context.Context, org, issueTypeID uuid.UUID) (bool, error) {
	return s.issueTypes[issueTypeID] == org, nil
}

func (s *cfStore) FindLayout(_ context.Context, projectID, issueTypeID uuid.UUID) ([]domain.LayoutItem, bool, error) {
	items, ok := s.layouts[projectID.String()+"/"+issueTypeID.String()]
	return items, ok, nil
}

func (s *cfStore) SaveLayout(_ context.Context, projectID, issueTypeID uuid.UUID, items []domain.LayoutItem) error {
	s.layouts[projectID.String()+"/"+issueTypeID.String()] = items
	return nil
}

// cfAuthorizer denies the listed actions.
type cfAuthorizer struct{ denied map[string]bool }

func (a cfAuthorizer) Authorize(_ context.Context, action string, _ uuid.UUID, _ string, _ uuid.UUID) error {
	if a.denied[action] {
		return apperror.New(apperror.KindForbidden, "forbidden project operation")
	}
	return nil
}

// cfBrowse answers the user visibility check from a fixed set.
type cfBrowse struct{ visible map[uuid.UUID]bool }

func (b cfBrowse) CanBrowse(_ context.Context, _, userID uuid.UUID) (bool, error) {
	return b.visible[userID], nil
}

type cfFixture struct {
	store   *cfStore
	service *CustomFieldService
	org     uuid.UUID
	project uuid.UUID
	taskTyp uuid.UUID
	admin   uuid.UUID
	member  uuid.UUID
	outside uuid.UUID
}

func newCFFixture(t *testing.T) *cfFixture {
	t.Helper()
	fx := &cfFixture{store: newCFStore(), org: uuid.New(), project: uuid.New(), taskTyp: uuid.New(), admin: uuid.New(), member: uuid.New(), outside: uuid.New()}
	fx.store.projects[fx.project] = fx.org
	fx.store.issueTypes[fx.taskTyp] = fx.org
	roles := roleChecker{roles: map[uuid.UUID]string{fx.admin: "admin", fx.member: "member"}}
	clk := clock.NewFixed(time.Date(2026, 11, 27, 10, 0, 0, 0, time.UTC))
	fx.service = NewCustomFieldService(fx.store, roles, cfAuthorizer{}, &database.FakeTxManager{}, clk)
	fx.service.newID = seqIDs()
	return fx
}

func seqIDs() func() string {
	n := 0
	return func() string {
		n++
		return "opt-" + strings.Repeat("x", n)
	}
}

func (fx *cfFixture) newField(t *testing.T, name string, typ domain.FieldType, options ...domain.OptionInput) *domain.CustomField {
	t.Helper()
	field, err := fx.service.CreateCustomField(context.Background(), fx.admin, fx.org, port.CustomFieldInput{Name: name, Type: typ, Options: options})
	if err != nil {
		t.Fatalf("create %s: %v", name, err)
	}
	return field
}

func TestCustomFieldWritesNeedOrgAdminAndReadsNeedMembership(t *testing.T) {
	fx := newCFFixture(t)
	ctx := context.Background()
	_, err := fx.service.CreateCustomField(ctx, fx.member, fx.org, port.CustomFieldInput{Name: "x", Type: domain.FieldText})
	codeOf(t, err, apperror.KindForbidden, "org.forbidden")
	_, err = fx.service.CreateCustomField(ctx, fx.outside, fx.org, port.CustomFieldInput{Name: "x", Type: domain.FieldText})
	codeOf(t, err, apperror.KindNotFound, "org.not_found")
	if _, err := fx.service.ListCustomFields(ctx, fx.member, fx.org); err != nil {
		t.Errorf("member must read definitions: %v", err)
	}
	_, err = fx.service.ListCustomFields(ctx, fx.outside, fx.org)
	codeOf(t, err, apperror.KindNotFound, "org.not_found")
}

func TestCreateCustomFieldValidatesAndAssignsOptionIDs(t *testing.T) {
	fx := newCFFixture(t)
	ctx := context.Background()
	field, err := fx.service.CreateCustomField(ctx, fx.admin, fx.org, port.CustomFieldInput{
		Name: "Severidad", Type: domain.FieldSelect,
		Options: []domain.OptionInput{{Label: "Alta", Color: "red"}, {Label: "Baja"}},
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if len(field.Options) != 2 || field.Options[0].ID == "" || field.Options[1].Position != 1 {
		t.Errorf("options = %+v, want two with ids and positions", field.Options)
	}

	_, err = fx.service.CreateCustomField(ctx, fx.admin, fx.org, port.CustomFieldInput{Name: "severidad", Type: domain.FieldText})
	codeOf(t, err, apperror.KindConflict, "custom_field.name_taken")
	_, err = fx.service.CreateCustomField(ctx, fx.admin, fx.org, port.CustomFieldInput{Name: "  ", Type: domain.FieldText})
	codeOf(t, err, apperror.KindValidation, "custom_field.name_required")
	_, err = fx.service.CreateCustomField(ctx, fx.admin, fx.org, port.CustomFieldInput{Name: "t", Type: domain.FieldText, Options: []domain.OptionInput{{Label: "a"}}})
	codeOf(t, err, apperror.KindValidation, "custom_field.invalid_options")
	_, err = fx.service.CreateCustomField(ctx, fx.admin, fx.org, port.CustomFieldInput{Name: "n", Type: domain.FieldNumber, NumberMin: ptrFloat(9), NumberMax: ptrFloat(1)})
	codeOf(t, err, apperror.KindValidation, "custom_field.invalid_settings")
}

func ptrFloat(v float64) *float64 { return &v }

func TestUpdateCustomFieldArchivesUsedOptionsAndDropsUnused(t *testing.T) {
	fx := newCFFixture(t)
	ctx := context.Background()
	field := fx.newField(t, "Estado", domain.FieldSelect, domain.OptionInput{Label: "Uno"}, domain.OptionInput{Label: "Dos"})
	used, unused := field.Options[0], field.Options[1]
	fx.store.usage[field.ID.String()+"/"+used.ID] = 3

	updated, err := fx.service.UpdateCustomField(ctx, fx.admin, fx.org, field.ID, port.CustomFieldPatch{
		Options: &[]domain.OptionInput{},
	})
	if err != nil {
		t.Fatalf("update: %v", err)
	}
	if len(updated.Options) != 1 || updated.Options[0].ID != used.ID || !updated.Options[0].Archived {
		t.Errorf("options = %+v, want only the used option archived", updated.Options)
	}
	for _, option := range updated.Options {
		if option.ID == unused.ID {
			t.Errorf("unused option must be dropped")
		}
	}

	_, err = fx.service.UpdateCustomField(ctx, fx.member, fx.org, field.ID, port.CustomFieldPatch{})
	codeOf(t, err, apperror.KindForbidden, "org.forbidden")
	_, err = fx.service.UpdateCustomField(ctx, fx.admin, fx.org, uuid.New(), port.CustomFieldPatch{})
	codeOf(t, err, apperror.KindNotFound, "custom_field.not_found")
}

func TestDeleteCustomFieldRemovesValues(t *testing.T) {
	fx := newCFFixture(t)
	field := fx.newField(t, "Enlace", domain.FieldURL)
	if err := fx.service.DeleteCustomField(context.Background(), fx.admin, fx.org, field.ID); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if len(fx.store.removedValues) != 1 || fx.store.removedValues[0] != field.ID {
		t.Errorf("values of the field must be removed with it, got %v", fx.store.removedValues)
	}
	err := fx.service.DeleteCustomField(context.Background(), fx.admin, fx.org, field.ID)
	codeOf(t, err, apperror.KindNotFound, "custom_field.not_found")
}

func TestReplaceContextsChecksScopeDuplicatesAndDefaults(t *testing.T) {
	fx := newCFFixture(t)
	ctx := context.Background()
	field := fx.newField(t, "Fecha", domain.FieldDate)
	foreignProject := uuid.New()
	fx.store.projects[foreignProject] = uuid.New()

	_, err := fx.service.ReplaceCustomFieldContexts(ctx, fx.admin, fx.org, field.ID, []port.ContextInput{{ProjectID: &foreignProject}})
	codeOf(t, err, apperror.KindValidation, "custom_field.context_invalid")

	project := fx.project
	_, err = fx.service.ReplaceCustomFieldContexts(ctx, fx.admin, fx.org, field.ID, []port.ContextInput{
		{ProjectID: &project, IssueTypeID: &fx.taskTyp}, {ProjectID: &project, IssueTypeID: &fx.taskTyp},
	})
	codeOf(t, err, apperror.KindValidation, "custom_field.context_duplicate")

	_, err = fx.service.ReplaceCustomFieldContexts(ctx, fx.admin, fx.org, field.ID, []port.ContextInput{
		{Default: json.RawMessage(`"2026-13-45"`)},
	})
	codeOf(t, err, apperror.KindValidation, "custom_field.invalid_default")

	saved, err := fx.service.ReplaceCustomFieldContexts(ctx, fx.admin, fx.org, field.ID, []port.ContextInput{
		{ProjectID: &project, Required: true, Default: json.RawMessage(`"2026-12-01"`)},
	})
	if err != nil {
		t.Fatalf("replace: %v", err)
	}
	if len(saved.Contexts) != 1 || !saved.Contexts[0].Required || saved.Contexts[0].DefaultValue != "2026-12-01" {
		t.Errorf("contexts = %+v", saved.Contexts)
	}
}

func TestProjectLayoutNeedsProjectPermissionsAndValidatesEntries(t *testing.T) {
	fx := newCFFixture(t)
	ctx := context.Background()
	field := fx.newField(t, "Cliente", domain.FieldText)
	project := fx.project
	if _, err := fx.service.ReplaceCustomFieldContexts(ctx, fx.admin, fx.org, field.ID, []port.ContextInput{{ProjectID: &project}}); err != nil {
		t.Fatal(err)
	}

	view, err := fx.service.ProjectLayout(ctx, fx.member, "", fx.project, fx.taskTyp)
	if err != nil {
		t.Fatalf("layout default: %v", err)
	}
	if view.Configured || len(view.Items) != len(domain.SystemFieldKeys)+1 {
		t.Errorf("default layout = configured %v with %d items", view.Configured, len(view.Items))
	}

	fx.service.authorizer = cfAuthorizer{denied: map[string]bool{actionEditProject: true}}
	_, err = fx.service.ReplaceProjectLayout(ctx, fx.member, "", fx.project, fx.taskTyp, nil)
	if apperror.KindOf(err) != apperror.KindForbidden {
		t.Fatalf("layout write by a non editor: %v", err)
	}
	fx.service.authorizer = cfAuthorizer{}

	_, err = fx.service.ReplaceProjectLayout(ctx, fx.admin, "", fx.project, fx.taskTyp, []domain.LayoutItem{
		{Kind: domain.LayoutCustom, FieldID: uuid.New()},
	})
	codeOf(t, err, apperror.KindValidation, "project.field_layout_invalid")

	_, err = fx.service.ReplaceProjectLayout(ctx, fx.admin, "", fx.project, uuid.New(), nil)
	codeOf(t, err, apperror.KindValidation, "issue.catalog_value_unknown")

	view, err = fx.service.ReplaceProjectLayout(ctx, fx.admin, "", fx.project, fx.taskTyp, []domain.LayoutItem{
		{Kind: domain.LayoutCustom, FieldID: field.ID, Required: true},
		{Kind: domain.LayoutSystem, Key: "description"},
	})
	if err != nil {
		t.Fatalf("save layout: %v", err)
	}
	if !view.Configured || view.Items[0].Field == nil || view.Items[0].Field.ID != field.ID || !view.Items[0].Item.Required {
		t.Errorf("layout must put the custom field first and required, got %+v", view.Items[0])
	}
	if view.Items[1].Item.Key != "description" {
		t.Errorf("second entry = %+v, want description", view.Items[1].Item)
	}
}

func TestScopeForMergesLayoutRequiredFlag(t *testing.T) {
	fx := newCFFixture(t)
	ctx := context.Background()
	field := fx.newField(t, "Lote", domain.FieldText)
	project := fx.project
	if _, err := fx.service.ReplaceCustomFieldContexts(ctx, fx.admin, fx.org, field.ID, []port.ContextInput{{ProjectID: &project}}); err != nil {
		t.Fatal(err)
	}
	scope, err := fx.service.ScopeFor(ctx, fx.project, fx.taskTyp)
	if err != nil || len(scope) != 1 || scope[0].Required {
		t.Fatalf("scope = %+v, err %v", scope, err)
	}
	if _, err := fx.service.ReplaceProjectLayout(ctx, fx.admin, "", fx.project, fx.taskTyp, []domain.LayoutItem{
		{Kind: domain.LayoutCustom, FieldID: field.ID, Required: true},
	}); err != nil {
		t.Fatal(err)
	}
	scope, _ = fx.service.ScopeFor(ctx, fx.project, fx.taskTyp)
	if len(scope) != 1 || !scope[0].Required {
		t.Errorf("layout required flag must make the field required, got %+v", scope)
	}
}

// issueFixture wires an issue service with the custom field service on the in-memory stores.
func issueCustomFixture(t *testing.T) (*IssueService, *cfFixture, *fakeIssueRepo, uuid.UUID) {
	t.Helper()
	fx := newCFFixture(t)
	repo := newFakeIssueRepo()
	service := NewIssueService(repo, &fakeUserDirectory{}, denyAuthorizer{})
	visible := uuid.New()
	service.WithCustomFields(fx.service, cfBrowse{visible: map[uuid.UUID]bool{visible: true}})
	return service, fx, repo, visible
}

func TestIssueCreateValidatesCustomValuesAndAppliesDefaults(t *testing.T) {
	service, fx, _, _ := issueCustomFixture(t)
	ctx := context.Background()
	severity := fx.newField(t, "Severidad", domain.FieldSelect, domain.OptionInput{Label: "Alta"})
	ref := fx.newField(t, "Referencia", domain.FieldURL)
	project := fx.project
	if _, err := fx.service.ReplaceCustomFieldContexts(ctx, fx.admin, fx.org, severity.ID, []port.ContextInput{
		{ProjectID: &project, Required: true},
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := fx.service.ReplaceCustomFieldContexts(ctx, fx.admin, fx.org, ref.ID, []port.ContextInput{
		{ProjectID: &project, Default: json.RawMessage(`"https://example.com"`)},
	}); err != nil {
		t.Fatal(err)
	}
	reporter := uuid.New()
	base := port.CreateInput{Summary: "Resumen", Description: "Descripcion", Priority: domain.PriorityMedium, ReporterID: reporter, GlobalRole: "member", ProjectID: &project}

	// Missing required value: one problem for the required field.
	_, err := service.Create(ctx, base)
	codeOf(t, err, apperror.KindValidation, "issue.custom_field_invalid")
	if details := apperror.DetailsOf(err); len(details) != 1 || details[0].Field != "custom."+severity.ID.String() {
		t.Errorf("details = %+v, want one entry for the required field", details)
	}

	// Unknown field and invalid option: two problems reported together.
	bad := base
	bad.Custom = map[string]json.RawMessage{
		severity.ID.String(): json.RawMessage(`"nope"`),
		uuid.NewString():     json.RawMessage(`"x"`),
	}
	_, err = service.Create(ctx, bad)
	codeOf(t, err, apperror.KindValidation, "issue.custom_field_invalid")
	if len(apperror.DetailsOf(err)) != 2 {
		t.Errorf("details = %+v, want two problems", apperror.DetailsOf(err))
	}

	good := base
	good.Custom = map[string]json.RawMessage{severity.ID.String(): json.RawMessage(`"` + severity.Options[0].ID + `"`)}
	issue, err := service.Create(ctx, good)
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if issue.Custom[severity.ID.String()] != severity.Options[0].ID {
		t.Errorf("custom = %+v", issue.Custom)
	}
	if issue.Custom[ref.ID.String()] != "https://example.com" {
		t.Errorf("default must be applied, custom = %+v", issue.Custom)
	}
}

func TestIssueUpdateChecksUserVisibility(t *testing.T) {
	service, fx, _, visible := issueCustomFixture(t)
	ctx := context.Background()
	owner := fx.newField(t, "Responsable", domain.FieldUser)
	project := fx.project
	if _, err := fx.service.ReplaceCustomFieldContexts(ctx, fx.admin, fx.org, owner.ID, []port.ContextInput{{ProjectID: &project}}); err != nil {
		t.Fatal(err)
	}
	reporter := uuid.New()
	issue, err := service.Create(ctx, port.CreateInput{Summary: "Resumen", Description: "Descripcion", Priority: domain.PriorityMedium,
		ReporterID: reporter, GlobalRole: "member", ProjectID: &project})
	if err != nil {
		t.Fatal(err)
	}
	update := func(custom map[string]json.RawMessage) error {
		_, err := service.UpdateDetails(ctx, issue.ID, reporter, "member", port.UpdateDetailsInput{
			Summary: "Resumen", Description: "Descripcion", Priority: domain.PriorityMedium, IssueType: domain.IssueTypeTask, Custom: custom,
		})
		return err
	}

	hidden := uuid.New()
	err = update(map[string]json.RawMessage{owner.ID.String(): json.RawMessage(`"` + hidden.String() + `"`)})
	codeOf(t, err, apperror.KindValidation, "issue.custom_field_invalid")

	if err := update(map[string]json.RawMessage{owner.ID.String(): json.RawMessage(`"` + visible.String() + `"`)}); err != nil {
		t.Fatalf("visible user: %v", err)
	}
	// Without a custom object the edit does not touch values.
	if err := update(nil); err != nil {
		t.Errorf("edit without custom must pass: %v", err)
	}
	if err := update(map[string]json.RawMessage{owner.ID.String(): json.RawMessage(`null`)}); err != nil {
		// Clearing an optional field is allowed.
		t.Errorf("clearing optional field: %v", err)
	}
}
