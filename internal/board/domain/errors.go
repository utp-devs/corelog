package domain

import "github.com/jhonsferg/corelog/internal/platform/apperror"

var ErrInvalidBoard = apperror.New(apperror.KindValidation, "invalid board")
