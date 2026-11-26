package apperror

import "errors"

type Kind string

const (
	KindValidation   Kind = "validation"
	KindNotFound     Kind = "not_found"
	KindConflict     Kind = "conflict"
	KindUnauthorized Kind = "unauthorized"
	KindForbidden    Kind = "forbidden"
	KindRateLimited  Kind = "rate_limited"
	KindInternal     Kind = "internal"
)

// FieldError describes a problem with one input field.
type FieldError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

type Error struct {
	Kind    Kind
	Message string
	// Code is a stable machine-readable identifier (for example "issue_not_found").
	Code    string
	Details []FieldError
	cause   error
}

func (e *Error) Error() string {
	if e.cause != nil {
		return e.Message + ": " + e.cause.Error()
	}
	return e.Message
}

func (e *Error) Unwrap() error { return e.cause }

func New(kind Kind, message string) *Error {
	return &Error{Kind: kind, Message: message}
}

func Wrap(kind Kind, message string, cause error) *Error {
	return &Error{Kind: kind, Message: message, cause: cause}
}

// WithCode creates an error with a stable machine-readable code.
func WithCode(kind Kind, code, message string) *Error {
	return &Error{Kind: kind, Code: code, Message: message}
}

// WithDetails appends field-level details to the error and returns the same error.
func (e *Error) WithDetails(d ...FieldError) *Error {
	e.Details = append(e.Details, d...)
	return e
}

func KindOf(err error) Kind {
	var appErr *Error
	if errors.As(err, &appErr) {
		return appErr.Kind
	}
	return KindInternal
}

// CodeOf returns the machine-readable code of the first *Error in the chain, or "".
func CodeOf(err error) string {
	var appErr *Error
	if errors.As(err, &appErr) {
		return appErr.Code
	}
	return ""
}

// DetailsOf returns the field details of the first *Error in the chain, or nil.
func DetailsOf(err error) []FieldError {
	var appErr *Error
	if errors.As(err, &appErr) {
		return appErr.Details
	}
	return nil
}

// WrapWithCode creates an error with a stable code that keeps cause in the chain.
func WrapWithCode(kind Kind, code, message string, cause error) *Error {
	return &Error{Kind: kind, Code: code, Message: message, cause: cause}
}
