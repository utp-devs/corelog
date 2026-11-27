package domain

import (
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"

	"github.com/jhonsferg/corelog/internal/platform/apperror"
)

// CatalogKind names one of the configurable catalogs of an organization.
type CatalogKind string

const (
	CatalogIssueType  CatalogKind = "issue_type"
	CatalogPriority   CatalogKind = "priority"
	CatalogResolution CatalogKind = "resolution"
)

// Hierarchy levels of issue types. Exactly one epic-level and one subtask-level type exist per organization.
const (
	HierarchySubtask  = -1
	HierarchyStandard = 0
	HierarchyEpic     = 1
)

const (
	maxCatalogNameLength        = 60
	maxPriorityNameLength       = 40
	maxCatalogDescriptionLength = 500
)

// IconKeys is the palette of icon keys accepted for issue types and priorities. The clients map keys to glyphs.
var IconKeys = []string{
	"task", "bug", "story", "epic", "subtask", "star", "flag", "bolt", "alert", "check",
	"layers", "tag", "book", "rocket", "shield", "clock", "circle", "arrow-up",
	"arrow-down", "minus", "chevrons-up", "chevrons-down",
}

// ColorKeys is the palette of color keys accepted for issue types and priorities.
var ColorKeys = []string{
	"gray", "red", "orange", "amber", "green", "teal", "blue", "indigo", "purple", "pink",
}

// CatalogItem is one entry of a catalog. Icon and Color are empty for resolutions. HierarchyLevel is only
// meaningful for issue types. LegacyKey links the item to the value of the legacy enum columns.
type CatalogItem struct {
	ID             uuid.UUID
	OrganizationID uuid.UUID
	Kind           CatalogKind
	Name           string
	Description    string
	Icon           string
	Color          string
	HierarchyLevel int
	Position       int
	IsDefault      bool
	LegacyKey      string
	Usage          int
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

// Typed errors of the catalogs. Codes are stable; messages are shown to users.
var (
	ErrCatalogNotFound       = apperror.WithCode(apperror.KindNotFound, "catalog.not_found", "el elemento del catalogo no existe")
	ErrCatalogNameRequired   = apperror.WithCode(apperror.KindValidation, "catalog.name_required", "el nombre es obligatorio")
	ErrCatalogNameTooLong    = apperror.WithCode(apperror.KindValidation, "catalog.name_too_long", "el nombre es demasiado largo")
	ErrCatalogDescTooLong    = apperror.WithCode(apperror.KindValidation, "catalog.description_too_long", "la descripcion es demasiado larga")
	ErrCatalogInvalidIcon    = apperror.WithCode(apperror.KindValidation, "catalog.invalid_icon", "el icono no pertenece a la paleta")
	ErrCatalogInvalidColor   = apperror.WithCode(apperror.KindValidation, "catalog.invalid_color", "el color no pertenece a la paleta")
	ErrCatalogInvalidLevel   = apperror.WithCode(apperror.KindValidation, "catalog.invalid_hierarchy_level", "el nivel jerarquico no es valido")
	ErrCatalogNameTaken      = apperror.WithCode(apperror.KindConflict, "catalog.name_taken", "ya existe un elemento con ese nombre")
	ErrCatalogHierarchyTaken = apperror.WithCode(apperror.KindConflict, "catalog.hierarchy_level_taken", "ya existe un tipo de incidencia con ese nivel jerarquico")
	ErrCatalogInUse          = apperror.WithCode(apperror.KindConflict, "catalog.in_use", "el elemento esta en uso; indica un elemento de reemplazo en reassign_to")
	ErrCatalogReassignSelf   = apperror.WithCode(apperror.KindValidation, "catalog.reassign_to_self", "el elemento de reemplazo no puede ser el mismo que se elimina")
	ErrCatalogReassignTarget = apperror.WithCode(apperror.KindValidation, "catalog.reassign_target_invalid", "el elemento de reemplazo no existe o no es compatible")
	ErrCatalogRequiredKind   = apperror.WithCode(apperror.KindValidation, "catalog.last_required_item", "no se puede eliminar el ultimo elemento obligatorio de su tipo")
	ErrCatalogOrderMismatch  = apperror.WithCode(apperror.KindValidation, "catalog.order_mismatch", "la lista de orden debe contener exactamente los elementos del catalogo")
	ErrCatalogTypeNotEnabled = apperror.WithCode(apperror.KindValidation, "issue.type_not_enabled", "el tipo de incidencia no esta habilitado en el proyecto")
	ErrProjectTypesRequired  = apperror.WithCode(apperror.KindValidation, "project.issue_types_required", "el proyecto necesita un tipo estandar, el tipo epica y el tipo subtarea")
	ErrCatalogValueUnknown   = apperror.WithCode(apperror.KindValidation, "issue.catalog_value_unknown", "el tipo o la prioridad indicados no existen en la organizacion")
)

// CatalogRef is the display data of a catalog item as seen from an issue.
type CatalogRef struct {
	ID             uuid.UUID
	Name           string
	Icon           string
	Color          string
	HierarchyLevel int
}

// RefOf returns the display reference of a catalog item.
func RefOf(item *CatalogItem) *CatalogRef {
	if item == nil {
		return nil
	}
	return &CatalogRef{ID: item.ID, Name: item.Name, Icon: item.Icon, Color: item.Color, HierarchyLevel: item.HierarchyLevel}
}

// NormalizeCatalogName trims a name and checks its length in runes.
func NormalizeCatalogName(name string, kind CatalogKind) (string, error) {
	trimmed := strings.TrimSpace(name)
	if trimmed == "" {
		return "", ErrCatalogNameRequired
	}
	limit := maxCatalogNameLength
	if kind == CatalogPriority {
		limit = maxPriorityNameLength
	}
	if utf8.RuneCountInString(trimmed) > limit {
		return "", ErrCatalogNameTooLong
	}
	return trimmed, nil
}

// NormalizeCatalogDescription trims a description and checks its length.
func NormalizeCatalogDescription(description string) (string, error) {
	trimmed := strings.TrimSpace(description)
	if utf8.RuneCountInString(trimmed) > maxCatalogDescriptionLength {
		return "", ErrCatalogDescTooLong
	}
	return trimmed, nil
}

// ValidateIcon accepts only keys of the icon palette.
func ValidateIcon(icon string) error {
	if !containsKey(IconKeys, icon) {
		return ErrCatalogInvalidIcon
	}
	return nil
}

// ValidateColor accepts only keys of the color palette.
func ValidateColor(color string) error {
	if !containsKey(ColorKeys, color) {
		return ErrCatalogInvalidColor
	}
	return nil
}

// ValidateHierarchyLevel accepts the three levels of issue types.
func ValidateHierarchyLevel(level int) error {
	switch level {
	case HierarchySubtask, HierarchyStandard, HierarchyEpic:
		return nil
	default:
		return ErrCatalogInvalidLevel
	}
}

// ValidateCatalogItem checks the fields of an item of the given kind before it is stored.
func ValidateCatalogItem(item CatalogItem) error {
	if _, err := NormalizeCatalogName(item.Name, item.Kind); err != nil {
		return err
	}
	if _, err := NormalizeCatalogDescription(item.Description); err != nil {
		return err
	}
	if item.Kind != CatalogResolution {
		if err := ValidateIcon(item.Icon); err != nil {
			return err
		}
		if err := ValidateColor(item.Color); err != nil {
			return err
		}
	}
	if item.Kind == CatalogIssueType {
		return ValidateHierarchyLevel(item.HierarchyLevel)
	}
	return nil
}

// CheckRemovalKeepsRequired verifies that removing removed from items leaves at least one item of each kind
// that the organization needs. For issue types the epic-level and subtask-level types are mandatory, and a
// standard type must remain. Priorities and resolutions need at least one item.
func CheckRemovalKeepsRequired(items []*CatalogItem, removed CatalogItem) error {
	var remaining []*CatalogItem
	for _, item := range items {
		if item.ID != removed.ID {
			remaining = append(remaining, item)
		}
	}
	switch removed.Kind {
	case CatalogIssueType:
		counts := map[int]int{}
		for _, item := range remaining {
			counts[item.HierarchyLevel]++
		}
		if counts[HierarchyStandard] == 0 || counts[HierarchyEpic] == 0 || counts[HierarchySubtask] == 0 {
			return ErrCatalogRequiredKind
		}
	default:
		if len(remaining) == 0 {
			return ErrCatalogRequiredKind
		}
	}
	return nil
}

// ValidateReorder checks that ids is a permutation of the ids of items.
func ValidateReorder(items []*CatalogItem, ids []uuid.UUID) error {
	if len(ids) != len(items) {
		return ErrCatalogOrderMismatch
	}
	known := make(map[uuid.UUID]bool, len(items))
	for _, item := range items {
		known[item.ID] = true
	}
	seen := make(map[uuid.UUID]bool, len(ids))
	for _, id := range ids {
		if !known[id] || seen[id] {
			return ErrCatalogOrderMismatch
		}
		seen[id] = true
	}
	return nil
}

// ReassignTargetValid checks that issues of removed can move to target: same organization kind, not the same
// item, and for issue types the same hierarchy level.
func ReassignTargetValid(removed, target CatalogItem) error {
	if target.ID == removed.ID {
		return ErrCatalogReassignSelf
	}
	if target.Kind != removed.Kind || target.OrganizationID != removed.OrganizationID {
		return ErrCatalogReassignTarget
	}
	if removed.Kind == CatalogIssueType && target.HierarchyLevel != removed.HierarchyLevel {
		return ErrCatalogReassignTarget
	}
	return nil
}

// LegacyPriorityValue returns the value of the legacy priority enum that mirrors a catalog priority.
// Items without a legacy key mirror the neutral value.
func LegacyPriorityValue(item *CatalogItem) Priority {
	if item == nil || item.LegacyKey == "" {
		return PriorityMedium
	}
	return Priority(item.LegacyKey)
}

// LegacyIssueTypeValue returns the value of the legacy issue type enum that mirrors a catalog type.
func LegacyIssueTypeValue(item *CatalogItem) IssueType {
	if item == nil || item.LegacyKey == "" {
		return IssueTypeTask
	}
	return IssueType(item.LegacyKey)
}

func containsKey(keys []string, key string) bool {
	for _, k := range keys {
		if k == key {
			return true
		}
	}
	return false
}
