package domain

import (
	"fmt"
	"strings"

	"github.com/google/uuid"

	"github.com/jhonsferg/corelog/internal/platform/apperror"
)

// Field names of a move mapping. They are the keys of MoveMapping in the request and of RequiredMapping.
const (
	MappingIssueType = "issue_type"
	MappingPriority  = "priority"
	MappingStatus    = "status"
)

var (
	ErrMoveSameProject    = apperror.WithCode(apperror.KindValidation, "issue.move_same_project", "la incidencia ya pertenece a ese proyecto")
	ErrMoveMappingInvalid = apperror.WithCode(apperror.KindValidation, "issue.move_mapping_invalid", "el valor del mapeo no existe en el proyecto destino")
	ErrMoveMappingField   = apperror.WithCode(apperror.KindValidation, "issue.move_mapping_field", "el campo del mapeo no es valido: usa issue_type, priority o status")
)

// MoveMapping maps the values of the source issue to values of the target project. Missing entries mean the
// value is kept when the target accepts it.
type MoveMapping struct {
	IssueTypes map[uuid.UUID]uuid.UUID
	Priorities map[uuid.UUID]uuid.UUID
	Statuses   map[Status]Status
}

// RequiredMapping is a value of the source issue that the target project does not accept and that the caller
// must map. Options are the values the target accepts.
type RequiredMapping struct {
	Field   string
	From    string
	Options []string
}

// MoveMappingRequiredError is the 422 answer of a move that needs mappings. Every required mapping is one detail:
// the field is field_mapping.<field>.<from> and the message lists the accepted values.
func MoveMappingRequiredError(required []RequiredMapping) error {
	details := make([]apperror.FieldError, 0, len(required))
	for _, item := range required {
		details = append(details, apperror.FieldError{
			Field:   fmt.Sprintf("field_mapping.%s.%s", item.Field, item.From),
			Message: "elija un valor del proyecto destino: " + strings.Join(item.Options, ", "),
		})
	}
	return apperror.WithCode(apperror.KindValidation, "issue.move_mapping_required",
		"el proyecto destino no admite algunos valores de la incidencia; indica el mapeo").WithDetails(details...)
}

// ValidMappingField reports whether a mapping names one of the mappable fields.
func ValidMappingField(field string) bool {
	switch field {
	case MappingIssueType, MappingPriority, MappingStatus:
		return true
	default:
		return false
	}
}

// KeyAliasText renders a key from a project key and an issue number, used in the change history of a move.
func KeyAliasText(projectKey string, number int) string {
	return fmt.Sprintf("%s-%d", projectKey, number)
}
