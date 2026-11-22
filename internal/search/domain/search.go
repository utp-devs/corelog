package domain

import (
	"strings"
	"unicode/utf8"

	"github.com/jhonsferg/corelog/internal/platform/apperror"
)

const (
	minQueryRunes = 2
	maxQueryRunes = 100
	// DefaultLimit y MaxLimit acotan cuantos resultados devuelve cada tipo.
	DefaultLimit = 10
	MaxLimit     = 25
)

var (
	ErrQueryTooShort = apperror.New(apperror.KindValidation, "the search needs at least 2 characters")
	ErrQueryTooLong  = apperror.New(apperror.KindValidation, "the search is at most 100 characters")
)

// Kind es el tipo de resultado: proyecto o incidencia.
type Kind string

const (
	KindProject Kind = "project"
	KindIssue   Kind = "issue"
)

// Hit es un resultado de busqueda con lo necesario para enlazarlo.
type Hit struct {
	Kind     Kind
	ID       string
	Title    string
	Subtitle string
	Link     string
}

// NormalizeQuery recorta la consulta y valida su longitud en caracteres (no en bytes).
func NormalizeQuery(raw string) (string, error) {
	query := strings.TrimSpace(raw)
	length := utf8.RuneCountInString(query)
	if length < minQueryRunes {
		return "", ErrQueryTooShort
	}
	if length > maxQueryRunes {
		return "", ErrQueryTooLong
	}
	return query, nil
}

// ClampLimit devuelve un limite entre 1 y MaxLimit, o el valor por defecto si no se indica.
func ClampLimit(limit int) int {
	if limit <= 0 {
		return DefaultLimit
	}
	if limit > MaxLimit {
		return MaxLimit
	}
	return limit
}
