package domain

import "github.com/jhonsferg/corelog/internal/platform/apperror"

var ErrInvalidBoard = apperror.WithCode(apperror.KindValidation, "board.invalid", "invalid board")
var ErrBoardNotFound = apperror.WithCode(apperror.KindNotFound, "board.not_found", "tablero no encontrado")

var ErrColumnNotFound = apperror.WithCode(apperror.KindNotFound, "board.column_not_found", "column not found")
var ErrCardNotFound = apperror.WithCode(apperror.KindNotFound, "board.card_not_found", "card not found")
var ErrWIPLimitExceeded = apperror.WithCode(apperror.KindConflict, "board.wip_limit_reached", "wip limit exceeded")
var ErrInvalidWIPLimit = apperror.WithCode(apperror.KindValidation, "board.invalid_wip_limit", "invalid wip limit")

var ErrBoardNameTaken = apperror.WithCode(apperror.KindConflict, "board.name_taken", "ya existe un tablero con ese nombre en el proyecto")
var ErrTooManyBoards = apperror.WithCode(apperror.KindConflict, "board.too_many", "el proyecto alcanzó el máximo de tableros")
var ErrDefaultBoardNotDeletable = apperror.WithCode(apperror.KindConflict, "board.default_not_deletable", "el tablero principal del proyecto no se puede eliminar")
var ErrInvalidQuery = apperror.WithCode(apperror.KindValidation, "board.invalid_query", "la consulta del filtro no es válida")
var ErrInvalidSettings = apperror.WithCode(apperror.KindValidation, "board.invalid_settings", "la configuración del tablero no es válida")
var ErrInvalidColumns = apperror.WithCode(apperror.KindValidation, "board.invalid_columns", "las columnas del tablero no son válidas")
var ErrInvalidMove = apperror.WithCode(apperror.KindValidation, "board.invalid_move", "el movimiento de la tarjeta no es válido")
var ErrRankConflict = apperror.WithCode(apperror.KindConflict, "board.rank_conflict", "no se pudo calcular la posición de la tarjeta, recarga el tablero")

// NewAmbiguousTransition is returned when several transitions lead from the card status into the target
// column. Its details list the candidates: field is the transition id and message its name.
func NewAmbiguousTransition(candidates ...apperror.FieldError) error {
	return apperror.WithCode(apperror.KindConflict, "board.ambiguous_transition", "varias transiciones llevan a esa columna, elige una").WithDetails(candidates...)
}

// NewRequiredFieldMissing is returned when the transition into the target column needs input. The first detail
// has field "transition_id" and the transition id as message; the next ones have field "fields.<name>" (or
// "comment", "resolution", "assignee") and a Spanish message. The client opens the transition dialog.
func NewRequiredFieldMissing(transitionID string, fields ...apperror.FieldError) error {
	details := append([]apperror.FieldError{{Field: "transition_id", Message: transitionID}}, fields...)
	return apperror.WithCode(apperror.KindValidation, "issue.required_field_missing", "la transición necesita datos adicionales").WithDetails(details...)
}
