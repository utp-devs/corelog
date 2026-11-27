package domain

import (
	"strconv"

	"github.com/google/uuid"

	"github.com/jhonsferg/corelog/internal/platform/apperror"
)

// LayoutKind tells whether a layout entry is a built-in field or a custom field.
type LayoutKind string

const (
	LayoutSystem LayoutKind = "system"
	LayoutCustom LayoutKind = "custom"
)

// SystemFieldKeys lists the built-in fields a layout can place, in their default order.
var SystemFieldKeys = []string{
	"summary", "description", "priority", "assignee", "parent", "due_date", "story_points", "labels",
}

// ErrFieldLayoutInvalid is a request whose layout does not validate. Problems travel in the details.
var ErrFieldLayoutInvalid = apperror.WithCode(apperror.KindValidation, "project.field_layout_invalid", "la disposicion de campos no es valida")

// LayoutItem is one entry of a field layout. Key is set for system entries, FieldID for custom ones.
type LayoutItem struct {
	Kind     LayoutKind
	Key      string
	FieldID  uuid.UUID
	Required bool
}

// ValidateLayout checks a layout sent by a client: known kinds, known system keys, custom fields that exist
// in the organization, and no entry twice. It returns the problems as field errors.
func ValidateLayout(items []LayoutItem, organizationFieldIDs map[uuid.UUID]bool) []apperror.FieldError {
	var problems []apperror.FieldError
	seenKeys := map[string]bool{}
	seenFields := map[uuid.UUID]bool{}
	for index, item := range items {
		switch item.Kind {
		case LayoutSystem:
			if !containsString(SystemFieldKeys, item.Key) {
				problems = append(problems, apperror.FieldError{Field: indexField(index), Message: "el campo del sistema no existe"})
				continue
			}
			if seenKeys[item.Key] {
				problems = append(problems, apperror.FieldError{Field: indexField(index), Message: "el campo aparece dos veces"})
			}
			seenKeys[item.Key] = true
		case LayoutCustom:
			if !organizationFieldIDs[item.FieldID] {
				problems = append(problems, apperror.FieldError{Field: indexField(index), Message: "el campo personalizado no existe en la organizacion"})
				continue
			}
			if seenFields[item.FieldID] {
				problems = append(problems, apperror.FieldError{Field: indexField(index), Message: "el campo aparece dos veces"})
			}
			seenFields[item.FieldID] = true
		default:
			problems = append(problems, apperror.FieldError{Field: indexField(index), Message: "el tipo de entrada debe ser system o custom"})
		}
	}
	return problems
}

// ResolveLayout returns the layout a project shows for an issue type. Stored entries keep their order; custom
// entries whose field is not in scope are dropped. Missing system fields and in-scope custom fields are
// appended in default order, so a new custom field shows up without a layout change. Without a stored layout
// the result is the default order. Required flags come from the stored entries (system) and from the scope
// (custom, where the scope already merged the stored flag).
func ResolveLayout(stored []LayoutItem, scope []ScopedField) []LayoutItem {
	inScope := make(map[uuid.UUID]ScopedField, len(scope))
	for _, entry := range scope {
		inScope[entry.Field.ID] = entry
	}
	storedRequired := map[string]bool{}
	for _, item := range stored {
		if item.Kind == LayoutSystem {
			storedRequired[item.Key] = item.Required
		}
	}
	out := make([]LayoutItem, 0, len(SystemFieldKeys)+len(scope))
	placedKeys := map[string]bool{}
	placedFields := map[uuid.UUID]bool{}
	for _, item := range stored {
		switch item.Kind {
		case LayoutSystem:
			if placedKeys[item.Key] || !containsString(SystemFieldKeys, item.Key) {
				continue
			}
			placedKeys[item.Key] = true
			out = append(out, LayoutItem{Kind: LayoutSystem, Key: item.Key, Required: item.Required})
		case LayoutCustom:
			entry, ok := inScope[item.FieldID]
			if !ok || placedFields[item.FieldID] {
				continue
			}
			placedFields[item.FieldID] = true
			out = append(out, LayoutItem{Kind: LayoutCustom, FieldID: item.FieldID, Required: entry.Required})
		}
	}
	for _, key := range SystemFieldKeys {
		if placedKeys[key] {
			continue
		}
		out = append(out, LayoutItem{Kind: LayoutSystem, Key: key, Required: defaultSystemRequired(key, storedRequired)})
	}
	for _, entry := range scope {
		if placedFields[entry.Field.ID] {
			continue
		}
		out = append(out, LayoutItem{Kind: LayoutCustom, FieldID: entry.Field.ID, Required: entry.Required})
	}
	return out
}

// defaultSystemRequired is the required flag of a system entry that the stored layout does not mention.
// Only the summary is required by default; the other built-in fields follow the stored flags when present.
func defaultSystemRequired(key string, stored map[string]bool) bool {
	if required, ok := stored[key]; ok {
		return required
	}
	return key == "summary"
}

func containsString(list []string, value string) bool {
	for _, item := range list {
		if item == value {
			return true
		}
	}
	return false
}

func indexField(index int) string {
	return "items[" + strconv.Itoa(index) + "]"
}
