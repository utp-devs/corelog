package domain

import (
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/jhonsferg/corelog/internal/platform/apperror"
)

func assertCatalogErr(t *testing.T, err error, kind apperror.Kind, code string) {
	t.Helper()
	if err == nil {
		t.Fatalf("expected error %s, got nil", code)
	}
	if got := apperror.KindOf(err); got != kind {
		t.Errorf("kind = %s, want %s (err %v)", got, kind, err)
	}
	if got := apperror.CodeOf(err); got != code {
		t.Errorf("code = %s, want %s", got, code)
	}
}

func TestNormalizeCatalogName(t *testing.T) {
	cases := []struct {
		name    string
		kind    CatalogKind
		input   string
		want    string
		errCode string
	}{
		{"trims spaces", CatalogIssueType, "  Tarea  ", "Tarea", ""},
		{"empty is required", CatalogIssueType, "   ", "", "catalog.name_required"},
		{"type name limit", CatalogIssueType, strings.Repeat("a", 61), "", "catalog.name_too_long"},
		{"priority name limit is shorter", CatalogPriority, strings.Repeat("a", 41), "", "catalog.name_too_long"},
		{"accents count as one rune", CatalogIssueType, strings.Repeat("é", 60), strings.Repeat("é", 60), ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := NormalizeCatalogName(tc.input, tc.kind)
			if tc.errCode != "" {
				assertCatalogErr(t, err, apperror.KindValidation, tc.errCode)
				return
			}
			if err != nil {
				t.Fatalf("unexpected error %v", err)
			}
			if got != tc.want {
				t.Errorf("name = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestValidatePalette(t *testing.T) {
	if err := ValidateIcon("bug"); err != nil {
		t.Errorf("bug icon rejected: %v", err)
	}
	assertCatalogErr(t, ValidateIcon("unicorn"), apperror.KindValidation, "catalog.invalid_icon")
	if err := ValidateColor("teal"); err != nil {
		t.Errorf("teal rejected: %v", err)
	}
	assertCatalogErr(t, ValidateColor("#ff0000"), apperror.KindValidation, "catalog.invalid_color")
}

func TestValidateHierarchyLevel(t *testing.T) {
	for _, level := range []int{-1, 0, 1} {
		if err := ValidateHierarchyLevel(level); err != nil {
			t.Errorf("level %d rejected: %v", level, err)
		}
	}
	assertCatalogErr(t, ValidateHierarchyLevel(2), apperror.KindValidation, "catalog.invalid_hierarchy_level")
}

func TestValidateCatalogItemSkipsPaletteForResolutions(t *testing.T) {
	item := CatalogItem{Kind: CatalogResolution, Name: "Hecho"}
	if err := ValidateCatalogItem(item); err != nil {
		t.Errorf("resolution without icon rejected: %v", err)
	}
	item = CatalogItem{Kind: CatalogPriority, Name: "Alta", Icon: "arrow-up", Color: "orange"}
	if err := ValidateCatalogItem(item); err != nil {
		t.Errorf("priority rejected: %v", err)
	}
}

func typeItems() []*CatalogItem {
	return []*CatalogItem{
		{ID: uuid.New(), Kind: CatalogIssueType, HierarchyLevel: HierarchyEpic, Name: "Epica"},
		{ID: uuid.New(), Kind: CatalogIssueType, HierarchyLevel: HierarchyStandard, Name: "Tarea"},
		{ID: uuid.New(), Kind: CatalogIssueType, HierarchyLevel: HierarchyStandard, Name: "Error"},
		{ID: uuid.New(), Kind: CatalogIssueType, HierarchyLevel: HierarchySubtask, Name: "Subtarea"},
	}
}

func TestCheckRemovalKeepsRequiredIssueTypes(t *testing.T) {
	items := typeItems()
	// A standard type can go while another standard type remains.
	if err := CheckRemovalKeepsRequired(items, *items[1]); err != nil {
		t.Errorf("removing one of two standard types rejected: %v", err)
	}
	// The only standard type cannot go.
	onlyStandard := []*CatalogItem{items[0], items[1], items[3]}
	assertCatalogErr(t, CheckRemovalKeepsRequired(onlyStandard, *items[1]), apperror.KindValidation, "catalog.last_required_item")
	// The epic and subtask types are always required.
	assertCatalogErr(t, CheckRemovalKeepsRequired(items, *items[0]), apperror.KindValidation, "catalog.last_required_item")
	assertCatalogErr(t, CheckRemovalKeepsRequired(items, *items[3]), apperror.KindValidation, "catalog.last_required_item")
}

func TestCheckRemovalKeepsRequiredOtherKinds(t *testing.T) {
	one := []*CatalogItem{{ID: uuid.New(), Kind: CatalogPriority, Name: "Media"}}
	assertCatalogErr(t, CheckRemovalKeepsRequired(one, *one[0]), apperror.KindValidation, "catalog.last_required_item")
	two := []*CatalogItem{one[0], {ID: uuid.New(), Kind: CatalogPriority, Name: "Baja"}}
	if err := CheckRemovalKeepsRequired(two, *one[0]); err != nil {
		t.Errorf("removing one of two priorities rejected: %v", err)
	}
}

func TestValidateReorder(t *testing.T) {
	items := typeItems()
	ids := []uuid.UUID{items[3].ID, items[2].ID, items[1].ID, items[0].ID}
	if err := ValidateReorder(items, ids); err != nil {
		t.Errorf("permutation rejected: %v", err)
	}
	assertCatalogErr(t, ValidateReorder(items, ids[:3]), apperror.KindValidation, "catalog.order_mismatch")
	dup := []uuid.UUID{items[0].ID, items[0].ID, items[1].ID, items[2].ID}
	assertCatalogErr(t, ValidateReorder(items, dup), apperror.KindValidation, "catalog.order_mismatch")
	foreign := []uuid.UUID{items[0].ID, items[1].ID, items[2].ID, uuid.New()}
	assertCatalogErr(t, ValidateReorder(items, foreign), apperror.KindValidation, "catalog.order_mismatch")
}

func TestReassignTargetValid(t *testing.T) {
	org := uuid.New()
	standard := &CatalogItem{ID: uuid.New(), OrganizationID: org, Kind: CatalogIssueType, HierarchyLevel: HierarchyStandard}
	otherStandard := &CatalogItem{ID: uuid.New(), OrganizationID: org, Kind: CatalogIssueType, HierarchyLevel: HierarchyStandard}
	subtask := &CatalogItem{ID: uuid.New(), OrganizationID: org, Kind: CatalogIssueType, HierarchyLevel: HierarchySubtask}
	otherOrg := &CatalogItem{ID: uuid.New(), OrganizationID: uuid.New(), Kind: CatalogIssueType, HierarchyLevel: HierarchyStandard}

	if err := ReassignTargetValid(*standard, *otherStandard); err != nil {
		t.Errorf("compatible target rejected: %v", err)
	}
	assertCatalogErr(t, ReassignTargetValid(*standard, *standard), apperror.KindValidation, "catalog.reassign_to_self")
	assertCatalogErr(t, ReassignTargetValid(*standard, *subtask), apperror.KindValidation, "catalog.reassign_target_invalid")
	assertCatalogErr(t, ReassignTargetValid(*standard, *otherOrg), apperror.KindValidation, "catalog.reassign_target_invalid")
}

func TestLegacyMirrorValues(t *testing.T) {
	if got := LegacyPriorityValue(&CatalogItem{LegacyKey: "critical"}); got != PriorityCritical {
		t.Errorf("critical mirror = %s", got)
	}
	if got := LegacyPriorityValue(&CatalogItem{}); got != PriorityMedium {
		t.Errorf("custom priority mirror = %s, want medium", got)
	}
	if got := LegacyIssueTypeValue(&CatalogItem{LegacyKey: "epic"}); got != IssueTypeEpic {
		t.Errorf("epic mirror = %s", got)
	}
	if got := LegacyIssueTypeValue(nil); got != IssueTypeTask {
		t.Errorf("nil type mirror = %s, want task", got)
	}
}

func TestChangeStatusRecordsAndClearsResolution(t *testing.T) {
	issue, err := NewIssue("Resumen", "Descripcion", PriorityMedium, uuid.New())
	if err != nil {
		t.Fatal(err)
	}
	resolution := uuid.New()
	issue.ResolutionID = &resolution

	if err := issue.ChangeStatus(StatusInProgress); err != nil {
		t.Fatal(err)
	}
	if issue.ResolvedAt != nil {
		t.Fatal("in progress issue has resolved_at")
	}
	if err := issue.ChangeStatus(StatusResolved); err != nil {
		t.Fatal(err)
	}
	if issue.ResolvedAt == nil {
		t.Fatal("resolved issue without resolved_at")
	}
	first := *issue.ResolvedAt
	if err := issue.ChangeStatus(StatusClosed); err != nil {
		t.Fatal(err)
	}
	if !issue.ResolvedAt.Equal(first) {
		t.Error("resolved_at moved when the issue went from resolved to closed")
	}

	reopened, err := NewIssue("Otro", "Descripcion", PriorityMedium, uuid.New())
	if err != nil {
		t.Fatal(err)
	}
	if err := reopened.ChangeStatus(StatusInProgress); err != nil {
		t.Fatal(err)
	}
	if err := reopened.ChangeStatus(StatusResolved); err != nil {
		t.Fatal(err)
	}
	reopened.ResolutionID = &resolution
	if err := reopened.ChangeStatus(StatusInProgress); err != nil {
		t.Fatal(err)
	}
	if reopened.ResolvedAt != nil || reopened.ResolutionID != nil {
		t.Error("reopening must clear resolved_at and the resolution")
	}
}
