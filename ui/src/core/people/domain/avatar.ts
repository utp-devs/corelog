import { AVATAR_MAX_BYTES, AVATAR_MIME_TYPES } from './member'
import { httpStatusOf, isAppError } from '@/core/shared/app-error'

export const AVATAR_TYPE_MESSAGE = 'Formato no válido. Usa una imagen PNG, JPEG o WebP.'
export const AVATAR_SIZE_MESSAGE = 'La imagen supera los 2 MB. Elige una más ligera.'

/** Client-side check before uploading. Returns a Spanish message, or null when the file is acceptable. */
export function validateAvatarFile(file: { type: string; size: number }): string | null {
  if (!(AVATAR_MIME_TYPES as readonly string[]).includes(file.type)) return AVATAR_TYPE_MESSAGE
  if (file.size > AVATAR_MAX_BYTES) return AVATAR_SIZE_MESSAGE
  return null
}

/** Message for the stable avatar error codes returned by the API, or null for any other failure. */
export function avatarErrorMessage(error: unknown): string | null {
  if (!isAppError(error)) {
    return httpStatusOf(error) === 413 ? AVATAR_SIZE_MESSAGE : null
  }
  if (error.code === 'user.avatar_too_large' || error.status === 413) return AVATAR_SIZE_MESSAGE
  if (error.code === 'user.avatar_invalid') return 'La imagen no es válida. Usa un PNG, JPEG o WebP.'
  return null
}
