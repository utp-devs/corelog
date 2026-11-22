package domain

import "github.com/jhonsferg/corelog/internal/platform/apperror"

var ErrInvalidBoard = apperror.New(apperror.KindValidation, "invalid board")
var ErrBoardNotFound = apperror.New(apperror.KindNotFound, "board not found")
