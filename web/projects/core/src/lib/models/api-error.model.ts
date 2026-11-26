/** Field-level error inside the CoreLog error body (present for validation.failed). */
export interface ApiErrorDetail {
  readonly field: string;
  readonly message: string;
}

/** Body returned by the CoreLog API on errors: { error: { message, code?, details? } }. */
export interface ErrorResponse {
  readonly error: {
    readonly message: string;
    readonly code?: string;
    readonly details?: readonly ApiErrorDetail[];
  };
}

/**
 * Normalized error used by every API call. `status` is 0 when no HTTP response was received
 * (network failure or timeout). `code` is stable for the client (e.g. `validation.failed`,
 * `network.unreachable`, `http.404`).
 */
export interface ApiError {
  readonly status: number;
  readonly code: string;
  readonly message: string;
  readonly details: readonly ApiErrorDetail[];
}
