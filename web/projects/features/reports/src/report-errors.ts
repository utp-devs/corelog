const MESSAGES: Readonly<Record<string, string>> = {
  'report.kind_unknown': 'Este informe no existe.',
  'report.range_invalid':
    'El rango de fechas no es válido: la fecha de inicio no puede ser posterior a la de fin.',
  'report.range_too_large':
    'El rango de fechas no puede superar 366 días. Acótalo e inténtalo de nuevo.',
  'report.param_invalid':
    'Uno de los parámetros del informe no es válido. Revísalos e inténtalo de nuevo.',
  'report.sprint_not_found': 'El sprint elegido ya no existe en este proyecto.',
  'report.version_required': 'Elige una versión para ver este informe.',
  'report.version_not_found': 'La versión elegida ya no existe en este proyecto.',
  'report.filter_invalid': 'El filtro no es válido. Revisa la consulta e inténtalo de nuevo.',
  'report.filter_too_broad':
    'El filtro devuelve demasiadas incidencias. Acótalo e inténtalo de nuevo.',
  'report.too_many_issues':
    'El informe abarca demasiadas incidencias. Acota el rango de fechas o añade un filtro.',
};

export const REPORT_LOAD_FAILED =
  'No se pudo cargar el informe. Comprueba tu conexión e inténtalo de nuevo.';
export const REPORT_EXPORT_FAILED = 'No se pudo exportar el informe a CSV. Inténtalo de nuevo.';

/** Spanish text of a failed report request: the specific `report.*` code first, then a generic message. */
export function reportErrorMessage(
  error: { readonly code?: string } | null | undefined,
  fallback = REPORT_LOAD_FAILED,
): string {
  const code = error?.code;
  return code && MESSAGES[code] ? MESSAGES[code] : fallback;
}

/** Whether the failure is a problem with the parameters (the user can fix it) rather than an outage. */
export function isReportParamError(error: { readonly code?: string } | null | undefined): boolean {
  return !!error?.code && error.code in MESSAGES && error.code !== 'report.kind_unknown';
}
