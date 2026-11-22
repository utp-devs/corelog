package domain

import "github.com/jhonsferg/corelog/internal/platform/apperror"

var ErrInvalidBoard = apperror.New(apperror.KindValidation, "invalid board")
var ErrBoardNotFound = apperror.New(apperror.KindNotFound, "board not found")

var ErrColumnNotFound = apperror.New(apperror.KindNotFound, "column not found")
var ErrCardNotFound = apperror.New(apperror.KindNotFound, "card not found")
var ErrWIPLimitExceeded = apperror.New(apperror.KindConflict, "wip limit exceeded")
var ErrInvalidWIPLimit = apperror.New(apperror.KindValidation, "invalid wip limit")
