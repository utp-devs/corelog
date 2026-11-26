import { extractErrorMessage } from '@/adapters/out/http/api-client'
import { httpStatusOf } from '@/core/shared/app-error'

export const FORBIDDEN_DIRECTORY_MESSAGE =
  'No tienes permiso para ver el directorio de esta organización. Pide acceso a un administrador.'

/** Message for a failed directory load: guests (403) get the explanation, other failures the server message. */
export function describeDirectoryError(error: unknown): string {
  if (httpStatusOf(error) === 403) return FORBIDDEN_DIRECTORY_MESSAGE
  return extractErrorMessage(error, 'No se pudo cargar el directorio.')
}
