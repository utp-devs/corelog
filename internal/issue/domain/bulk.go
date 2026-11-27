package domain

import (
	"strings"

	"github.com/google/uuid"

	"github.com/jhonsferg/corelog/internal/platform/apperror"
)

// MaxBulkIssues is the largest number of issues one bulk request may change.
const MaxBulkIssues = 200

// BulkAction is the change a bulk request applies to every issue it names.
type BulkAction string

const (
	BulkAssign      BulkAction = "assign"
	BulkSetPriority BulkAction = "set_priority"
	BulkSetLabels   BulkAction = "set_labels"
	BulkSetField    BulkAction = "set_field"
	BulkTransition  BulkAction = "transition"
	BulkMove        BulkAction = "move"
	BulkDelete      BulkAction = "delete"
)

var (
	ErrBulkEmpty         = apperror.WithCode(apperror.KindValidation, "issue.bulk_empty", "la peticion debe indicar al menos una incidencia")
	ErrBulkTooMany       = apperror.WithCode(apperror.KindValidation, "issue.bulk_too_many", "una peticion masiva admite como maximo 200 incidencias")
	ErrBulkActionUnknown = apperror.WithCode(apperror.KindValidation, "issue.bulk_action_unknown", "la accion masiva no existe")
	ErrBulkParams        = apperror.WithCode(apperror.KindValidation, "issue.bulk_params_invalid", "los parametros de la accion masiva no son validos")
	ErrLabelNotInProject = apperror.WithCode(apperror.KindValidation, "issue.label_not_in_project", "la etiqueta no pertenece al proyecto de la incidencia")
)

// ParseBulkAction checks that the action is one of the supported bulk actions.
func ParseBulkAction(raw string) (BulkAction, error) {
	action := BulkAction(raw)
	switch action {
	case BulkAssign, BulkSetPriority, BulkSetLabels, BulkSetField, BulkTransition, BulkMove, BulkDelete:
		return action, nil
	default:
		return "", ErrBulkActionUnknown
	}
}

// ValidateBulkRefs checks the number of issue references of a bulk request. Blank references are invalid.
func ValidateBulkRefs(refs []string) error {
	if len(refs) == 0 {
		return ErrBulkEmpty
	}
	if len(refs) > MaxBulkIssues {
		return ErrBulkTooMany
	}
	for _, ref := range refs {
		if strings.TrimSpace(ref) == "" {
			return ErrBulkParams
		}
	}
	return nil
}

// DedupeRefs removes repeated references and keeps the first occurrence of each.
func DedupeRefs(refs []string) []string {
	seen := make(map[string]bool, len(refs))
	out := make([]string, 0, len(refs))
	for _, ref := range refs {
		key := strings.TrimSpace(ref)
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, key)
	}
	return out
}

// ValidateLabelChange checks that a label change names at least one label and no label twice on the same side.
func ValidateLabelChange(add, remove []uuid.UUID) error {
	if len(add) == 0 && len(remove) == 0 {
		return ErrBulkParams
	}
	for _, id := range add {
		if id == uuid.Nil {
			return ErrBulkParams
		}
	}
	for _, id := range remove {
		if id == uuid.Nil {
			return ErrBulkParams
		}
	}
	return nil
}
