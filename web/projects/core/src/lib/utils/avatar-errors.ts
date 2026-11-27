import { AVATAR_MAX_BYTES, AVATAR_MIME_TYPES } from '../models/people.model';
import { asApiError } from './api-error';

export const AVATAR_TYPE_MESSAGE = 'Formato no válido. Usa una imagen PNG, JPEG o WebP.';
export const AVATAR_SIZE_MESSAGE = 'La imagen supera los 2 MB. Elige una más ligera.';
export const AVATAR_INVALID_MESSAGE = 'La imagen no es válida. Usa un PNG, JPEG o WebP.';

/** Client-side check before uploading. Returns a Spanish message, or null when the file is acceptable. */
export function validateAvatarFile(file: {
  readonly type: string;
  readonly size: number;
}): string | null {
  if (!AVATAR_MIME_TYPES.includes(file.type)) return AVATAR_TYPE_MESSAGE;
  if (file.size > AVATAR_MAX_BYTES) return AVATAR_SIZE_MESSAGE;
  return null;
}

/** Message for the stable avatar error codes returned by the API, or null for any other failure. */
export function avatarErrorMessage(error: unknown): string | null {
  const apiError = asApiError(error);
  if (apiError.code === 'user.avatar_too_large' || apiError.status === 413)
    return AVATAR_SIZE_MESSAGE;
  if (apiError.code === 'user.avatar_invalid') return AVATAR_INVALID_MESSAGE;
  return null;
}
