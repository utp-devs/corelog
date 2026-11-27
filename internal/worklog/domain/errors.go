package domain

import "github.com/jhonsferg/corelog/internal/platform/apperror"

// Stable error codes of the worklog and estimate endpoints.
const (
	CodeWorklogNotFound  = "worklog.not_found"
	CodeIssueNotFound    = "issue.not_found"
	CodeRangeInvalid     = "worklog.range_invalid"
	CodeGroupByInvalid   = "worklog.group_by_invalid"
	CodeModeInvalid      = "worklog.remaining_mode_invalid"
	CodeDurationRequired = "worklog.duration_required"
)

// Errors returned by the worklog use cases. Messages are shown to people, so they are Spanish.
var (
	ErrWorklogNotFound  = apperror.WithCode(apperror.KindNotFound, CodeWorklogNotFound, "El registro de tiempo no existe")
	ErrIssueNotFound    = apperror.WithCode(apperror.KindNotFound, CodeIssueNotFound, "La incidencia no existe")
	ErrRangeInvalid     = apperror.WithCode(apperror.KindValidation, CodeRangeInvalid, "El rango de fechas no es valido. Usa AAAA-MM-DD y un maximo de 366 dias")
	ErrGroupByInvalid   = apperror.WithCode(apperror.KindValidation, CodeGroupByInvalid, "group_by debe ser user, issue o day")
	ErrDurationRequired = apperror.WithCode(apperror.KindValidation, CodeDurationRequired, "Indica el tiempo como texto (time_spent) o en segundos (time_spent_seconds), no ambos")
)
