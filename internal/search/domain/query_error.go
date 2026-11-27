package domain

import (
	"github.com/jhonsferg/corelog/internal/platform/apperror"
)

// Stable error codes of the issue search.
const (
	CodeSyntaxError          = "search.syntax_error"
	CodeUnknownField         = "search.unknown_field"
	CodeInvalidQuery         = "search.invalid_query"
	CodeQueryTooLong         = "search.query_too_long"
	CodeQueryTooComplex      = "search.query_too_complex"
	CodeQueryTimeout         = "search.query_timeout"
	CodeFieldUnavailable     = "search.field_unavailable"
	CodeOperatorUnavailable  = "search.operator_unavailable"
	CodeScopeRequired        = "search.scope_required"
	CodePageOutOfRange       = "search.page_out_of_range"
	CodeFilterNotFound       = "filter.not_found"
	CodeFilterNameTaken      = "filter.name_taken"
	CodeFilterInvalid        = "filter.invalid"
	CodeFilterForbidden      = "filter.forbidden"
	CodeShareInvalid         = "filter.share_invalid"
	CodeSubscriptionInvalid  = "filter.subscription_invalid"
	CodeSubscriptionNotFound = "filter.subscription_not_found"
)

// Problem is one located problem of a query. Pos and Len are in characters (runes) of the query text, which
// is what an editor counts.
type Problem struct {
	Pos         int
	Len         int
	Code        string
	Message     string
	Suggestions []string
	Expected    []string
}

// QueryError is a rejected query with the located problems. It wraps a typed apperror so that generic error
// handling still maps it to 422.
type QueryError struct {
	Problems []Problem
	base     *apperror.Error
}

// NewQueryError builds the error for a list of problems. The first problem decides the code and message.
func NewQueryError(problems []Problem) *QueryError {
	code := CodeInvalidQuery
	message := "la consulta no es válida"
	if len(problems) > 0 {
		if problems[0].Code != "" {
			code = problems[0].Code
		}
		message = problems[0].Message
	}
	return &QueryError{Problems: problems, base: apperror.WithCode(apperror.KindValidation, code, message)}
}

func (e *QueryError) Error() string { return e.base.Error() }

// Unwrap exposes the typed application error.
func (e *QueryError) Unwrap() error { return e.base }

// Warning is a non-fatal remark about an accepted query.
type Warning struct {
	Code    string
	Message string
}

var (
	ErrQueryTimeout = apperror.WithCode(apperror.KindValidation, CodeQueryTimeout,
		"la consulta tardó demasiado en ejecutarse; acótala con más condiciones")
	ErrCoreQLTooLong = apperror.WithCode(apperror.KindValidation, CodeQueryTooLong,
		"la consulta supera el máximo de 4000 caracteres")
	ErrPageOutOfRange = apperror.WithCode(apperror.KindValidation, CodePageOutOfRange,
		"la página solicitada queda fuera del rango permitido (máximo 10000 resultados navegables)")
)
