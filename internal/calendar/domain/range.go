package domain

import (
	"time"

	"github.com/jhonsferg/corelog/internal/platform/apperror"
)

// MaxRangeDays limita la consulta del calendario para que una peticion no lea todo el historial.
const MaxRangeDays = 366

var (
	ErrInvalidRange = apperror.WithCode(apperror.KindValidation, "calendar.invalid_range", "invalid calendar range: from must not be after to, and the range is at most one year")
)

// ValidateRange comprueba que el rango sea coherente y no pase del maximo.
func ValidateRange(from, to time.Time) error {
	if from.After(to) {
		return ErrInvalidRange
	}
	if to.Sub(from).Hours()/24 > MaxRangeDays {
		return ErrInvalidRange
	}
	return nil
}
