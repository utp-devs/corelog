package domain

import "github.com/jhonsferg/corelog/internal/platform/apperror"

var ErrInvalidBoard = apperror.WithCode(apperror.KindValidation, "board.invalid", "invalid board")
var ErrBoardNotFound = apperror.WithCode(apperror.KindNotFound, "board.not_found", "board not found")

var ErrColumnNotFound = apperror.WithCode(apperror.KindNotFound, "board.column_not_found", "column not found")
var ErrCardNotFound = apperror.WithCode(apperror.KindNotFound, "board.card_not_found", "card not found")
var ErrWIPLimitExceeded = apperror.WithCode(apperror.KindConflict, "board.wip_limit_reached", "wip limit exceeded")
var ErrInvalidWIPLimit = apperror.WithCode(apperror.KindValidation, "board.invalid_wip_limit", "invalid wip limit")
