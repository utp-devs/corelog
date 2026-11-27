package port

import "github.com/jhonsferg/corelog/internal/platform/apperror"

// Typed errors of the workflow module. Messages are Spanish because they reach the user.
var (
	ErrWorkflowNotFound  = apperror.WithCode(apperror.KindNotFound, "workflow.not_found", "el proyecto no tiene flujo de trabajo")
	ErrVersionConflict   = apperror.WithCode(apperror.KindConflict, "workflow.version_conflict", "el flujo de trabajo cambió; recárgalo e inténtalo de nuevo")
	ErrStatusNameTaken   = apperror.WithCode(apperror.KindConflict, "workflow.status_name_taken", "ya existe un estado con ese nombre")
	ErrStatusInUse       = apperror.WithCode(apperror.KindConflict, "workflow.status_in_use", "el estado está en uso por incidencias o flujos de trabajo")
	ErrStatusNotFound    = apperror.WithCode(apperror.KindNotFound, "workflow.status_not_found", "el estado no existe")
	ErrStatusNameInvalid = apperror.WithCode(apperror.KindValidation, "workflow.status_name_invalid", "el nombre del estado es obligatorio y admite hasta 60 caracteres")
	ErrStatusCategory    = apperror.WithCode(apperror.KindValidation, "workflow.invalid_category", "la categoría del estado no es válida")
)
