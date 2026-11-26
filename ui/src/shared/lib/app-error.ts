/** One invalid input field reported by the API (`error.details`). */
export interface FieldErrorDetail {
  field: string
  message: string
}

export interface AppErrorInit {
  status: number
  message: string
  code?: string | null
  details?: FieldErrorDetail[]
  /** Message sent by the server in `error.message`, or null when the body had none. */
  serverMessage?: string | null
  cause?: unknown
}

/**
 * Normalised failure of an API call. `status` is 0 for network errors and
 * timeouts, `code` is the stable machine code from the API (null when absent).
 */
export class AppError extends Error {
  readonly status: number
  readonly code: string | null
  readonly details: FieldErrorDetail[]
  readonly serverMessage: string | null

  constructor(init: AppErrorInit) {
    super(init.message, { cause: init.cause })
    this.name = 'AppError'
    this.status = init.status
    this.code = init.code ?? null
    this.details = init.details ?? []
    this.serverMessage = init.serverMessage ?? null
  }
}

export function isAppError(error: unknown): error is AppError {
  return error instanceof AppError
}

/** Maps each invalid field to its first message, e.g. `{ email: 'es obligatorio' }`. */
export function fieldErrors(error: unknown): Record<string, string> {
  const result: Record<string, string> = {}
  if (!isAppError(error)) return result
  for (const detail of error.details) {
    if (!(detail.field in result)) result[detail.field] = detail.message
  }
  return result
}

/** HTTP status of a failure, for AppError and for plain `{ response: { status } }` rejections. */
export function httpStatusOf(error: unknown): number | undefined {
  if (isAppError(error)) return error.status
  return (error as { response?: { status?: number } } | null)?.response?.status
}
