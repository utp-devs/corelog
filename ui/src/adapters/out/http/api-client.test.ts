import axios, { AxiosError, type AxiosAdapter, type InternalAxiosRequestConfig } from 'axios'
import { afterEach, describe, expect, it } from 'vitest'

import { AppError, isAppError } from '@/core/shared/app-error'
import { apiClient, extractErrorMessage } from './api-client'

const originalAdapter = apiClient.defaults.adapter

function failWith(build: (config: InternalAxiosRequestConfig) => unknown): AxiosAdapter {
  return (config) => Promise.reject(build(config as InternalAxiosRequestConfig))
}

function httpFailure(status: number, data: unknown): AxiosAdapter {
  return failWith((config) => {
    const response = { data, status, statusText: '', headers: {}, config }
    return new AxiosError('Request failed', AxiosError.ERR_BAD_REQUEST, config, null, response)
  })
}

async function captureFailure(adapter: AxiosAdapter): Promise<unknown> {
  apiClient.defaults.adapter = adapter
  try {
    await apiClient.get('/projects')
  } catch (error) {
    return error
  }
  throw new Error('expected the request to fail')
}

afterEach(() => {
  apiClient.defaults.adapter = originalAdapter
})

describe('api client error normalisation', () => {
  it('turns an error body into AppError with code, details and the server message', async () => {
    const error = await captureFailure(
      httpFailure(422, {
        error: {
          message: 'invalid request body',
          code: 'validation.failed',
          details: [{ field: 'email', message: 'debe ser un correo válido' }],
        },
      }),
    )
    expect(isAppError(error)).toBe(true)
    const appError = error as AppError
    expect(appError.status).toBe(422)
    expect(appError.code).toBe('validation.failed')
    expect(appError.message).toBe('invalid request body')
    expect(appError.details).toEqual([{ field: 'email', message: 'debe ser un correo válido' }])
    expect(appError.cause).toBeInstanceOf(AxiosError)
  })

  it('uses a Spanish fallback message when the body has no message', async () => {
    const error = (await captureFailure(httpFailure(403, {}))) as AppError
    expect(error.status).toBe(403)
    expect(error.code).toBeNull()
    expect(error.message).toBe('No tienes permiso para realizar esta acción.')
    expect(error.serverMessage).toBeNull()
  })

  it('maps network errors to status 0 with a Spanish message', async () => {
    const error = (await captureFailure(
      failWith((config) => new AxiosError('Network Error', AxiosError.ERR_NETWORK, config)),
    )) as AppError
    expect(error.status).toBe(0)
    expect(error.message).toBe('No se pudo conectar con el servidor. Revisa tu conexión e inténtalo de nuevo.')
  })

  it('maps timeouts to status 0 with a timeout message', async () => {
    const error = (await captureFailure(
      failWith((config) => new AxiosError('timeout of 1ms exceeded', AxiosError.ECONNABORTED, config)),
    )) as AppError
    expect(error.status).toBe(0)
    expect(error.message).toBe('El servidor tardó demasiado en responder. Inténtalo de nuevo.')
  })

  it('lets cancellations through unchanged', async () => {
    const cancel = new axios.CanceledError('canceled')
    const error = await captureFailure(failWith(() => cancel))
    expect(error).toBe(cancel)
  })

  it('keeps the caller fallback when the server sent no message', async () => {
    const error = await captureFailure(httpFailure(500, {}))
    expect(extractErrorMessage(error, 'No se pudieron cargar los proyectos')).toBe('No se pudieron cargar los proyectos')
  })

  it('prefers the server message in extractErrorMessage', async () => {
    const error = await captureFailure(httpFailure(409, { error: { message: 'Esa clave ya existe' } }))
    expect(extractErrorMessage(error, 'fallback')).toBe('Esa clave ya existe')
  })
})
