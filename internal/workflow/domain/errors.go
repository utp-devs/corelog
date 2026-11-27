// Package domain holds the pure workflow model: statuses, transitions, rules and the engine
// that decides which transitions an actor may run and what a transition changes.
package domain

import "github.com/jhonsferg/corelog/internal/platform/apperror"

// Stable error codes of the workflow module.
const (
	CodeTransitionNotAllowed = "workflow.transition_not_allowed"
	CodeValidationFailed     = "workflow.validation_failed"
	CodeInvalidWorkflow      = "workflow.invalid"
	CodeMalformed            = "workflow.malformed"
	CodeUnsupportedVersion   = "workflow.unsupported_version"
	CodeUnknownRuleType      = "workflow.unknown_rule_type"
	CodeUnknownTemplate      = "workflow.unknown_template"
)

// Codes of the individual validation issues reported by Workflow.Validate.
const (
	IssueNoInitialStatus      = "workflow.no_initial_status"
	IssueUnknownInitialStatus = "workflow.unknown_initial_status"
	IssueUnreachableStatus    = "workflow.unreachable_status"
	IssueNoDoneStatus         = "workflow.no_done_status"
	IssueDuplicateTransition  = "workflow.duplicate_transition_name"
	IssueUnknownStatus        = "workflow.unknown_status"
	IssueDuplicateStatusName  = "workflow.duplicate_status_name"
	IssueDuplicateID          = "workflow.duplicate_id"
	IssueRequired             = "workflow.required"
	IssueInvalidCategory      = "workflow.invalid_category"
	IssueSelfTransition       = "workflow.self_transition"
	IssueInvalidRule          = "workflow.invalid_rule"
	IssueInvalidField         = "workflow.invalid_field_name"
)

// Typed errors with stable codes and Spanish messages.
var (
	ErrTransitionNotAllowed = apperror.WithCode(apperror.KindForbidden, CodeTransitionNotAllowed, "la transición no está permitida")
	ErrValidationFailed     = apperror.WithCode(apperror.KindValidation, CodeValidationFailed, "la transición no cumple los requisitos")
	ErrMalformed            = apperror.WithCode(apperror.KindValidation, CodeMalformed, "la definición del flujo de trabajo está mal formada")
	ErrUnsupportedVersion   = apperror.WithCode(apperror.KindValidation, CodeUnsupportedVersion, "versión de definición de flujo no soportada")
	ErrUnknownTemplate      = apperror.WithCode(apperror.KindValidation, CodeUnknownTemplate, "plantilla de flujo desconocida")
)

// Issue is one problem found while validating a workflow. Path is JSON-path style, for
// example "transitions[3].to".
type Issue struct {
	Path    string
	Code    string
	Message string
}

// IssuesError converts issues into a single typed error carrying one detail per issue.
// It returns nil when there are no issues.
func IssuesError(issues []Issue) error {
	if len(issues) == 0 {
		return nil
	}
	err := apperror.WithCode(apperror.KindValidation, CodeInvalidWorkflow, "el flujo de trabajo no es válido")
	for _, issue := range issues {
		err.WithDetails(apperror.FieldError{Field: issue.Path, Message: issue.Message})
	}
	return err
}

func notAllowed(message string) error {
	return apperror.WithCode(apperror.KindForbidden, CodeTransitionNotAllowed, message)
}
