/** Field-level error inside the CoreLog error body (present for validation.failed). */
export interface ApiErrorDetail {
  readonly field: string;
  readonly message: string;
  /** Stable code of the problem, present on workflow validation details. */
  readonly code?: string;
}

/** Removed workflow status that still holds issues and needs a target status. */
export interface ApiMappingRequirement {
  readonly statusId: string;
  readonly name: string;
  readonly issueCount: number;
}

/** Body returned by the CoreLog API on errors: { error: { message, code?, details? } }. */
export interface ErrorResponse {
  readonly error: {
    readonly message: string;
    readonly code?: string;
    readonly details?: readonly unknown[];
    readonly mapping_required?: readonly unknown[];
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
  /** Statuses that need a target when a workflow save removes them. */
  readonly mappingRequired?: readonly ApiMappingRequirement[];
  /** Seconds the server asks to wait before retrying (Retry-After header of a 429 or a lockout). */
  readonly retryAfterSeconds?: number;
}
