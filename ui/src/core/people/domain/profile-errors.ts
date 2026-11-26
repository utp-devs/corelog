import { fieldErrors, isAppError } from '@/core/shared/app-error'

/** Form field names (camelCase) for the snake_case fields the API reports in error details. */
const FIELD_BY_API_NAME: Record<string, string> = {
  name: 'name',
  display_name: 'displayName',
  job_title: 'jobTitle',
  department: 'department',
  timezone: 'timezone',
}

const FIELD_BY_CODE: Record<string, { field: string; message: string }> = {
  'user.timezone_invalid': { field: 'timezone', message: 'Zona horaria no válida.' },
}

/**
 * Maps a failed profile save to form fields. Uses the `details` of the API error first and falls back
 * to stable error codes. Returns an empty object when no error belongs to a field.
 */
export function profileFieldErrors(error: unknown): Record<string, string> {
  const result: Record<string, string> = {}
  for (const [apiField, message] of Object.entries(fieldErrors(error))) {
    const field = FIELD_BY_API_NAME[apiField]
    if (field) result[field] = message
  }
  if (isAppError(error) && error.code && FIELD_BY_CODE[error.code] && !result[FIELD_BY_CODE[error.code].field]) {
    const { field, message } = FIELD_BY_CODE[error.code]
    result[field] = message
  }
  return result
}
