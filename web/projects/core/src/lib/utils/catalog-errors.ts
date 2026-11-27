import { isApiError } from './api-error';

const MESSAGES: Readonly<Record<string, string>> = {
  'catalog.name_taken': 'Ya existe un elemento con ese nombre. Elige otro.',
  'catalog.hierarchy_level_taken':
    'Ya existe un tipo con ese nivel jerárquico: solo puede haber una épica y una subtarea.',
  'catalog.in_use':
    'Hay incidencias que usan este elemento. Elige a qué elemento moverlas antes de eliminarlo.',
  'catalog.reassign_target_invalid':
    'El elemento elegido para recibir las incidencias no es válido.',
  'catalog.reassign_to_self': 'Elige un elemento distinto del que vas a eliminar.',
  'catalog.last_required_item':
    'No se puede eliminar: es el último elemento obligatorio. La épica y la subtarea siempre deben existir, y cada catálogo necesita al menos un elemento.',
  'catalog.order_mismatch':
    'El orden no coincide con el catálogo actual. Recarga la página e inténtalo de nuevo.',
  'project.issue_types_required':
    'El proyecto necesita al menos un tipo estándar, la épica y la subtarea.',
};

/** Spanish message for a catalog failure, keyed by the API code. Falls back to the server text. */
export function catalogErrorMessage(error: unknown, fallback: string): string {
  if (!isApiError(error)) return fallback;
  return MESSAGES[error.code] ?? (error.message || fallback);
}
