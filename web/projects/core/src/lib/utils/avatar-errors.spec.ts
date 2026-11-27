import { ApiError } from '../models/api-error.model';
import {
  AVATAR_INVALID_MESSAGE,
  AVATAR_SIZE_MESSAGE,
  AVATAR_TYPE_MESSAGE,
  avatarErrorMessage,
  validateAvatarFile,
} from './avatar-errors';

const apiError = (overrides: Partial<ApiError>): ApiError => ({
  status: 500,
  code: 'http.500',
  message: 'x',
  details: [],
  ...overrides,
});

describe('validateAvatarFile', () => {
  it('accepts PNG, JPEG and WebP up to 2 MB', () => {
    expect(validateAvatarFile({ type: 'image/png', size: 10 })).toBeNull();
    expect(validateAvatarFile({ type: 'image/jpeg', size: 10 })).toBeNull();
    expect(validateAvatarFile({ type: 'image/webp', size: 2 * 1024 * 1024 })).toBeNull();
  });

  it('rejects other types', () => {
    expect(validateAvatarFile({ type: 'image/gif', size: 10 })).toBe(AVATAR_TYPE_MESSAGE);
    expect(validateAvatarFile({ type: 'application/pdf', size: 10 })).toBe(AVATAR_TYPE_MESSAGE);
  });

  it('rejects files above 2 MB', () => {
    expect(validateAvatarFile({ type: 'image/png', size: 2 * 1024 * 1024 + 1 })).toBe(
      AVATAR_SIZE_MESSAGE,
    );
  });
});

describe('avatarErrorMessage', () => {
  it('maps the stable avatar codes to Spanish messages', () => {
    expect(avatarErrorMessage(apiError({ status: 413, code: 'user.avatar_too_large' }))).toBe(
      AVATAR_SIZE_MESSAGE,
    );
    expect(avatarErrorMessage(apiError({ status: 422, code: 'user.avatar_invalid' }))).toBe(
      AVATAR_INVALID_MESSAGE,
    );
  });

  it('returns null for other failures', () => {
    expect(avatarErrorMessage(apiError({ status: 500, code: 'http.500' }))).toBeNull();
    expect(avatarErrorMessage(new Error('boom'))).toBeNull();
  });

  it('maps a 413 without a stable code to the size message', () => {
    expect(avatarErrorMessage(apiError({ status: 413, code: 'http.413' }))).toBe(
      AVATAR_SIZE_MESSAGE,
    );
  });
});
