import { describe, expect, it } from 'vitest'

import { AppError, fieldErrors, httpStatusOf, isAppError } from './app-error'

describe('AppError', () => {
  it('exposes status, code, message and details', () => {
    const error = new AppError({
      status: 422,
      code: 'validation.failed',
      message: 'invalid request body',
      details: [{ field: 'email', message: 'es obligatorio' }],
    })
    expect(error).toBeInstanceOf(Error)
    expect(error.name).toBe('AppError')
    expect(error.status).toBe(422)
    expect(error.code).toBe('validation.failed')
    expect(error.message).toBe('invalid request body')
    expect(error.details).toEqual([{ field: 'email', message: 'es obligatorio' }])
  })

  it('defaults code to null, details to an empty list and serverMessage to null', () => {
    const error = new AppError({ status: 0, message: 'offline' })
    expect(error.code).toBeNull()
    expect(error.details).toEqual([])
    expect(error.serverMessage).toBeNull()
  })

  it('recognises AppError instances only', () => {
    expect(isAppError(new AppError({ status: 500, message: 'x' }))).toBe(true)
    expect(isAppError(new Error('x'))).toBe(false)
    expect(isAppError({ status: 500 })).toBe(false)
  })
})

describe('fieldErrors', () => {
  it('maps each field to its first message', () => {
    const error = new AppError({
      status: 422,
      message: 'x',
      details: [
        { field: 'email', message: 'primero' },
        { field: 'email', message: 'segundo' },
        { field: 'password', message: 'obligatoria' },
      ],
    })
    expect(fieldErrors(error)).toEqual({ email: 'primero', password: 'obligatoria' })
  })

  it('returns an empty record for other errors', () => {
    expect(fieldErrors(new Error('x'))).toEqual({})
    expect(fieldErrors(undefined)).toEqual({})
  })
})

describe('httpStatusOf', () => {
  it('reads the status of AppError and of legacy response-shaped rejections', () => {
    expect(httpStatusOf(new AppError({ status: 409, message: 'x' }))).toBe(409)
    expect(httpStatusOf({ response: { status: 403 } })).toBe(403)
    expect(httpStatusOf(new Error('x'))).toBeUndefined()
  })
})
