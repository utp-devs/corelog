package domain

import (
	"regexp"
	"strings"

	"github.com/jhonsferg/corelog/internal/platform/apperror"
)

// ErrInvalidKey is returned when a project key does not match the key rule.
var ErrInvalidKey = apperror.WithCode(apperror.KindValidation, "project.invalid_key", "la clave del proyecto debe tener entre 2 y 10 caracteres, empezar con una letra y usar solo letras mayusculas y numeros")

var keyPattern = regexp.MustCompile(`^[A-Z][A-Z0-9]{1,9}$`)

// NormalizeKey trims the key and converts it to upper case. Keys are stored in this form.
func NormalizeKey(key string) string {
	return strings.ToUpper(strings.TrimSpace(key))
}

// ValidateKey normalizes a project key and checks it against the key rule.
func ValidateKey(key string) (string, error) {
	normalized := NormalizeKey(key)
	if !keyPattern.MatchString(normalized) {
		return "", ErrInvalidKey
	}
	return normalized, nil
}
