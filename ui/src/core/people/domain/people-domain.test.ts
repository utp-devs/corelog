import { describe, expect, it } from 'vitest'

import { AppError } from '@/core/shared/app-error'
import { avatarErrorMessage, AVATAR_SIZE_MESSAGE, AVATAR_TYPE_MESSAGE, validateAvatarFile } from './avatar'
import { profileFieldErrors } from './profile-errors'
import { FALLBACK_TIMEZONES, timezoneOptions } from './timezones'

describe('validateAvatarFile', () => {
  it('accepts PNG, JPEG and WebP up to 2 MB', () => {
    expect(validateAvatarFile({ type: 'image/png', size: 10 })).toBeNull()
    expect(validateAvatarFile({ type: 'image/jpeg', size: 10 })).toBeNull()
    expect(validateAvatarFile({ type: 'image/webp', size: 2 * 1024 * 1024 })).toBeNull()
  })

  it('rejects other types', () => {
    expect(validateAvatarFile({ type: 'image/gif', size: 10 })).toBe(AVATAR_TYPE_MESSAGE)
    expect(validateAvatarFile({ type: 'application/pdf', size: 10 })).toBe(AVATAR_TYPE_MESSAGE)
  })

  it('rejects files above 2 MB', () => {
    expect(validateAvatarFile({ type: 'image/png', size: 2 * 1024 * 1024 + 1 })).toBe(AVATAR_SIZE_MESSAGE)
  })
})

describe('avatarErrorMessage', () => {
  it('maps the stable avatar codes to Spanish messages', () => {
    expect(avatarErrorMessage(new AppError({ status: 413, code: 'user.avatar_too_large', message: 'x' }))).toBe(AVATAR_SIZE_MESSAGE)
    expect(avatarErrorMessage(new AppError({ status: 422, code: 'user.avatar_invalid', message: 'x' }))).toBe(
      'La imagen no es válida. Usa un PNG, JPEG o WebP.',
    )
  })

  it('returns null for other failures', () => {
    expect(avatarErrorMessage(new AppError({ status: 500, code: null, message: 'x' }))).toBeNull()
    expect(avatarErrorMessage(new Error('boom'))).toBeNull()
  })
})

describe('profileFieldErrors', () => {
  it('maps API detail fields (snake_case) to form fields (camelCase)', () => {
    const error = new AppError({
      status: 422,
      code: 'validation.failed',
      message: 'x',
      details: [
        { field: 'job_title', message: 'Demasiado largo' },
        { field: 'display_name', message: 'Demasiado largo' },
        { field: 'unknown_field', message: 'ignorado' },
      ],
    })

    expect(profileFieldErrors(error)).toEqual({ jobTitle: 'Demasiado largo', displayName: 'Demasiado largo' })
  })

  it('maps the timezone error code even without details', () => {
    const error = new AppError({ status: 422, code: 'user.timezone_invalid', message: 'x' })
    expect(profileFieldErrors(error)).toEqual({ timezone: 'Zona horaria no válida.' })
  })

  it('returns an empty object for errors that belong to no field', () => {
    expect(profileFieldErrors(new AppError({ status: 500, code: null, message: 'x' }))).toEqual({})
    expect(profileFieldErrors(new Error('x'))).toEqual({})
  })
})

describe('timezoneOptions', () => {
  it('returns a sorted-free unique list that always includes the current zone', () => {
    const zones = timezoneOptions('Mars/Olympus')
    expect(zones[0]).toBe('Mars/Olympus')
    expect(new Set(zones).size).toBe(zones.length)
  })

  it('lists the runtime zones, which include the common fallbacks', () => {
    const zones = timezoneOptions('UTC')
    expect(zones.length).toBeGreaterThanOrEqual(FALLBACK_TIMEZONES.length)
    expect(zones).toContain('America/Bogota')
  })
})
