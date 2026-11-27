import { asApiError } from './api-error';

/** Form field names (camelCase) for the snake_case fields the API reports in error details. */
const FIELD_BY_API_NAME = new Map<string, string>([
  ['name', 'name'],
  ['display_name', 'displayName'],
  ['job_title', 'jobTitle'],
  ['department', 'department'],
  ['timezone', 'timezone'],
]);

const FIELD_BY_CODE = new Map<string, { readonly field: string; readonly message: string }>([
  ['user.timezone_invalid', { field: 'timezone', message: 'Zona horaria no válida.' }],
]);

/**
 * Maps a failed profile save to form fields. Uses the details of the API error first and falls back to
 * stable error codes. Returns an empty object when no error belongs to a field.
 */
export function profileFieldErrors(error: unknown): Record<string, string> {
  const apiError = asApiError(error);
  const result: Record<string, string> = {};
  for (const detail of apiError.details) {
    const field = FIELD_BY_API_NAME.get(detail.field);
    if (field !== undefined && !Object.hasOwn(result, field)) {
      result[field] = detail.message;
    }
  }
  const byCode = FIELD_BY_CODE.get(apiError.code);
  if (byCode !== undefined && !Object.hasOwn(result, byCode.field)) {
    result[byCode.field] = byCode.message;
  }
  return result;
}
