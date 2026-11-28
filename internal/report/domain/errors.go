package domain

import "github.com/jhonsferg/corelog/internal/platform/apperror"

// Stable error codes of the report module.
var (
	ErrKindUnknown     = apperror.WithCode(apperror.KindNotFound, "report.kind_unknown", "El informe no existe")
	ErrRangeInvalid    = apperror.WithCode(apperror.KindValidation, "report.range_invalid", "El rango de fechas no es válido: usa from y to con formato AAAA-MM-DD y from no posterior a to")
	ErrRangeTooLarge   = apperror.WithCode(apperror.KindValidation, "report.range_too_large", "El rango de fechas no puede superar 366 días")
	ErrParamInvalid    = apperror.WithCode(apperror.KindValidation, "report.param_invalid", "Un parámetro del informe no es válido")
	ErrSprintNotFound  = apperror.WithCode(apperror.KindNotFound, "report.sprint_not_found", "El sprint no existe en el proyecto")
	ErrVersionRequired = apperror.WithCode(apperror.KindValidation, "report.version_required", "Indica la versión con el parámetro version")
	ErrVersionNotFound = apperror.WithCode(apperror.KindNotFound, "report.version_not_found", "La versión no existe en el proyecto")
	ErrFilterInvalid   = apperror.WithCode(apperror.KindValidation, "report.filter_invalid", "El filtro no es válido")
	ErrFilterTooBroad  = apperror.WithCode(apperror.KindValidation, "report.filter_too_broad", "El filtro devuelve demasiadas incidencias; acótalo")
	ErrTooManyRows     = apperror.WithCode(apperror.KindValidation, "report.too_many_issues", "El informe abarca demasiadas incidencias; acota el rango o añade un filtro")
)
