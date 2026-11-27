package domain

import (
	"encoding/json"
	"fmt"
	"math"
	"net/url"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"

	"github.com/jhonsferg/corelog/internal/platform/apperror"
)

// ErrCustomFieldInvalid is returned when a request carries values that do not validate. The per-field
// problems travel in the error details.
func ErrCustomFieldInvalid(problems []FieldProblem) error {
	err := apperror.WithCode(apperror.KindValidation, "issue.custom_field_invalid", "algunos campos personalizados no son validos")
	details := make([]apperror.FieldError, 0, len(problems))
	for _, problem := range problems {
		details = append(details, apperror.FieldError{Field: "custom." + problem.FieldID, Message: problem.Message})
	}
	return err.WithDetails(details...)
}

// FieldProblem explains why the value of one field was rejected.
type FieldProblem struct {
	FieldID string
	Message string
}

// ScopedField is a field that applies to issues of one project and issue type. Required and DefaultValue come
// from the matching contexts (required if any context requires it, default from the most specific context).
type ScopedField struct {
	Field        *CustomField
	Required     bool
	DefaultValue any
}

// BuildScope resolves the fields that apply to a project and an issue type. A field applies when one of its
// contexts matches: the project matches when the context project is nil or equal, the type likewise. A
// stored layout entry that marks the field required also makes it required.
func BuildScope(fields []*CustomField, layoutRequired map[uuid.UUID]bool, projectID, issueTypeID uuid.UUID) []ScopedField {
	scope := make([]ScopedField, 0, len(fields))
	for _, field := range fields {
		var matched []FieldContext
		for _, ctx := range field.Contexts {
			if ctx.ProjectID != nil && *ctx.ProjectID != projectID {
				continue
			}
			if ctx.IssueTypeID != nil && *ctx.IssueTypeID != issueTypeID {
				continue
			}
			matched = append(matched, ctx)
		}
		if len(matched) == 0 {
			continue
		}
		entry := ScopedField{Field: field, Required: layoutRequired[field.ID]}
		bestScore := -1
		for _, ctx := range matched {
			if ctx.Required {
				entry.Required = true
			}
			score := 0
			if ctx.ProjectID != nil {
				score += 2
			}
			if ctx.IssueTypeID != nil {
				score++
			}
			if score > bestScore {
				bestScore = score
				entry.DefaultValue = ctx.DefaultValue
			}
		}
		scope = append(scope, entry)
	}
	sort.SliceStable(scope, func(i, j int) bool {
		if scope[i].Field.Position != scope[j].Field.Position {
			return scope[i].Field.Position < scope[j].Field.Position
		}
		return scope[i].Field.Name < scope[j].Field.Name
	})
	return scope
}

// CustomWrite describes one write of custom values on an issue.
type CustomWrite struct {
	Scope []ScopedField
	// Current holds the stored values of the issue. Keys outside the scope are kept untouched on update.
	Current map[string]any
	// Input is the raw "custom" object of the request. A JSON null clears the value.
	Input map[string]json.RawMessage
	// Create applies the defaults of the contexts to fields the input does not mention.
	Create bool
	// RequireAll checks every required field; otherwise only required fields named in Input are checked.
	RequireAll bool
}

// CustomResult is the outcome of ApplyCustomValues. Written holds the normalized values of the input (nil for
// a cleared value), so callers can run checks that need the data store (user visibility).
type CustomResult struct {
	Values   map[string]any
	Written  map[string]any
	Problems []FieldProblem
}

// ApplyCustomValues validates the input against the scope and merges it into the stored values.
func ApplyCustomValues(write CustomWrite) CustomResult {
	byID := make(map[string]ScopedField, len(write.Scope))
	for _, entry := range write.Scope {
		byID[entry.Field.ID.String()] = entry
	}
	values := make(map[string]any, len(write.Current))
	for key, value := range write.Current {
		values[key] = value
	}
	result := CustomResult{Written: map[string]any{}}

	inputKeys := make([]string, 0, len(write.Input))
	for key := range write.Input {
		inputKeys = append(inputKeys, key)
	}
	sort.Strings(inputKeys)
	// Keys that failed validation are not checked again for the required rule.
	failed := map[string]bool{}
	for _, key := range inputKeys {
		entry, ok := byID[key]
		if !ok {
			failed[key] = true
			result.Problems = append(result.Problems, FieldProblem{FieldID: key, Message: "el campo no existe o no aplica a este tipo de incidencia en este proyecto"})
			continue
		}
		normalized, err := NormalizeFieldValue(entry.Field, write.Input[key])
		if err != nil {
			failed[key] = true
			result.Problems = append(result.Problems, FieldProblem{FieldID: key, Message: err.Error()})
			continue
		}
		result.Written[key] = normalized
		if normalized == nil {
			delete(values, key)
		} else {
			values[key] = normalized
		}
	}

	if write.Create {
		for _, entry := range write.Scope {
			key := entry.Field.ID.String()
			if _, given := write.Input[key]; given || entry.DefaultValue == nil {
				continue
			}
			values[key] = entry.DefaultValue
		}
	}

	for _, entry := range write.Scope {
		if !entry.Required {
			continue
		}
		key := entry.Field.ID.String()
		if failed[key] {
			continue
		}
		if !write.RequireAll && !write.Create {
			if _, given := write.Input[key]; !given {
				continue
			}
		}
		if isEmptyValue(values[key]) {
			result.Problems = append(result.Problems, FieldProblem{FieldID: key, Message: "el campo es obligatorio"})
		}
	}

	result.Values = values
	return result
}

// PruneCustomValues drops the stored values whose fields are outside the scope (used when the issue type
// changes and the new type does not show them).
func PruneCustomValues(current map[string]any, scope []ScopedField) map[string]any {
	inScope := make(map[string]struct{}, len(scope))
	for _, entry := range scope {
		inScope[entry.Field.ID.String()] = struct{}{}
	}
	out := make(map[string]any, len(current))
	for key, value := range current {
		if _, ok := inScope[key]; ok {
			out[key] = value
		}
	}
	return out
}

func isEmptyValue(value any) bool {
	switch typed := value.(type) {
	case nil:
		return true
	case string:
		return typed == ""
	case []string:
		return len(typed) == 0
	}
	return false
}

// NormalizeFieldValue validates one raw JSON value for a field and returns its normalized form. A JSON null,
// an empty text or an empty list return nil, which clears the value. The error message is Spanish and names
// the rule that failed.
func NormalizeFieldValue(field *CustomField, raw json.RawMessage) (any, error) {
	trimmed := strings.TrimSpace(string(raw))
	if trimmed == "" || trimmed == "null" {
		return nil, nil
	}
	switch field.Type {
	case FieldText:
		text, err := decodeString(raw)
		if err != nil {
			return nil, err
		}
		text = strings.TrimSpace(text)
		if utf8.RuneCountInString(text) > MaxTextValueLength {
			return nil, fmt.Errorf("el texto supera los %d caracteres", MaxTextValueLength)
		}
		return emptyToNil(text), nil
	case FieldParagraph:
		text, err := decodeString(raw)
		if err != nil {
			return nil, err
		}
		text = strings.TrimSpace(text)
		if utf8.RuneCountInString(text) > MaxParagraphValueLength {
			return nil, fmt.Errorf("el parrafo supera los %d caracteres", MaxParagraphValueLength)
		}
		return emptyToNil(text), nil
	case FieldNumber:
		var number float64
		if err := json.Unmarshal(raw, &number); err != nil {
			return nil, fmt.Errorf("el valor debe ser un numero")
		}
		if math.IsNaN(number) || math.IsInf(number, 0) {
			return nil, fmt.Errorf("el numero no es finito")
		}
		if field.NumberMin != nil && number < *field.NumberMin {
			return nil, fmt.Errorf("el numero es menor que el minimo %g", *field.NumberMin)
		}
		if field.NumberMax != nil && number > *field.NumberMax {
			return nil, fmt.Errorf("el numero es mayor que el maximo %g", *field.NumberMax)
		}
		return number, nil
	case FieldDate:
		text, err := decodeString(raw)
		if err != nil {
			return nil, err
		}
		parsed, err := time.Parse("2006-01-02", text)
		if err != nil || parsed.Format("2006-01-02") != text {
			return nil, fmt.Errorf("la fecha debe tener el formato AAAA-MM-DD")
		}
		return text, nil
	case FieldDateTime:
		text, err := decodeString(raw)
		if err != nil {
			return nil, err
		}
		parsed, err := time.Parse(time.RFC3339, text)
		if err != nil {
			return nil, fmt.Errorf("la fecha y hora debe ser RFC 3339")
		}
		return parsed.UTC().Format(time.RFC3339), nil
	case FieldSelect:
		id, err := decodeString(raw)
		if err != nil {
			return nil, err
		}
		if err := requireActiveOption(field, id); err != nil {
			return nil, err
		}
		return id, nil
	case FieldMultiSelect:
		var ids []string
		if err := json.Unmarshal(raw, &ids); err != nil {
			return nil, fmt.Errorf("el valor debe ser una lista de opciones")
		}
		if len(ids) > MaxMultiSelectValues {
			return nil, fmt.Errorf("la lista admite como maximo %d opciones", MaxMultiSelectValues)
		}
		seen := make(map[string]struct{}, len(ids))
		for _, id := range ids {
			if _, dup := seen[id]; dup {
				return nil, fmt.Errorf("la opcion aparece dos veces")
			}
			seen[id] = struct{}{}
			if err := requireActiveOption(field, id); err != nil {
				return nil, err
			}
		}
		if len(ids) == 0 {
			return nil, nil
		}
		return ids, nil
	case FieldCheckbox:
		var flag bool
		if err := json.Unmarshal(raw, &flag); err != nil {
			return nil, fmt.Errorf("el valor debe ser verdadero o falso")
		}
		return flag, nil
	case FieldUser:
		text, err := decodeString(raw)
		if err != nil {
			return nil, err
		}
		id, err := uuid.Parse(text)
		if err != nil {
			return nil, fmt.Errorf("el usuario debe ser un identificador valido")
		}
		return id.String(), nil
	case FieldURL:
		text, err := decodeString(raw)
		if err != nil {
			return nil, err
		}
		text = strings.TrimSpace(text)
		if utf8.RuneCountInString(text) > MaxURLValueLength {
			return nil, fmt.Errorf("la URL supera los %d caracteres", MaxURLValueLength)
		}
		parsed, err := url.Parse(text)
		if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" {
			return nil, fmt.Errorf("la URL debe empezar por http:// o https://")
		}
		return text, nil
	case FieldLabels:
		var labels []string
		if err := json.Unmarshal(raw, &labels); err != nil {
			return nil, fmt.Errorf("el valor debe ser una lista de etiquetas")
		}
		out := make([]string, 0, len(labels))
		seen := make(map[string]struct{}, len(labels))
		for _, label := range labels {
			label = strings.TrimSpace(label)
			length := utf8.RuneCountInString(label)
			if length == 0 || length > MaxLabelValueLength {
				return nil, fmt.Errorf("cada etiqueta debe tener entre 1 y %d caracteres", MaxLabelValueLength)
			}
			if _, dup := seen[label]; dup {
				continue
			}
			seen[label] = struct{}{}
			out = append(out, label)
		}
		if len(out) > MaxLabelValues {
			return nil, fmt.Errorf("se admiten como maximo %d etiquetas", MaxLabelValues)
		}
		if len(out) == 0 {
			return nil, nil
		}
		return out, nil
	}
	return nil, fmt.Errorf("el tipo de campo no es valido")
}

// NormalizeDefault validates a default value of a context with the same rules as a value.
func NormalizeDefault(field *CustomField, raw json.RawMessage) (any, error) {
	value, err := NormalizeFieldValue(field, raw)
	if err != nil {
		return nil, ErrCustomFieldDefault
	}
	return value, nil
}

func decodeString(raw json.RawMessage) (string, error) {
	var text string
	if err := json.Unmarshal(raw, &text); err != nil {
		return "", fmt.Errorf("el valor debe ser texto")
	}
	return text, nil
}

func emptyToNil(text string) any {
	if text == "" {
		return nil
	}
	return text
}

func requireActiveOption(field *CustomField, id string) error {
	for _, option := range field.Options {
		if option.ID != id {
			continue
		}
		if option.Archived {
			return fmt.Errorf("la opcion esta archivada")
		}
		return nil
	}
	return fmt.Errorf("la opcion no existe en el campo")
}
