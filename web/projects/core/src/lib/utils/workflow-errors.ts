import { isApiError } from './api-error';

const MESSAGES: Readonly<Record<string, string>> = {
  'workflow.version_conflict':
    'Alguien guardó cambios en este flujo mientras lo editabas. Recarga para ver la última versión.',
  'workflow.status_in_use':
    'Hay incidencias, historial o flujos que usan este estado. Elige a qué estado pasarlos antes de eliminarlo.',
  'workflow.category_in_use':
    'No se puede cambiar la categoría: dejaría algún flujo sin un estado válido de inicio o de cierre.',
  'workflow.reassign_invalid': 'El estado elegido para recibir las incidencias no es válido.',
  'workflow.status_mapping_required':
    'Hay estados que se eliminan con incidencias dentro. Indica a qué estado pasan.',
  'workflow.invalid': 'El flujo de trabajo no es válido. Revisa los errores indicados.',
  'workflow.transition_not_allowed': 'No puedes ejecutar esta transición en esta incidencia.',
  'workflow.validation_failed': 'Faltan datos para completar esta transición.',
  'issue.status_conflict':
    'Otra persona cambió el estado de la incidencia. Se actualizó la información; inténtalo de nuevo.',
  'issue.transition_unavailable':
    'No hay una transición del flujo que lleve la incidencia a ese estado desde el actual.',
  'issue.resolution_not_found': 'La resolución elegida ya no existe. Elige otra.',
};

/** Spanish message for a workflow failure, keyed by the API code. Falls back to the server text. */
export function workflowErrorMessage(error: unknown, fallback: string): string {
  if (!isApiError(error)) return fallback;
  return MESSAGES[error.code] ?? (error.message || fallback);
}

/** True when the error is the optimistic-locking conflict of a workflow save. */
export function isWorkflowVersionConflict(error: unknown): boolean {
  return isApiError(error) && error.code === 'workflow.version_conflict';
}

/** Message shown after a transition failed, with the situations the user can act on. */
export function transitionErrorMessage(error: unknown): string {
  return workflowErrorMessage(error, 'No se pudo cambiar el estado. Inténtalo de nuevo.');
}
