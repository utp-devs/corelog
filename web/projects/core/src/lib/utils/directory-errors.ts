import { asApiError } from './api-error';

export const FORBIDDEN_DIRECTORY_MESSAGE =
  'No tienes permiso para ver el directorio de esta organización. Pide acceso a un administrador.';

/** Message for a failed directory load: guests (403) get the explanation, other failures the API message. */
export function describeDirectoryError(error: unknown): string {
  const apiError = asApiError(error);
  return apiError.status === 403 ? FORBIDDEN_DIRECTORY_MESSAGE : apiError.message;
}
