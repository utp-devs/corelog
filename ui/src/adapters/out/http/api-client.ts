import axios from 'axios'

import { env } from '@/shared/config/env'
import { AppError, isAppError, type FieldErrorDetail } from '@/core/shared/app-error'
import { tokenStorage } from '../storage/token-storage'

export const apiClient = axios.create({
  baseURL: env.apiBaseUrl,
  headers: { 'Content-Type': 'application/json' },
})

apiClient.interceptors.request.use((config) => {
  const token = tokenStorage.get()
  if (token) {
    config.headers.set('Authorization', `Bearer ${token}`)
  }
  return config
})

export interface ApiErrorBody {
  error: { message: string; code?: string; details?: FieldErrorDetail[] }
}

const NETWORK_MESSAGE = 'No se pudo conectar con el servidor. Revisa tu conexión e inténtalo de nuevo.'
const TIMEOUT_MESSAGE = 'El servidor tardó demasiado en responder. Inténtalo de nuevo.'
const UNEXPECTED_MESSAGE = 'Ocurrió un error inesperado.'

function fallbackMessageFor(status: number): string {
  if (status === 401) return 'Tu sesión ha expirado o no es válida.'
  if (status === 403) return 'No tienes permiso para realizar esta acción.'
  if (status === 404) return 'No se encontró el recurso solicitado.'
  if (status === 409) return 'La operación entra en conflicto con el estado actual.'
  if (status === 422) return 'Revisa los datos e inténtalo de nuevo.'
  if (status === 429) return 'Demasiadas solicitudes. Inténtalo más tarde.'
  if (status >= 500) return 'Error del servidor. Inténtalo de nuevo más tarde.'
  return UNEXPECTED_MESSAGE
}

function readDetails(raw: unknown): FieldErrorDetail[] {
  if (!Array.isArray(raw)) return []
  return raw.filter(
    (d): d is FieldErrorDetail =>
      typeof d === 'object' && d !== null && typeof d.field === 'string' && typeof d.message === 'string',
  )
}

/** Converts any failure of the HTTP layer into an AppError. Cancellations pass through unchanged. */
export function normalizeApiError(error: unknown): unknown {
  if (isAppError(error) || axios.isCancel(error)) return error
  if (!axios.isAxiosError(error)) {
    return new AppError({ status: 0, message: UNEXPECTED_MESSAGE, cause: error })
  }

  if (error.response) {
    const status = error.response.status
    const body = error.response.data as Partial<ApiErrorBody> | undefined
    const serverMessage = typeof body?.error?.message === 'string' ? body.error.message : null
    return new AppError({
      status,
      code: typeof body?.error?.code === 'string' ? body.error.code : null,
      details: readDetails(body?.error?.details),
      serverMessage,
      message: serverMessage ?? fallbackMessageFor(status),
      cause: error,
    })
  }

  const timedOut = error.code === 'ECONNABORTED' || error.code === 'ETIMEDOUT'
  return new AppError({
    status: 0,
    message: timedOut ? TIMEOUT_MESSAGE : NETWORK_MESSAGE,
    cause: error,
  })
}

apiClient.interceptors.response.use(undefined, (error: unknown) => Promise.reject(normalizeApiError(error)))

/**
 * Message for a UI toast or inline error. Uses the server message when present,
 * otherwise the caller's fallback (kept from before AppError existed).
 */
export function extractErrorMessage(error: unknown, fallback = 'Something went wrong'): string {
  if (isAppError(error)) return error.serverMessage ?? fallback
  if (axios.isAxiosError<ApiErrorBody>(error)) {
    return error.response?.data?.error?.message ?? fallback
  }
  return fallback
}
