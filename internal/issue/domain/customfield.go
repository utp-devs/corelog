package domain

import (
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"

	"github.com/jhonsferg/corelog/internal/platform/apperror"
)

// FieldType is the kind of value a custom field holds.
type FieldType string

const (
	FieldText        FieldType = "text"
	FieldParagraph   FieldType = "paragraph"
	FieldNumber      FieldType = "number"
	FieldDate        FieldType = "date"
	FieldDateTime    FieldType = "datetime"
	FieldSelect      FieldType = "select"
	FieldMultiSelect FieldType = "multiselect"
	FieldCheckbox    FieldType = "checkbox"
	FieldUser        FieldType = "user"
	FieldURL         FieldType = "url"
	FieldLabels      FieldType = "labels"
)

// FieldTypes lists every accepted field type.
var FieldTypes = []FieldType{
	FieldText, FieldParagraph, FieldNumber, FieldDate, FieldDateTime, FieldSelect,
	FieldMultiSelect, FieldCheckbox, FieldUser, FieldURL, FieldLabels,
}

// IsValid reports whether t is one of FieldTypes.
func (t FieldType) IsValid() bool {
	for _, known := range FieldTypes {
		if t == known {
			return true
		}
	}
	return false
}

// HasOptions reports whether the type takes a list of options.
func (t FieldType) HasOptions() bool {
	return t == FieldSelect || t == FieldMultiSelect
}

// Limits of custom field definitions and values.
const (
	MaxFieldNameLength        = 60
	MaxFieldDescriptionLength = 500
	MaxFieldOptions           = 100
	MaxOptionLabelLength      = 60
	MaxTextValueLength        = 255
	MaxParagraphValueLength   = 20000
	MaxURLValueLength         = 2048
	MaxMultiSelectValues      = 50
	MaxLabelValueLength       = 40
	MaxLabelValues            = 20
)

// FieldOption is one choice of a select or multiselect field. Archived options keep their id so existing
// values still resolve, but new values cannot use them.
type FieldOption struct {
	ID       string
	Label    string
	Color    string
	Position int
	Archived bool
}

// FieldContext scopes a field to a project and an issue type. A nil project or type means "all".
type FieldContext struct {
	ID          uuid.UUID
	FieldID     uuid.UUID
	ProjectID   *uuid.UUID
	IssueTypeID *uuid.UUID
	Required    bool
	// DefaultValue is the normalized default applied on create, nil when the context has none.
	DefaultValue any
	Position     int
}

// CustomField is an organization-level definition. Contexts are loaded with it.
type CustomField struct {
	ID             uuid.UUID
	OrganizationID uuid.UUID
	Name           string
	Description    string
	Type           FieldType
	Options        []FieldOption
	// NumberMin and NumberMax bound number values when set.
	NumberMin *float64
	NumberMax *float64
	Position  int
	Contexts  []FieldContext
	CreatedAt time.Time
	UpdatedAt time.Time
}

// Typed errors of custom fields. Codes are stable; messages are shown to users.
var (
	ErrCustomFieldNotFound     = apperror.WithCode(apperror.KindNotFound, "custom_field.not_found", "el campo personalizado no existe")
	ErrCustomFieldNameRequired = apperror.WithCode(apperror.KindValidation, "custom_field.name_required", "el nombre del campo es obligatorio")
	ErrCustomFieldNameTooLong  = apperror.WithCode(apperror.KindValidation, "custom_field.name_too_long", "el nombre del campo es demasiado largo")
	ErrCustomFieldDescTooLong  = apperror.WithCode(apperror.KindValidation, "custom_field.description_too_long", "la descripcion del campo es demasiado larga")
	ErrCustomFieldInvalidType  = apperror.WithCode(apperror.KindValidation, "custom_field.invalid_type", "el tipo de campo no es valido")
	ErrCustomFieldNameTaken    = apperror.WithCode(apperror.KindConflict, "custom_field.name_taken", "ya existe un campo con ese nombre en la organizacion")
	ErrCustomFieldOptions      = apperror.WithCode(apperror.KindValidation, "custom_field.invalid_options", "las opciones del campo no son validas")
	ErrCustomFieldSettings     = apperror.WithCode(apperror.KindValidation, "custom_field.invalid_settings", "los limites del campo no son validos")
	ErrCustomFieldContext      = apperror.WithCode(apperror.KindValidation, "custom_field.context_invalid", "el proyecto o el tipo de incidencia no pertenece a la organizacion")
	ErrCustomFieldContextDup   = apperror.WithCode(apperror.KindValidation, "custom_field.context_duplicate", "el mismo proyecto y tipo de incidencia aparece dos veces")
	ErrCustomFieldDefault      = apperror.WithCode(apperror.KindValidation, "custom_field.invalid_default", "el valor por defecto no es valido para el tipo de campo")
)

// NormalizeFieldName trims a field name and checks its length in runes.
func NormalizeFieldName(name string) (string, error) {
	trimmed := strings.TrimSpace(name)
	if trimmed == "" {
		return "", ErrCustomFieldNameRequired
	}
	if utf8.RuneCountInString(trimmed) > MaxFieldNameLength {
		return "", ErrCustomFieldNameTooLong
	}
	return trimmed, nil
}

// NormalizeFieldDescription trims a description and checks its length.
func NormalizeFieldDescription(description string) (string, error) {
	trimmed := strings.TrimSpace(description)
	if utf8.RuneCountInString(trimmed) > MaxFieldDescriptionLength {
		return "", ErrCustomFieldDescTooLong
	}
	return trimmed, nil
}

// ValidateDefinition checks the parts of a definition that do not need other data: the type, the options
// (only select and multiselect take them; ids and labels are distinct; colors come from the palette) and the
// number bounds (min <= max, number fields only).
func ValidateDefinition(f *CustomField) error {
	if !f.Type.IsValid() {
		return ErrCustomFieldInvalidType
	}
	if !f.Type.HasOptions() && len(f.Options) > 0 {
		return ErrCustomFieldOptions
	}
	if len(f.Options) > MaxFieldOptions {
		return ErrCustomFieldOptions
	}
	seenIDs := make(map[string]struct{}, len(f.Options))
	seenLabels := make(map[string]struct{}, len(f.Options))
	for _, option := range f.Options {
		label := strings.TrimSpace(option.Label)
		if option.ID == "" || label == "" || utf8.RuneCountInString(label) > MaxOptionLabelLength {
			return ErrCustomFieldOptions
		}
		if option.Color != "" && ValidateColor(option.Color) != nil {
			return ErrCustomFieldOptions
		}
		if _, dup := seenIDs[option.ID]; dup {
			return ErrCustomFieldOptions
		}
		key := strings.ToLower(label)
		if _, dup := seenLabels[key]; dup {
			return ErrCustomFieldOptions
		}
		seenIDs[option.ID] = struct{}{}
		seenLabels[key] = struct{}{}
	}
	if f.NumberMin != nil || f.NumberMax != nil {
		if f.Type != FieldNumber {
			return ErrCustomFieldSettings
		}
		if f.NumberMin != nil && f.NumberMax != nil && *f.NumberMin > *f.NumberMax {
			return ErrCustomFieldSettings
		}
	}
	return nil
}

// OptionInput is an option sent by a client. An empty ID creates a new option.
type OptionInput struct {
	ID       string
	Label    string
	Color    string
	Archived bool
}

// MergeOptions applies the options list sent by a client to the stored options. Incoming ids must exist in
// the stored list or be empty (a new option whose id comes from newID). Stored options missing from the
// incoming list are returned as removed: the caller archives the ones values still use and drops the rest.
func MergeOptions(existing []FieldOption, incoming []OptionInput, newID func() string) (kept, removed []FieldOption, err error) {
	if len(incoming) > MaxFieldOptions {
		return nil, nil, ErrCustomFieldOptions
	}
	known := make(map[string]struct{}, len(existing))
	for _, option := range existing {
		known[option.ID] = struct{}{}
	}
	used := make(map[string]struct{}, len(incoming))
	kept = make([]FieldOption, 0, len(incoming))
	for index, in := range incoming {
		option := FieldOption{ID: in.ID, Label: strings.TrimSpace(in.Label), Color: in.Color, Archived: in.Archived, Position: index}
		if in.ID == "" {
			option.ID = newID()
		} else if _, ok := known[in.ID]; !ok {
			return nil, nil, ErrCustomFieldOptions
		}
		if _, dup := used[option.ID]; dup {
			return nil, nil, ErrCustomFieldOptions
		}
		used[option.ID] = struct{}{}
		kept = append(kept, option)
	}
	for _, option := range existing {
		if _, ok := used[option.ID]; !ok {
			removed = append(removed, option)
		}
	}
	return kept, removed, nil
}
